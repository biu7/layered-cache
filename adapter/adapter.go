package adapter

import (
	"context"
	"time"
)

type Adapter interface {
	Set(ctx context.Context, key string, value []byte, expire time.Duration) error
	MSet(ctx context.Context, values map[string][]byte, expire time.Duration) error

	Get(ctx context.Context, key string) ([]byte, error)
	MGet(ctx context.Context, keys []string) (map[string][]byte, error)

	Delete(ctx context.Context, key string) error
}
