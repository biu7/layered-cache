package main

import "context"

type TypedCache[T any] struct {
	cache Cache
}

func Typed[T any](cache Cache) *TypedCache[T] {
	return &TypedCache[T]{cache: cache}
}

func (c *TypedCache[T]) Get(ctx context.Context, key string, opts ...GetOption) (*T, error) {
	var result T
	err := c.cache.Get(ctx, key, &result, opts...)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *TypedCache[T]) MGet(ctx context.Context, keys []string, opts ...GetOption) (map[string]T, error) {
	var result map[string]T
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
