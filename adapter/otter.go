package adapter

import (
	"context"
	"fmt"
	"time"

	"github.com/biu7/layered-cache/errors"
	"github.com/maypok86/otter"
)

var _ Adapter = (*OtterAdapter)(nil)

type OtterAdapter struct {
	client *otter.CacheWithVariableTTL[string, string]
}

func NewOtterAdapter(maxMemory int) (*OtterAdapter, error) {
	if maxMemory <= 0 {
		return nil, fmt.Errorf("otter create: invalid maxMemory: %d", maxMemory)
	}
	cache, err := otter.MustBuilder[string, string](maxMemory).
		WithVariableTTL().
		Cost(func(key string, value string) uint32 {
			return uint32(len(key) + len(value))
		}).
		Build()
	if err != nil {
		return nil, fmt.Errorf("otter create: capacity %d: %w", maxMemory, err)
	}
	return &OtterAdapter{
		client: &cache,
	}, nil
}

func NewOtterAdapterWithClient(client *otter.CacheWithVariableTTL[string, string]) (*OtterAdapter, error) {
	if client == nil {
		return nil, fmt.Errorf("otter create: cache is nil")
	}
	return &OtterAdapter{
		client: client,
	}, nil
}

func (o *OtterAdapter) Set(ctx context.Context, key string, value string, expire time.Duration) error {
	if expire < time.Second {
		return fmt.Errorf("otter set %s: %w", key, errors.ErrInvalidExpireTime)
	}
	success := o.client.Set(key, value, expire)
	if !success {
		return fmt.Errorf("otter set dropped: %s", key)
	}
	return nil
}

func (o *OtterAdapter) MSet(ctx context.Context, values map[string]string, expire time.Duration) error {
	if expire < time.Second {
		return fmt.Errorf("otter mset: %w", errors.ErrInvalidExpireTime)
	}
	for key, value := range values {
		_ = o.client.Set(key, value, expire)
	}
	return nil
}

func (o *OtterAdapter) Get(ctx context.Context, key string) (string, error) {
	val, success := o.client.Get(key)
	if !success {
		return "", errors.ErrNotFound
	}
	return val, nil
}

func (o *OtterAdapter) MGet(ctx context.Context, keys []string) (map[string]string, error) {
	ret := make(map[string]string)
	for _, key := range keys {
		val, success := o.client.Get(key)
		if !success {
			continue
		}
		ret[key] = val
	}
	return ret, nil
}

func (o *OtterAdapter) Delete(ctx context.Context, key string) error {
	o.client.Delete(key)
	return nil
}
