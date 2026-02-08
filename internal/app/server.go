package app

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lucasmeller1/excel_api/internal/clickhouse"
	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/redis"
)

type customServer struct {
	Server          *http.Server
	ShutdownTimeout time.Duration
}

func NewServer(cfg *config.Config, ch *clickhouse.HTTPClickhouseClient, redis *redis.RedisClient) *customServer {
	r := getRoutes(cfg, ch, redis)

	server := &customServer{
		Server: &http.Server{
			Addr:              cfg.Server.Addr,
			Handler:           r,
			ReadTimeout:       cfg.Server.ReadTimeout,
			ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
			WriteTimeout:      cfg.Server.WriteTimeout,
			IdleTimeout:       cfg.Server.IdleTimeout,
			MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
		},
		ShutdownTimeout: cfg.Server.ShutdownTimeout,
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
