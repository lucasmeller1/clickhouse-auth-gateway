package main

import (
	//"fmt"
	//"github.com/joho/godotenv"
	//"log"

	"github.com/lucasmeller1/excel_api/internal/app"
	//"github.com/lucasmeller1/excel_api/internal/auth"
	"github.com/lucasmeller1/excel_api/internal/clickhouse"
	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/redis"
	//"os"
)

func main() {
	cfg := config.Load()
	redis := redis.NewRedis(cfg.Redis)
	ch := clickhouse.NewHTTPCSV(cfg.Clickhouse, cfg.PublicSchemas, redis)

	/*
		signedToken, err := auth.CreateSignedToken(cfg.Auth, []string{"Contabil_1", "Operacional_1"})
		if err != nil {
			log.Fatalf("failed to create a token: %v", err)
		}
		log.Println("Bearer", signedToken)
	*/

	server := app.NewServer(cfg, ch, redis)
	server.Run()
}
