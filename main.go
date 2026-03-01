package main

import (
	"github.com/lucasmeller1/clickhouse-auth-gateway/internal/app"
	"github.com/lucasmeller1/clickhouse-auth-gateway/internal/clickhouse"
	"github.com/lucasmeller1/clickhouse-auth-gateway/internal/config"
	"github.com/lucasmeller1/clickhouse-auth-gateway/internal/redis"
)

func main() {
	cfg := config.Load()
	redis := redis.NewRedis(cfg.Redis)
	ch := clickhouse.NewHTTPClickhouse(cfg, redis)

	server := app.NewServer(cfg, ch, redis)
	server.Run()
}
