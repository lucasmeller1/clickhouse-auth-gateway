package redis

import (
	"context"
	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
	//"log"
	"time"
)

type RedisClient struct {
	client *redis.Client
	prefix string
	group  singleflight.Group
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
		group:  singleflight.Group{},
	}
}

func (r *RedisClient) GetWithSingleflight(ctx context.Context, key string, ttl time.Duration, getDataFunc func() ([]byte, error)) ([]byte, error) {
	val, err := r.GetCachedResponse(ctx, key)
	if err == nil && val != nil {
		//log.Printf("found %s in cache", key)
		return val, nil
	}

	v, err, _ := r.group.Do(key, func() (any, error) {
		val, err := r.GetCachedResponse(ctx, key)
		if err == nil && val != nil {
			return val, nil
		}

		//log.Printf("NOT found %s in cache", key)
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

func (r *RedisClient) InvalidateCache(ctx context.Context, key string) error {
	fullKey := r.prefix + key
	return r.client.Del(ctx, fullKey).Err()
}
