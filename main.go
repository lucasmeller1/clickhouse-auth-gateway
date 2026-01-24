package main

import (
	//"fmt"
	"github.com/joho/godotenv"
	"github.com/lucasmeller1/excel_api/internal/auth"
	"log"
	"os"
)

var groupsMapping = map[string]string{
	"11111111-1111-1111-1111-111111111112": "Contabil_2",
	"11111111-1111-1111-1111-111111111113": "Contabil_3",
	"11111111-1111-1111-1111-111111111114": "Contabil_4",
	"11111111-1111-1111-1111-111111111115": "Contabil_5",
	"11111111-1111-1111-1111-111111111116": "Contabil_6",
	"11111111-1111-1111-1111-111111111117": "Contabil_7",
	"22222222-2222-2222-2222-222222222221": "Financeiro_1",
	"22222222-2222-2222-2222-222222222222": "Financeiro_2",
	"22222222-2222-2222-2222-222222222223": "Financeiro_3",
	"22222222-2222-2222-2222-222222222224": "Financeiro_4",
	"22222222-2222-2222-2222-222222222225": "Financeiro_5",
	"22222222-2222-2222-2222-222222222226": "Financeiro_6",
	"22222222-2222-2222-2222-222222222227": "Financeiro_7",
	"33333333-3333-3333-3333-333333333331": "Operacional_1",
	"33333333-3333-3333-3333-333333333332": "Operacional_2",
	"33333333-3333-3333-3333-333333333333": "Operacional_3",
	"33333333-3333-3333-3333-333333333334": "Operacional_4",
	"33333333-3333-3333-3333-333333333335": "Operacional_5",
	"33333333-3333-3333-3333-333333333336": "Operacional_6",
	"33333333-3333-3333-3333-333333333337": "Operacional_7",
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("error to load godotend: %v", err)
	}

	tid := os.Getenv("TENANT_ID")
	issuer := os.Getenv("ISSUER_JWT")
	audience := os.Getenv("AUDIENCE_JWT")
	kid := os.Getenv("KID_JWT")
	if tid == "" || issuer == "" || audience == "" || kid == "" {
		log.Fatal("tenant_id, kid, issuer or audience is empty")
	}

	signedToken, err := auth.CreateSignedToken(tid, kid, []string{"Contabil_3", "Operacional_4"})
	if err != nil {
		log.Fatalf("failed to create a token: %v", err)
	}
	log.Println(signedToken)

}
