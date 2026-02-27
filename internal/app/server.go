package app

import (
	"context"
	"fmt"
	"github.com/lucasmeller1/excel_api/internal/clickhouse"
	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/redis"
	"github.com/lucasmeller1/excel_api/internal/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
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
			Handler:           otelhttp.NewHandler(publicRouter, "Public Server"),
			ReadTimeout:       cfg.Server.ReadTimeout,
			ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
			WriteTimeout:      cfg.Server.WriteTimeout,
			IdleTimeout:       cfg.Server.IdleTimeout,
			MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
		},
		// internal docker server to invalidate tables when updated
		PrivateServer: &http.Server{
			Addr:         ":8081",
			Handler:      otelhttp.NewHandler(privateRouter, "Private Server"),
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
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
		slog.Error("failed to start Otel", "error", err)
		stop()
	}

	var wg sync.WaitGroup

	// --- Start Public Server ---
	wg.Go(func() {
		slog.Info(fmt.Sprintf("public API started on port %s\n", svr.PublicServer.Addr))
		if err := svr.PublicServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("public server error", "error", err)
			stop()
		}
	})

	// --- Start Internal Server ---
	wg.Go(func() {
		slog.Info(fmt.Sprintf("private API started on port %s\n", svr.PrivateServer.Addr))
		if err := svr.PrivateServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("private server error", "error", err)
			stop()
		}
	})

	<-ctx.Done()
	slog.Info("shutdown initiated")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), svr.ShutdownTimeout)
	defer cancel()

	var shutdownWg sync.WaitGroup

	shutdownWg.Go(func() {
		if err := svr.PublicServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("public server shutdown failed", "error", err)
		}
	})

	shutdownWg.Go(func() {
		if err := svr.PrivateServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("private server shutdown failed", "error", err)
		}
	})

	shutdownWg.Wait()

	if err := svr.redisClient.Close(); err != nil {
		slog.Error("error closing redis", "error", err)
	}

	if err := otelShutdown(shutdownCtx); err != nil {
		slog.Error("error shutting down OTel", "error", err)
	}

	wg.Wait()
	slog.Info("shutdown completed")
}
