package clickhouse

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	// "time"

	// "log"
	"strings"

	"net/http"

	"github.com/lucasmeller1/excel_api/internal/handlers"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	name          = "github.com/lucasmeller1/excel_api/internal/clickhouse"
	maxExportSize = 100 << 20
)

var (
	meter            = otel.Meter(name)
	apiRequestCount  metric.Int64Counter
	deliveryDuration metric.Float64Histogram
)

func init() {
	var err error

	apiRequestCount, err = meter.Int64Counter(
		"api.request.count",
		metric.WithDescription("Total requests handled by the export API."),
	)
	if err != nil {
		fmt.Printf("failed to create metric: %v\n", err)
	}

	deliveryDuration, err = meter.Float64Histogram(
		"table.delivery.duration",
		metric.WithDescription("Time taken to deliver the table to the user."),
		metric.WithUnit("s"),
	)
	if err != nil {
		fmt.Printf("failed to create histogram: %v\n", err)
	}
}

func (c *HTTPClickhouseClient) QueryCSV(ctx context.Context, sql string) (*http.Response, error) {
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

func (c *HTTPClickhouseClient) ExportCSV(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	status := "success"
	httpStatus := http.StatusOK
	cacheStatus := "unknown"

	ctx := r.Context()
	isCacheMiss := false

	dbName := strings.TrimSpace(r.URL.Query().Get("database"))
	tableName := strings.TrimSpace(r.URL.Query().Get("table"))

	defer func() {
		duration := time.Since(startTime).Seconds()

		attrs := []attribute.KeyValue{
			attribute.String("db", dbName),
			attribute.String("table", tableName),
			attribute.String("cache_status", cacheStatus),
			attribute.String("status", status),
			attribute.Int("http.status_code", httpStatus),
		}

		deliveryDuration.Record(ctx, duration, metric.WithAttributes(attrs...))
		apiRequestCount.Add(ctx, 1, metric.WithAttributes(attrs...))
	}()

	statusCode, err := ValidateDatabase(r, c.publicSchemas)
	if err != nil {
		status = "error"
		httpStatus = statusCode
		handlers.JsonError(w, statusCode, err.Error())
		return
	}

	cacheKey := fmt.Sprintf("csv:%s:%s", dbName, tableName)

	ttl := c.TTLTablesInRedis

	gzipData, err := c.redis.GetWithSingleflight(ctx, cacheKey, ttl, func() ([]byte, error) {
		isCacheMiss = true
		sql := fmt.Sprintf("SELECT * FROM %s.%s", quoteIdentifier(dbName), quoteIdentifier(tableName))

		resp, err := c.QueryCSV(ctx, sql)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		limitedReader := io.LimitReader(resp.Body, maxExportSize+1)
		data, err := io.ReadAll(limitedReader)
		if err != nil {
			return nil, err
		}

		return data, nil
	})

	if err != nil {
		status = "error"
		httpStatus = http.StatusInternalServerError
		handlers.JsonError(w, httpStatus, err.Error())
		return
	}

	cacheStatus = "hit"
	if isCacheMiss {
		cacheStatus = "miss"
	}

	httpStatus = c.serveGzip(w, r, gzipData)
	if httpStatus >= 400 {
		status = "error"
	}
}

func (c *HTTPClickhouseClient) serveGzip(w http.ResponseWriter, r *http.Request, data []byte) int {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="data.csv"`)

	clientAcceptsGzip := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")

	if clientAcceptsGzip {
		w.Header().Set("Content-Encoding", "gzip")
		if _, err := w.Write(data); err != nil {
			return http.StatusInternalServerError
		}
		return http.StatusOK
	}

	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		handlers.JsonError(w, http.StatusInternalServerError, "Decompression error")
		return http.StatusInternalServerError
	}
	defer gzReader.Close()

	if _, err := io.Copy(w, gzReader); err != nil {
		return http.StatusInternalServerError
	}
	return http.StatusOK
}
