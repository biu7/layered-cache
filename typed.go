package cache

import (
	"context"
)

type KeyGenerator[ID comparable] func(id ID) string

type TypedLoaderFunc[ID comparable, T any] func(ctx context.Context, id ID) (T, error)

type TypedBatchLoaderFunc[ID comparable, T any] func(ctx context.Context, ids []ID) (map[ID]T, error)

type TypedCache[ID comparable, T any] struct {
	cache  Cache
	keyGen KeyGenerator[ID]
}

func Typed[ID comparable, T any](cache Cache, keyGen KeyGenerator[ID]) *TypedCache[ID, T] {
	return &TypedCache[ID, T]{cache: cache, keyGen: keyGen}
}

func (c *TypedCache[ID, T]) Get(ctx context.Context, id ID, loader TypedLoaderFunc[ID, T], opts ...GetOption) (T, error) {
	if loader != nil {
		opts = append(opts, WithLoader(func(ctx context.Context, _ string) (any, error) {
			return loader(ctx, id)
		}))
	}
	key := c.keyGen(id)
	var result T
	err := c.cache.Get(ctx, key, &result, opts...)
	return result, err
}

func (c *TypedCache[ID, T]) MGet(ctx context.Context, ids []ID, loader TypedBatchLoaderFunc[ID, T], opts ...GetOption) (map[ID]T, error) {
	var keyList = make([]string, 0, len(ids))
	var key2ID = make(map[string]ID, len(ids))
	var id2Key = make(map[ID]string, len(ids))

	for _, id := range ids {
		key := c.keyGen(id)
		keyList = append(keyList, key)
		key2ID[key] = id
		id2Key[id] = key
	}

	if loader != nil {
		opts = append(opts, WithBatchLoader(func(ctx context.Context, keys []string) (map[string]any, error) {
			var loaderIds = make([]ID, 0, len(keys))
			for _, key := range keys {
				loaderIds = append(loaderIds, key2ID[key])
			}

			values, err := loader(ctx, loaderIds)
			if err != nil {
				return nil, err
			}
			result := make(map[string]any, len(keys))
			for k, v := range values {
				result[id2Key[k]] = v
			}
			return result, nil
		}))
	}

	var result = make(map[string]T)
	err := c.cache.MGet(ctx, keyList, &result, opts...)
	if err != nil {
		return nil, err
	}

	var ret = make(map[ID]T, len(ids))
	for key, value := range result {
		ret[key2ID[key]] = value
	}

	return ret, err
}

func (c *TypedCache[ID, T]) Set(ctx context.Context, key string, value T, opts ...SetOption) error {
	return c.cache.Set(ctx, key, value, opts...)
}

func (c *TypedCache[ID, T]) MSet(ctx context.Context, keyValues map[string]T, opts ...SetOption) error {
	convertedValues := make(map[string]any, len(keyValues))
	for key, value := range keyValues {
		convertedValues[key] = value
	}
	return c.cache.MSet(ctx, convertedValues, opts...)
}

func (c *TypedCache[ID, T]) Delete(ctx context.Context, key string) error {
	return c.cache.Delete(ctx, key)
}
