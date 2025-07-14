package adapter

import (
	"context"
	"time"
)

type RemoteAdapter interface {
	Set(ctx context.Context, key string, value []byte, expire time.Duration) error
	MSet(ctx context.Context, values map[string][]byte, expire time.Duration) error

	Get(ctx context.Context, key string) ([]byte, error)
	MGet(ctx context.Context, keys []string) (map[string][]byte, error)

	Delete(ctx context.Context, key string) error
}

type MemoryAdapter interface {
	Set(key string, value []byte, expire time.Duration) int32
	MSet(values map[string][]byte, expire time.Duration) int32

	Get(key string) ([]byte, bool)
	MGet(keys []string) map[string][]byte

	Delete(key string)
}
