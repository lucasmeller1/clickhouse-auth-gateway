package middleware

import (
	"context"
	"net/http"

	"github.com/lucasmeller1/excel_api/internal/auth"
	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/handlers"
	"github.com/lucasmeller1/excel_api/internal/redis"
)

func AuthMiddleware(cfg config.AuthConfig, redisClient *redis.RedisClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bearerToken, err := auth.GetBearerToken(r.Header)
			if err != nil {
				handlers.JsonError(w, http.StatusBadRequest, "missing bearer token")
				return
			}

			claims, err := auth.IsValidJWTEntra(bearerToken, cfg, redisClient)
			if err != nil {
				handlers.JsonError(w, http.StatusUnauthorized, err.Error())
				return
			}

			ctx := context.WithValue(r.Context(), auth.ClaimsContextKey, claims)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
