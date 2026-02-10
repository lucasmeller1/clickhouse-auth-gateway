package app

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	httprateredis "github.com/go-chi/httprate-redis"
	"github.com/lucasmeller1/excel_api/internal/auth"
	"github.com/lucasmeller1/excel_api/internal/clickhouse"
	"github.com/lucasmeller1/excel_api/internal/config"
	apimw "github.com/lucasmeller1/excel_api/internal/middleware"
	"github.com/lucasmeller1/excel_api/internal/redis"
)

func getPublicRoutes(cfg *config.Config, ch *clickhouse.HTTPClickhouseClient, redisClient *redis.RedisClient) chi.Router {
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)

	// r.Use(httprate.Limit(
	// 	cfg.Server.MaxRequests,
	// 	cfg.Server.MaxRequestsInterval,
	// 	httprate.WithKeyByIP(),
	// 	httprateredis.WithRedisLimitCounter(&httprateredis.Config{
	// 		Host: cfg.Redis.Hostname, Port: uint16(cfg.Redis.Port),
	// 	}),
	// ))

	// unauthenticated
	r.Group(func(r chi.Router) {
		r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ok"))
		})
	})

	// authenticated
	r.Group(func(r chi.Router) {
		r.Use(apimw.AuthMiddleware(cfg.Auth, redisClient))

		r.Use(httprate.Limit(
			cfg.Server.MaxRequests,
			cfg.Server.MaxRequestsInterval,
			httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
				oid, err := auth.GetUserOID(r.Context())
				if err != nil {
					return httprate.KeyByIP(r)
				}
				return oid, nil
			}),
			httprateredis.WithRedisLimitCounter(&httprateredis.Config{
				Host: cfg.Redis.Hostname,
				Port: uint16(cfg.Redis.Port),
			}),
		))

		r.Get("/export", ch.ExportCSV)
		r.Get("/tables", ch.GetUserTables)
	})

	return r
}

func GetPrivateRoutes(cfg *config.Config, redis *redis.RedisClient) chi.Router {
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)

	r.Group(func(r chi.Router) {
		r.Use(apimw.PrivateMiddleware(cfg.PrivateServer))

		r.Post("/invalidate", redis.InvalidateCacheEndpoint)
	})

	return r
}
