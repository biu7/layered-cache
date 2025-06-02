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

func MGet[T any](cache Cache, ctx context.Context, keys []string, opts ...GetOption) (map[string]T, error) {
	var result map[string]T
	err := cache.MGet(ctx, keys, &result, opts...)
	return result, err
}
