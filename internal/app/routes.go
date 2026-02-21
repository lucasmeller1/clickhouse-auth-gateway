package app

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/lucasmeller1/excel_api/internal/auth"
	"github.com/lucasmeller1/excel_api/internal/clickhouse"
	"github.com/lucasmeller1/excel_api/internal/config"
	apimw "github.com/lucasmeller1/excel_api/internal/middleware"
	"github.com/lucasmeller1/excel_api/internal/redis"
)

func getPublicRoutes(cfg *config.Config, ch *clickhouse.HTTPClickhouseClient, redisClient *redis.RedisClient) chi.Router {
	r := chi.NewRouter()

	redisCounter, err := redis.NewRateLimiter(cfg.Redis)
	if err != nil {
		log.Fatalf("failed to create Redis rate limiter: %v", err)
	}

	exportEDP := fmt.Sprintf("/v%s/%s", cfg.Endpoints.Version, cfg.Endpoints.Export)
	tablesEDP := fmt.Sprintf("/v%s/%s", cfg.Endpoints.Version, cfg.Endpoints.Tables)

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

		r.Group(func(r chi.Router) {
			r.Use(customRateLimiter(redisCounter, cfg.Server.MaxRequestsExportEDP, cfg.Server.MaxRequestsIntervalExportEDP, cfg.Endpoints.Export))
			r.Get(exportEDP, ch.ExportCSV)
		})

		r.Group(func(r chi.Router) {
			r.Use(customRateLimiter(redisCounter, cfg.Server.MaxRequestsTablesEDP, cfg.Server.MaxRequestsIntervalTablesEDP, cfg.Endpoints.Tables))
			r.Get(tablesEDP, ch.GetUserTables)
		})
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

func customRateLimiter(counter httprate.LimitCounter, max int, interval time.Duration, endpoint string) func(http.Handler) http.Handler {
	return httprate.Limit(
		max,
		interval,
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			return endpoint + ":" + auth.GetUserOID(r.Context()), nil
		}),
		httprate.WithLimitCounter(counter),
	)
}
