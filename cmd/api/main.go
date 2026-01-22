package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanCaamano/inventory-back/internal/storage/s3client"

	"github.com/jonathanCaamano/inventory-back/internal/application/auth"
	"github.com/jonathanCaamano/inventory-back/internal/application/groups"
	"github.com/jonathanCaamano/inventory-back/internal/application/me"
	"github.com/jonathanCaamano/inventory-back/internal/application/products"
	"github.com/jonathanCaamano/inventory-back/internal/application/users"
	"github.com/jonathanCaamano/inventory-back/internal/config"
	dgroup "github.com/jonathanCaamano/inventory-back/internal/domain/group"
	dproduct "github.com/jonathanCaamano/inventory-back/internal/domain/product"
	duser "github.com/jonathanCaamano/inventory-back/internal/domain/user"
	ihttp "github.com/jonathanCaamano/inventory-back/internal/http"
	inull "github.com/jonathanCaamano/inventory-back/internal/infrastructure/nullrepo"
	ipg "github.com/jonathanCaamano/inventory-back/internal/infrastructure/postgres"
)

var (
	userRepo    duser.Repository
	groupRepo   dgroup.Repository
	productRepo dproduct.Repository
	s3c         *s3client.Client
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// --- DEV SIN DEPENDENCIAS ---
	if cfg.DevNoDeps {
		log.Println("running in DEV_NO_DEPS mode (no db, no s3)")

		userRepo = inull.NewUserRepo()
		groupRepo = inull.NewGroupRepo()
		productRepo = inull.NewProductRepo()
		s3c = nil

	} else {
		// --- MODO NORMAL ---
		pool, err := pgxpool.New(ctx, cfg.DBURL)
		if err != nil {
			log.Fatal(err)
		}
		defer pool.Close()

		userRepo = ipg.NewUserRepo(pool)
		groupRepo = ipg.NewGroupRepo(pool)
		productRepo = ipg.NewProductRepo(pool)

		ur := ipg.NewUserRepo(pool)
		if err := ur.EnsureBootstrapAdmin(
			ctx,
			cfg.BootstrapAdminUsername,
			cfg.BootstrapAdminPassword,
		); err != nil {
			log.Fatal(err)
		}

		userRepo = ur

		s3c, err = s3client.New(ctx, s3client.Options{
			Endpoint:       cfg.S3Endpoint,
			Region:         cfg.S3Region,
			Bucket:         cfg.S3Bucket,
			AccessKey:      cfg.S3AccessKey,
			SecretKey:      cfg.S3SecretKey,
			ForcePathStyle: cfg.S3ForcePathStyle,
			PublicBaseURL:  cfg.S3PublicBaseURL,
		})
		if err != nil {
			log.Fatal(err)
		}
	}
	authSvc := auth.New(userRepo, groupRepo, cfg.JWTSecret, cfg.JWTTTL)
	groupSvc := groups.New(groupRepo)
	userSvc := users.New(userRepo)
	productSvc := products.New(productRepo, groupRepo)
	meSvc := me.New(userRepo, groupRepo)

	h := ihttp.NewRouter(cfg, authSvc, productSvc, groupSvc, userSvc, meSvc, s3c)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
