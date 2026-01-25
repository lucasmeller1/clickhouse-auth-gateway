package app

import (
	"context"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/lucasmeller1/excel_api/internal/config"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type customServer struct {
	Server          *http.Server
	ShutdownTimeout time.Duration
}

func NewServer(cfg config.HTTPConfig) *customServer {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	server := &customServer{
		Server: &http.Server{
			Addr:              cfg.Addr,
			Handler:           r,
			ReadTimeout:       cfg.ReadTimeout,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
			MaxHeaderBytes:    cfg.MaxHeaderBytes,
		},
		ShutdownTimeout: cfg.ShutdownTimeout,
	}

	return server
}

func (svr *customServer) Run() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	defer stop()

	go func(addrPort string) {
		log.Printf("server started on port %s\n", addrPort)

		if err := svr.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}(svr.Server.Addr)

	<-ctx.Done()
	log.Println("shutdown initiated")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), svr.ShutdownTimeout)
	defer cancel()

	if err := svr.Server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown failed: %v", err)
	}

	log.Println("shutdown completed")
}
