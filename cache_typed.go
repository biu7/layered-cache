package cache

import (
	"context"
)

type TypedCache[T any] struct {
	cache Cache
}

func Typed[T any](cache Cache) *TypedCache[T] {
	return &TypedCache[T]{cache: cache}
}

type TypedLoaderFunc[T any] func(ctx context.Context, key string) (T, error)

type TypedBatchLoaderFunc[T any] func(ctx context.Context, keys []string) (map[string]T, error)

func (c *TypedCache[T]) Get(ctx context.Context, key string, loader TypedLoaderFunc[T], opts ...GetOption) (T, error) {
	if loader != nil {
		opts = append(opts, WithLoader(func(ctx context.Context, key string) (any, error) {
			return loader(ctx, key)
		}))
	}

	var result T
	err := c.cache.Get(ctx, key, &result, opts...)
	return result, err
}

func (c *TypedCache[T]) MGet(ctx context.Context, keys []string, loader TypedBatchLoaderFunc[T], opts ...GetOption) (map[string]T, error) {
	if loader != nil {
		opts = append(opts, WithBatchLoader(func(ctx context.Context, keys []string) (map[string]any, error) {
			values, err := loader(ctx, keys)
			if err != nil {
				return nil, err
			}
			result := make(map[string]any, len(keys))
			for k, v := range values {
				result[k] = v
			}
			return result, nil
		}))
	}

	var result = make(map[string]T)
	err := c.cache.MGet(ctx, keys, &result, opts...)
	return result, err
}

func (c *TypedCache[T]) Set(ctx context.Context, key string, value T, opts ...SetOption) error {
	return c.cache.Set(ctx, key, value, opts...)
}

func (c *TypedCache[T]) MSet(ctx context.Context, keyValues map[string]T, opts ...SetOption) error {
	convertedValues := make(map[string]any, len(keyValues))
	for key, value := range keyValues {
		convertedValues[key] = value
	}
	return c.cache.MSet(ctx, convertedValues, opts...)
}

func (c *TypedCache[T]) Delete(ctx context.Context, key string) error {
	return c.cache.Delete(ctx, key)
}
