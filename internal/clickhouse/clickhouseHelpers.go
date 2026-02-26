package clickhouse

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"github.com/lucasmeller1/excel_api/internal/auth"
	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/queue"
	"github.com/lucasmeller1/excel_api/internal/redis"
	"github.com/lucasmeller1/excel_api/internal/telemetry"
	"github.com/lucasmeller1/excel_api/internal/utils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"
)

type HTTPClickhouseClient struct {
	baseURL          string
	user             string
	pass             string
	client           *http.Client
	publicSchemas    []string
	redis            *redis.RedisClient
	TTLTablesInRedis time.Duration
	exportLimiter    *limiter.ExportLimiter
	endpoints        *config.EndpointsConfig
	schemaConfigs    *config.SchemaConfig
}

func NewHTTPClickhouse(cfg *config.Config, redisClient *redis.RedisClient) *HTTPClickhouseClient {
	// used for HTTP requests to Clickhouse
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DisableCompression = true
	t.MaxIdleConns = cfg.Clickhouse.TransportConfig.MaxIdleConns
	t.MaxIdleConnsPerHost = cfg.Clickhouse.TransportConfig.MaxIdleConnsPerHost
	t.IdleConnTimeout = cfg.Clickhouse.TransportConfig.IdleConnTimeout

	return &HTTPClickhouseClient{
		baseURL: cfg.Clickhouse.Hostname,
		user:    cfg.Clickhouse.User,
		pass:    cfg.Clickhouse.Password,
		client: &http.Client{
			Timeout:   time.Second * time.Duration(cfg.Clickhouse.ClientTimeout),
			Transport: t,
		},
		publicSchemas:    cfg.Clickhouse.PublicSchemas,
		redis:            redisClient,
		TTLTablesInRedis: cfg.Clickhouse.TTLTablesInRedis,
		exportLimiter:    limiter.NewExportLimiter(cfg.Clickhouse.QueueSizeLimiter),
		endpoints:        &cfg.Endpoints,
		schemaConfigs:    &cfg.SchemasGUIDs,
	}
}

func (c *HTTPClickhouseClient) GetExportURL(r *http.Request) string {
	newURL := *r.URL
	newURL.Host = r.Host
	newURL.Scheme = "https"
	newURL.Path = strings.Replace(r.URL.Path, c.endpoints.Tables, c.endpoints.Export, 1)

	return newURL.String()
}

func (c *HTTPClickhouseClient) QueryCSV(ctx context.Context, sql string) (*http.Response, error) {
	ctx, span := tracer.Start(ctx, "ClickHouse.Query", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	start := time.Now()
	defer func() {
		span.SetAttributes(
			attribute.Float64(
				"db.duration_ms",
				float64(time.Since(start).Milliseconds()),
			),
		)
	}()

	span.SetAttributes(
		attribute.String("sql_query", sql),
	)

	query := fmt.Sprintf(
		"%s FORMAT CSVWithNames SETTINGS max_result_bytes=%d, result_overflow_mode='break'",
		sql, maxExportSize,
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"?enable_http_compression=1",
		bytes.NewBufferString(query),
	)
	if err != nil {
		telemetry.RecordSpanError(span, err)
		return nil, errors.New("failed to create POST request to database")
	}

	req.SetBasicAuth(c.user, c.pass)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := c.client.Do(req)
	if err != nil {
		telemetry.RecordSpanError(span, err)
		return nil, errors.New("failed to send POST request to database")
	}

	span.SetAttributes(
		attribute.Int("http.status_code", resp.StatusCode),
		attribute.String("net.peer.name", c.baseURL),
	)

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()

		var reader io.Reader = resp.Body

		if resp.Header.Get("Content-Encoding") == "gzip" {
			gzReader, err := gzip.NewReader(resp.Body)
			if err != nil {
				telemetry.RecordSpanError(span, err)
				return nil, fmt.Errorf("failed to unzip error response: %w", err)
			}
			defer gzReader.Close()
			reader = gzReader
		}

		body, err := io.ReadAll(reader)
		if err != nil {
			telemetry.RecordSpanError(span, err)
			return nil, fmt.Errorf("failed to read error body: %w", err)
		}

		errorText := strings.TrimSpace(string(body))
		cleanErr := normalizeClickhouseError(errorText)
		err = errors.New(cleanErr)
		telemetry.RecordSpanError(span, err)
		return nil, err
	}

	if resp.Header.Get("Content-Encoding") != "gzip" {
		return nil, errors.New("unexpected non-gzip response from ClickHouse")
	}

	return resp, nil
}

func normalizeClickhouseError(s string) string {
	if strings.Contains(s, "UNKNOWN_TABLE") {
		return "Table does not exist"
	}

	if strings.Contains(s, "UNKNOWN_DATABASE") {
		return "Database does not exist"
	}

	return "internal server error"
}

func (c *HTTPClickhouseClient) ValidateDatabase(r *http.Request, publicSchemas []string) (int, error) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		return http.StatusInternalServerError, errors.New("failed to get authorization claims from context")
	}

	database := strings.TrimSpace(r.URL.Query().Get("database"))
	table := strings.TrimSpace(r.URL.Query().Get("table"))

	// because tables can be created at any time and will not be validated like the schemas,
	// there will be just a simple check for empty tables
	if database == "" || table == "" {
		return http.StatusBadRequest, errors.New("missing database or table")
	}

	if !(utils.IsValidIdentifier(database) && utils.IsValidIdentifier(table)) {
		return http.StatusBadRequest, errors.New("database or table not valid")
	}

	// check if requested schema is from the public ones
	if slices.Contains(publicSchemas, database) {
		return http.StatusOK, nil
	}

	// get GUID from know schemas (sector_level)
	databaseGUID, ok := c.schemaConfigs.LookupGUIDBySchema(database)
	if !ok {
		return http.StatusBadRequest, errors.New("unknown database schema")
	}

	// check if the requested one exist in the claims
	if slices.Contains(claims.Groups, databaseGUID) {
		return http.StatusOK, nil
	}

	return http.StatusForbidden, errors.New("no permisson for database")
}
