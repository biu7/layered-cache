package main

import (
	"context"
	"fmt"
	"time"

	"github.com/biu7/layered-cache/adapter"
	"github.com/biu7/layered-cache/errors"
	"github.com/biu7/layered-cache/serializer"
	"golang.org/x/sync/singleflight"
)

type Cache interface {
	Get(ctx context.Context, key string, target interface{}, opts ...GetOption) error
	MGet(ctx context.Context, keys []string, target interface{}, opts ...GetOption) error
	Set(ctx context.Context, key string, value interface{}, opts ...SetOption) error
	MSet(ctx context.Context, keyValues map[string]interface{}, opts ...SetOption) error
	Delete(ctx context.Context, key string) error
}

// LayeredCache 分层缓存实现
type LayeredCache struct {
	// 适配器
	memoryAdapter adapter.Adapter
	redisAdapter  adapter.Adapter

	// 序列化器
	serializer serializer.Serializer

	// 默认TTL
	defaultMemoryTTL time.Duration
	defaultRedisTTL  time.Duration

	// 默认缺失值缓存设置
	defaultCacheMissing bool
	defaultMissingTTL   time.Duration

	// singleflight，防止并发请求重复调用 loader
	sf singleflight.Group
}

// NewCache 创建新的缓存实例
func NewCache(opts ...Option) (Cache, error) {
	config := newOptions()

	applyOptions(config, opts...)

	if config.memoryAdapter == nil && config.redisAdapter == nil {
		return nil, errors.ErrAdapterRequired
	}

	if err := validateTTL(config.defaultMemoryTTL, config.defaultRedisTTL); err != nil {
		return nil, err
	}

	cache := &LayeredCache{
		memoryAdapter: config.memoryAdapter,
		redisAdapter:  config.redisAdapter,
		serializer:    config.serializer,

		defaultMemoryTTL:    config.defaultMemoryTTL,
		defaultRedisTTL:     config.defaultRedisTTL,
		defaultCacheMissing: config.defaultCacheMissing,
		defaultMissingTTL:   config.defaultMissingTTL,
	}

	return cache, nil
}

// Get 获取缓存值
func (c *LayeredCache) Get(ctx context.Context, key string, target interface{}, opts ...GetOption) error {
	// 解析Get选项
	config := newGetOptions()
	applyGetOptions(config, opts...)

	var cacheErrors []error

	// 1. 先从内存缓存获取
	if c.memoryAdapter != nil {
		if data, err := c.memoryAdapter.Get(ctx, key); err == nil {
			return c.serializer.Unmarshal([]byte(data), target)
		} else if !IsNotFound(err) {
			cacheErrors = append(cacheErrors, fmt.Errorf("memory cache get failed: %w", err))
		}
	}

	// 2. 如果内存缓存未命中，尝试从Redis获取
	if c.redisAdapter != nil {
		if data, err := c.redisAdapter.Get(ctx, key); err == nil {
			if c.memoryAdapter != nil {
				_ = c.memoryAdapter.Set(ctx, key, data, c.defaultMemoryTTL)
			}
			err = c.serializer.Unmarshal([]byte(data), target)
			if err != nil {
				cacheErrors = append(cacheErrors, err)
				return errors.Join(cacheErrors...)
			}
			return nil
		} else if !IsNotFound(err) {
			cacheErrors = append(cacheErrors, fmt.Errorf("redis cache get failed: %w", err))
		}
	}

	if config.loader == nil {
		// 如果有缓存错误，将缓存错误与 NotFound 错误组合
		if len(cacheErrors) > 0 {
			cacheErrors = append(cacheErrors, errors.ErrNotFound)
			return errors.Join(cacheErrors...)
		}
		return errors.ErrNotFound
	}

	// 3. 如果缓存都未命中，从 loader 加载
	result, err, _ := c.sf.Do(key, func() (interface{}, error) {
		return c.loadAndCache(ctx, key, config)
	})

	if err != nil {
		// 如果有缓存错误，将缓存错误与 loader 错误组合
		if len(cacheErrors) > 0 {
			cacheErrors = append(cacheErrors, fmt.Errorf("loader failed: %w", err))
			return errors.Join(cacheErrors...)
		}
		return err
	}

	// 将结果设置到目标
	return c.serializer.Unmarshal(result.([]byte), target)
}

// loadAndCache 加载数据并缓存
func (c *LayeredCache) loadAndCache(ctx context.Context, key string, config *getOptions) ([]byte, error) {
	// 调用 loader 获取数据
	value, err := config.loader(ctx, key)
	if err != nil && !IsNotFound(err) {
		return nil, err
	}

	// 检查是否为缺失值（nil 或 error）
	isMissing := IsNotFound(err) || value == nil

	// 判断是否应该缓存缺失值
	shouldCacheMissing := c.shouldCacheMissing(config.cacheMissing)
	if isMissing && !shouldCacheMissing {
		return nil, errors.ErrNotFound
	}

	// 序列化并存储到缓存
	data, err := c.serializer.Marshal(value)
	if err != nil {
		return nil, err
	}

	// 计算TTL
	memoryTTL, redisTTL := c.calculateLoaderTTL(config, isMissing && shouldCacheMissing)

	// 校验TTL
	if err := validateTTL(memoryTTL, 0); err != nil {
		return nil, err
	}
	c.memoryAdapter.Set(ctx, key, string(data), memoryTTL)

	// 设置到Redis缓存
	if c.redisAdapter != nil {
		// 校验TTL
		if err := validateTTL(0, redisTTL); err != nil {
			return nil, err
		}
		c.redisAdapter.Set(ctx, key, string(data), redisTTL)
	}

	return data, nil
}

// Set 设置缓存值
func (c *LayeredCache) Set(ctx context.Context, key string, value interface{}, opts ...SetOption) error {
	// 解析Set选项
	config := newSetOptions()
	applySetOptions(config, opts...)

	// 序列化值
	data, err := c.serializer.Marshal(value)
	if err != nil {
		return err
	}

	// 计算TTL（Set操作中不是缺失值）
	memoryTTL, redisTTL := c.calculateSetTTL(config)

	// 校验TTL
	if err := validateTTL(memoryTTL, 0); err != nil {
		return err
	}

	// 设置到内存缓存
	if err := c.memoryAdapter.Set(ctx, key, string(data), memoryTTL); err != nil {
		return err
	}

	// 设置到Redis缓存
	if c.redisAdapter != nil {
		// 校验TTL
		if err := validateTTL(0, redisTTL); err != nil {
			return err
		}
		if err := c.redisAdapter.Set(ctx, key, string(data), redisTTL); err != nil {
			return err
		}
	}

	return nil
}

// calculateLoaderTTL 计算内存和Redis缓存的TTL
func (c *LayeredCache) calculateLoaderTTL(config *getOptions, isMissing bool) (memoryTTL, redisTTL time.Duration) {
	if isMissing {
		// 缺失值使用较短的TTL
		missingTTL := config.missingTTL
		if missingTTL <= 0 {
			missingTTL = c.defaultMissingTTL
		}
		return missingTTL, missingTTL
	}

	// 正常值使用配置的TTL
	memoryTTL = config.memoryTTL
	if memoryTTL == 0 {
		memoryTTL = c.defaultMemoryTTL
	}

	redisTTL = config.redisTTL
	if redisTTL == 0 {
		redisTTL = c.defaultRedisTTL
	}

	return memoryTTL, redisTTL
}

// calculateSetTTL 计算Set操作的TTL
func (c *LayeredCache) calculateSetTTL(config *setOptions) (memoryTTL, redisTTL time.Duration) {
	memoryTTL = config.memoryTTL
	if memoryTTL == 0 {
		memoryTTL = c.defaultMemoryTTL
	}

	redisTTL = config.redisTTL
	if redisTTL == 0 {
		redisTTL = c.defaultRedisTTL
	}

	return memoryTTL, redisTTL
}

// shouldCacheMissing 判断是否应该缓存缺失值
func (c *LayeredCache) shouldCacheMissing(operationCacheMissing *bool) bool {
	if operationCacheMissing != nil {
		return *operationCacheMissing // 操作级设置了，使用操作级设置
	}
	return c.defaultCacheMissing // 否则使用全局默认设置
}

// Delete 删除缓存值
func (c *LayeredCache) Delete(ctx context.Context, key string) error {
	// 从内存缓存删除
	if err := c.memoryAdapter.Delete(ctx, key); err != nil {
		return err
	}

	// 从Redis缓存删除
	if c.redisAdapter != nil {
		if err := c.redisAdapter.Delete(ctx, key); err != nil {
			return err
		}
	}

	return nil
}

// MGet 批量获取缓存值
func (c *LayeredCache) MGet(ctx context.Context, keys []string, target interface{}, opts ...GetOption) error {
	// 解析Get选项
	config := newGetOptions()
	applyGetOptions(config, opts...)

	// 创建结果映射
	result := make(map[string]interface{})
	missingKeys := make([]string, 0)
	var cacheErrors []error

	// 1. 先从内存缓存批量获取
	if c.memoryAdapter != nil {
		if data, err := c.memoryAdapter.MGet(ctx, keys); err == nil {
			for key, value := range data {
				var item interface{}
				if err := c.serializer.Unmarshal([]byte(value), &item); err == nil {
					result[key] = item
				}
			}
		} else {
			cacheErrors = append(cacheErrors, fmt.Errorf("memory cache mget failed: %w", err))
		}
	}

	// 找出缺失的键
	for _, key := range keys {
		if _, exists := result[key]; !exists {
			missingKeys = append(missingKeys, key)
		}
	}

	// 2. 如果内存缓存未完全命中，尝试从Redis获取缺失的键
	if len(missingKeys) > 0 && c.redisAdapter != nil {
		if data, err := c.redisAdapter.MGet(ctx, missingKeys); err == nil {
			// 更新缺失键列表
			stillMissingKeys := make([]string, 0)
			for _, key := range missingKeys {
				if value, exists := data[key]; exists {
					var item interface{}
					if err := c.serializer.Unmarshal([]byte(value), &item); err == nil {
						result[key] = item
						// 回填到内存缓存
						if c.memoryAdapter != nil {
							_ = c.memoryAdapter.Set(ctx, key, value, c.defaultMemoryTTL)
						}
					} else {
						stillMissingKeys = append(stillMissingKeys, key)
					}
				} else {
					stillMissingKeys = append(stillMissingKeys, key)
				}
			}
			missingKeys = stillMissingKeys
		} else {
			cacheErrors = append(cacheErrors, fmt.Errorf("redis cache mget failed: %w", err))
		}
	}

	// 3. 如果仍有缺失的键且有loader，从loader加载
	if len(missingKeys) > 0 && config.loader != nil {
		var loaderErrors []error
		for _, key := range missingKeys {
			// 使用singleflight防止重复加载
			loadedData, err, _ := c.sf.Do(key, func() (interface{}, error) {
				return c.loadAndCache(ctx, key, config)
			})

			if err != nil {
				loaderErrors = append(loaderErrors, fmt.Errorf("loader failed for key %s: %w", key, err))
				continue
			}

			var item interface{}
			if err := c.serializer.Unmarshal(loadedData.([]byte), &item); err == nil {
				result[key] = item
			}
		}

		// 如果有loader错误，将其与缓存错误组合
		if len(loaderErrors) > 0 {
			cacheErrors = append(cacheErrors, loaderErrors...)
		}
	} else if len(missingKeys) > 0 && config.loader == nil {
		// 如果有缺失的键但没有loader，添加NotFound错误
		cacheErrors = append(cacheErrors, fmt.Errorf("keys not found and no loader provided: %v", missingKeys))
	}

	// 将结果设置到目标
	resultData, err := c.serializer.Marshal(result)
	if err != nil {
		// 如果有缓存错误，将其与序列化错误组合
		if len(cacheErrors) > 0 {
			cacheErrors = append(cacheErrors, fmt.Errorf("serialization failed: %w", err))
			return errors.Join(cacheErrors...)
		}
		return err
	}

	if err := c.serializer.Unmarshal(resultData, target); err != nil {
		// 如果有缓存错误，将其与反序列化错误组合
		if len(cacheErrors) > 0 {
			cacheErrors = append(cacheErrors, fmt.Errorf("deserialization failed: %w", err))
			return errors.Join(cacheErrors...)
		}
		return err
	}

	// 如果有缓存错误但操作最终成功，返回组合的错误
	if len(cacheErrors) > 0 {
		return errors.Join(cacheErrors...)
	}

	return nil
}

// MSet 批量设置缓存值
func (c *LayeredCache) MSet(ctx context.Context, keyValues map[string]interface{}, opts ...SetOption) error {
	// 解析Set选项
	config := newSetOptions()
	applySetOptions(config, opts...)

	// 计算TTL（MSet操作中不是缺失值）
	memoryTTL, redisTTL := c.calculateSetTTL(config)

	// 序列化所有值
	serializedData := make(map[string]string)
	for key, value := range keyValues {
		data, err := c.serializer.Marshal(value)
		if err != nil {
			return err
		}
		serializedData[key] = string(data)
	}

	// 设置到内存缓存
	if c.memoryAdapter != nil {
		// 校验TTL
		if err := validateTTL(memoryTTL, 0); err != nil {
			return err
		}
		if err := c.memoryAdapter.MSet(ctx, serializedData, memoryTTL); err != nil {
			return err
		}
	}

	// 设置到Redis缓存
	if c.redisAdapter != nil {
		// 校验TTL
		if err := validateTTL(0, redisTTL); err != nil {
			return err
		}
		if err := c.redisAdapter.MSet(ctx, serializedData, redisTTL); err != nil {
			return err
		}
	}

	return nil
}

func IsNotFound(err error) bool {
	return errors.Is(err, errors.ErrNotFound)
}
