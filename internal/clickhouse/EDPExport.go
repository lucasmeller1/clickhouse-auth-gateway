package clickhouse

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lucasmeller1/clickhouse-auth-gateway/internal/handlers"
	"github.com/lucasmeller1/clickhouse-auth-gateway/internal/telemetry"
	"github.com/lucasmeller1/clickhouse-auth-gateway/internal/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	name          = "github.com/lucasmeller1/clickhouse-auth-gateway/internal/clickhouse"
	maxExportSize = 100 << 20
)

var (
	meter            = otel.Meter(name)
	tracer           = otel.Tracer(name)
	apiRequestCount  metric.Int64Counter
	deliveryDuration metric.Float64Histogram
	activeExports    metric.Int64Gauge
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
		"table.export.processing.duration",
		metric.WithDescription("Time taken to export table."),
		metric.WithUnit("s"),
	)
	if err != nil {
		fmt.Printf("failed to create histogram: %v\n", err)
	}

	activeExports, err = meter.Int64Gauge(
		"active.export",
		metric.WithDescription("Number of active exports."),
	)
	if err != nil {
		fmt.Printf("failed to create metric active.export: %v\n", err)
	}
}

func (c *HTTPClickhouseClient) ExportCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ctx, span := tracer.Start(ctx, "ClickHouse.ExportEndpoint", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	startTime := time.Now()
	status := "success"
	httpStatus := http.StatusOK
	cacheStatus := "unknown"
	dbName := ""
	tableName := ""

	isCacheMiss := false

	defer func() {
		activeExports.Record(ctx, int64(c.exportLimiter.Active()))

		duration := time.Since(startTime).Seconds()

		attrs := []attribute.KeyValue{
			attribute.String("db", dbName),
			attribute.String("table", tableName),
			attribute.String("cache_status", cacheStatus),
			attribute.String("status", status),
			attribute.Int("http.status_code", httpStatus),
			attribute.Bool("gzip_request", strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")),
		}

		span.SetAttributes(
			attribute.String("cache.status", cacheStatus),
		)

		deliveryDuration.Record(ctx, duration, metric.WithAttributes(attrs...))
		apiRequestCount.Add(ctx, 1, metric.WithAttributes(attrs...))
	}()

	statusCode, err := c.ValidateDatabase(r, c.publicSchemas)
	if err != nil {
		telemetry.RecordSpanError(span, err)

		status = "error"
		httpStatus = statusCode
		handlers.JsonError(w, statusCode, err.Error())
		return
	}

	dbName = strings.TrimSpace(r.URL.Query().Get("database"))
	tableName = strings.TrimSpace(r.URL.Query().Get("table"))

	cacheKey := fmt.Sprintf("csv:%s:%s", dbName, tableName)

	ttl := c.TTLTablesInRedis

	gzipData, err := c.redis.GetWithSingleflight(ctx, cacheKey, ttl, func(sfCtx context.Context) ([]byte, error) {
		if err := c.exportLimiter.Acquire(sfCtx); err != nil {
			return nil, err
		}
		defer c.exportLimiter.Release()

		activeExports.Record(sfCtx, int64(c.exportLimiter.Active()))

		isCacheMiss = true
		sql := fmt.Sprintf("SELECT * FROM %s.%s", utils.QuoteIdentifier(dbName), utils.QuoteIdentifier(tableName))

		resp, err := c.QueryCSV(sfCtx, sql)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		limitedReader := io.LimitReader(resp.Body, maxExportSize+1)
		data, err := io.ReadAll(limitedReader)
		if err != nil {
			return nil, err
		}

		if int64(len(data)) > maxExportSize {
			return nil, fmt.Errorf("export exceeds max allowed size (%d MB)", maxExportSize>>20)
		}

		return data, nil
	})

	if err != nil {
		if utils.IsCanceled(ctx, err) {
			status = "canceled"
			httpStatus = 499
			span.SetAttributes(attribute.Bool("client.canceled", true))
			return
		}

		status = "error"
		telemetry.RecordSpanError(span, err)
		httpStatus = http.StatusInternalServerError

		if errors.Is(err, ErrTableNotFound) {
			httpStatus = http.StatusBadRequest
		}

		handlers.JsonError(w, httpStatus, err.Error())
		return
	}

	cacheStatus = "hit"
	if isCacheMiss {
		cacheStatus = "miss"
	}

	httpStatus = c.serveGzip(ctx, w, r, gzipData)
	if httpStatus == 499 {
		status = "canceled"
	} else if httpStatus >= 400 {
		status = "error"
	}
}

func (c *HTTPClickhouseClient) serveGzip(ctx context.Context, w http.ResponseWriter, r *http.Request, data []byte) int {
	ctx, span := tracer.Start(ctx, "HTTP.StreamCSV", trace.WithSpanKind(trace.SpanKindServer))
	defer span.End()

	sizeMiB := utils.BytesToMiB(len(data))

	span.SetAttributes(
		attribute.Float64("response.size_mib", sizeMiB),
	)

	start := time.Now()
	defer func() {
		span.SetAttributes(
			attribute.Float64(
				"stream.duration_ms",
				float64(time.Since(start).Milliseconds()),
			),
		)
	}()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="data.csv"`)

	clientAcceptsGzip := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")

	if clientAcceptsGzip {
		w.Header().Set("Content-Encoding", "gzip")
		if _, err := w.Write(data); err != nil {
			if utils.IsCanceled(ctx, err) {
				span.SetAttributes(attribute.Bool("client.canceled", true))
				return 499
			}
			telemetry.RecordSpanError(span, err)
			return http.StatusInternalServerError
		}
		return http.StatusOK
	}

	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		telemetry.RecordSpanError(span, err)
		handlers.JsonError(w, http.StatusInternalServerError, "Decompression error")
		return http.StatusInternalServerError
	}
	defer gzReader.Close()

	if _, err := io.Copy(w, gzReader); err != nil {
		if utils.IsCanceled(ctx, err) {
			span.SetAttributes(attribute.Bool("client.canceled", true))
			return 499
		}
		telemetry.RecordSpanError(span, err)
		return http.StatusInternalServerError
	}
	return http.StatusOK
}
