package cache

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

const separator = ":"

type TypedCache[ID comparable, T any] struct {
	cache Cache
}

func Typed[ID comparable, T any](cache Cache) *TypedCache[ID, T] {
	return &TypedCache[ID, T]{cache: cache}
}

type TypedLoaderFunc[ID comparable, T any] func(ctx context.Context, id ID) (T, error)

type TypedBatchLoaderFunc[ID comparable, T any] func(ctx context.Context, ids []ID) (map[ID]T, error)

func (c *TypedCache[ID, T]) Get(ctx context.Context, keyPrefix string, id ID, loader TypedLoaderFunc[ID, T], opts ...GetOption) (T, error) {
	if loader != nil {
		opts = append(opts, WithLoader(func(ctx context.Context, _ string) (any, error) {
			return loader(ctx, id)
		}))
	}

	var result T
	err := c.cache.Get(ctx, c.buildKey(keyPrefix, id), &result, opts...)
	return result, err
}

func (c *TypedCache[ID, T]) MGet(ctx context.Context, keyPrefix string, ids []ID, loader TypedBatchLoaderFunc[ID, T], opts ...GetOption) (map[ID]T, error) {
	var keys = make([]string, 0, len(ids))
	var key2ID = make(map[string]ID, len(ids))

	for _, id := range ids {
		key := c.buildKey(keyPrefix, id)
		keys = append(keys, key)
		key2ID[key] = id
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
			result := make(map[string]any, len(values))
			for id, value := range values {
				result[c.buildKey(keyPrefix, id)] = value
			}
			return result, nil
		}))
	}

	var ret = make(map[string]T)
	err := c.cache.MGet(ctx, keys, &ret, opts...)
	if err != nil {
		return nil, err
	}

	var result = make(map[ID]T, len(ret))
	for key, value := range ret {
		result[key2ID[key]] = value
	}

	return result, nil
}

func (c *TypedCache[ID, T]) Set(ctx context.Context, keyPrefix string, id ID, value T, opts ...SetOption) error {
	return c.cache.Set(ctx, c.buildKey(keyPrefix, id), value, opts...)
}

func (c *TypedCache[ID, T]) MSet(ctx context.Context, keyPrefix string, values map[ID]T, opts ...SetOption) error {
	setValues := make(map[string]any, len(values))
	for id, value := range values {
		setValues[c.buildKey(keyPrefix, id)] = value
	}
	return c.cache.MSet(ctx, setValues, opts...)
}

func (c *TypedCache[ID, T]) Delete(ctx context.Context, keyPrefix string, id ID) error {
	return c.cache.Delete(ctx, c.buildKey(keyPrefix, id))
}

func (c *TypedCache[ID, T]) buildKey(keyPrefix string, id ID) string {
	var builder strings.Builder
	builder.WriteString(keyPrefix)
	builder.WriteString(separator)

	switch v := any(id).(type) {
	case string:
		builder.WriteString(v)
	case int:
		builder.WriteString(strconv.FormatInt(int64(v), 10))
	case int32:
		builder.WriteString(strconv.FormatInt(int64(v), 10))
	case int64:
		builder.WriteString(strconv.FormatInt(v, 10))
	default:
		// 以上足够覆盖 99% 的场景，其他类型直接 fmt 处理
		builder.WriteString(fmt.Sprintf("%v", v))
	}
	return builder.String()
}
