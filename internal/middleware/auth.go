package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/lucasmeller1/excel_api/internal/auth"
	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/handlers"
	"github.com/lucasmeller1/excel_api/internal/redis"
	"github.com/lucasmeller1/excel_api/internal/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const name = "github.com/lucasmeller1/excel_api/internal/middleware"

var (
	tracer = otel.Tracer(name)
)

func AuthPublicMiddleware(cfg config.AuthConfig, redisClient *redis.RedisClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			ctx, span := tracer.Start(ctx, "Middleware.Public.Auth")
			defer span.End()

			bearerToken, err := auth.GetBearerToken(r.Header)
			if err != nil {
				urlEntraID := fmt.Sprintf(`Bearer authorization_uri="https://login.microsoftonline.com/%s/oauth2/v2.0/authorize"`, cfg.TenantID)
				w.Header().Set("WWW-Authenticate", urlEntraID)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			claims, err := auth.ValidateEntraJWT(ctx, bearerToken, cfg, redisClient)

			if err != nil {
				telemetry.RecordSpanError(span, err)

				if errors.Is(err, redis.ErrRedisConnection) {
					handlers.JsonError(w, http.StatusInternalServerError, "internal server error during authentication")
					return
				}

				handlers.JsonError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			if claims.OID == "" {
				handlers.JsonError(w, http.StatusUnauthorized, "missing oid claim")
				return
			}

			span.SetAttributes(attribute.String("oid", claims.OID))

			if labeler, ok := otelhttp.LabelerFromContext(ctx); ok {
				labeler.Add(attribute.String("user.oid", claims.OID))
			}

			ctx = context.WithValue(ctx, auth.ClaimsContextKey, claims)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func AuthPrivateMiddleware(cfg config.PrivateServerConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx, span := tracer.Start(ctx, "Middleware.Private.Auth")
			defer span.End()

			bearerToken, err := auth.GetBearerToken(r.Header)
			if err != nil {
				telemetry.RecordSpanError(span, fmt.Errorf("failed to get bearerToken: %w", err))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if bearerToken != cfg.InvalidateCacheToken {
				telemetry.RecordSpanError(span, fmt.Errorf("invalid bearer token"))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
