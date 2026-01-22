package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jonathanCaamano/inventory-back/internal/http/openapi"
	"github.com/jonathanCaamano/inventory-back/internal/http/swaggerui"
	"github.com/jonathanCaamano/inventory-back/internal/storage/s3client"

	"github.com/jonathanCaamano/inventory-back/internal/application/auth"
	"github.com/jonathanCaamano/inventory-back/internal/application/groups"
	"github.com/jonathanCaamano/inventory-back/internal/application/me"
	"github.com/jonathanCaamano/inventory-back/internal/application/products"
	"github.com/jonathanCaamano/inventory-back/internal/application/users"
	"github.com/jonathanCaamano/inventory-back/internal/config"
	"github.com/jonathanCaamano/inventory-back/internal/http/handlers"
	"github.com/jonathanCaamano/inventory-back/internal/http/middleware"
)

func NewRouter(cfg config.Config, authSvc *auth.Service, productSvc *products.Service, groupSvc *groups.Service, userSvc *users.Service, meSvc *me.Service, s3c *s3client.Client) http.Handler {

	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-Id"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(middleware.RequestID)
	r.Use(middleware.Recover)

	health := handlers.NewHealth()
	auth := handlers.NewAuth(authSvc)
	users := handlers.NewUsers(userSvc)
	meh := handlers.NewMe(meSvc)
	groups := handlers.NewGroups(groupSvc)
	products := handlers.NewProducts(cfg, productSvc, s3c)

	r.Get("/health", health.Health)
	r.Get("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(200)
		_, _ = w.Write(openapi.Spec)
	})
	r.Get("/swagger", swaggerui.Handler)
	r.Get("/swagger/", swaggerui.Handler)
	r.Route("/api/v1", func(api chi.Router) {
		api.Post("/auth/login", auth.Login)
		api.Post("/auth/register", auth.Register)
		api.Get("/auth/groups", groups.PublicList)

		api.Group(func(pr chi.Router) {
			pr.Use(middleware.JWT(cfg.JWTSecret))
			pr.Get("/me", meh.WhoAmI)
			pr.Get("/groups", groups.ListForMe)
			pr.Route("/products", func(rp chi.Router) {
				rp.Get("/", products.Search)
				rp.Post("/", products.Create)
				rp.Get("/{id}", products.GetByID)
				rp.Put("/{id}", products.Update)
				rp.Delete("/{id}", products.Delete)
				rp.Post("/{id}/images/presign", products.PresignImage)
				rp.Post("/{id}/images", products.AddImage)
				rp.Put("/{id}/contact", products.UpsertContact)
			})

			pr.Route("/admin", func(ad chi.Router) {
				ad.Use(middleware.RequireAdmin)
				ad.Post("/users", users.AdminCreate)
				ad.Get("/users", users.AdminList)
				ad.Post("/groups", groups.AdminCreate)
				ad.Get("/groups/{id}/members", groups.AdminListMembers)
				ad.Post("/groups/{id}/members", groups.AdminAddMember)
				ad.Delete("/groups/{id}/members", groups.AdminRemoveMember)
			})
		})
	})

	return r
}
