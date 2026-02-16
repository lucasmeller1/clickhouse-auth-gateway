package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/lucasmeller1/excel_api/internal/auth"
	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/handlers"
	"github.com/lucasmeller1/excel_api/internal/redis"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const name = "github.com/lucasmeller1/excel_api/internal/middleware"

var (
	meter              = otel.Meter(name)
	authDuration       metric.Float64Histogram
	numberUserRequests metric.Int64Counter
)

func init() {
	var err error

	authDuration, err = meter.Float64Histogram(
		"auth.validation.duration",
		metric.WithDescription("Time spent validating JWT tokens."),
		metric.WithUnit("s"),
	)
	if err != nil {
		fmt.Printf("failed to create metric: %v\n", err)
	}

	numberUserRequests, err = meter.Int64Counter(
		"number.user.requests",
		metric.WithDescription("Number of requests per user."),
	)
	if err != nil {
		fmt.Printf("failed to create metric: %v\n", err)
	}
}

func AuthMiddleware(cfg config.AuthConfig, redisClient *redis.RedisClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			bearerToken, err := auth.GetBearerToken(r.Header)
			if err != nil {
				urlEntraID := fmt.Sprintf(`Bearer authorization_uri="https://login.microsoftonline.com/%s/oauth2/v2.0/authorize"`, cfg.TenantID)
				w.Header().Set("WWW-Authenticate", urlEntraID)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			claims, err := auth.ValidateEntraJWT(r.Context(), bearerToken, cfg, redisClient)

			authDuration.Record(r.Context(), time.Since(start).Seconds())

			if err != nil {
				handlers.JsonError(w, http.StatusUnauthorized, err.Error())
				return
			}

			numberUserRequests.Add(
				r.Context(),
				1,
				metric.WithAttributes(
					attribute.String("oid", claims.OID),
				),
			)

			if cfg.Debug == "1" {
				log.Println(bearerToken)
			}

			ctx := context.WithValue(r.Context(), auth.ClaimsContextKey, claims)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func PrivateMiddleware(cfg config.PrivateServerConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bearerToken, err := auth.GetBearerToken(r.Header)
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if bearerToken != cfg.InvalidateCacheToken {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
