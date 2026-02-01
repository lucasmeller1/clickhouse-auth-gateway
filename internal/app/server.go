package app

import (
	"context"
	"github.com/lucasmeller1/excel_api/internal/clickhouse"
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

func NewServer(cfg *config.Config, ch *clickhouse.HTTPCSVClient) *customServer {
	r := getRoutes(cfg, ch)

	server := &customServer{
		Server: &http.Server{
			Addr:              cfg.HTTP.Addr,
			Handler:           r,
			ReadTimeout:       cfg.HTTP.ReadTimeout,
			ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
			WriteTimeout:      cfg.HTTP.WriteTimeout,
			IdleTimeout:       cfg.HTTP.IdleTimeout,
			MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
		},
		ShutdownTimeout: cfg.HTTP.ShutdownTimeout,
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
