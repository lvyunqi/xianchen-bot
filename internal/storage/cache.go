package storage

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"xianlv/internal/config"
)

type Cache struct {
	client  *redis.Client
	enabled bool
}

func OpenCache(cfg config.RedisConfig) (*Cache, error) {
	cache := &Cache{enabled: cfg.Enabled}
	if !cfg.Enabled {
		return cache, nil
	}
	cache.client = redis.NewClient(&redis.Options{Addr: cfg.Address, Password: cfg.Password, DB: cfg.Database})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := cache.client.Ping(ctx).Err(); err != nil {
		cache.enabled = false
		_ = cache.client.Close()
		cache.client = nil
		return cache, nil
	}
	return cache, nil
}
func (c *Cache) Get(ctx context.Context, key string) (string, bool) {
	if c == nil || !c.enabled {
		return "", false
	}
	value, err := c.client.Get(ctx, key).Result()
	return value, err == nil
}
func (c *Cache) Set(ctx context.Context, key, value string, ttl time.Duration) {
	if c != nil && c.enabled {
		_ = c.client.Set(ctx, key, value, ttl).Err()
	}
}
func (c *Cache) Delete(ctx context.Context, keys ...string) {
	if c != nil && c.enabled && len(keys) > 0 {
		_ = c.client.Del(ctx, keys...).Err()
	}
}
func (c *Cache) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}
