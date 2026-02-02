package redis

import (
	"context"
	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/redis/go-redis/v9"
	"time"
)

type RedisClient struct {
	client *redis.Client
	prefix string
}

func NewRedis(cfg config.RedisConfig) *RedisClient {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		Protocol: 3,
	})

	return &RedisClient{
		client: rdb,
		prefix: "excel_api:",
	}
}

func (r *RedisClient) GetCachedResponse(ctx context.Context, key string) ([]byte, error) {
	fullKey := r.prefix + key
	val, err := r.client.Get(ctx, fullKey).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return val, err
}

func (r *RedisClient) SetCachedResponse(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	fullKey := r.prefix + key
	return r.client.Set(ctx, fullKey, data, ttl).Err()
}
