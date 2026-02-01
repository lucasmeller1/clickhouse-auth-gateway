package app

import (
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/lucasmeller1/excel_api/internal/clickhouse"
	"github.com/lucasmeller1/excel_api/internal/config"
	apimw "github.com/lucasmeller1/excel_api/internal/middleware"
	"net/http"
)

func getRoutes(cfg *config.Config, ch *clickhouse.HTTPCSVClient) chi.Router {
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)

	// unauthenticated
	r.Group(func(r chi.Router) {
		r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		})
	})

	// authenticated
	r.Group(func(r chi.Router) {
		r.Use(apimw.AuthMiddleware(cfg.Auth))

		r.Get("/tables", ch.ExportCSV)
	})

	return r
}
