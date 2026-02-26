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
	"github.com/lucasmeller1/excel_api/internal/handlers"
	apimw "github.com/lucasmeller1/excel_api/internal/middleware"
	"github.com/lucasmeller1/excel_api/internal/redis"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func getPublicRoutes(cfg *config.Config, ch *clickhouse.HTTPClickhouseClient, redisClient *redis.RedisClient) chi.Router {
	r := chi.NewRouter()

	redisCounter, err := redis.NewRateLimiter(cfg.Redis)
	if err != nil {
		log.Fatalf("failed to create Redis rate limiter: %v", err)
	}

	exportEDP := fmt.Sprintf("/v%s/%s", cfg.Endpoints.Version, cfg.Endpoints.Export)
	tablesEDP := fmt.Sprintf("/v%s/%s", cfg.Endpoints.Version, cfg.Endpoints.Tables)
	healthEDP := "/healthz"

	r.Use(OTelChiRouteMiddleware)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)

	// unauthenticated
	r.Group(func(r chi.Router) {
		r.Get(healthEDP, func(w http.ResponseWriter, r *http.Request) {
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
	r.Use(OTelChiRouteMiddleware)
	r.Use(chimw.Recoverer)

	cacheHandler := handlers.NewCacheHandler(redis)

	r.Group(func(r chi.Router) {
		r.Use(apimw.AuthPrivateMiddleware(cfg.PrivateServer))

		r.Post("/deleteCache", cacheHandler.DeleteCacheEndpoint)
	})

	return r
}

func customRateLimiter(counter httprate.LimitCounter, max int, interval time.Duration, endpoint string) func(http.Handler) http.Handler {
	return httprate.Limit(
		max,
		interval,
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			userOID, _ := auth.GetUserOID(r.Context())
			return endpoint + ":" + userOID, nil
		}),
		httprate.WithLimitCounter(counter),
	)
}

func OTelChiRouteMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)

		rctx := chi.RouteContext(r.Context())
		if rctx != nil {
			routePattern := rctx.RoutePattern()

			if routePattern != "" {
				if labeler, ok := otelhttp.LabelerFromContext(r.Context()); ok {
					labeler.Add(attribute.String("http.route", routePattern))
				}

				span := trace.SpanFromContext(r.Context())
				span.SetName(r.Method + " " + routePattern)
				span.SetAttributes(attribute.String("http.route", routePattern))
			}
		}
	})
}
