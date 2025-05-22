package adapter

import (
	"context"
	"time"
)

type Adapter interface {
	Set(ctx context.Context, key string, value string, expire time.Duration) error
	MSet(ctx context.Context, values map[string]string, expire time.Duration) error

	Get(ctx context.Context, key string) (string, error)
	MGet(ctx context.Context, keys []string) (map[string]string, error)

	Delete(ctx context.Context, key string) error
}
