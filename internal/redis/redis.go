package redis

import (
	"context"
	"fmt"

	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"

	"time"
)

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

	return &RedisClient{
		Client: rdb,
		prefix: "excel_api:",
		group:  singleflight.Group{},
	}
}

func (r *RedisClient) GetWithSingleflight(ctx context.Context, key string, ttl time.Duration, getDataFunc func() ([]byte, error)) ([]byte, error) {
	val, err := r.GetCachedResponse(ctx, key)
	if err == nil && val != nil {
		return val, nil
	}

	v, err, _ := r.group.Do(key, func() (any, error) {
		val, err := r.GetCachedResponse(ctx, key)
		if err == nil && val != nil {
			return val, nil
		}

		data, fetchErr := getDataFunc()
		if fetchErr != nil {
			return nil, fetchErr
		}

		_ = r.SetCachedResponse(ctx, key, data, ttl)
		return data, nil
	})

	if err != nil {
		return nil, err
	}

	return v.([]byte), nil
}

func (r *RedisClient) GetCachedResponse(ctx context.Context, key string) ([]byte, error) {
	fullKey := r.prefix + key
	val, err := r.Client.Get(ctx, fullKey).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return val, err
}

func (r *RedisClient) SetCachedResponse(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	fullKey := r.prefix + key
	return r.Client.Set(ctx, fullKey, data, ttl).Err()
}

func (r *RedisClient) InvalidateCache(ctx context.Context, key string) error {
	fullKey := r.prefix + key
	return r.Client.Del(ctx, fullKey).Err()
}

func (r *RedisClient) Close() error {
	return r.Client.Close()
}
