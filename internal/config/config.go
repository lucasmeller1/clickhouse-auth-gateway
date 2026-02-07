package config

import (
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"
	"strconv"
	"time"
)

func mustEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("env %s must not be empty", name)
	}
	return value
}

func Load() *Config {
	_ = godotenv.Load()

	tid := mustEnv("TENANT_ID")
	issuer := fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", tid)
	audience := mustEnv("AUDIENCE_JWT")
	addrPort := mustEnv("HTTP_PORT")
	publicSchemas := []string{"Atualizacoes", "Consultas"}

	userClickhouse := mustEnv("CLICKHOUSE_USER")
	passwordClickhouse := mustEnv("CLICKHOUSE_PASSWORD")
	schemaClickhouse := mustEnv("CLICKHOUSE_SCHEMA")
	hostnameClickhouse := mustEnv("CLICKHOUSE_HOSTNAME")

	redisAddr := mustEnv("REDIS_HOSTNAME")
	redisPassword := mustEnv("REDIS_PASSWORD")
	redisDB, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		log.Fatalf("failed to convert redis DB: %v", err)
	}

	config := Config{
		Auth: AuthConfig{
			TenantID: tid,
			Issuer:   issuer,
			Audience: audience,
		},
		HTTP: HTTPConfig{
			Addr:              ":" + addrPort,
			ReadTimeout:       10 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
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
			Hostname:      "http://" + hostnameClickhouse,
			ClientTimeout: 60,
		},
		Redis: RedisConfig{
			Addr:     redisAddr,
			Password: redisPassword,
			DB:       redisDB,
		},
	}

	return &config
}

func LookupSchemaByGUID(s string) (string, bool) {
	schema, ok := GUIDToSchema[s]
	return schema, ok
}

func LookupGUIDBySchema(s string) (string, bool) {
	guid, ok := SchemaToGUID[s]
	return guid, ok
}
