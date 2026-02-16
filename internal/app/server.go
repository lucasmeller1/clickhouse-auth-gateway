package app

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/lucasmeller1/excel_api/internal/clickhouse"
	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/redis"
	"github.com/lucasmeller1/excel_api/internal/telemetry"
)

type customServer struct {
	PublicServer    *http.Server
	PrivateServer   *http.Server
	ShutdownTimeout time.Duration
	redisClient     *redis.RedisClient
}

func NewServer(cfg *config.Config, ch *clickhouse.HTTPClickhouseClient, redis *redis.RedisClient) *customServer {
	publicRouter := getPublicRoutes(cfg, ch, redis)
	privateRouter := GetPrivateRoutes(cfg, redis)

	server := &customServer{
		// main server - API Gateway Clickhouse
		PublicServer: &http.Server{
			Addr:              cfg.Server.Addr,
			Handler:           publicRouter,
			ReadTimeout:       cfg.Server.ReadTimeout,
			ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
			WriteTimeout:      cfg.Server.WriteTimeout,
			IdleTimeout:       cfg.Server.IdleTimeout,
			MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
		},
		// internal docker server to invalidate tables when updated
		PrivateServer: &http.Server{
			Addr:         ":8081",
			Handler:      privateRouter,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		},

		ShutdownTimeout: cfg.Server.ShutdownTimeout,
		redisClient:     redis,
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

	otelShutdown, err := telemetry.SetupOTelSDK(ctx)
	if err != nil {
		log.Printf("failed to start Otel: %v", err)
		stop()
	}

	var wg sync.WaitGroup

	// --- Start Public Server ---
	wg.Go(func() {
		log.Printf("public API started on port %s\n", svr.PublicServer.Addr)
		if err := svr.PublicServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("public server error: %v", err)
			stop()
		}
	})

	// --- Start Internal Server ---
	wg.Go(func() {
		log.Printf("internal API started on port %s\n", svr.PrivateServer.Addr)
		if err := svr.PrivateServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("internal server error: %v", err)
			stop()
		}
	})

	<-ctx.Done()
	log.Println("shutdown initiated")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), svr.ShutdownTimeout)
	defer cancel()

	var shutdownWg sync.WaitGroup

	shutdownWg.Go(func() {
		if err := svr.PublicServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("public server shutdown failed: %v", err)
		}
	})

	shutdownWg.Go(func() {
		if err := svr.PrivateServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("internal server shutdown failed: %v", err)
		}
	})

	shutdownWg.Wait()

	if err := svr.redisClient.Close(); err != nil {
		log.Printf("error closing redis: %v", err)
	}

	if err := otelShutdown(shutdownCtx); err != nil {
		log.Printf("error shutting down OTel: %v", err)
	}

	wg.Wait()
	log.Println("shutdown completed")
}
