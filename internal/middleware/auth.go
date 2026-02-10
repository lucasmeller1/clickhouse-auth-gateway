package middleware

import (
	"context"
	"fmt"
	"log"
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
				urlEntraID := fmt.Sprintf(`Bearer authorization_uri="https://login.microsoftonline.com/%s/oauth2/v2.0/authorize"`, cfg.TenantID)
				w.Header().Set("WWW-Authenticate", urlEntraID)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			claims, err := auth.ValidateEntraJWT(r.Context(), bearerToken, cfg, redisClient)
			if err != nil {
				handlers.JsonError(w, http.StatusUnauthorized, err.Error())
				return
			}

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
