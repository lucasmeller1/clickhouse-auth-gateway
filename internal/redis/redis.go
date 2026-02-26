package redis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/go-chi/httprate"
	httprateredis "github.com/go-chi/httprate-redis"
	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/telemetry"
	"github.com/lucasmeller1/excel_api/internal/utils"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/singleflight"
)

const name = "github.com/lucasmeller1/excel_api/internal/redis"

var (
	tracer             = otel.Tracer(name)
	ErrRedisConnection = errors.New("infrastructure_fault_redis")
)

type RedisClient struct {
	Client *redis.Client
	prefix string
	group  singleflight.Group
}

func NewRateLimiter(cfg config.RedisConfig) (httprate.LimitCounter, error) {
	return httprateredis.NewRedisLimitCounter(&httprateredis.Config{
		Host:     cfg.Hostname,
		Port:     uint16(cfg.Port),
		Password: cfg.Password,
	})
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
		slog.Error("failed to start redis", "error", err)
		os.Exit(1)
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
	if err != nil {
		redisError := fmt.Errorf("%w: %v", ErrRedisConnection, err)
		telemetry.RecordSpanError(span, redisError)
		return nil, redisError
	}
	if val != nil {
		span.SetAttributes(attribute.Bool("cache.hit", true))
		return val, nil
	}

	v, err, shared := r.group.Do(key, func() (any, error) {
		detachedCtx := context.WithoutCancel(ctx)
		sfCtx, cancel := context.WithTimeout(detachedCtx, time.Minute)
		defer cancel()

		sfCtx, sfSpan := tracer.Start(sfCtx, "Redis.Singleflight.Fetch", trace.WithSpanKind(trace.SpanKindInternal))
		defer sfSpan.End()

		val, err := r.GetCachedResponse(sfCtx, key)
		if err != nil {
			redisError := fmt.Errorf("%w: %v", ErrRedisConnection, err)
			telemetry.RecordSpanError(sfSpan, redisError)
			return nil, redisError
		}
		if val != nil {
			return val, nil
		}

		data, fetchErr := getDataFunc(sfCtx)
		if fetchErr != nil {
			telemetry.RecordSpanError(sfSpan, fetchErr)
			return nil, fetchErr
		}

		err = r.SetCachedResponse(sfCtx, key, data, ttl)
		if err != nil {
			telemetry.RecordSpanError(sfSpan, fmt.Errorf("failed to SET redis key: %v", key))
		}

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
		telemetry.RecordSpanError(span, err)
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
		attribute.Float64("redis.size_mib", utils.BytesToMiB(len(data))),
		attribute.Int64("redis.ttl_seconds", int64(ttl.Seconds())),
	)

	err := r.Client.Set(ctx, fullKey, data, ttl).Err()
	if err != nil {
		telemetry.RecordSpanError(span, err)
	}

	return err
}

func (r *RedisClient) DeleteKey(ctx context.Context, key string) (int64, error) {
	ctx, span := tracer.Start(ctx, "Redis.DEL", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	fullKey := r.prefix + key

	span.SetAttributes(
		attribute.String("redis.key", fullKey),
	)

	deleted, err := r.Client.Del(ctx, fullKey).Result()
	if err != nil {
		telemetry.RecordSpanError(span, err)
		return 0, err
	}

	span.SetAttributes(attribute.Int64("redis.deleted_count", deleted))
	return deleted, nil
}

func (r *RedisClient) Close() error {
	return r.Client.Close()
}
