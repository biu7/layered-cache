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
	t.Run("创建TypedCache - string ID类型", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, string](cache)

		assert.NotNil(t, typedCache)
		assert.NotNil(t, typedCache.cache)
	})

	t.Run("创建TypedCache - int ID类型", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[int, string](cache)

		assert.NotNil(t, typedCache)
		assert.NotNil(t, typedCache.cache)
	})

	t.Run("创建TypedCache - 结构体值类型", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[int, TestProduct](cache)

		assert.NotNil(t, typedCache)
		assert.NotNil(t, typedCache.cache)
	})

	t.Run("创建TypedCache - 切片值类型", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, []TestProduct](cache)

		assert.NotNil(t, typedCache)
		assert.NotNil(t, typedCache.cache)
	})
}

func TestTypedCache_KeyBuilding(t *testing.T) {
	ctx := context.Background()
	cache := createTestCache(t)

	t.Run("string ID key building", func(t *testing.T) {
		typedCache := Typed[string, string](cache)

		// 通过Set和底层缓存验证key构建是否正确
		err := typedCache.Set(ctx, "user", "john", "John Doe")
		assert.NoError(t, err)

		// 验证底层缓存中的key格式
		var result string
		err = cache.Get(ctx, "user:john", &result)
		assert.NoError(t, err)
		assert.Equal(t, "John Doe", result)
	})

	t.Run("int ID key building", func(t *testing.T) {
		typedCache := Typed[int, string](cache)

		err := typedCache.Set(ctx, "user", 123, "User 123")
		assert.NoError(t, err)

		var result string
		err = cache.Get(ctx, "user:123", &result)
		assert.NoError(t, err)
		assert.Equal(t, "User 123", result)
	})

	t.Run("int32 ID key building", func(t *testing.T) {
		typedCache := Typed[int32, string](cache)

		err := typedCache.Set(ctx, "user", int32(456), "User 456")
		assert.NoError(t, err)

		var result string
		err = cache.Get(ctx, "user:456", &result)
		assert.NoError(t, err)
		assert.Equal(t, "User 456", result)
	})

	t.Run("int64 ID key building", func(t *testing.T) {
		typedCache := Typed[int64, string](cache)

		err := typedCache.Set(ctx, "user", int64(789), "User 789")
		assert.NoError(t, err)

		var result string
		err = cache.Get(ctx, "user:789", &result)
		assert.NoError(t, err)
		assert.Equal(t, "User 789", result)
	})

	t.Run("custom comparable type key building", func(t *testing.T) {
		type UserID int
		typedCache := Typed[UserID, string](cache)

		err := typedCache.Set(ctx, "user", UserID(999), "User 999")
		assert.NoError(t, err)

		var result string
		err = cache.Get(ctx, "user:999", &result)
		assert.NoError(t, err)
		assert.Equal(t, "User 999", result)
	})
}

func TestTypedCache_Set(t *testing.T) {
	ctx := context.Background()

	t.Run("设置字符串值 - string ID", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, string](cache)

		keyPrefix := "test"
		id := "user123"
		value := "hello world"

		err := typedCache.Set(ctx, keyPrefix, id, value)
		assert.NoError(t, err)

		// 验证值是否正确存储
		var result string
		err = cache.Get(ctx, "test:user123", &result)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("设置字符串值 - int ID", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[int, string](cache)

		keyPrefix := "user"
		id := 42
		value := "john doe"

		err := typedCache.Set(ctx, keyPrefix, id, value)
		assert.NoError(t, err)

		// 验证值是否正确存储
		var result string
		err = cache.Get(ctx, "user:42", &result)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("设置结构体值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[int, TestProduct](cache)

		keyPrefix := "product"
		id := 1
		value := TestProduct{
			ID:    1,
			Name:  "Test Product",
			Price: 99.99,
		}

		err := typedCache.Set(ctx, keyPrefix, id, value)
		assert.NoError(t, err)

		// 验证值是否正确存储
		var result TestProduct
		err = cache.Get(ctx, "product:1", &result)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("设置切片值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, []TestProduct](cache)

		keyPrefix := "products"
		id := "category1"
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

		err := typedCache.Set(ctx, keyPrefix, id, value)
		assert.NoError(t, err)

		// 验证值是否正确存储
		result, err := typedCache.Get(ctx, keyPrefix, id, nil)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("设置带TTL的值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, string](cache)

		keyPrefix := "temp"
		id := "session1"
		value := "test value"

		err := typedCache.Set(ctx, keyPrefix, id, value, WithTTL(time.Second, 2*time.Second))
		assert.NoError(t, err)

		// 验证值是否正确存储
		var result string
		err = cache.Get(ctx, "temp:session1", &result)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("设置复杂嵌套结构", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[int, TestOrder](cache)

		keyPrefix := "order"
		id := 1
		value := TestOrder{
			ID: 1,
			Products: []TestProduct{
				{ID: 1, Name: "Product 1", Price: 10.00},
				{ID: 2, Name: "Product 2", Price: 20.00},
			},
			Total: 30.00,
		}

		err := typedCache.Set(ctx, keyPrefix, id, value)
		assert.NoError(t, err)

		// 验证值是否正确存储
		var result TestOrder
		err = cache.Get(ctx, "order:1", &result)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})
}

func TestTypedCache_Get(t *testing.T) {
	ctx := context.Background()

	t.Run("获取存在的值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, string](cache)

		keyPrefix := "test"
		id := "key1"
		expected := "test value"

		// 先设置值
		err := typedCache.Set(ctx, keyPrefix, id, expected)
		assert.NoError(t, err)

		// 获取值
		result, err := typedCache.Get(ctx, keyPrefix, id, nil)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})

	t.Run("获取不存在的值 - 无loader", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, string](cache)

		keyPrefix := "test"
		id := "nonexistent"

		result, err := typedCache.Get(ctx, keyPrefix, id, nil)
		assert.Error(t, err)
		assert.Empty(t, result)
	})

	t.Run("获取不存在的值 - 有loader", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, string](cache)

		keyPrefix := "test"
		id := "nonexistent"
		expected := "loaded value"

		loader := func(ctx context.Context, id string) (string, error) {
			return expected, nil
		}

		result, err := typedCache.Get(ctx, keyPrefix, id, loader)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)

		// 验证值是否被缓存
		result2, err := typedCache.Get(ctx, keyPrefix, id, nil)
		assert.NoError(t, err)
		assert.Equal(t, expected, result2)
	})

	t.Run("获取结构体值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[int, TestProduct](cache)

		keyPrefix := "product"
		id := 1
		expected := TestProduct{
			ID:    1,
			Name:  "Test Product",
			Price: 99.99,
		}

		// 先设置值
		err := typedCache.Set(ctx, keyPrefix, id, expected)
		assert.NoError(t, err)

		// 获取值
		result, err := typedCache.Get(ctx, keyPrefix, id, nil)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})

	t.Run("loader返回错误", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, string](cache)

		keyPrefix := "test"
		id := "error"
		expectedError := errors.New("loader error")

		loader := func(ctx context.Context, id string) (string, error) {
			return "", expectedError
		}

		result, err := typedCache.Get(ctx, keyPrefix, id, loader)
		assert.Error(t, err)
		assert.Equal(t, expectedError, err)
		assert.Empty(t, result)
	})

	t.Run("上下文取消", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, string](cache)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // 立即取消

		keyPrefix := "test"
		id := "key1"
		result, err := typedCache.Get(ctx, keyPrefix, id, nil)
		assert.Error(t, err)
		assert.Empty(t, result)
	})
}

func TestTypedCache_MSet(t *testing.T) {
	ctx := context.Background()

	t.Run("批量设置字符串值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, string](cache)

		keyPrefix := "test"
		values := map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		}

		err := typedCache.MSet(ctx, keyPrefix, values)
		assert.NoError(t, err)

		// 验证所有值都被正确存储
		for id, expected := range values {
			result, err := typedCache.Get(ctx, keyPrefix, id, nil)
			assert.NoError(t, err)
			assert.Equal(t, expected, result)
		}
	})

	t.Run("批量设置结构体值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[int, TestProduct](cache)

		keyPrefix := "product"
		values := map[int]TestProduct{
			1: {ID: 1, Name: "Product 1", Price: 10.00},
			2: {ID: 2, Name: "Product 2", Price: 20.00},
			3: {ID: 3, Name: "Product 3", Price: 30.00},
		}

		err := typedCache.MSet(ctx, keyPrefix, values)
		assert.NoError(t, err)

		// 验证所有值都被正确存储
		for id, expected := range values {
			result, err := typedCache.Get(ctx, keyPrefix, id, nil)
			assert.NoError(t, err)
			assert.Equal(t, expected, result)
		}
	})

	t.Run("批量设置带TTL", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, string](cache)

		keyPrefix := "temp"
		values := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}

		err := typedCache.MSet(ctx, keyPrefix, values, WithTTL(time.Second, 2*time.Second))
		assert.NoError(t, err)

		// 验证所有值都被正确存储
		for id, expected := range values {
			result, err := typedCache.Get(ctx, keyPrefix, id, nil)
			assert.NoError(t, err)
			assert.Equal(t, expected, result)
		}
	})

	t.Run("批量设置空map", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, string](cache)

		keyPrefix := "test"
		values := map[string]string{}

		err := typedCache.MSet(ctx, keyPrefix, values)
		assert.NoError(t, err)
	})
}

func TestTypedCache_MGet(t *testing.T) {
	ctx := context.Background()

	t.Run("批量获取存在的值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, string](cache)

		keyPrefix := "test"
		values := map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		}

		// 先设置值
		err := typedCache.MSet(ctx, keyPrefix, values)
		assert.NoError(t, err)

		// 批量获取
		ids := []string{"key1", "key2", "key3"}
		result, err := typedCache.MGet(ctx, keyPrefix, ids, nil)
		assert.NoError(t, err)
		assert.Equal(t, values, result)
	})

	t.Run("批量获取不存在的值 - 无loader", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, string](cache)

		keyPrefix := "test"
		ids := []string{"nonexistent1", "nonexistent2"}
		result, err := typedCache.MGet(ctx, keyPrefix, ids, nil)
		assert.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("批量获取不存在的值 - 有loader", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, string](cache)

		keyPrefix := "test"
		ids := []string{"key1", "key2", "key3"}
		expected := map[string]string{
			"key1": "loaded value 1",
			"key2": "loaded value 2",
			"key3": "loaded value 3",
		}

		loader := func(ctx context.Context, ids []string) (map[string]string, error) {
			result := make(map[string]string)
			for _, id := range ids {
				result[id] = expected[id]
			}
			return result, nil
		}

		result, err := typedCache.MGet(ctx, keyPrefix, ids, loader)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)

		// 验证值是否被缓存
		result2, err := typedCache.MGet(ctx, keyPrefix, ids, nil)
		assert.NoError(t, err)
		assert.Equal(t, expected, result2)
	})

	t.Run("批量获取结构体值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[int, TestProduct](cache)

		keyPrefix := "product"
		values := map[int]TestProduct{
			1: {ID: 1, Name: "Product 1", Price: 10.00},
			2: {ID: 2, Name: "Product 2", Price: 20.00},
		}

		// 先设置值
		err := typedCache.MSet(ctx, keyPrefix, values)
		assert.NoError(t, err)

		// 批量获取
		ids := []int{1, 2}
		result, err := typedCache.MGet(ctx, keyPrefix, ids, nil)
		assert.NoError(t, err)
		assert.Equal(t, values, result)
	})

	t.Run("批量获取部分命中", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, string](cache)

		keyPrefix := "test"
		// 先设置部分值
		err := typedCache.Set(ctx, keyPrefix, "key1", "value1")
		assert.NoError(t, err)

		ids := []string{"key1", "key2"}
		expected := map[string]string{
			"key1": "value1",
			"key2": "loaded value 2",
		}

		loader := func(ctx context.Context, ids []string) (map[string]string, error) {
			result := make(map[string]string)
			for _, id := range ids {
				if id == "key2" {
					result[id] = "loaded value 2"
				}
			}
			return result, nil
		}

		result, err := typedCache.MGet(ctx, keyPrefix, ids, loader)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})

	t.Run("loader返回错误", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, string](cache)

		keyPrefix := "test"
		ids := []string{"key1", "key2"}
		expectedError := errors.New("loader error")

		loader := func(ctx context.Context, ids []string) (map[string]string, error) {
			return nil, expectedError
		}

		result, err := typedCache.MGet(ctx, keyPrefix, ids, loader)
		assert.Error(t, err)
		assert.Equal(t, expectedError, err)
		assert.Empty(t, result)
	})
}

func TestTypedCache_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("删除存在的值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, string](cache)

		keyPrefix := "test"
		id := "key1"
		value := "test value"

		// 先设置值
		err := typedCache.Set(ctx, keyPrefix, id, value)
		assert.NoError(t, err)

		// 验证值存在
		result, err := typedCache.Get(ctx, keyPrefix, id, nil)
		assert.NoError(t, err)
		assert.Equal(t, value, result)

		// 删除值
		err = typedCache.Delete(ctx, keyPrefix, id)
		assert.NoError(t, err)

		// 验证值不存在
		result, err = typedCache.Get(ctx, keyPrefix, id, nil)
		assert.Error(t, err)
		assert.Empty(t, result)
	})

	t.Run("删除不存在的值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, string](cache)

		keyPrefix := "test"
		id := "nonexistent"

		// 删除不存在的值应该成功
		err := typedCache.Delete(ctx, keyPrefix, id)
		assert.NoError(t, err)
	})

	t.Run("删除结构体值", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[int, TestProduct](cache)

		keyPrefix := "product"
		id := 1
		value := TestProduct{
			ID:    1,
			Name:  "Test Product",
			Price: 99.99,
		}

		// 先设置值
		err := typedCache.Set(ctx, keyPrefix, id, value)
		assert.NoError(t, err)

		// 删除值
		err = typedCache.Delete(ctx, keyPrefix, id)
		assert.NoError(t, err)

		// 验证值不存在
		result, err := typedCache.Get(ctx, keyPrefix, id, nil)
		assert.Error(t, err)
		assert.Equal(t, TestProduct{}, result)
	})

	t.Run("上下文取消", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[string, string](cache)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // 立即取消

		keyPrefix := "test"
		id := "key1"
		err := typedCache.Delete(ctx, keyPrefix, id)
		assert.Error(t, err)
	})
}

func TestTypedCache_MemoryOnly(t *testing.T) {
	ctx := context.Background()

	t.Run("内存缓存基本操作", func(t *testing.T) {
		cache := createMemoryOnlyCache(t)
		typedCache := Typed[string, string](cache)

		keyPrefix := "memory"
		id := "key1"
		value := "memory value"

		// 设置
		err := typedCache.Set(ctx, keyPrefix, id, value)
		assert.NoError(t, err)

		// 获取
		result, err := typedCache.Get(ctx, keyPrefix, id, nil)
		assert.NoError(t, err)
		assert.Equal(t, value, result)

		// 删除
		err = typedCache.Delete(ctx, keyPrefix, id)
		assert.NoError(t, err)

		// 验证删除
		result, err = typedCache.Get(ctx, keyPrefix, id, nil)
		assert.Error(t, err)
		assert.Empty(t, result)
	})
}

func TestTypedCache_RedisOnly(t *testing.T) {
	ctx := context.Background()

	t.Run("Redis缓存基本操作", func(t *testing.T) {
		cache := createRedisOnlyCache(t)
		typedCache := Typed[string, string](cache)

		keyPrefix := "redis"
		id := "key1"
		value := "redis value"

		// 设置
		err := typedCache.Set(ctx, keyPrefix, id, value)
		assert.NoError(t, err)

		// 获取
		result, err := typedCache.Get(ctx, keyPrefix, id, nil)
		assert.NoError(t, err)
		assert.Equal(t, value, result)

		// 删除
		err = typedCache.Delete(ctx, keyPrefix, id)
		assert.NoError(t, err)

		// 验证删除
		result, err = typedCache.Get(ctx, keyPrefix, id, nil)
		assert.Error(t, err)
		assert.Empty(t, result)
	})
}

func TestTypedCache_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	cache := createTestCache(t)
	typedCache := Typed[string, string](cache)

	const numGoroutines = 100
	const numOperations = 10

	t.Run("并发读写", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()

				for j := 0; j < numOperations; j++ {
					keyPrefix := fmt.Sprintf("concurrent-%d", id)
					keyID := fmt.Sprintf("key-%d", j)
					value := fmt.Sprintf("value-%d-%d", id, j)

					// 设置
					err := typedCache.Set(ctx, keyPrefix, keyID, value)
					assert.NoError(t, err)

					// 获取
					result, err := typedCache.Get(ctx, keyPrefix, keyID, nil)
					assert.NoError(t, err)
					assert.Equal(t, value, result)

					// 删除
					err = typedCache.Delete(ctx, keyPrefix, keyID)
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
		stringCache := Typed[string, string](cache)
		intCache := Typed[int, string](cache)
		productCache := Typed[int, TestProduct](cache)

		// 字符串ID缓存
		err := stringCache.Set(ctx, "user", "john", "John Doe")
		assert.NoError(t, err)

		// 整数ID缓存
		err = intCache.Set(ctx, "user", 42, "User 42")
		assert.NoError(t, err)

		// 结构体值缓存
		product := TestProduct{ID: 1, Name: "Test", Price: 99.99}
		err = productCache.Set(ctx, "product", 1, product)
		assert.NoError(t, err)

		// 验证获取
		str, err := stringCache.Get(ctx, "user", "john", nil)
		assert.NoError(t, err)
		assert.Equal(t, "John Doe", str)

		userStr, err := intCache.Get(ctx, "user", 42, nil)
		assert.NoError(t, err)
		assert.Equal(t, "User 42", userStr)

		prod, err := productCache.Get(ctx, "product", 1, nil)
		assert.NoError(t, err)
		assert.Equal(t, product, prod)
	})
}

func TestTypedCache_EdgeCases(t *testing.T) {
	ctx := context.Background()
	cache := createTestCache(t)

	t.Run("空字符串值", func(t *testing.T) {
		typedCache := Typed[string, string](cache)
		keyPrefix := "empty"
		id := "key1"
		value := ""

		err := typedCache.Set(ctx, keyPrefix, id, value)
		assert.NoError(t, err)

		result, err := typedCache.Get(ctx, keyPrefix, id, nil)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("零值结构体", func(t *testing.T) {
		typedCache := Typed[int, TestProduct](cache)
		keyPrefix := "product"
		id := 0
		value := TestProduct{} // 零值

		err := typedCache.Set(ctx, keyPrefix, id, value)
		assert.NoError(t, err)

		result, err := typedCache.Get(ctx, keyPrefix, id, nil)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("nil slice", func(t *testing.T) {
		typedCache := Typed[string, []string](cache)
		keyPrefix := "list"
		id := "key1"
		var value []string // nil slice

		err := typedCache.Set(ctx, keyPrefix, id, value)
		assert.NoError(t, err)

		result, err := typedCache.Get(ctx, keyPrefix, id, nil)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("nil map", func(t *testing.T) {
		typedCache := Typed[string, map[string]int](cache)
		keyPrefix := "map"
		id := "key1"
		var value map[string]int // nil map

		err := typedCache.Set(ctx, keyPrefix, id, value)
		assert.NoError(t, err)

		result, err := typedCache.Get(ctx, keyPrefix, id, nil)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("不同ID类型的key构建", func(t *testing.T) {
		// 测试int32 ID
		int32Cache := Typed[int32, string](cache)
		err := int32Cache.Set(ctx, "test", int32(123), "int32 value")
		assert.NoError(t, err)

		result, err := int32Cache.Get(ctx, "test", int32(123), nil)
		assert.NoError(t, err)
		assert.Equal(t, "int32 value", result)

		// 测试int64 ID
		int64Cache := Typed[int64, string](cache)
		err = int64Cache.Set(ctx, "test", int64(456), "int64 value")
		assert.NoError(t, err)

		result, err = int64Cache.Get(ctx, "test", int64(456), nil)
		assert.NoError(t, err)
		assert.Equal(t, "int64 value", result)
	})
}
