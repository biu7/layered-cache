package cache

import (
	"bytes"
	"context"
	"reflect"
	"time"

	"github.com/biu7/layered-cache/errors"
	"github.com/biu7/layered-cache/serializer"
	"github.com/biu7/layered-cache/storage"
	"golang.org/x/sync/singleflight"
)

var (
	notFoundPlaceholder = []byte("__CACHE_NOT_FOUND__")
	ErrNotFound         = errors.ErrNotFound
)

type Cache interface {
	Set(ctx context.Context, key string, value any, opts ...SetOption) error
	MSet(ctx context.Context, keyValues map[string]any, opts ...SetOption) error
	Delete(ctx context.Context, key string) error

	Get(ctx context.Context, key string, target any, opts ...GetOption) error
	MGet(ctx context.Context, keys []string, target any, opts ...GetOption) error
}

// LayeredCache 分层缓存实现
type LayeredCache struct {
	// 适配器
	memory storage.Memory
	remote storage.Remote

	// 序列化器
	serializer serializer.Serializer

	// 默认TTL
	defaultMemoryTTL time.Duration
	defaultRemoteTTL time.Duration

	// 默认缺失值缓存设置
	defaultCacheNotFound    bool
	defaultCacheNotFoundTTL time.Duration

	// singleflight，防止并发请求重复调用 loader
	sf singleflight.Group
}

// NewCache 创建新的缓存实例
func NewCache(opts ...Option) (Cache, error) {
	config := newOptions()

	if err := applyOptions(config, opts...); err != nil {
		return nil, err
	}

	cache := &LayeredCache{
		memory:     config.memoryAdapter,
		remote:     config.remoteAdapter,
		serializer: config.serializer,

		defaultMemoryTTL: config.defaultMemoryTTL,
		defaultRemoteTTL: config.defaultRemoteTTL,

		defaultCacheNotFound:    config.defaultCacheNotFound,
		defaultCacheNotFoundTTL: config.defaultCacheNotFoundTTL,
	}

	return cache, nil
}

// Set 设置缓存
func (c *LayeredCache) Set(ctx context.Context, key string, value any, opts ...SetOption) error {
	config := newSetOptions()
	if err := applySetOptions(config, opts...); err != nil {
		return err
	}

	data, err := c.Marshal(value)
	if err != nil {
		return err
	}

	memoryTTL, remoteTTL := c.calculateSetTTL(config)

	if c.memory != nil {
		c.memory.Set(key, data, memoryTTL)
	}

	if c.remote != nil {
		if err = c.remote.Set(ctx, key, data, remoteTTL); err != nil {
			return err
		}
	}

	return nil
}

// MSet 批量设置缓存
func (c *LayeredCache) MSet(ctx context.Context, keyValues map[string]any, opts ...SetOption) error {
	config := newSetOptions()
	if err := applySetOptions(config, opts...); err != nil {
		return err
	}

	memoryTTL, remoteTTL := c.calculateSetTTL(config)

	serializedData := make(map[string][]byte)
	for key, value := range keyValues {
		data, err := c.Marshal(value)
		if err != nil {
			return err
		}
		serializedData[key] = data
	}

	// 设置到内存缓存
	if c.memory != nil {
		c.memory.MSet(serializedData, memoryTTL)
	}

	// 设置到Redis缓存
	if c.remote != nil {
		if err := c.remote.MSet(ctx, serializedData, remoteTTL); err != nil {
			return err
		}
	}

	return nil
}

// Delete 删除缓存值
func (c *LayeredCache) Delete(ctx context.Context, key string) error {
	if c.memory != nil {
		c.memory.Delete(key)
	}

	if c.remote != nil {
		if err := c.remote.Delete(ctx, key); err != nil {
			return err
		}
	}

	return nil
}

// Get 获取缓存值
func (c *LayeredCache) Get(ctx context.Context, key string, target any, opts ...GetOption) error {
	// 解析Get选项
	config := newGetOptions()
	if err := applyGetOptions(config, opts...); err != nil {
		return err
	}

	if c.memory != nil {
		if data, exists := c.memory.Get(key); exists {
			if bytes.Equal(data, notFoundPlaceholder) {
				return errors.ErrNotFound
			}
			return c.Unmarshal(data, target)
		}
	}

	if c.remote != nil {
		if data, err := c.remote.Get(ctx, key); err == nil {
			if bytes.Equal(data, notFoundPlaceholder) {
				return errors.ErrNotFound
			}
			// 写回内存缓存
			if c.memory != nil {
				memoryTTL, _ := c.calculateLoaderTTL(config, false)
				c.memory.Set(key, data, memoryTTL)
			}

			return c.Unmarshal(data, target)
		} else if !IsNotFound(err) {
			return err
		}
	}

	if config.loader == nil {
		return errors.ErrNotFound
	}

	result, err, _ := c.sf.Do(key, func() (any, error) {
		return c.loadAndCache(ctx, key, config)
	})

	if err != nil {
		return err
	}

	return c.Unmarshal(result.([]byte), target)
}

// loadAndCache 加载数据并缓存
func (c *LayeredCache) loadAndCache(ctx context.Context, key string, config *getOptions) ([]byte, error) {
	// 调用 loader 获取数据
	value, err := config.loader(ctx, key)
	if err != nil && !IsNotFound(err) {
		return nil, err
	}

	// 检查是否为缺失值（nil 或 error）
	isNotFound := IsNotFound(err) || value == nil
	if isNotFound {
		value = notFoundPlaceholder
	}

	// 判断是否应该缓存缺失值
	cacheNotFound := c.shouldCacheNotFound(config.cacheNotFound)
	if isNotFound && !cacheNotFound {
		return nil, errors.ErrNotFound
	}

	// 序列化并存储到缓存
	data, err := c.Marshal(value)
	if err != nil {
		return nil, err
	}

	// 计算TTL
	memoryTTL, remoteTTL := c.calculateLoaderTTL(config, isNotFound && cacheNotFound)

	if c.memory != nil {
		c.memory.Set(key, data, memoryTTL)
	}

	// 设置到Redis缓存
	if c.remote != nil {
		if err = c.remote.Set(ctx, key, data, remoteTTL); err != nil {
			return nil, err
		}
	}

	if isNotFound {
		return nil, errors.ErrNotFound
	}

	return data, nil
}

// MGet 批量获取缓存值
// target 必须是指向 map[string]T 的指针，例如 &map[string]User{}
func (c *LayeredCache) MGet(ctx context.Context, keys []string, target any, opts ...GetOption) error {
	// 解析Get选项
	config := newGetOptions()
	if err := applyGetOptions(config, opts...); err != nil {
		return err
	}

	if len(keys) == 0 {
		return nil
	}

	// 验证 target 类型
	if err := c.validateMGetTarget(target); err != nil {
		return err
	}

	result := make(map[string][]byte)
	missingKeys := make([]string, 0, len(keys))

	// 从内存缓存中批量获取
	if c.memory != nil {
		memoryData := c.memory.MGet(keys)
		for _, key := range keys {
			if data, exists := memoryData[key]; exists {
				if bytes.Equal(data, notFoundPlaceholder) {
					continue
				}

				result[key] = data
			} else {
				missingKeys = append(missingKeys, key)
			}
		}
	} else {
		missingKeys = keys
	}

	// 批量获取没有命中内存缓存的键
	if c.remote != nil && len(missingKeys) > 0 {
		redisData, err := c.remote.MGet(ctx, missingKeys)
		if err != nil && !IsNotFound(err) {
			return err
		}

		writeBackData := make(map[string][]byte)
		remainingKeys := make([]string, 0, len(missingKeys))

		for _, key := range missingKeys {
			if data, exists := redisData[key]; exists {
				if bytes.Equal(data, notFoundPlaceholder) {
					continue
				}

				result[key] = data

				if c.memory != nil {
					writeBackData[key] = data
				}
			} else {
				remainingKeys = append(remainingKeys, key)
			}
		}

		// 批量写回内存缓存
		if c.memory != nil && len(writeBackData) > 0 {
			memoryTTL, _ := c.calculateLoaderTTL(config, false)
			c.memory.MSet(writeBackData, memoryTTL)
		}

		missingKeys = remainingKeys
	}

	// 使用 batchLoader 加载剩余的键
	if len(missingKeys) > 0 && config.batchLoader != nil {
		batchKey := c.buildBatchKey(missingKeys)
		batchResult, err, _ := c.sf.Do(batchKey, func() (any, error) {
			return c.batchLoadAndCache(ctx, missingKeys, config)
		})

		if err != nil {
			return err
		}

		loadedData := batchResult.(map[string][]byte)
		for key, data := range loadedData {
			result[key] = data
		}
	}

	if len(result) == 0 {
		return nil
	}

	return c.unmarshalBatch(result, target)
}

// validateMGetTarget 验证 MGet 的 target 参数类型
func (c *LayeredCache) validateMGetTarget(target any) error {
	if target == nil {
		return errors.ErrInvalidMGetTarget
	}

	// 检查是否为指针
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return errors.ErrInvalidMGetTarget
	}

	// 检查指针指向的是否为 map
	elemType := targetValue.Elem().Type()
	if elemType.Kind() != reflect.Map {
		return errors.ErrInvalidMGetTarget
	}

	// 检查 map 的 key 类型是否为 string
	if elemType.Key().Kind() != reflect.String {
		return errors.ErrInvalidMGetTarget
	}

	return nil
}

// buildBatchKey 构建批量操作的 singleflight key
func (c *LayeredCache) buildBatchKey(keys []string) string {
	if len(keys) == 0 {
		return "batch:"
	}
	if len(keys) == 1 {
		return "batch:" + keys[0]
	}

	// 计算总长度
	totalLen := 6 // "batch:" 的长度
	for _, key := range keys {
		totalLen += len(key) + 1 // +1 for comma
	}

	// 构建字符串
	result := make([]byte, totalLen-1) // -1 因为最后一个键后面没有逗号
	copy(result, "batch:")
	pos := 6

	for i, key := range keys {
		if i > 0 {
			result[pos] = ','
			pos++
		}
		copy(result[pos:], key)
		pos += len(key)
	}

	return string(result)
}

// unmarshalBatch 批量反序列化结果到 target
func (c *LayeredCache) unmarshalBatch(data map[string][]byte, target any) error {
	targetValue := reflect.ValueOf(target).Elem()
	targetType := targetValue.Type()
	valueType := targetType.Elem()

	// 创建新的 map
	newMap := reflect.MakeMap(targetType)

	// 处理每个键值对
	for key, value := range data {
		// 创建值类型的新实例
		newValue := reflect.New(valueType)

		// 反序列化
		if err := c.Unmarshal(value, newValue.Interface()); err != nil {
			return err
		}

		// 设置到 map 中
		newMap.SetMapIndex(reflect.ValueOf(key), newValue.Elem())
	}

	// 设置结果
	targetValue.Set(newMap)
	return nil
}

// batchLoadAndCache 批量加载数据并缓存
func (c *LayeredCache) batchLoadAndCache(ctx context.Context, keys []string, config *getOptions) (map[string][]byte, error) {
	// 调用 batchLoader 获取数据
	values, err := config.batchLoader(ctx, keys)
	if err != nil && !IsNotFound(err) {
		return nil, err
	}

	result := make(map[string][]byte)      // 正常值
	missingData := make(map[string][]byte) // 缺失值

	cacheNotFound := c.shouldCacheNotFound(config.cacheNotFound)

	// 处理每个键值对
	for _, key := range keys {
		value, exists := values[key]

		// 检查是否为缺失值（nil 或不存在）
		isNotFound := !exists || value == nil

		// 判断是否应该缓存缺失值
		if isNotFound && !cacheNotFound {
			continue
		}

		if isNotFound {
			missingData[key] = notFoundPlaceholder
			continue
		}

		// 序列化并存储到缓存
		var data []byte
		data, err = c.Marshal(value)
		if err != nil {
			return nil, err
		}
		result[key] = data
	}

	// 写入正常值缓存
	if len(result) > 0 {
		// 计算正常值的TTL
		memoryTTL, remoteTTL := c.calculateLoaderTTL(config, false)

		// 设置到内存缓存
		if c.memory != nil {
			c.memory.MSet(result, memoryTTL)
		}

		// 设置到Redis缓存
		if c.remote != nil {
			if err = c.remote.MSet(ctx, result, remoteTTL); err != nil {
				return nil, err
			}
		}
	}

	// 写入缺失值缓存
	if len(missingData) > 0 {
		// 计算缺失值的TTL
		memoryTTL, remoteTTL := c.calculateLoaderTTL(config, true)

		// 设置到内存缓存
		if c.memory != nil {
			c.memory.MSet(missingData, memoryTTL)
		}

		// 设置到Redis缓存
		if c.remote != nil {
			if err = c.remote.MSet(ctx, missingData, remoteTTL); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

// calculateLoaderTTL 计算内存和Redis缓存的TTL
func (c *LayeredCache) calculateLoaderTTL(config *getOptions, isNotFound bool) (memoryTTL, remoteTTL time.Duration) {
	if isNotFound {
		cacheNotFoundTTL := c.defaultCacheNotFoundTTL
		if config.cacheNotFoundTTL != nil {
			cacheNotFoundTTL = *config.cacheNotFoundTTL
		}
		return cacheNotFoundTTL, cacheNotFoundTTL
	}

	memoryTTL = c.defaultMemoryTTL
	if config.memoryTTL != nil {
		memoryTTL = *config.memoryTTL
	}

	remoteTTL = c.defaultRemoteTTL
	if config.remoteTTL != nil {
		remoteTTL = *config.remoteTTL
	}

	return memoryTTL, remoteTTL
}

// calculateSetTTL 计算Set操作的TTL
func (c *LayeredCache) calculateSetTTL(config *setOptions) (memoryTTL, remoteTTL time.Duration) {
	memoryTTL = c.defaultMemoryTTL
	if config.memoryTTL != nil {
		memoryTTL = *config.memoryTTL
	}

	remoteTTL = c.defaultRemoteTTL
	if config.remoteTTL != nil {
		remoteTTL = *config.remoteTTL
	}

	return memoryTTL, remoteTTL
}

// shouldCacheNotFound 判断是否应该缓存缺失值
func (c *LayeredCache) shouldCacheNotFound(optCacheNotFound *bool) bool {
	if optCacheNotFound != nil {
		return *optCacheNotFound
	}
	return c.defaultCacheNotFound
}

func (c *LayeredCache) Marshal(val any) ([]byte, error) {
	switch v := val.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	}

	return c.serializer.Marshal(val)
}

func (c *LayeredCache) Unmarshal(b []byte, val any) error {
	if len(b) == 0 {
		return nil
	}

	switch v := val.(type) {
	case *[]byte:
		clone := make([]byte, len(b))
		copy(clone, b)
		*v = clone
		return nil
	case *string:
		*v = string(b)
		return nil
	}

	return c.serializer.Unmarshal(b, val)
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, errors.ErrNotFound)
}

func validMemoryTTL(memoryTTL time.Duration) error {
	if memoryTTL <= 0 {
		return errors.ErrInvalidMemoryExpireTime
	}
	return nil
}

func validRemoteTTL(remoteTTL time.Duration) error {
	if remoteTTL <= 0 {
		return errors.ErrInvalidRedisExpireTime
	}
	return nil
}

func validCacheMissTTL(cacheMissTTL time.Duration) error {
	if cacheMissTTL <= 0 {
		return errors.ErrInvalidCacheNotFondTTL
	}
	return nil
}
