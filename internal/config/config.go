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

func mustConvertStringToIntEnv(s string) int {
	number, err := strconv.Atoi(os.Getenv(s))
	if err != nil {
		log.Fatalf("failed to convert %s: %v", s, err)
	}
	return number
}

func Load() *Config {
	_ = godotenv.Load()

	schemaConfig, err := LoadSchemaConfig("/schema_guids.json")
	if err != nil {
		log.Fatalf("failed to load schema config: %v", err)
	}

	addrPort := ":" + mustEnv("SERVER_PORT")

	tid := mustEnv("TENANT_ID")
	issuer := fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", tid)
	audience := mustEnv("AUDIENCE_JWT")
	debug := mustEnv("DEBUG")

	publicSchemas := []string{"Atualizacoes", "Consultas"}
	userClickhouse := mustEnv("CLICKHOUSE_USER")
	passwordClickhouse := mustEnv("CLICKHOUSE_PASSWORD")
	schemaClickhouse := mustEnv("CLICKHOUSE_SCHEMA")
	hostnameClickhouse := fmt.Sprintf("http://%s:%s", mustEnv("CLICKHOUSE_HOSTNAME"), mustEnv("CLICKHOUSE_PORT"))
	queueSizeLimiter := mustConvertStringToIntEnv("QUEUE_SIZE_LIMITER")

	redisHostname := mustEnv("REDIS_HOSTNAME")
	redisPort := mustConvertStringToIntEnv("REDIS_PORT")
	redisPassword := mustEnv("REDIS_PASSWORD")
	redisDB := mustConvertStringToIntEnv("REDIS_DB")

	invalidateCacheToken := mustEnv("INVALIDATE_CACHE_TOKEN")

	config := Config{

		// related to EntraID Auth
		Auth: AuthConfig{
			TenantID: tid,
			Issuer:   issuer,
			Audience: audience,
			Debug:    debug,
		},

		// related to golang public server
		Server: ServerConfig{
			Addr:              addrPort,
			ReadTimeout:       10 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
			MaxHeaderBytes:    1 << 20,

			MaxRequestsExportEDP:         200,
			MaxRequestsIntervalExportEDP: time.Minute,

			MaxRequestsTablesEDP:         30,
			MaxRequestsIntervalTablesEDP: time.Minute,

			// time to close all (both server, redis connection and otel)
			ShutdownTimeout: 5 * time.Second,
		},

		// related to clickhouse conn, http client and cache
		Clickhouse: ClickhouseConfig{
			User:          userClickhouse,
			Password:      passwordClickhouse,
			Schema:        schemaClickhouse,
			Hostname:      hostnameClickhouse,
			ClientTimeout: 60,
			PublicSchemas: publicSchemas,

			TransportConfig: HTTPTransportClickhouse{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
			TTLTablesInRedis: time.Minute,
			QueueSizeLimiter: queueSizeLimiter,
		},

		// related to redis cache
		Redis: RedisConfig{
			Hostname: redisHostname,
			Port:     redisPort,
			Password: redisPassword,
			DB:       redisDB,
		},

		// related to golang private server
		PrivateServer: PrivateServerConfig{
			InvalidateCacheToken: invalidateCacheToken,
		},

		Endpoints: EndpointsConfig{
			Export:  "exportar",
			Tables:  "tabelas",
			Version: "1",
		},

		SchemasGUIDs: *schemaConfig,
	}

	return &config
}
