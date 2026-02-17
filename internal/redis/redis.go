package redis

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/handlers"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/singleflight"

	"time"
)

const name = "github.com/lucasmeller1/excel_api/internal/redis"

var tracer = otel.Tracer(name)

type RedisClient struct {
	Client *redis.Client
	prefix string
	group  singleflight.Group
}

func NewRedis(cfg config.RedisConfig) *RedisClient {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Hostname, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		Protocol: 3,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("failed to start redis")
	}

	return &RedisClient{
		Client: rdb,
		prefix: "excel_api:",
		group:  singleflight.Group{},
	}
}

func (r *RedisClient) GetWithSingleflight(ctx context.Context, key string, ttl time.Duration, getDataFunc func(ctx context.Context) ([]byte, error)) ([]byte, error) {
	ctx, span := tracer.Start(ctx, "Redis.GetWithSingleflight", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(attribute.String("cache.key", key))

	val, err := r.GetCachedResponse(ctx, key)
	if err == nil && val != nil {
		span.SetAttributes(attribute.Bool("cache.hit", true))
		return val, nil
	}

	v, err, shared := r.group.Do(key, func() (any, error) {
		detachedCtx := context.WithoutCancel(ctx)
		sfCtx, cancel := context.WithTimeout(detachedCtx, time.Minute*2)
		defer cancel()

		sfCtx, sfSpan := tracer.Start(sfCtx, "Redis.Singleflight.Fetch", trace.WithSpanKind(trace.SpanKindInternal))
		defer sfSpan.End()

		val, err := r.GetCachedResponse(sfCtx, key)
		if err == nil && val != nil {
			return val, nil
		}

		data, fetchErr := getDataFunc(sfCtx)
		if fetchErr != nil {
			sfSpan.RecordError(fetchErr)
			return nil, fetchErr
		}

		_ = r.SetCachedResponse(sfCtx, key, data, ttl)

		return data, nil
	})

	if err != nil {
		return nil, err
	}

	span.SetAttributes(attribute.Bool("singleflight.shared", shared))

	return v.([]byte), nil
}

func (r *RedisClient) GetCachedResponse(ctx context.Context, key string) ([]byte, error) {
	ctx, span := tracer.Start(ctx, "Redis.GET", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	fullKey := r.prefix + key

	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.operation", "GET"),
	)

	val, err := r.Client.Get(ctx, fullKey).Bytes()
	if err == redis.Nil {
		span.SetAttributes(attribute.Bool("cache.hit", false))
		return nil, nil
	}

	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	span.SetAttributes(attribute.Bool("cache.hit", true))
	return val, err
}

func (r *RedisClient) SetCachedResponse(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	ctx, span := tracer.Start(ctx, "Redis.SET", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	fullKey := r.prefix + key

	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.operation", "SET"),
		attribute.Float64("redis.value_size", handlers.BytesToMiB(len(data))),
		attribute.Int64("redis.ttl_seconds", int64(ttl.Seconds())),
	)

	err := r.Client.Set(ctx, fullKey, data, ttl).Err()
	if err != nil {
		span.RecordError(err)
	}

	return err
}

func (r *RedisClient) InvalidateCache(ctx context.Context, key string) (int64, error) {
	ctx, span := tracer.Start(ctx, "Redis.DEL", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	fullKey := r.prefix + key

	span.SetAttributes(
		attribute.String("redis.key", fullKey),
	)

	deleted, err := r.Client.Del(ctx, fullKey).Result()
	if err != nil {
		span.RecordError(err)
		return 0, err
	}

	span.SetAttributes(attribute.Int64("redis.deleted_count", deleted))
	return deleted, nil
}

func (r *RedisClient) Close() error {
	return r.Client.Close()
}

// not idiomatic, change later
func (redis *RedisClient) InvalidateCacheEndpoint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ctx, span := tracer.Start(ctx, "Redis.InvalidateCacheEndpoint", trace.WithSpanKind(trace.SpanKindServer))
	defer span.End()

	successfulInvalidation := false

	defer func() {
		span.SetAttributes(
			attribute.Bool("redis.invalidate.success", successfulInvalidation),
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

	deleted, err := redis.InvalidateCache(ctx, cacheKey)
	if err != nil {
		handlers.RecordSpanError(span, fmt.Errorf("failed to invalidate cached table: %w", err))
		http.Error(w, "failed to invalidate cache", http.StatusInternalServerError)
		return
	}

	span.SetAttributes(attribute.Int64("redis.deleted_count", deleted))
	successfulInvalidation = true
	w.WriteHeader(http.StatusOK)
}
