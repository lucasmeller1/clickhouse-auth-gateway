package config

import (
	"github.com/joho/godotenv"
	"log"
	"os"
	"strings"
	"time"
)

var groupsMapping = map[string]string{
	"11111111-1111-1111-1111-111111111111": "Contabil_1",
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

type AuthConfig struct {
	TenantID string
	Issuer   string
	Audience string
	KeyID    string
	JWKSURL  string
}

type HTTPConfig struct {
	Addr              string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int

	ShutdownTimeout time.Duration
}

type PublicSchemasConfig struct {
	Schemas []string
}

type ClickhouseConfig struct {
	User     string
	Password string
	Schema   string
	Hostname string

	ClientTimeout int
}

type Config struct {
	HTTP          HTTPConfig
	Auth          AuthConfig
	PublicSchemas PublicSchemasConfig
	Clickhouse    ClickhouseConfig
	/*
		Redis struct {
			Addr string
			DB   int
		}

	*/
}

func mustEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("env %s must not be empty", name)
	}
	return value
}

func schemaStringToSlice(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}

	raw := strings.Split(s, ",")

	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))

	for _, v := range raw {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}

		if _, exists := seen[v]; exists {
			continue
		}

		seen[v] = struct{}{}
		out = append(out, v)
	}

	return out
}

func Load() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("error to load godotend: %v", err)
	}

	tid := mustEnv("TENANT_ID")
	issuer := mustEnv("ISSUER_JWT")
	audience := mustEnv("AUDIENCE_JWT")
	kid := mustEnv("KID_JWT")
	addrPort := mustEnv("HTTP_PORT")
	publicSchemas := schemaStringToSlice(os.Getenv("PUBLIC_SCHEMAS"))

	userClickhouse := mustEnv("CLICKHOUSE_USER")
	passwordClickhouse := mustEnv("CLICKHOUSE_PASSWORD")
	schemaClickhouse := mustEnv("CLICKHOUSE_SCHEMA")
	hostnameClickhouse := mustEnv("CLICKHOUSE_HOSTNAME")

	config := Config{
		Auth: AuthConfig{
			TenantID: tid,
			Issuer:   issuer,
			Audience: audience,
			KeyID:    kid,
		},
		HTTP: HTTPConfig{
			Addr:              addrPort,
			ReadTimeout:       10 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
			MaxHeaderBytes:    1 << 20,
			ShutdownTimeout:   5 * time.Second,
		},
		PublicSchemas: PublicSchemasConfig{
			Schemas: publicSchemas,
		},
		Clickhouse: ClickhouseConfig{
			User:          userClickhouse,
			Password:      passwordClickhouse,
			Schema:        schemaClickhouse,
			Hostname:      hostnameClickhouse,
			ClientTimeout: 60,
		},
	}

	return &config
}
