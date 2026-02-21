package clickhouse

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"net/http"

	"github.com/lucasmeller1/excel_api/internal/auth"
	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/handlers"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (c *HTTPClickhouseClient) GetUserTables(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := tracer.Start(ctx, "ClickHouse.GetUserTables", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	startTime := time.Now()
	status := "success"
	httpStatus := http.StatusOK

	defer func() {
		span.SetAttributes(
			attribute.String("status", status),
			attribute.Int("http.status_code", httpStatus),
			attribute.Float64("handler.duration_ms", float64(time.Since(startTime).Milliseconds())),
		)
	}()

	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		err := errors.New("failed to get authorization claims from context")
		handlers.RecordSpanError(span, err)
		status = "error"
		httpStatus = http.StatusInternalServerError

		handlers.JsonError(w, httpStatus, "failed to get authorization claims from context")
		return
	}

	span.SetAttributes(
		attribute.Int("auth.group_count", len(claims.Groups)),
	)

	authorizedSet := make(map[string]struct{})
	for _, s := range c.publicSchemas {
		authorizedSet[s] = struct{}{}
	}

	for _, guid := range claims.Groups {
		if schema, found := config.LookupSchemaByGUID(guid); found {
			authorizedSet[schema] = struct{}{}
		}
	}

	if len(authorizedSet) == 0 {
		err := errors.New("no authorized databases available")
		handlers.RecordSpanError(span, err)
		status = "error"
		httpStatus = http.StatusForbidden

		handlers.JsonError(w, httpStatus, "no authorized databases available")
		return
	}

	span.SetAttributes(
		attribute.Int("authorized.schema_count", len(authorizedSet)),
	)

	var schemas []string
	for s := range authorizedSet {
		schemas = append(schemas, fmt.Sprintf("'%s'", s))
	}
	inClause := strings.Join(schemas, ",")

	fullURL := c.GetExportURL(r)

	sql := fmt.Sprintf(`
		SELECT 
			database AS "Database", 
			name AS "Table", 
			concat(
				'%s?database=',
				database,
				'&table=',
				name
			) AS "URL Download"
		FROM system.tables
		WHERE database IN (%s)
		ORDER BY
			database ASC,
			name ASC`,
		fullURL,
		inClause,
	)

	resp, err := c.QueryCSV(ctx, sql)
	if err != nil {
		handlers.RecordSpanError(span, err)
		status = "error"
		httpStatus = http.StatusInternalServerError

		handlers.JsonError(w, httpStatus, err.Error())
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")

	streamStart := time.Now()

	clientAcceptsGzip := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
	responseSize := 0

	if resp.Header.Get("Content-Encoding") == "gzip" && clientAcceptsGzip {
		w.Header().Set("Content-Encoding", "gzip")
		n, err := io.Copy(w, resp.Body)
		if err != nil {
			handlers.RecordSpanError(span, err)
			status = "error"
			httpStatus = http.StatusInternalServerError
			span.SetAttributes(
				attribute.Bool("stream.error", true),
			)
			return
		}
		responseSize = int(n)
	} else if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			handlers.RecordSpanError(span, err)
			status = "error"
			httpStatus = http.StatusInternalServerError
			span.SetAttributes(
				attribute.String("stream.error", "failed to create gzip reader"),
			)
			return
		}
		defer gz.Close()

		n, err := io.Copy(w, gz)
		if err != nil {
			handlers.RecordSpanError(span, err)
			status = "error"
			httpStatus = http.StatusInternalServerError
			span.SetAttributes(
				attribute.Bool("stream.error", true),
			)
			return
		}
		responseSize = int(n)
	} else {
		n, err := io.Copy(w, resp.Body)
		if err != nil {
			handlers.RecordSpanError(span, err)
			status = "error"
			httpStatus = http.StatusInternalServerError
			span.SetAttributes(
				attribute.Bool("stream.error", true),
			)
			return
		}
		responseSize = int(n)
	}

	span.SetAttributes(
		attribute.Float64("stream.duration_ms", float64(time.Since(streamStart).Milliseconds())),
		attribute.Float64("response.size_mib", handlers.BytesToMiB(responseSize)),
	)
}
