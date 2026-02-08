package main

import (
	"github.com/lucasmeller1/excel_api/internal/app"
	"github.com/lucasmeller1/excel_api/internal/clickhouse"
	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/redis"
)

func main() {
	cfg := config.Load()
	redis := redis.NewRedis(cfg.Redis)
	ch := clickhouse.NewHTTPClickhouse(cfg.Clickhouse, redis)

	server := app.NewServer(cfg, ch, redis)
	server.Run()
}
