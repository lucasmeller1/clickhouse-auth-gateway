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

	// unauthenticated
	r.Group(func(r chi.Router) {
		r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ok"))
		})
	})

	// authenticated
	r.Group(func(r chi.Router) {
		r.Use(apimw.AuthPublicMiddleware(cfg.Auth, redisClient))

		r.Use(httprate.Limit(
			cfg.Server.MaxRequests,
			cfg.Server.MaxRequestsInterval,
			httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
				return auth.GetUserOID(r.Context())
			}),
			httprateredis.WithRedisLimitCounter(&httprateredis.Config{
				Host:     cfg.Redis.Hostname,
				Port:     uint16(cfg.Redis.Port),
				Password: cfg.Redis.Password,
			}),
		))

		r.Get("/v1/export", ch.ExportCSV)
		r.Get("/v1/tables", ch.GetUserTables)
	})

	return r
}

func GetPrivateRoutes(cfg *config.Config, redis *redis.RedisClient) chi.Router {
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)

	r.Group(func(r chi.Router) {
		r.Use(apimw.AuthPrivateMiddleware(cfg.PrivateServer))

		r.Post("/invalidate", redis.InvalidateCacheEndpoint)
	})

	return r
}
