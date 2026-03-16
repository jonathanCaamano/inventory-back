package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jonathanCaamano/inventory-back/internal/config"
	"github.com/jonathanCaamano/inventory-back/internal/database"
	"github.com/jonathanCaamano/inventory-back/internal/handlers"
	"github.com/jonathanCaamano/inventory-back/internal/middleware"
	"github.com/jonathanCaamano/inventory-back/internal/models"
	"github.com/jonathanCaamano/inventory-back/internal/repository"
	"github.com/jonathanCaamano/inventory-back/internal/services"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		slog.Error("database connection failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := database.Migrate(db); err != nil {
		slog.Error("migration failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// MinIO — graceful degradation
	var minioSvc *services.MinIOService
	if svc, err := services.NewMinIOService(cfg); err != nil {
		slog.Warn("MinIO not available — image upload disabled", slog.String("error", err.Error()))
	} else {
		minioSvc = svc
	}

	// Repositories
	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	productRepo := repository.NewProductRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)

	// Services
	authSvc := services.NewAuthService(userRepo, tokenRepo, cfg.JWTSecret, cfg.JWTAccessTTLHours)

	// Handlers
	authHandler := handlers.NewAuthHandler(authSvc, userRepo)
	userHandler := handlers.NewUserHandler(userRepo, authSvc)
	productHandler := handlers.NewProductHandler(productRepo, categoryRepo, minioSvc)
	categoryHandler := handlers.NewCategoryHandler(categoryRepo)
	statsHandler := handlers.NewStatsHandler(db)

	var minioCheck func() bool
	if minioSvc != nil {
		minioCheck = minioSvc.Ping
	}
	healthHandler := handlers.NewHealthHandler(db, minioCheck)

	// Seed default admin on first run
	seedAdmin(userRepo, authSvc)

	// Schedule periodic refresh token purge
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := tokenRepo.PurgeExpired(); err != nil {
				slog.Error("purge refresh tokens", slog.String("error", err.Error()))
			}
		}
	}()

	// Router
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())

	// Global middlewares
	r.Use(middleware.RequestID())
	r.Use(middleware.RequestLogger(logger))
	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-ID"},
		ExposeHeaders:    []string{"Content-Length", "X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	// Limit request body size (global)
	r.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, cfg.MaxRequestSize)
		c.Next()
	})

	// Health / liveness probes
	r.GET("/health", healthHandler.Health)
	r.GET("/livez", healthHandler.Live)

	// Swagger UI — public, no auth required
	swaggerHandler := handlers.NewSwaggerHandler()
	r.GET("/swagger", swaggerHandler.UI)
	r.GET("/swagger/", swaggerHandler.UI)
	r.GET("/swagger/doc.json", swaggerHandler.Spec)

	api := r.Group("/api/v1")

	// Public routes
	api.POST("/auth/login",
		middleware.LoginRateLimiter(10, 15*time.Minute),
		authHandler.Login,
	)
	api.POST("/auth/refresh", authHandler.Refresh)

	// Authenticated routes
	auth := api.Group("")
	auth.Use(middleware.AuthRequired(authSvc))
	{
		auth.GET("/auth/me", authHandler.Me)
		auth.POST("/auth/logout", authHandler.Logout)
		auth.POST("/auth/logout-all", authHandler.LogoutAll)

		// Stats (all authenticated)
		auth.GET("/stats", statsHandler.GetStats)

		// Products — viewer+
		auth.GET("/products", productHandler.List)
		auth.GET("/products/:id", productHandler.Get)

		// Products — manager+
		manage := auth.Group("")
		manage.Use(middleware.RequireRole(models.RoleAdmin, models.RoleManager))
		{
			manage.POST("/products", productHandler.Create)
			manage.PUT("/products/:id", productHandler.Update)
			manage.POST("/products/:id/image", productHandler.UploadImage)
			manage.PATCH("/products/:id/stock", productHandler.AdjustStock)
		}

		// Products — admin only
		adminOnly := auth.Group("")
		adminOnly.Use(middleware.RequireRole(models.RoleAdmin))
		{
			adminOnly.DELETE("/products/:id", productHandler.Delete)
		}

		// Categories — viewer+
		auth.GET("/categories", categoryHandler.List)
		auth.GET("/categories/:id", categoryHandler.Get)

		// Categories — manager+
		manage.POST("/categories", categoryHandler.Create)
		manage.PUT("/categories/:id", categoryHandler.Update)

		// Categories — admin only
		adminOnly.DELETE("/categories/:id", categoryHandler.Delete)

		// Users — admin only
		adminOnly.GET("/users", userHandler.List)
		adminOnly.GET("/users/:id", userHandler.Get)
		adminOnly.POST("/users", userHandler.Create)
		adminOnly.PUT("/users/:id", userHandler.Update)
		adminOnly.DELETE("/users/:id", userHandler.Delete)
	}

	// HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info("server starting", slog.String("port", cfg.Port), slog.String("env", cfg.Env))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Graceful shutdown on SIGINT / SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", slog.String("error", err.Error()))
	}
	slog.Info("server stopped")
}

func seedAdmin(userRepo *repository.UserRepository, authSvc *services.AuthService) {
	users, err := userRepo.FindAll()
	if err != nil || len(users) > 0 {
		return
	}

	// Generate a random password for first boot
	hash, err := authSvc.HashPassword("Admin1234!")
	if err != nil {
		slog.Error("seed admin hash error", slog.String("error", err.Error()))
		return
	}

	admin := &models.User{
		Username:     "admin",
		Email:        "admin@inventory.local",
		PasswordHash: hash,
		Role:         models.RoleAdmin,
		Active:       true,
	}
	if err := userRepo.Create(admin); err != nil {
		slog.Error("seed admin create error", slog.String("error", err.Error()))
		return
	}
	slog.Warn("admin user seeded — change password immediately",
		slog.String("username", "admin"),
		slog.String("password", "Admin1234!"),
	)
}
