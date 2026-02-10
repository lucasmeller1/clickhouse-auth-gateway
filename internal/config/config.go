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

	redisHostname := mustEnv("REDIS_HOSTNAME")
	redisPort := mustConvertStringToIntEnv("REDIS_PORT")
	redisPassword := mustEnv("REDIS_PASSWORD")
	redisDB := mustConvertStringToIntEnv("REDIS_DB")

	invalidateCacheToken := mustEnv("INVALIDATE_CACHE_TOKEN")

	config := Config{

		Auth: AuthConfig{
			TenantID: tid,
			Issuer:   issuer,
			Audience: audience,
			Debug:    debug,
		},

		Server: ServerConfig{
			Addr:                addrPort,
			ReadTimeout:         10 * time.Second,
			ReadHeaderTimeout:   10 * time.Second,
			WriteTimeout:        60 * time.Second,
			IdleTimeout:         120 * time.Second,
			MaxHeaderBytes:      1 << 20,
			ShutdownTimeout:     5 * time.Second,
			MaxRequests:         100,
			MaxRequestsInterval: time.Minute,
		},

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
		},

		Redis: RedisConfig{
			Hostname: redisHostname,
			Port:     redisPort,
			Password: redisPassword,
			DB:       redisDB,
		},

		PrivateServer: PrivateServerConfig{
			InvalidateCacheToken: invalidateCacheToken,
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
