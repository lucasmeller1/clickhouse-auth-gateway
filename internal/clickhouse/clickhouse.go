package clickhouse

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"log"
	"net/http"
	"time"

	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/handlers"
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

func (c *HTTPCSVClient) QueryCSV(ctx context.Context, sql string) (*http.Response, error) {
	query := sql + " FORMAT CSVWithNames"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"?enable_http_compression=1",
		bytes.NewBufferString(query),
	)
	if err != nil {
		return nil, errors.New("failed to create POST request to database")
	}

	req.SetBasicAuth(c.user, c.pass)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.New("failed to send POST request to database")
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()

		var reader io.Reader = resp.Body

		if resp.Header.Get("Content-Encoding") == "gzip" {
			gzReader, err := gzip.NewReader(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to unzip error response: %w", err)
			}
			defer gzReader.Close()
			reader = gzReader
		}

		body, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read error body: %w", err)
		}

		errorText := strings.TrimSpace(string(body))
		cleanErr := normalizeClickhouseError(errorText)
		return nil, errors.New(cleanErr)
	}

	return resp, nil
}

func quoteIdentifier(identifier string) string {
	escaped := strings.ReplaceAll(identifier, `"`, `""`)
	return fmt.Sprintf(`"%s"`, escaped)
}

/*
func (c *HTTPCSVClient) ExportCSV(w http.ResponseWriter, r *http.Request) {
	statusCode, err := handlers.ValidateDatabase(r, c.publicSchemas)
	if err != nil {
		handlers.JsonError(w, statusCode, err.Error())
		return
	}

	ctx := r.Context()
	database := quoteIdentifier(strings.TrimSpace(r.URL.Query().Get("database")))
	table := quoteIdentifier(strings.TrimSpace(r.URL.Query().Get("table")))
	//cacheKey := fmt.Sprintf("csv:%s:%s", database, table)
	sql := fmt.Sprintf("SELECT * FROM %s.%s", database, table)

	resp, err := c.QueryCSV(ctx, sql)
	if err != nil {
		handlers.JsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="data.csv"`)

	clickhouseIsGzipped := resp.Header.Get("Content-Encoding") == "gzip"
	clientAcceptsGzip := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")

	if clickhouseIsGzipped && clientAcceptsGzip {
		w.Header().Set("Content-Encoding", "gzip")
		if _, err := io.Copy(w, resp.Body); err != nil {
			fmt.Printf("Error streaming gzip data: %v\n", err)
		}
		return
	}

	if clickhouseIsGzipped && !clientAcceptsGzip {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			handlers.JsonError(w, http.StatusInternalServerError, "Failed to decompress data stream")
			return
		}
		defer gzReader.Close()

		if _, err := io.Copy(w, gzReader); err != nil {
			fmt.Printf("Error streaming decompressed data: %v\n", err)
		}
		return
	}

	if _, err := io.Copy(w, resp.Body); err != nil {
		fmt.Printf("Error streaming text data: %v\n", err)
	}
}
*/

func (c *HTTPCSVClient) ExportCSV(w http.ResponseWriter, r *http.Request) {
	statusCode, err := handlers.ValidateDatabase(r, c.publicSchemas)
	if err != nil {
		handlers.JsonError(w, statusCode, err.Error())
		return
	}

	ctx := r.Context()
	dbName := strings.TrimSpace(r.URL.Query().Get("database"))
	tableName := strings.TrimSpace(r.URL.Query().Get("table"))

	// Create the unique key for this specific table
	cacheKey := fmt.Sprintf("csv:%s:%s", dbName, tableName)

	// Define the TTL (e.g., 1 hour, but Airflow will invalidate it sooner)
	ttl := 1 * time.Minute

	// Use Singleflight to get the GZIP data
	gzipData, err := c.redis.GetWithSingleflight(ctx, cacheKey, ttl, func() ([]byte, error) {
		// --- This block only runs ONCE for concurrent requests ---
		sql := fmt.Sprintf("SELECT * FROM %s.%s", quoteIdentifier(dbName), quoteIdentifier(tableName))

		resp, err := c.QueryCSV(ctx, sql)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		// Read the entire body into memory to store in Redis
		// Note: ClickHouse already sends GZIP because of your headers in QueryCSV
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		return data, nil
	})

	if err != nil {
		handlers.JsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// At this point, gzipData is definitely GZIP (from ClickHouse)
	c.serveGzip(w, r, gzipData)
}

// Helper to handle the decompression logic for the client
func (c *HTTPCSVClient) serveGzip(w http.ResponseWriter, r *http.Request, data []byte) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="data.csv"`)

	clientAcceptsGzip := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")

	if clientAcceptsGzip {
		w.Header().Set("Content-Encoding", "gzip")
		w.Write(data)
		return
	}

	// If client doesn't support gzip, we must decompress it before sending
	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		handlers.JsonError(w, http.StatusInternalServerError, "Decompression error")
		return
	}
	defer gzReader.Close()
	io.Copy(w, gzReader)
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

func (c *HTTPCSVClient) GetUserTables(w http.ResponseWriter, r *http.Request) {
	claims, ok := handlers.ClaimsFromContext(r.Context())
	if !ok {
		handlers.JsonError(w, http.StatusInternalServerError, "failed to parse authorization claims")
		return
	}

	authorizedSet := make(map[string]struct{})
	for _, s := range c.publicSchemas {
		authorizedSet[s] = struct{}{}
	}

	for _, guid := range claims.Groups {
		if schema, found := handlers.LookupSchemaByGUID(guid); found {
			authorizedSet[schema] = struct{}{}
		}
	}

	if len(authorizedSet) == 0 {
		handlers.JsonError(w, http.StatusForbidden, "no authorized databases found for your account")
		return
	}

	var schemas []string
	for s := range authorizedSet {
		schemas = append(schemas, fmt.Sprintf("'%s'", s))
	}
	inClause := strings.Join(schemas, ",")

	sql := fmt.Sprintf(`
		SELECT 
			database, 
			name, 
			concat(
				'http://%s/export?database=',
				database,
				'&table=',
				name
			) AS download_url
		FROM system.tables
		WHERE database IN (%s)
		ORDER BY
			database ASC,
			name ASC`,
		r.Host,
		inClause,
	)

	ctx := r.Context()
	resp, err := c.QueryCSV(ctx, sql)
	if err != nil {
		handlers.JsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")

	if resp.Header.Get("Content-Encoding") == "gzip" && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		io.Copy(w, resp.Body)
		return
	}

	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, _ := gzip.NewReader(resp.Body)
		defer gz.Close()
		io.Copy(w, gz)
		return
	}

	io.Copy(w, resp.Body)
}
