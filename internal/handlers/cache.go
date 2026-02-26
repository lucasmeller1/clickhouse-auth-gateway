package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/lucasmeller1/excel_api/internal/redis"
	"github.com/lucasmeller1/excel_api/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const name = "github.com/lucasmeller1/excel_api/internal/handlers"

var (
	tracer = otel.Tracer(name)
)

type CacheHandler struct {
	Redis *redis.RedisClient
}

func NewCacheHandler(r *redis.RedisClient) *CacheHandler {
	return &CacheHandler{Redis: r}
}

func (h *CacheHandler) DeleteCacheEndpoint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ctx, span := tracer.Start(ctx, "Redis.DeleteCacheEndpoint", trace.WithSpanKind(trace.SpanKindServer))
	defer span.End()

	successfulDelete := false

	defer func() {
		span.SetAttributes(
			attribute.Bool("redis.delete.success", successfulDelete),
		)
	}()

	dbName := strings.TrimSpace(r.URL.Query().Get("database"))
	tableName := strings.TrimSpace(r.URL.Query().Get("table"))

	if dbName == "" || tableName == "" {
		http.Error(w, "missing database or table parameter", http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.String("redis.dbName", dbName),
		attribute.String("redis.tableName", tableName),
	)

	cacheKey := fmt.Sprintf("csv:%s:%s", dbName, tableName)

	deleted, err := h.Redis.DeleteKey(ctx, cacheKey)
	if err != nil {
		telemetry.RecordSpanError(span, fmt.Errorf("failed to delete cached table: %w", err))
		http.Error(w, "failed to delete cache", http.StatusInternalServerError)
		return
	}

	span.SetAttributes(attribute.Int64("redis.deleted_count", deleted))
	successfulDelete = true

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"deleted_keys": %d}`, deleted)

}
