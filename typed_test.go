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

// 模拟Video结构体，对应用户场景
type Video struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Duration    int    `json:"duration"`
	URL         string `json:"url"`
	CreatedTime int64  `json:"created_time"`
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

// 针对用户遇到的问题添加更详细的 MGet 测试
func TestTypedCache_MGet_ExtensiveTests(t *testing.T) {
	ctx := context.Background()

	t.Run("大批量ID - 模拟用户场景 [int64, []*Video]", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[int64, []*Video](cache)

		keyPrefix := "videos"

		// 准备测试数据 - 15个ID，模拟用户场景
		allIDs := []int64{101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115}
		existingIDs := []int64{101, 103, 105, 107, 109, 111, 113}     // 7个存在的ID
		missingIDs := []int64{102, 104, 106, 108, 110, 112, 114, 115} // 8个不存在的ID

		// 预先设置存在的数据
		existingData := make(map[int64][]*Video)
		for _, id := range existingIDs {
			videos := []*Video{
				{ID: id*10 + 1, Title: fmt.Sprintf("Video %d-1", id), Duration: 120, URL: fmt.Sprintf("url%d-1", id)},
				{ID: id*10 + 2, Title: fmt.Sprintf("Video %d-2", id), Duration: 180, URL: fmt.Sprintf("url%d-2", id)},
			}
			existingData[id] = videos
			err := typedCache.Set(ctx, keyPrefix, id, videos)
			assert.NoError(t, err)
		}

		// 定义loader，模拟从数据库加载
		loaderCallCount := 0
		loader := func(ctx context.Context, ids []int64) (map[int64][]*Video, error) {
			loaderCallCount++
			t.Logf("Loader called with %d IDs: %v", len(ids), ids)

			result := make(map[int64][]*Video)
			for _, id := range ids {
				// 只为存在的ID返回数据
				for _, existingID := range existingIDs {
					if id == existingID {
						if videos, exists := existingData[id]; exists {
							result[id] = videos
						}
						break
					}
				}
			}
			t.Logf("Loader returning %d results", len(result))
			return result, nil
		}

		// 执行批量获取
		result, err := typedCache.MGet(ctx, keyPrefix, allIDs, loader)
		assert.NoError(t, err)

		// 验证结果
		assert.Equal(t, len(existingIDs), len(result), "返回的结果数量应该等于存在的ID数量")

		for _, id := range existingIDs {
			videos, exists := result[id]
			assert.True(t, exists, "ID %d 应该存在于结果中", id)
			assert.NotNil(t, videos, "ID %d 的值不应该为nil", id)
			assert.Len(t, videos, 2, "ID %d 应该有2个视频", id)

			expected := existingData[id]
			assert.Equal(t, expected, videos, "ID %d 的数据应该匹配", id)
		}

		// 验证不存在的ID确实不在结果中
		for _, id := range missingIDs {
			_, exists := result[id]
			assert.False(t, exists, "ID %d 不应该存在于结果中", id)
		}

		// 验证loader只被调用一次（对于missing的IDs）
		assert.Equal(t, 1, loaderCallCount, "loader应该只被调用一次")
	})

	t.Run("验证key映射一致性", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[int64, []*Video](cache)

		keyPrefix := "consistency_test"
		testIDs := []int64{1001, 1002, 1003, 1004, 1005}

		// 通过单个Set设置数据
		expectedData := make(map[int64][]*Video)
		for _, id := range testIDs {
			videos := []*Video{
				{ID: id, Title: fmt.Sprintf("Video %d", id), Duration: 300},
			}
			expectedData[id] = videos
			err := typedCache.Set(ctx, keyPrefix, id, videos)
			assert.NoError(t, err)
		}

		// 通过MGet获取数据
		result, err := typedCache.MGet(ctx, keyPrefix, testIDs, nil)
		assert.NoError(t, err)

		// 验证所有数据都能正确获取
		assert.Equal(t, len(testIDs), len(result), "所有设置的数据都应该能获取到")

		for _, id := range testIDs {
			videos, exists := result[id]
			assert.True(t, exists, "ID %d 应该存在", id)
			assert.Equal(t, expectedData[id], videos, "ID %d 的数据应该匹配", id)
		}

		// 通过单个Get验证一致性
		for _, id := range testIDs {
			videos, err := typedCache.Get(ctx, keyPrefix, id, nil)
			assert.NoError(t, err)
			assert.Equal(t, expectedData[id], videos, "单个Get的结果应该与MGet一致")
		}
	})

	t.Run("MGet部分命中详细验证", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[int64, []*Video](cache)

		keyPrefix := "partial_hit"
		allIDs := []int64{2001, 2002, 2003, 2004, 2005}
		cachedIDs := []int64{2001, 2003, 2005} // 预先缓存的ID
		loadedIDs := []int64{2002, 2004}       // 需要从loader加载的ID

		// 预先设置部分数据到缓存
		cachedData := make(map[int64][]*Video)
		for _, id := range cachedIDs {
			videos := []*Video{
				{ID: id, Title: fmt.Sprintf("Cached Video %d", id), Duration: 240},
			}
			cachedData[id] = videos
			err := typedCache.Set(ctx, keyPrefix, id, videos)
			assert.NoError(t, err)
		}

		// 准备loader数据
		loaderData := make(map[int64][]*Video)
		for _, id := range loadedIDs {
			videos := []*Video{
				{ID: id, Title: fmt.Sprintf("Loaded Video %d", id), Duration: 360},
			}
			loaderData[id] = videos
		}

		// 定义loader
		loaderCallCount := 0
		var loaderCalledIDs []int64
		loader := func(ctx context.Context, ids []int64) (map[int64][]*Video, error) {
			loaderCallCount++
			loaderCalledIDs = append(loaderCalledIDs, ids...)
			t.Logf("Loader called with IDs: %v", ids)

			result := make(map[int64][]*Video)
			for _, id := range ids {
				if videos, exists := loaderData[id]; exists {
					result[id] = videos
				}
			}
			return result, nil
		}

		// 执行MGet
		result, err := typedCache.MGet(ctx, keyPrefix, allIDs, loader)
		assert.NoError(t, err)

		// 验证结果包含所有数据
		assert.Equal(t, len(allIDs), len(result), "应该返回所有ID的数据")

		// 验证缓存的数据
		for _, id := range cachedIDs {
			videos, exists := result[id]
			assert.True(t, exists, "缓存的ID %d 应该存在", id)
			assert.Equal(t, cachedData[id], videos, "缓存的数据应该匹配")
		}

		// 验证loader加载的数据
		for _, id := range loadedIDs {
			videos, exists := result[id]
			assert.True(t, exists, "加载的ID %d 应该存在", id)
			assert.Equal(t, loaderData[id], videos, "加载的数据应该匹配")
		}

		// 验证loader被正确调用
		assert.Equal(t, 1, loaderCallCount, "loader应该被调用一次")
		assert.ElementsMatch(t, loadedIDs, loaderCalledIDs, "loader应该只被传入未缓存的ID")
	})

	t.Run("大量ID的性能和正确性测试", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[int64, []*Video](cache)

		keyPrefix := "large_batch"
		const batchSize = 50 // 增加到50个ID

		allIDs := make([]int64, batchSize)
		for i := 0; i < batchSize; i++ {
			allIDs[i] = int64(3000 + i)
		}

		// 预先设置一半数据
		halfSize := batchSize / 2
		cachedData := make(map[int64][]*Video)
		for i := 0; i < halfSize; i++ {
			id := allIDs[i]
			videos := []*Video{
				{ID: id, Title: fmt.Sprintf("Large Video %d", id), Duration: 480},
			}
			cachedData[id] = videos
			err := typedCache.Set(ctx, keyPrefix, id, videos)
			assert.NoError(t, err)
		}

		// 准备loader数据（另一半）
		loaderData := make(map[int64][]*Video)
		for i := halfSize; i < batchSize; i++ {
			id := allIDs[i]
			videos := []*Video{
				{ID: id, Title: fmt.Sprintf("Loaded Large Video %d", id), Duration: 600},
			}
			loaderData[id] = videos
		}

		loader := func(ctx context.Context, ids []int64) (map[int64][]*Video, error) {
			result := make(map[int64][]*Video)
			for _, id := range ids {
				if videos, exists := loaderData[id]; exists {
					result[id] = videos
				}
			}
			return result, nil
		}

		// 执行MGet
		start := time.Now()
		result, err := typedCache.MGet(ctx, keyPrefix, allIDs, loader)
		duration := time.Since(start)

		assert.NoError(t, err)
		t.Logf("MGet for %d IDs took %v", batchSize, duration)

		// 验证所有数据都正确返回
		assert.Equal(t, batchSize, len(result), "应该返回所有ID的数据")

		for i, id := range allIDs {
			videos, exists := result[id]
			assert.True(t, exists, "ID %d (index %d) 应该存在", id, i)
			assert.NotNil(t, videos, "ID %d 的数据不应该为nil", id)
			assert.Len(t, videos, 1, "ID %d 应该有1个视频", id)

			if i < halfSize {
				// 验证缓存数据
				assert.Equal(t, cachedData[id], videos, "缓存数据应该匹配 ID %d", id)
			} else {
				// 验证loader数据
				assert.Equal(t, loaderData[id], videos, "加载数据应该匹配 ID %d", id)
			}
		}
	})

	t.Run("边界情况 - 空切片和nil指针", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[int64, []*Video](cache)

		keyPrefix := "edge_cases"
		testCases := map[int64][]*Video{
			4001: nil,           // nil slice
			4002: []*Video{},    // empty slice
			4003: []*Video{nil}, // slice with nil pointer
			4004: []*Video{ // normal data
				{ID: 4004, Title: "Normal Video", Duration: 120},
			},
		}

		// 设置测试数据
		for id, videos := range testCases {
			err := typedCache.Set(ctx, keyPrefix, id, videos)
			assert.NoError(t, err)
		}

		// 批量获取
		ids := []int64{4001, 4002, 4003, 4004}
		result, err := typedCache.MGet(ctx, keyPrefix, ids, nil)
		assert.NoError(t, err)

		// 验证结果
		for id, expected := range testCases {
			actual, exists := result[id]
			assert.True(t, exists, "ID %d 应该存在", id)
			assert.Equal(t, expected, actual, "ID %d 的数据应该匹配", id)
		}
	})

	t.Run("并发MGet测试", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[int64, []*Video](cache)

		keyPrefix := "concurrent_mget"
		const numGoroutines = 10
		const idsPerGoroutine = 5

		// 预先设置一些数据
		for i := 0; i < numGoroutines*idsPerGoroutine; i++ {
			id := int64(5000 + i)
			videos := []*Video{
				{ID: id, Title: fmt.Sprintf("Concurrent Video %d", id), Duration: 300},
			}
			err := typedCache.Set(ctx, keyPrefix, id, videos)
			assert.NoError(t, err)
		}

		var wg sync.WaitGroup
		results := make([]map[int64][]*Video, numGoroutines)
		errors := make([]error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineIndex int) {
				defer wg.Done()

				ids := make([]int64, idsPerGoroutine)
				for j := 0; j < idsPerGoroutine; j++ {
					ids[j] = int64(5000 + goroutineIndex*idsPerGoroutine + j)
				}

				result, err := typedCache.MGet(ctx, keyPrefix, ids, nil)
				results[goroutineIndex] = result
				errors[goroutineIndex] = err
			}(i)
		}

		wg.Wait()

		// 验证所有goroutine都成功
		for i := 0; i < numGoroutines; i++ {
			assert.NoError(t, errors[i], "Goroutine %d 应该成功", i)
			assert.Equal(t, idsPerGoroutine, len(results[i]), "Goroutine %d 应该返回正确数量的结果", i)
		}
	})

	t.Run("loader返回部分数据的情况", func(t *testing.T) {
		cache := createTestCache(t)
		typedCache := Typed[int64, []*Video](cache)

		keyPrefix := "partial_loader"
		requestedIDs := []int64{6001, 6002, 6003, 6004, 6005}
		loaderReturnIDs := []int64{6001, 6003, 6005} // loader只返回部分数据

		loaderData := make(map[int64][]*Video)
		for _, id := range loaderReturnIDs {
			videos := []*Video{
				{ID: id, Title: fmt.Sprintf("Partial Video %d", id), Duration: 180},
			}
			loaderData[id] = videos
		}

		loader := func(ctx context.Context, ids []int64) (map[int64][]*Video, error) {
			t.Logf("Loader called with: %v", ids)
			result := make(map[int64][]*Video)
			for _, id := range ids {
				if videos, exists := loaderData[id]; exists {
					result[id] = videos
				}
			}
			t.Logf("Loader returning: %v", func() []int64 {
				var keys []int64
				for k := range result {
					keys = append(keys, k)
				}
				return keys
			}())
			return result, nil
		}

		result, err := typedCache.MGet(ctx, keyPrefix, requestedIDs, loader)
		assert.NoError(t, err)

		// 验证只返回loader有数据的ID
		assert.Equal(t, len(loaderReturnIDs), len(result), "应该只返回loader有数据的ID")

		for _, id := range loaderReturnIDs {
			videos, exists := result[id]
			assert.True(t, exists, "ID %d 应该存在", id)
			assert.Equal(t, loaderData[id], videos, "数据应该匹配")
		}

		// 验证没有数据的ID不在结果中
		noDataIDs := []int64{6002, 6004}
		for _, id := range noDataIDs {
			_, exists := result[id]
			assert.False(t, exists, "ID %d 不应该存在于结果中", id)
		}
	})
}

// 专门针对用户遇到的问题：15个ID中有些在数据库存在但没被获取到
func TestTypedCache_MGet_UserReportedIssue(t *testing.T) {
	ctx := context.Background()
	cache := createTestCache(t)
	typedCache := Typed[int64, []*Video](cache)

	t.Run("真实场景复现 - 15个ID的复杂混合情况", func(t *testing.T) {
		keyPrefix := "user_videos"

		// 15个ID，模拟用户的真实场景
		allIDs := []int64{1001, 1002, 1003, 1004, 1005, 1006, 1007, 1008, 1009, 1010, 1011, 1012, 1013, 1014, 1015}

		// 5个真的没有数据的ID
		noDataIDs := []int64{1002, 1004, 1006, 1008, 1010}

		// 7个已经在缓存中的ID
		cachedIDs := []int64{1001, 1003, 1005, 1007, 1009, 1011, 1013}

		// 3个在数据库中有数据但需要通过loader加载的ID
		dbOnlyIDs := []int64{1012, 1014, 1015}

		// 预先设置缓存数据
		cachedData := make(map[int64][]*Video)
		for _, id := range cachedIDs {
			videos := []*Video{
				{ID: id*100 + 1, Title: fmt.Sprintf("Cached Video %d-1", id), Duration: 120},
				{ID: id*100 + 2, Title: fmt.Sprintf("Cached Video %d-2", id), Duration: 180},
			}
			cachedData[id] = videos
			err := typedCache.Set(ctx, keyPrefix, id, videos)
			assert.NoError(t, err)
		}

		// 准备数据库数据（模拟数据库中实际存在的数据）
		dbData := make(map[int64][]*Video)
		for _, id := range dbOnlyIDs {
			videos := []*Video{
				{ID: id*100 + 1, Title: fmt.Sprintf("DB Video %d-1", id), Duration: 240},
				{ID: id*100 + 2, Title: fmt.Sprintf("DB Video %d-2", id), Duration: 300},
				{ID: id*100 + 3, Title: fmt.Sprintf("DB Video %d-3", id), Duration: 360},
			}
			dbData[id] = videos
		}

		// 定义loader，严格按照"数据库中存在的数据"返回
		loaderCallCount := 0
		var allRequestedIDs []int64
		loader := func(ctx context.Context, requestedIDs []int64) (map[int64][]*Video, error) {
			loaderCallCount++
			allRequestedIDs = append(allRequestedIDs, requestedIDs...)

			t.Logf("=== Loader调用详情 ===")
			t.Logf("Loader被调用，请求的ID数量: %d", len(requestedIDs))
			t.Logf("请求的ID列表: %v", requestedIDs)

			result := make(map[int64][]*Video)

			for _, id := range requestedIDs {
				if videos, exists := dbData[id]; exists {
					result[id] = videos
					t.Logf("为ID %d 返回了 %d 个视频", id, len(videos))
				} else {
					t.Logf("ID %d 在数据库中不存在，跳过", id)
				}
			}

			t.Logf("Loader返回结果数量: %d", len(result))
			var returnedIDs []int64
			for id := range result {
				returnedIDs = append(returnedIDs, id)
			}
			t.Logf("返回的ID: %v", returnedIDs)
			t.Logf("========================")

			return result, nil
		}

		// 执行MGet
		t.Logf("开始执行MGet，请求15个ID: %v", allIDs)
		result, err := typedCache.MGet(ctx, keyPrefix, allIDs, loader)
		assert.NoError(t, err)

		t.Logf("MGet执行完成，返回结果数量: %d", len(result))

		// 详细验证结果
		expectedTotalCount := len(cachedIDs) + len(dbOnlyIDs)
		assert.Equal(t, expectedTotalCount, len(result), "结果数量应该等于缓存数据+数据库数据的总数")

		// 验证缓存中的数据都存在
		for _, id := range cachedIDs {
			videos, exists := result[id]
			assert.True(t, exists, "缓存中的ID %d 应该存在于结果中", id)
			assert.NotNil(t, videos, "缓存中的ID %d 的值不应该为nil", id)
			assert.Equal(t, cachedData[id], videos, "缓存中的ID %d 数据应该匹配", id)
			t.Logf("✓ 缓存ID %d 验证通过", id)
		}

		// 验证数据库中的数据都存在（这是关键的验证点）
		for _, id := range dbOnlyIDs {
			videos, exists := result[id]
			assert.True(t, exists, "数据库中的ID %d 应该存在于结果中 - 这是用户遇到的关键问题!", id)
			assert.NotNil(t, videos, "数据库中的ID %d 的值不应该为nil", id)
			assert.Equal(t, dbData[id], videos, "数据库中的ID %d 数据应该匹配", id)
			t.Logf("✓ 数据库ID %d 验证通过", id)
		}

		// 验证没有数据的ID确实不在结果中
		for _, id := range noDataIDs {
			_, exists := result[id]
			assert.False(t, exists, "无数据的ID %d 不应该存在于结果中", id)
			t.Logf("✓ 无数据ID %d 验证通过（正确不存在）", id)
		}

		// 验证loader调用情况
		assert.Equal(t, 1, loaderCallCount, "loader应该只被调用一次")

		// 验证loader被调用时请求的ID应该是所有未缓存的ID
		expectedLoaderIDs := append(dbOnlyIDs, noDataIDs...)
		assert.ElementsMatch(t, expectedLoaderIDs, allRequestedIDs,
			"loader应该被请求所有未缓存的ID（包括有数据和无数据的）")

		t.Logf("=== 最终验证结果 ===")
		t.Logf("预期总结果数: %d, 实际总结果数: %d", expectedTotalCount, len(result))
		t.Logf("缓存命中: %d/%d", len(cachedIDs), len(cachedIDs))
		t.Logf("数据库命中: %d/%d", len(dbOnlyIDs), len(dbOnlyIDs))
		t.Logf("无数据ID: %d/%d (正确跳过)", len(noDataIDs), len(noDataIDs))

		// 如果测试通过到这里，说明MGet工作正常
		t.Logf("✅ 所有验证通过，MGet方法工作正常")
	})

	t.Run("Key构建一致性深度验证", func(t *testing.T) {
		keyPrefix := "key_consistency"
		testIDs := []int64{10001, 10002, 10003}

		// 使用buildKey方法构建期望的key
		var expectedKeys []string
		for _, id := range testIDs {
			expectedKeys = append(expectedKeys, fmt.Sprintf("%s:%d", keyPrefix, id))
		}

		// 通过TypedCache设置数据
		testData := make(map[int64][]*Video)
		for _, id := range testIDs {
			videos := []*Video{
				{ID: id, Title: fmt.Sprintf("Test Video %d", id), Duration: 120},
			}
			testData[id] = videos
			err := typedCache.Set(ctx, keyPrefix, id, videos)
			assert.NoError(t, err)
		}

		// 直接通过底层cache验证key是否正确构建
		for i, id := range testIDs {
			expectedKey := expectedKeys[i]
			var videos []*Video
			err := cache.Get(ctx, expectedKey, &videos)
			assert.NoError(t, err, "通过底层cache应该能获取到key: %s", expectedKey)
			assert.Equal(t, testData[id], videos, "底层cache获取的数据应该匹配")
		}

		// 通过MGet获取并验证一致性
		result, err := typedCache.MGet(ctx, keyPrefix, testIDs, nil)
		assert.NoError(t, err)

		assert.Equal(t, len(testIDs), len(result), "MGet应该返回所有数据")
		for _, id := range testIDs {
			videos, exists := result[id]
			assert.True(t, exists, "ID %d 应该存在", id)
			assert.Equal(t, testData[id], videos, "MGet返回的数据应该与直接设置的数据一致")
		}
	})
}
