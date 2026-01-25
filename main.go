package main

import (
	//"fmt"
	//"github.com/joho/godotenv"
	"github.com/lucasmeller1/excel_api/internal/app"
	"github.com/lucasmeller1/excel_api/internal/auth"
	"github.com/lucasmeller1/excel_api/internal/config"
	"log"
	//"os"
)

func main() {
	cfg := config.Load()

	signedToken, err := auth.CreateSignedToken(cfg.Auth, []string{"Contabil_3", "Operacional_4"})
	if err != nil {
		log.Fatalf("failed to create a token: %v", err)
	}
	log.Println(signedToken)

	server := app.NewServer(cfg.HTTP)
	server.Run()
}
