package clickhouse

import (
	"fmt"
	"strings"

	"log"
	"net/http"
	"time"

	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/redis"
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
