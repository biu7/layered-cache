package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// 测试用的结构体
type TestProduct struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

type TestOrder struct {
	ID       int           `json:"id"`
	Products []TestProduct `json:"products"`
	Total    float64       `json:"total"`
}

// 辅助函数：创建测试用的缓存实例
func createTestCache(t *testing.T) Cache {
	t.Helper()

	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	return cache
}

func createMemoryOnlyCache(t *testing.T) Cache {
	t.Helper()

	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
	)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	return cache
}

func createRedisOnlyCache(t *testing.T) Cache {
	t.Helper()

	cache, err := NewCache(
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	return cache
}

func TestTyped(t *testing.T) {
	t.Run("创建TypedCache - 字符串类型", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string](cache)

		assert.NotNil(t, typedCache)
		assert.NotNil(t, typedCache.cache)
	})

	t.Run("创建TypedCache - 整数类型", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[int](cache)

		assert.NotNil(t, typedCache)
		assert.NotNil(t, typedCache.cache)
	})

	t.Run("创建TypedCache - 结构体类型", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[TestProduct](cache)

		assert.NotNil(t, typedCache)
		assert.NotNil(t, typedCache.cache)
	})

	t.Run("创建TypedCache - 切片类型", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[[]TestProduct](cache)

		assert.NotNil(t, typedCache)
		assert.NotNil(t, typedCache.cache)
	})
}

func TestTypedCache_Set(t *testing.T) {
	ctx := context.Background()

	t.Run("设置字符串值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string](cache)

		key := "test-string"
		value := "hello world"

		err := typedCache.Set(ctx, key, value)
		assert.NoError(t, err)

		// 验证值是否正确存储
		var result string
		err = cache.Get(ctx, key, &result)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("设置整数值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[int](cache)

		key := "test-int"
		value := 42

		err := typedCache.Set(ctx, key, value)
		assert.NoError(t, err)

		// 验证值是否正确存储
		var result int
		err = cache.Get(ctx, key, &result)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("设置结构体值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[TestProduct](cache)

		key := "test-product"
		value := TestProduct{
			ID:    1,
			Name:  "Test Product",
			Price: 99.99,
		}

		err := typedCache.Set(ctx, key, value)
		assert.NoError(t, err)

		// 验证值是否正确存储
		var result TestProduct
		err = cache.Get(ctx, key, &result)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("设置切片值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[[]TestProduct](cache)

		key := "test-product"
		value := []TestProduct{
			{
				ID:    1,
				Name:  "Test Product",
				Price: 99.99,
			}, {
				ID:    2,
				Name:  "Test Product2",
				Price: 99.999,
			},
		}

		err := typedCache.Set(ctx, key, value)
		assert.NoError(t, err)

		// 验证值是否正确存储
		result, err := typedCache.Get(ctx, key, nil)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("设置带TTL的值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string](cache)

		key := "test-ttl"
		value := "test value"

		err := typedCache.Set(ctx, key, value, WithTTL(time.Second, 2*time.Second))
		assert.NoError(t, err)

		// 验证值是否正确存储
		var result string
		err = cache.Get(ctx, key, &result)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("设置复杂嵌套结构", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[TestOrder](cache)

		key := "test-order"
		value := TestOrder{
			ID: 1,
			Products: []TestProduct{
				{ID: 1, Name: "Product 1", Price: 10.00},
				{ID: 2, Name: "Product 2", Price: 20.00},
			},
			Total: 30.00,
		}

		err := typedCache.Set(ctx, key, value)
		assert.NoError(t, err)

		// 验证值是否正确存储
		var result TestOrder
		err = cache.Get(ctx, key, &result)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})
}

func TestTypedCache_Get(t *testing.T) {
	ctx := context.Background()

	t.Run("获取存在的值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string](cache)

		key := "test-key"
		expected := "test value"

		// 先设置值
		err := typedCache.Set(ctx, key, expected)
		assert.NoError(t, err)

		// 获取值
		result, err := typedCache.Get(ctx, key, nil)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})

	t.Run("获取不存在的值 - 无loader", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string](cache)

		key := "nonexistent-key"

		result, err := typedCache.Get(ctx, key, nil)
		assert.Error(t, err)
		assert.Empty(t, result)
	})

	t.Run("获取不存在的值 - 有loader", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string](cache)

		key := "nonexistent-key"
		expected := "loaded value"

		loader := func(ctx context.Context, key string) (string, error) {
			return expected, nil
		}

		result, err := typedCache.Get(ctx, key, loader)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)

		// 验证值是否被缓存
		result2, err := typedCache.Get(ctx, key, nil)
		assert.NoError(t, err)
		assert.Equal(t, expected, result2)
	})

	t.Run("获取结构体值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[TestProduct](cache)

		key := "test-product"
		expected := TestProduct{
			ID:    1,
			Name:  "Test Product",
			Price: 99.99,
		}

		// 先设置值
		err := typedCache.Set(ctx, key, expected)
		assert.NoError(t, err)

		// 获取值
		result, err := typedCache.Get(ctx, key, nil)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})

	t.Run("loader返回错误", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string](cache)

		key := "error-key"
		expectedError := errors.New("loader error")

		loader := func(ctx context.Context, key string) (string, error) {
			return "", expectedError
		}

		result, err := typedCache.Get(ctx, key, loader)
		assert.Error(t, err)
		assert.Equal(t, expectedError, err)
		assert.Empty(t, result)
	})

	t.Run("上下文取消", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string](cache)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // 立即取消

		key := "test-key"
		result, err := typedCache.Get(ctx, key, nil)
		assert.Error(t, err)
		assert.Empty(t, result)
	})
}

func TestTypedCache_MSet(t *testing.T) {
	ctx := context.Background()

	t.Run("批量设置字符串值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string](cache)

		keyValues := map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		}

		err := typedCache.MSet(ctx, keyValues)
		assert.NoError(t, err)

		// 验证所有值都被正确存储
		for key, expected := range keyValues {
			result, err := typedCache.Get(ctx, key, nil)
			assert.NoError(t, err)
			assert.Equal(t, expected, result)
		}
	})

	t.Run("批量设置结构体值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[TestProduct](cache)

		keyValues := map[string]TestProduct{
			"product1": {ID: 1, Name: "Product 1", Price: 10.00},
			"product2": {ID: 2, Name: "Product 2", Price: 20.00},
			"product3": {ID: 3, Name: "Product 3", Price: 30.00},
		}

		err := typedCache.MSet(ctx, keyValues)
		assert.NoError(t, err)

		// 验证所有值都被正确存储
		for key, expected := range keyValues {
			result, err := typedCache.Get(ctx, key, nil)
			assert.NoError(t, err)
			assert.Equal(t, expected, result)
		}
	})

	t.Run("批量设置带TTL", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string](cache)

		keyValues := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}

		err := typedCache.MSet(ctx, keyValues, WithTTL(time.Second, 2*time.Second))
		assert.NoError(t, err)

		// 验证所有值都被正确存储
		for key, expected := range keyValues {
			result, err := typedCache.Get(ctx, key, nil)
			assert.NoError(t, err)
			assert.Equal(t, expected, result)
		}
	})

	t.Run("批量设置空map", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string](cache)

		keyValues := map[string]string{}

		err := typedCache.MSet(ctx, keyValues)
		assert.NoError(t, err)
	})
}

func TestTypedCache_MGet(t *testing.T) {
	ctx := context.Background()

	t.Run("批量获取存在的值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string](cache)

		keyValues := map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		}

		// 先设置值
		err := typedCache.MSet(ctx, keyValues)
		assert.NoError(t, err)

		// 批量获取
		keys := []string{"key1", "key2", "key3"}
		result, err := typedCache.MGet(ctx, keys, nil)
		assert.NoError(t, err)
		assert.Equal(t, keyValues, result)
	})

	t.Run("批量获取不存在的值 - 无loader", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string](cache)

		keys := []string{"nonexistent1", "nonexistent2"}
		result, err := typedCache.MGet(ctx, keys, nil)
		assert.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("批量获取不存在的值 - 有loader", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string](cache)

		keys := []string{"key1", "key2", "key3"}
		expected := map[string]string{
			"key1": "loaded value 1",
			"key2": "loaded value 2",
			"key3": "loaded value 3",
		}

		loader := func(ctx context.Context, keys []string) (map[string]string, error) {
			result := make(map[string]string)
			for _, key := range keys {
				result[key] = expected[key]
			}
			return result, nil
		}

		result, err := typedCache.MGet(ctx, keys, loader)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)

		// 验证值是否被缓存
		result2, err := typedCache.MGet(ctx, keys, nil)
		assert.NoError(t, err)
		assert.Equal(t, expected, result2)
	})

	t.Run("批量获取结构体值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[TestProduct](cache)

		keyValues := map[string]TestProduct{
			"product1": {ID: 1, Name: "Product 1", Price: 10.00},
			"product2": {ID: 2, Name: "Product 2", Price: 20.00},
		}

		// 先设置值
		err := typedCache.MSet(ctx, keyValues)
		assert.NoError(t, err)

		// 批量获取
		keys := []string{"product1", "product2"}
		result, err := typedCache.MGet(ctx, keys, nil)
		assert.NoError(t, err)
		assert.Equal(t, keyValues, result)
	})

	t.Run("批量获取部分命中", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string](cache)

		// 先设置部分值
		err := typedCache.Set(ctx, "key1", "value1")
		assert.NoError(t, err)

		keys := []string{"key1", "key2"}
		expected := map[string]string{
			"key1": "value1",
			"key2": "loaded value 2",
		}

		loader := func(ctx context.Context, keys []string) (map[string]string, error) {
			result := make(map[string]string)
			for _, key := range keys {
				if key == "key2" {
					result[key] = "loaded value 2"
				}
			}
			return result, nil
		}

		result, err := typedCache.MGet(ctx, keys, loader)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})

	t.Run("loader返回错误", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string](cache)

		keys := []string{"key1", "key2"}
		expectedError := errors.New("loader error")

		loader := func(ctx context.Context, keys []string) (map[string]string, error) {
			return nil, expectedError
		}

		result, err := typedCache.MGet(ctx, keys, loader)
		assert.Error(t, err)
		assert.Equal(t, expectedError, err)
		assert.Empty(t, result)
	})
}

func TestTypedCache_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("删除存在的值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string](cache)

		key := "test-key"
		value := "test value"

		// 先设置值
		err := typedCache.Set(ctx, key, value)
		assert.NoError(t, err)

		// 验证值存在
		result, err := typedCache.Get(ctx, key, nil)
		assert.NoError(t, err)
		assert.Equal(t, value, result)

		// 删除值
		err = typedCache.Delete(ctx, key)
		assert.NoError(t, err)

		// 验证值不存在
		result, err = typedCache.Get(ctx, key, nil)
		assert.Error(t, err)
		assert.Empty(t, result)
	})

	t.Run("删除不存在的值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string](cache)

		key := "nonexistent-key"

		// 删除不存在的值应该成功
		err := typedCache.Delete(ctx, key)
		assert.NoError(t, err)
	})

	t.Run("删除结构体值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[TestProduct](cache)

		key := "test-product"
		value := TestProduct{
			ID:    1,
			Name:  "Test Product",
			Price: 99.99,
		}

		// 先设置值
		err := typedCache.Set(ctx, key, value)
		assert.NoError(t, err)

		// 删除值
		err = typedCache.Delete(ctx, key)
		assert.NoError(t, err)

		// 验证值不存在
		result, err := typedCache.Get(ctx, key, nil)
		assert.Error(t, err)
		assert.Equal(t, TestProduct{}, result)
	})

	t.Run("上下文取消", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string](cache)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // 立即取消

		key := "test-key"
		err := typedCache.Delete(ctx, key)
		assert.Error(t, err)
	})
}

func TestTypedCache_MemoryOnly(t *testing.T) {
	ctx := context.Background()

	t.Run("内存缓存基本操作", func(t *testing.T) {
		cache := createMemoryOnlyCache(t)
		typedCache := Typed[string](cache)

		key := "memory-key"
		value := "memory value"

		// 设置
		err := typedCache.Set(ctx, key, value)
		assert.NoError(t, err)

		// 获取
		result, err := typedCache.Get(ctx, key, nil)
		assert.NoError(t, err)
		assert.Equal(t, value, result)

		// 删除
		err = typedCache.Delete(ctx, key)
		assert.NoError(t, err)

		// 验证删除
		result, err = typedCache.Get(ctx, key, nil)
		assert.Error(t, err)
		assert.Empty(t, result)
	})
}

func TestTypedCache_RedisOnly(t *testing.T) {
	ctx := context.Background()

	t.Run("Redis缓存基本操作", func(t *testing.T) {
		cache := createRedisOnlyCache(t)
		typedCache := Typed[string](cache)

		key := "redis-key"
		value := "redis value"

		// 设置
		err := typedCache.Set(ctx, key, value)
		assert.NoError(t, err)

		// 获取
		result, err := typedCache.Get(ctx, key, nil)
		assert.NoError(t, err)
		assert.Equal(t, value, result)

		// 删除
		err = typedCache.Delete(ctx, key)
		assert.NoError(t, err)

		// 验证删除
		result, err = typedCache.Get(ctx, key, nil)
		assert.Error(t, err)
		assert.Empty(t, result)
	})
}

func TestTypedCache_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	cache := createTestCache(t)
	typedCache := Typed[string](cache)

	const numGoroutines = 100
	const numOperations = 10

	t.Run("并发读写", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()

				for j := 0; j < numOperations; j++ {
					key := fmt.Sprintf("concurrent-key-%d-%d", id, j)
					value := fmt.Sprintf("concurrent-value-%d-%d", id, j)

					// 设置
					err := typedCache.Set(ctx, key, value)
					assert.NoError(t, err)

					// 获取
					result, err := typedCache.Get(ctx, key, nil)
					assert.NoError(t, err)
					assert.Equal(t, value, result)

					// 删除
					err = typedCache.Delete(ctx, key)
					assert.NoError(t, err)
				}
			}(i)
		}

		wg.Wait()
	})
}

func TestTypedCache_DifferentTypes(t *testing.T) {
	ctx := context.Background()
	cache := createTestCache(t)

	t.Run("不同类型的TypedCache", func(t *testing.T) {
		stringCache := Typed[string](cache)
		intCache := Typed[int](cache)
		productCache := Typed[TestProduct](cache)

		// 字符串缓存
		err := stringCache.Set(ctx, "string-key", "string value")
		assert.NoError(t, err)

		// 整数缓存
		err = intCache.Set(ctx, "int-key", 42)
		assert.NoError(t, err)

		// 结构体缓存
		product := TestProduct{ID: 1, Name: "Test", Price: 99.99}
		err = productCache.Set(ctx, "product-key", product)
		assert.NoError(t, err)

		// 验证获取
		str, err := stringCache.Get(ctx, "string-key", nil)
		assert.NoError(t, err)
		assert.Equal(t, "string value", str)

		num, err := intCache.Get(ctx, "int-key", nil)
		assert.NoError(t, err)
		assert.Equal(t, 42, num)

		prod, err := productCache.Get(ctx, "product-key", nil)
		assert.NoError(t, err)
		assert.Equal(t, product, prod)
	})
}

func TestTypedCache_EdgeCases(t *testing.T) {
	ctx := context.Background()
	cache := createTestCache(t)

	t.Run("空字符串值", func(t *testing.T) {
		typedCache := Typed[string](cache)
		key := "empty-string-key"
		value := ""

		err := typedCache.Set(ctx, key, value)
		assert.NoError(t, err)

		result, err := typedCache.Get(ctx, key, nil)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("零值结构体", func(t *testing.T) {
		typedCache := Typed[TestProduct](cache)
		key := "zero-struct-key"
		value := TestProduct{} // 零值

		err := typedCache.Set(ctx, key, value)
		assert.NoError(t, err)

		result, err := typedCache.Get(ctx, key, nil)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("nil slice", func(t *testing.T) {
		typedCache := Typed[[]string](cache)
		key := "nil-slice-key"
		var value []string // nil slice

		err := typedCache.Set(ctx, key, value)
		assert.NoError(t, err)

		result, err := typedCache.Get(ctx, key, nil)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("nil map", func(t *testing.T) {
		typedCache := Typed[map[string]int](cache)
		key := "nil-map-key"
		var value map[string]int // nil map

		err := typedCache.Set(ctx, key, value)
		assert.NoError(t, err)

		result, err := typedCache.Get(ctx, key, nil)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})
}
