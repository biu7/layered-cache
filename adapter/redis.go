package adapter

import (
	"context"
	"fmt"
	"time"

	"github.com/biu7/layered-cache/errors"
	"github.com/redis/go-redis/v9"
)

var _ Adapter = (*RedisAdapter)(nil)

type RedisAdapter struct {
	client redis.Cmdable
}

func NewRedisAdapter(redisURL string) (*RedisAdapter, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("redis parse url: %w", err)
	}
	client := redis.NewClient(opt)
	return NewRedisAdapterWithClient(client), nil
}

func NewRedisAdapterWithClient(client redis.Cmdable) *RedisAdapter {
	return &RedisAdapter{client: client}
}

func (r *RedisAdapter) Set(ctx context.Context, key string, value []byte, expire time.Duration) error {
	err := r.client.Set(ctx, key, value, expire).Err()
	if err != nil {
		return fmt.Errorf("redis set %s: %w", key, err)
	}
	return nil
}

func (r *RedisAdapter) MSet(ctx context.Context, values map[string][]byte, expire time.Duration) error {
	pipeline := r.client.Pipeline()

	for key, val := range values {
		pipeline.Set(ctx, key, val, expire)
	}
	_, err := pipeline.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis mset: %w", err)
	}
	return nil
}

func (r *RedisAdapter) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errors.ErrNotFound
		}
		return nil, fmt.Errorf("redis get %s: %w", key, err)
	}
	return val, nil
}

func (r *RedisAdapter) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	ret := make(map[string][]byte, len(keys))
	if len(keys) == 0 {
		return ret, nil
	}
	vals, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("redis mget: %w", err)
	}
	for i, key := range keys {
		if vals[i] == nil {
			continue
		}
		ret[key] = []byte(vals[i].(string))
	}
	return ret, nil
}

func (r *RedisAdapter) Delete(ctx context.Context, key string) error {
	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("redis del %s: %w", key, err)
	}
	return nil
}
