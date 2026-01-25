package middleware

import (
	"github.com/lucasmeller1/excel_api/internal/auth"
	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/handlers"
	"net/http"
)

/*
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearerToken, err := handlers.GetBearerToken(r.Header)
		if err != nil {
			handlers.JsonError(w, http.StatusBadRequest, "missing bearer")
			return
		}

		isValid, ok := auth.IsValidJWT(bearerToken)
		if err != nil {
			handlers.JsonError(w, http.StatusBadRequest, "invalid token")
			return
		}

		if !isValid {
			handlers.JsonError(w, http.StatusUnauthorized, "invalid token")
			return
		}

	})
}
*/

func AuthMiddleware(cfg config.AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			bearerToken, err := handlers.GetBearerToken(r.Header)
			if err != nil {
				handlers.JsonError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}

			if err := auth.IsValidJWT(bearerToken, cfg); err != nil {
				handlers.JsonError(w, http.StatusUnauthorized, err.Error())
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
