package clickhouse

import (
	"fmt"
	"strings"

	"log"
	"net/http"
	"time"

	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/redis"

	"errors"
	"github.com/lucasmeller1/excel_api/internal/auth"
	"slices"
)

type HTTPCSVClient struct {
	baseURL       string
	user          string
	pass          string
	client        *http.Client
	publicSchemas []string
	redis         *redis.RedisClient
}

func NewHTTPCSV(cfg config.ClickhouseConfig, cfgPublicSchemas config.PublicSchemasConfig, redisClient *redis.RedisClient) *HTTPCSVClient {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DisableCompression = true
	t.MaxIdleConns = 100
	t.MaxIdleConnsPerHost = 100
	t.IdleConnTimeout = 90 * time.Second

	return &HTTPCSVClient{
		baseURL: cfg.Hostname,
		user:    cfg.User,
		pass:    cfg.Password,
		client: &http.Client{
			Timeout:   time.Second * time.Duration(cfg.ClientTimeout),
			Transport: t,
		},
		publicSchemas: cfgPublicSchemas.Schemas,
		redis:         redisClient,
	}
}

func quoteIdentifier(identifier string) string {
	escaped := strings.ReplaceAll(identifier, `"`, `""`)
	return fmt.Sprintf(`"%s"`, escaped)
}

func normalizeClickhouseError(s string) string {
	if strings.Contains(s, "UNKNOWN_TABLE") {
		return "Table does not exist"
	}

	if strings.Contains(s, "UNKNOWN_DATABASE") {
		return "Database does not exist"
	}

	log.Println("Clickhouse error:", s)
	return "internal server error"
}

func ValidateDatabase(r *http.Request, publicSchemas []string) (int, error) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		return http.StatusInternalServerError, errors.New("internal server error")
	}

	database := strings.TrimSpace(r.URL.Query().Get("database"))
	table := strings.TrimSpace(r.URL.Query().Get("table"))

	// because tables can be created at any time and will not be validate like the schemas,
	// there will be just a simple check for empty tables
	if database == "" || table == "" {
		return http.StatusBadRequest, errors.New("missing database or table")
	}

	// check if requested schema is from the public ones
	if slices.Contains(publicSchemas, database) {
		return http.StatusOK, nil
	}

	// get GUID from know schemas (sector_level)
	databaseGUID, ok := config.LookupGUIDBySchema(database)
	if !ok {
		return http.StatusBadRequest, errors.New("unknown database schema")
	}

	// check if the requested one exist in the claims
	if slices.Contains(claims.Groups, databaseGUID) {
		return http.StatusOK, nil
	}

	return http.StatusForbidden, errors.New("no permisson for database")
}
