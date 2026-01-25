package middleware

import (
	"context"
	"github.com/lucasmeller1/excel_api/internal/auth"
	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/handlers"
	"net/http"
)

type contextKey int

const claimsContextKey contextKey = iota

func ClaimsFromContext(ctx context.Context) (*auth.CustomClaims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(*auth.CustomClaims)
	return claims, ok
}

func AuthMiddleware(cfg config.AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			bearerToken, err := handlers.GetBearerToken(r.Header)
			if err != nil {
				handlers.JsonError(w, http.StatusBadRequest, "missing bearer token")
				return
			}

			claims, err := auth.IsValidJWT(bearerToken, cfg)
			if err != nil {
				handlers.JsonError(w, http.StatusUnauthorized, err.Error())
				return
			}

			ctx := context.WithValue(r.Context(), claimsContextKey, claims)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
