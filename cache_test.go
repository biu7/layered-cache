package cache

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/biu7/layered-cache/errors"
	"github.com/biu7/layered-cache/serializer"
	"github.com/biu7/layered-cache/storage"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func createMemoryAdapter(t *testing.T) storage.Memory {
	t.Helper()

	otter, err := storage.NewOtter(1024)
	if err != nil {
		panic(err)
	}
	return otter
}

func createRemoteAdapter(t *testing.T) storage.Remote {
	t.Helper()

	s, err := miniredis.Run()
	if err != nil {
		panic(err)
	}

	t.Cleanup(func() {
		s.Close()
	})

	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	return storage.NewRedisWithClient(client)
}

func createSerializer(t *testing.T) serializer.Serializer {
	t.Helper()

	return serializer.NewStdJson()
}

func TestNewCache(t *testing.T) {
	tests := []struct {
		name    string
		options []Option
		wantErr error
	}{
		{
			name: "成功创建缓存 - 仅内存适配器",
			options: []Option{
				WithConfigMemory(createMemoryAdapter(t)),
			},
			wantErr: nil,
		},
		{
			name: "成功创建缓存 - 仅Redis适配器",
			options: []Option{
				WithConfigRemote(createRemoteAdapter(t)),
			},
			wantErr: nil,
		},
		{
			name: "成功创建缓存 - 内存和Redis适配器",
			options: []Option{
				WithConfigMemory(createMemoryAdapter(t)),
				WithConfigRemote(createRemoteAdapter(t)),
			},
			wantErr: nil,
		},
		{
			name: "成功创建缓存 - 自定义序列化器",
			options: []Option{
				WithConfigMemory(createMemoryAdapter(t)),
				WithConfigSerializer(createSerializer(t)),
			},
			wantErr: nil,
		},
		{
			name: "成功创建缓存 - 自定义TTL",
			options: []Option{
				WithConfigMemory(createMemoryAdapter(t)),
				WithConfigRemote(createRemoteAdapter(t)),
				WithConfigDefaultTTL(10*time.Minute, 24*time.Hour),
			},
			wantErr: nil,
		},
		{
			name: "成功创建缓存 - 启用缺失值缓存",
			options: []Option{
				WithConfigMemory(createMemoryAdapter(t)),
				WithConfigDefaultCacheNotFound(true, 30*time.Second),
			},
			wantErr: nil,
		},
		{
			name: "失败 - 没有适配器",
			options: []Option{
				WithConfigSerializer(createSerializer(t)),
			},
			wantErr: errors.ErrAdapterRequired,
		},
		{
			name: "失败 - 无效的内存TTL",
			options: []Option{
				WithConfigMemory(createMemoryAdapter(t)),
				WithConfigDefaultTTL(0, 24*time.Hour),
			},
			wantErr: errors.ErrInvalidMemoryExpireTime,
		},
		{
			name: "失败 - 负的内存TTL",
			options: []Option{
				WithConfigMemory(createMemoryAdapter(t)),
				WithConfigDefaultTTL(-1*time.Minute, 24*time.Hour),
			},
			wantErr: errors.ErrInvalidMemoryExpireTime,
		},
		{
			name: "失败 - 无效的Redis TTL",
			options: []Option{
				WithConfigRemote(createRemoteAdapter(t)),
				WithConfigDefaultTTL(5*time.Minute, 0),
			},
			wantErr: errors.ErrInvalidRedisExpireTime,
		},
		{
			name: "失败 - 负的Redis TTL",
			options: []Option{
				WithConfigRemote(createRemoteAdapter(t)),
				WithConfigDefaultTTL(5*time.Minute, -1*time.Hour),
			},
			wantErr: errors.ErrInvalidRedisExpireTime,
		},
		{
			name: "失败 - 无效的缺失值缓存TTL",
			options: []Option{
				WithConfigMemory(createMemoryAdapter(t)),
				WithConfigDefaultCacheNotFound(true, 0),
			},
			wantErr: errors.ErrInvalidCacheNotFondTTL,
		},
		{
			name: "失败 - 负的缺失值缓存TTL",
			options: []Option{
				WithConfigMemory(createMemoryAdapter(t)),
				WithConfigDefaultCacheNotFound(true, -1*time.Second),
			},
			wantErr: errors.ErrInvalidCacheNotFondTTL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := NewCache(tt.options...)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("NewCache() expected error %v, got nil", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("NewCache() error = %v, want %v", err, tt.wantErr)
					return
				}
				return
			}

			if err != nil {
				t.Errorf("NewCache() unexpected error = %v", err)
				return
			}

			if cache == nil {
				t.Error("NewCache() returned nil cache")
				return
			}

			// 检查返回的缓存是否为正确的类型
			layeredCache, ok := cache.(*LayeredCache)
			if !ok {
				t.Error("NewCache() returned incorrect type")
				return
			}

			// 验证缓存实例的基本属性
			if layeredCache.serializer == nil {
				t.Error("NewCache() serializer is nil")
			}
		})
	}
}

func TestNewCache_MemoryOnly(t *testing.T) {
	cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
	if err != nil {
		t.Fatalf("NewCache() unexpected error = %v", err)
	}

	layeredCache := cache.(*LayeredCache)

	if layeredCache.memory == nil {
		t.Error("memory adapter is nil")
	}

	if layeredCache.remote != nil {
		t.Error("remote adapter should be nil")
	}
}

func TestNewCache_RedisOnly(t *testing.T) {
	cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
	if err != nil {
		t.Fatalf("NewCache() unexpected error = %v", err)
	}

	layeredCache := cache.(*LayeredCache)

	if layeredCache.memory != nil {
		t.Error("memory adapter should be nil")
	}

	if layeredCache.remote == nil {
		t.Error("remote adapter is nil")
	}
}

func TestNewCache_BothAdapters(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() unexpected error = %v", err)
	}

	layeredCache := cache.(*LayeredCache)

	if layeredCache.memory == nil {
		t.Error("memory adapter is nil")
	}

	if layeredCache.remote == nil {
		t.Error("remote adapter is nil")
	}
}

func TestNewCache_CustomTTL(t *testing.T) {
	memoryTTL := 30 * time.Minute
	remoteTTL := 48 * time.Hour

	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
		WithConfigDefaultTTL(memoryTTL, remoteTTL),
	)
	if err != nil {
		t.Fatalf("NewCache() unexpected error = %v", err)
	}

	layeredCache := cache.(*LayeredCache)

	if layeredCache.defaultMemoryTTL != memoryTTL {
		t.Errorf("defaultMemoryTTL = %v, want %v", layeredCache.defaultMemoryTTL, memoryTTL)
	}

	if layeredCache.defaultRemoteTTL != remoteTTL {
		t.Errorf("defaultRemoteTTL = %v, want %v", layeredCache.defaultRemoteTTL, remoteTTL)
	}
}

func TestNewCache_CustomCacheMissing(t *testing.T) {
	missingTTL := 45 * time.Second

	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigDefaultCacheNotFound(true, missingTTL),
	)
	if err != nil {
		t.Fatalf("NewCache() unexpected error = %v", err)
	}

	layeredCache := cache.(*LayeredCache)

	if layeredCache.defaultCacheNotFound != true {
		t.Errorf("defaultCacheNotFound = %v, want %v", layeredCache.defaultCacheNotFound, true)
	}

	if layeredCache.defaultCacheNotFoundTTL != missingTTL {
		t.Errorf("defaultCacheNotFoundTTL = %v, want %v", layeredCache.defaultCacheNotFoundTTL, missingTTL)
	}
}

func TestNewCache_CustomSerializer(t *testing.T) {
	customSerializer := createSerializer(t)

	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigSerializer(customSerializer),
	)
	if err != nil {
		t.Fatalf("NewCache() unexpected error = %v", err)
	}

	layeredCache := cache.(*LayeredCache)

	if layeredCache.serializer == nil {
		t.Error("serializer is nil")
	}
}

func TestLayeredCache_Set(t *testing.T) {
	tests := []struct {
		name         string
		setupCache   func(t *testing.T) Cache
		key          string
		value        any
		options      []SetOption
		wantErr      error
		validateFunc func(t *testing.T, cache Cache, key string, value any)
	}{
		{
			name: "成功设置到内存缓存 - 字符串",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			key:     "test-key",
			value:   "test-value",
			wantErr: nil,
			validateFunc: func(t *testing.T, cache Cache, key string, value any) {
				validateSetInAdapters(t, cache, key, value, 5*time.Minute) // 默认内存TTL
			},
		},
		{
			name: "成功设置到Redis缓存 - 字符串",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			key:     "test-key",
			value:   "test-value",
			wantErr: nil,
			validateFunc: func(t *testing.T, cache Cache, key string, value any) {
				validateSetInAdapters(t, cache, key, value, 14*24*time.Hour) // 默认Redis TTL
			},
		},
		{
			name: "成功设置到双层缓存 - 结构体",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			key:     "user-123",
			value:   TestUser{ID: 123, Name: "John", Email: "john@example.com"},
			wantErr: nil,
			validateFunc: func(t *testing.T, cache Cache, key string, value any) {
				validateSetInAdapters(t, cache, key, value, 14*24*time.Hour) // 默认Redis TTL
			},
		},
		{
			name: "成功设置 - 自定义TTL",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			key:     "custom-ttl-key",
			value:   "custom-ttl-value",
			options: []SetOption{WithTTL(30*time.Second, 2*time.Minute)},
			wantErr: nil,
			validateFunc: func(t *testing.T, cache Cache, key string, value any) {
				validateSetInAdapters(t, cache, key, value, 2*time.Minute) // 自定义Redis TTL
			},
		},
		{
			name: "成功设置 - 字节数组",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			key:     "bytes-key",
			value:   []byte("bytes-value"),
			wantErr: nil,
			validateFunc: func(t *testing.T, cache Cache, key string, value any) {
				validateSetInAdapters(t, cache, key, value, 5*time.Minute) // 默认内存TTL
			},
		},
		{
			name: "成功设置 - nil值",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			key:     "nil-key",
			value:   nil,
			wantErr: nil,
			validateFunc: func(t *testing.T, cache Cache, key string, value any) {
				validateSetInAdapters(t, cache, key, value, 5*time.Minute) // 默认内存TTL
			},
		},
		{
			name: "成功设置 - 空字符串",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			key:     "empty-key",
			value:   "",
			wantErr: nil,
			validateFunc: func(t *testing.T, cache Cache, key string, value any) {
				validateSetInAdapters(t, cache, key, value, 5*time.Minute) // 默认内存TTL
			},
		},
		{
			name: "失败 - 无效的内存TTL",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			key:     "invalid-ttl-key",
			value:   "test-value",
			options: []SetOption{WithTTL(0, time.Hour)},
			wantErr: errors.ErrInvalidMemoryExpireTime,
		},
		{
			name: "失败 - 负的内存TTL",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			key:     "negative-ttl-key",
			value:   "test-value",
			options: []SetOption{WithTTL(-time.Minute, time.Hour)},
			wantErr: errors.ErrInvalidMemoryExpireTime,
		},
		{
			name: "失败 - 无效的Redis TTL",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			key:     "invalid-redis-ttl-key",
			value:   "test-value",
			options: []SetOption{WithTTL(time.Hour, 0)},
			wantErr: errors.ErrInvalidRedisExpireTime,
		},
		{
			name: "失败 - 负的Redis TTL",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			key:     "negative-redis-ttl-key",
			value:   "test-value",
			options: []SetOption{WithTTL(time.Hour, -time.Minute)},
			wantErr: errors.ErrInvalidRedisExpireTime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := tt.setupCache(t)
			ctx := context.Background()

			err := cache.Set(ctx, tt.key, tt.value, tt.options...)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("Set() expected error %v, got nil", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Set() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("Set() unexpected error = %v", err)
				return
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, cache, tt.key, tt.value)
			}
		})
	}
}

func TestLayeredCache_Set_MemoryOnly(t *testing.T) {
	cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	key := "memory-only-key"
	value := "memory-only-value"

	err = cache.Set(ctx, key, value)
	if err != nil {
		t.Errorf("Set() error = %v", err)
		return
	}

	// 直接验证适配器中的数据
	validateSetInAdapters(t, cache, key, value, 5*time.Minute) // 默认内存TTL
}

func TestLayeredCache_Set_RedisOnly(t *testing.T) {
	cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	key := "redis-only-key"
	value := "redis-only-value"

	err = cache.Set(ctx, key, value)
	if err != nil {
		t.Errorf("Set() error = %v", err)
		return
	}

	// 直接验证适配器中的数据
	validateSetInAdapters(t, cache, key, value, 14*24*time.Hour) // 默认Redis TTL
}

func TestLayeredCache_Set_BothCaches(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	key := "both-caches-key"
	value := TestUser{ID: 456, Name: "Jane", Email: "jane@example.com"}

	err = cache.Set(ctx, key, value)
	if err != nil {
		t.Errorf("Set() error = %v", err)
		return
	}

	// 直接验证适配器中的数据
	validateSetInAdapters(t, cache, key, value, 14*24*time.Hour) // 默认Redis TTL
}

func TestLayeredCache_Set_ComplexTypes(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name  string
		key   string
		value any
	}{
		{
			name:  "结构体",
			key:   "struct-key",
			value: TestUser{ID: 789, Name: "Bob", Email: "bob@example.com"},
		},
		{
			name:  "数组",
			key:   "array-key",
			value: []int{1, 2, 3, 4, 5},
		},
		{
			name: "映射",
			key:  "map-key",
			value: map[string]int{
				"one":   1,
				"two":   2,
				"three": 3,
			},
		},
		{
			name: "嵌套结构",
			key:  "nested-key",
			value: TestNestedStruct{
				User: TestUser{ID: 999, Name: "Nested", Email: "nested@example.com"},
				Tags: []string{"tag1", "tag2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cache.Set(ctx, tt.key, tt.value)
			if err != nil {
				t.Errorf("Set() error = %v", err)
				return
			}

			// 直接验证适配器中的数据
			validateSetInAdapters(t, cache, tt.key, tt.value, 14*24*time.Hour) // 默认Redis TTL
		})
	}
}

func TestLayeredCache_Set_ContextCancellation(t *testing.T) {
	cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	// 创建一个已经取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消context

	key := "cancelled-key"
	value := "cancelled-value"

	err = cache.Set(ctx, key, value)
	if err == nil {
		t.Error("Set() expected error due to cancelled context, got nil")
	}
}

// 辅助类型和函数
type TestUser struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type TestNestedStruct struct {
	User TestUser `json:"user"`
	Tags []string `json:"tags"`
}

func slicesEqual[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mapsEqual[K comparable, V comparable](a, b map[K]V) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// validateSetInAdapters 验证数据是否正确设置到适配器中
func validateSetInAdapters(t *testing.T, cache Cache, key string, value any, expectedTTL time.Duration) {
	t.Helper()

	layeredCache := cache.(*LayeredCache)

	// 验证内存适配器
	if layeredCache.memory != nil {
		memoryData, exists := layeredCache.memory.Get(key)
		if !exists {
			t.Errorf("键 %s 未在内存适配器中找到", key)
			return
		}
		validateStoredData(t, memoryData, value, layeredCache.serializer, "内存适配器")
		// 内存适配器（Otter）无法获取 TTL，所以跳过 TTL 校验
	}

	// 验证Redis适配器
	if layeredCache.remote != nil {
		redisData, err := layeredCache.remote.Get(context.Background(), key)
		if err != nil {
			t.Errorf("键 %s 未在Redis适配器中找到: %v", key, err)
			return
		}
		validateStoredData(t, redisData, value, layeredCache.serializer, "Redis适配器")

		// 验证Redis TTL
		if expectedTTL > 0 {
			actualTTL, err := layeredCache.remote.TTL(context.Background(), key)
			if err != nil {
				t.Errorf("Redis适配器获取TTL失败: %v", err)
				return
			}

			// TTL 应该在预期值的合理范围内（允许1秒的误差）
			if actualTTL <= 0 || actualTTL > expectedTTL || actualTTL < expectedTTL-time.Second {
				t.Errorf("Redis TTL = %v, want 在 %v 到 %v 之间", actualTTL, expectedTTL-time.Second, expectedTTL)
			}
		}
	}
}

// validateStoredData 验证存储的数据是否与原始值匹配
func validateStoredData(t *testing.T, storedData []byte, originalValue any, serializer serializer.Serializer, adapterName string) {
	t.Helper()

	switch v := originalValue.(type) {
	case []byte:
		if !bytes.Equal(storedData, v) {
			t.Errorf("%s存储的数据 = %v, want %v", adapterName, storedData, v)
		}
	case string:
		expected := []byte(v)
		if !bytes.Equal(storedData, expected) {
			t.Errorf("%s存储的数据 = %v, want %v", adapterName, storedData, expected)
		}
	default:
		// 对于复杂类型，通过反序列化验证
		switch original := originalValue.(type) {
		case TestUser:
			var result TestUser
			if err := serializer.Unmarshal(storedData, &result); err != nil {
				t.Errorf("%s反序列化失败: %v", adapterName, err)
				return
			}
			if result != original {
				t.Errorf("%s反序列化结果 = %v, want %v", adapterName, result, original)
			}
		case []int:
			var result []int
			if err := serializer.Unmarshal(storedData, &result); err != nil {
				t.Errorf("%s反序列化失败: %v", adapterName, err)
				return
			}
			if !slicesEqual(result, original) {
				t.Errorf("%s反序列化结果 = %v, want %v", adapterName, result, original)
			}
		case map[string]int:
			var result map[string]int
			if err := serializer.Unmarshal(storedData, &result); err != nil {
				t.Errorf("%s反序列化失败: %v", adapterName, err)
				return
			}
			if !mapsEqual(result, original) {
				t.Errorf("%s反序列化结果 = %v, want %v", adapterName, result, original)
			}
		case TestNestedStruct:
			var result TestNestedStruct
			if err := serializer.Unmarshal(storedData, &result); err != nil {
				t.Errorf("%s反序列化失败: %v", adapterName, err)
				return
			}
			if result.User != original.User || !slicesEqual(result.Tags, original.Tags) {
				t.Errorf("%s反序列化结果 = %v, want %v", adapterName, result, original)
			}
		case nil:
			var result interface{}
			if err := serializer.Unmarshal(storedData, &result); err != nil {
				t.Errorf("%s反序列化失败: %v", adapterName, err)
				return
			}
			if result != nil {
				t.Errorf("%s反序列化结果 = %v, want nil", adapterName, result)
			}
		default:
			// 对于其他类型，尝试通过序列化比较
			expectedData, err := serializer.Marshal(originalValue)
			if err != nil {
				t.Errorf("序列化原始值失败: %v", err)
				return
			}
			if !bytes.Equal(storedData, expectedData) {
				t.Errorf("%s存储的数据与序列化后的原始值不匹配", adapterName)
			}
		}
	}
}

// validateMSetInAdapters 验证批量数据是否正确设置到适配器中
func validateMSetInAdapters(t *testing.T, cache Cache, keyValues map[string]any, expectedTTL time.Duration) {
	t.Helper()

	layeredCache := cache.(*LayeredCache)

	// 验证内存适配器
	if layeredCache.memory != nil {
		for key, expectedValue := range keyValues {
			memoryData, exists := layeredCache.memory.Get(key)
			if !exists {
				t.Errorf("键 %s 未在内存适配器中找到", key)
				continue
			}
			validateStoredData(t, memoryData, expectedValue, layeredCache.serializer, "内存适配器")
		}
		// 内存适配器（Otter）无法获取 TTL，所以跳过 TTL 校验
	}

	// 验证Redis适配器
	if layeredCache.remote != nil {
		for key, expectedValue := range keyValues {
			redisData, err := layeredCache.remote.Get(context.Background(), key)
			if err != nil {
				t.Errorf("键 %s 未在Redis适配器中找到: %v", key, err)
				continue
			}
			validateStoredData(t, redisData, expectedValue, layeredCache.serializer, "Redis适配器")

			// 验证Redis TTL
			if expectedTTL > 0 {
				actualTTL, err := layeredCache.remote.TTL(context.Background(), key)
				if err != nil {
					t.Errorf("Redis适配器获取TTL失败 (键: %s): %v", key, err)
					continue
				}

				// TTL 应该在预期值的合理范围内（允许1秒的误差）
				if actualTTL <= 0 || actualTTL > expectedTTL || actualTTL < expectedTTL-time.Second {
					t.Errorf("Redis TTL (键: %s) = %v, want 在 %v 到 %v 之间", key, actualTTL, expectedTTL-time.Second, expectedTTL)
				}
			}
		}
	}
}

func TestLayeredCache_MSet(t *testing.T) {
	tests := []struct {
		name         string
		setupCache   func(t *testing.T) Cache
		keyValues    map[string]any
		options      []SetOption
		wantErr      error
		validateFunc func(t *testing.T, cache Cache, keyValues map[string]any)
	}{
		{
			name: "成功批量设置到内存缓存 - 混合类型",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			keyValues: map[string]any{
				"string-key": "string-value",
				"int-key":    123,
				"user-key":   TestUser{ID: 1, Name: "Alice", Email: "alice@example.com"},
				"bytes-key":  []byte("bytes-value"),
			},
			wantErr: nil,
			validateFunc: func(t *testing.T, cache Cache, keyValues map[string]any) {
				validateMSetInAdapters(t, cache, keyValues, 5*time.Minute) // 默认内存TTL
			},
		},
		{
			name: "成功批量设置到Redis缓存 - 混合类型",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			keyValues: map[string]any{
				"string-key": "string-value",
				"array-key":  []int{1, 2, 3},
				"map-key":    map[string]string{"hello": "world"},
				"nil-key":    nil,
			},
			wantErr: nil,
			validateFunc: func(t *testing.T, cache Cache, keyValues map[string]any) {
				validateMSetInAdapters(t, cache, keyValues, 14*24*time.Hour) // 默认Redis TTL
			},
		},
		{
			name: "成功批量设置到双层缓存 - 复杂类型",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			keyValues: map[string]any{
				"user-1": TestUser{ID: 1, Name: "User1", Email: "user1@example.com"},
				"user-2": TestUser{ID: 2, Name: "User2", Email: "user2@example.com"},
				"nested": TestNestedStruct{
					User: TestUser{ID: 3, Name: "Nested", Email: "nested@example.com"},
					Tags: []string{"tag1", "tag2"},
				},
				"array": []string{"item1", "item2", "item3"},
			},
			wantErr: nil,
			validateFunc: func(t *testing.T, cache Cache, keyValues map[string]any) {
				validateMSetInAdapters(t, cache, keyValues, 14*24*time.Hour) // 默认Redis TTL
			},
		},
		{
			name: "成功批量设置 - 自定义TTL",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			keyValues: map[string]any{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
			options: []SetOption{WithTTL(1*time.Minute, 5*time.Minute)},
			wantErr: nil,
			validateFunc: func(t *testing.T, cache Cache, keyValues map[string]any) {
				validateMSetInAdapters(t, cache, keyValues, 5*time.Minute) // 自定义Redis TTL
			},
		},
		{
			name: "成功批量设置 - 空映射",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			keyValues: map[string]any{},
			wantErr:   nil,
			validateFunc: func(t *testing.T, cache Cache, keyValues map[string]any) {
				// 空映射，无需验证
			},
		},
		{
			name: "成功批量设置 - 包含空字符串和nil",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			keyValues: map[string]any{
				"empty-string": "",
				"nil-value":    nil,
				"zero-int":     0,
			},
			wantErr: nil,
			validateFunc: func(t *testing.T, cache Cache, keyValues map[string]any) {
				validateMSetInAdapters(t, cache, keyValues, 5*time.Minute) // 默认内存TTL
			},
		},
		{
			name: "失败 - 无效的内存TTL",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			keyValues: map[string]any{
				"key1": "value1",
			},
			options: []SetOption{WithTTL(0, time.Hour)},
			wantErr: errors.ErrInvalidMemoryExpireTime,
		},
		{
			name: "失败 - 负的内存TTL",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			keyValues: map[string]any{
				"key1": "value1",
			},
			options: []SetOption{WithTTL(-1*time.Minute, time.Hour)},
			wantErr: errors.ErrInvalidMemoryExpireTime,
		},
		{
			name: "失败 - 无效的Redis TTL",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			keyValues: map[string]any{
				"key1": "value1",
			},
			options: []SetOption{WithTTL(time.Minute, 0)},
			wantErr: errors.ErrInvalidRedisExpireTime,
		},
		{
			name: "失败 - 负的Redis TTL",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			keyValues: map[string]any{
				"key1": "value1",
			},
			options: []SetOption{WithTTL(time.Minute, -1*time.Hour)},
			wantErr: errors.ErrInvalidRedisExpireTime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := tt.setupCache(t)
			ctx := context.Background()

			err := cache.MSet(ctx, tt.keyValues, tt.options...)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("MSet() expected error %v, got nil", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("MSet() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("MSet() error = %v", err)
				return
			}

			// 验证结果
			if tt.validateFunc != nil {
				tt.validateFunc(t, cache, tt.keyValues)
			}
		})
	}
}

func TestLayeredCache_MSet_MemoryOnly(t *testing.T) {
	cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	keyValues := map[string]any{
		"memory-key-1": "memory-value-1",
		"memory-key-2": TestUser{ID: 100, Name: "MemoryUser", Email: "memory@example.com"},
		"memory-key-3": []int{10, 20, 30},
	}

	err = cache.MSet(ctx, keyValues)
	if err != nil {
		t.Errorf("MSet() error = %v", err)
		return
	}

	// 验证批量设置结果
	validateMSetInAdapters(t, cache, keyValues, 5*time.Minute) // 默认内存TTL
}

func TestLayeredCache_MSet_RedisOnly(t *testing.T) {
	cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	keyValues := map[string]any{
		"redis-key-1": "redis-value-1",
		"redis-key-2": map[string]int{"count": 42},
		"redis-key-3": []byte("redis-bytes"),
	}

	err = cache.MSet(ctx, keyValues)
	if err != nil {
		t.Errorf("MSet() error = %v", err)
		return
	}

	// 验证批量设置结果
	validateMSetInAdapters(t, cache, keyValues, 14*24*time.Hour) // 默认Redis TTL
}

func TestLayeredCache_MSet_BothCaches(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	keyValues := map[string]any{
		"both-key-1": "both-value-1",
		"both-key-2": TestUser{ID: 200, Name: "BothUser", Email: "both@example.com"},
		"both-key-3": TestNestedStruct{
			User: TestUser{ID: 300, Name: "Nested", Email: "nested@example.com"},
			Tags: []string{"both", "cache"},
		},
	}

	err = cache.MSet(ctx, keyValues)
	if err != nil {
		t.Errorf("MSet() error = %v", err)
		return
	}

	// 验证批量设置结果
	validateMSetInAdapters(t, cache, keyValues, 14*24*time.Hour) // 默认Redis TTL
}

func TestLayeredCache_MSet_LargeDataset(t *testing.T) {
	// 为大数据集测试创建更大内存限制的适配器
	largeMemoryAdapter, err := storage.NewOtter(10240) // 10KB内存限制
	if err != nil {
		t.Fatalf("NewOtter() error = %v", err)
	}

	cache, err := NewCache(
		WithConfigMemory(largeMemoryAdapter),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()

	// 生成大量数据
	keyValues := make(map[string]any)
	for i := 0; i < 100; i++ {
		keyValues[fmt.Sprintf("key-%d", i)] = fmt.Sprintf("value-%d", i)
	}

	err = cache.MSet(ctx, keyValues)
	if err != nil {
		t.Errorf("MSet() error = %v", err)
		return
	}

	// 验证批量设置结果
	validateMSetInAdapters(t, cache, keyValues, 14*24*time.Hour) // 默认Redis TTL
}

func TestLayeredCache_MSet_ContextCancellation(t *testing.T) {
	cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	// 创建一个已经取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	keyValues := map[string]any{
		"cancel-key": "cancel-value",
	}

	err = cache.MSet(ctx, keyValues)
	// 注意：上下文取消可能不会立即影响 MSet，因为 Redis 操作可能已经开始
	// 但我们至少验证它不会panic
	assert.Equal(t, true, errors.Is(err, context.Canceled))
	if err != nil {
		t.Logf("MSet() with cancelled context returned error: %v", err)
	}
}

// validateDeleteInAdapters 验证数据是否正确从适配器中删除
func validateDeleteInAdapters(t *testing.T, cache Cache, key string) {
	t.Helper()

	layeredCache := cache.(*LayeredCache)

	// 验证内存适配器中的数据已被删除
	if layeredCache.memory != nil {
		if data, exists := layeredCache.memory.Get(key); exists {
			t.Errorf("键 %s 在内存适配器中仍然存在，数据: %v", key, data)
		}
	}

	// 验证Redis适配器中的数据已被删除
	if layeredCache.remote != nil {
		if data, err := layeredCache.remote.Get(context.Background(), key); err == nil {
			t.Errorf("键 %s 在Redis适配器中仍然存在，数据: %v", key, data)
		}
	}
}

// validateKeyExists 验证键是否存在于适配器中
func validateKeyExists(t *testing.T, cache Cache, key string) {
	t.Helper()

	layeredCache := cache.(*LayeredCache)
	found := false

	// 检查内存适配器
	if layeredCache.memory != nil {
		if _, exists := layeredCache.memory.Get(key); exists {
			found = true
		}
	}

	// 检查Redis适配器
	if layeredCache.remote != nil {
		if _, err := layeredCache.remote.Get(context.Background(), key); err == nil {
			found = true
		}
	}

	// 只要在任一适配器中找到键就算成功
	if !found {
		t.Errorf("键 %s 在任何适配器中都不存在", key)
	}
}

func TestLayeredCache_Delete(t *testing.T) {
	tests := []struct {
		name         string
		setupCache   func(t *testing.T) Cache
		setupData    func(t *testing.T, cache Cache) string // 返回要删除的键
		wantErr      bool
		validateFunc func(t *testing.T, cache Cache, key string)
	}{
		{
			name: "成功从内存缓存删除",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) string {
				ctx := context.Background()
				key := "memory-delete-key"
				value := "memory-delete-value"

				err := cache.Set(ctx, key, value)
				if err != nil {
					t.Fatalf("Set() error = %v", err)
				}

				// 验证数据已设置
				validateKeyExists(t, cache, key)
				return key
			},
			wantErr: false,
			validateFunc: func(t *testing.T, cache Cache, key string) {
				validateDeleteInAdapters(t, cache, key)
			},
		},
		{
			name: "成功从Redis缓存删除",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) string {
				ctx := context.Background()
				key := "redis-delete-key"
				value := "redis-delete-value"

				err := cache.Set(ctx, key, value)
				if err != nil {
					t.Fatalf("Set() error = %v", err)
				}

				// 验证数据已设置
				validateKeyExists(t, cache, key)
				return key
			},
			wantErr: false,
			validateFunc: func(t *testing.T, cache Cache, key string) {
				validateDeleteInAdapters(t, cache, key)
			},
		},
		{
			name: "成功从双层缓存删除",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) string {
				ctx := context.Background()
				key := "both-delete-key"
				value := TestUser{ID: 123, Name: "DeleteUser", Email: "delete@example.com"}

				err := cache.Set(ctx, key, value)
				if err != nil {
					t.Fatalf("Set() error = %v", err)
				}

				// 验证数据已设置
				validateKeyExists(t, cache, key)
				return key
			},
			wantErr: false,
			validateFunc: func(t *testing.T, cache Cache, key string) {
				validateDeleteInAdapters(t, cache, key)
			},
		},
		{
			name: "删除不存在的键 - 内存缓存",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) string {
				return "non-existent-key"
			},
			wantErr: false, // 删除不存在的键不应该报错
			validateFunc: func(t *testing.T, cache Cache, key string) {
				// 验证键确实不存在
				validateDeleteInAdapters(t, cache, key)
			},
		},
		{
			name: "删除不存在的键 - Redis缓存",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) string {
				return "non-existent-redis-key"
			},
			wantErr: false, // 删除不存在的键不应该报错
			validateFunc: func(t *testing.T, cache Cache, key string) {
				// 验证键确实不存在
				validateDeleteInAdapters(t, cache, key)
			},
		},
		{
			name: "删除不存在的键 - 双层缓存",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) string {
				return "non-existent-both-key"
			},
			wantErr: false, // 删除不存在的键不应该报错
			validateFunc: func(t *testing.T, cache Cache, key string) {
				// 验证键确实不存在
				validateDeleteInAdapters(t, cache, key)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := tt.setupCache(t)
			key := tt.setupData(t, cache)

			ctx := context.Background()
			err := cache.Delete(ctx, key)

			if tt.wantErr && err == nil {
				t.Errorf("Delete() expected error, got nil")
				return
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Delete() unexpected error = %v", err)
				return
			}

			// 验证删除结果
			if tt.validateFunc != nil {
				tt.validateFunc(t, cache, key)
			}
		})
	}
}

func TestLayeredCache_Delete_MemoryOnly(t *testing.T) {
	cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	key := "memory-only-delete-key"
	value := "memory-only-delete-value"

	// 设置数据
	err = cache.Set(ctx, key, value)
	if err != nil {
		t.Errorf("Set() error = %v", err)
		return
	}

	// 验证数据存在
	validateKeyExists(t, cache, key)

	// 删除数据
	err = cache.Delete(ctx, key)
	if err != nil {
		t.Errorf("Delete() error = %v", err)
		return
	}

	// 验证数据已删除
	validateDeleteInAdapters(t, cache, key)
}

func TestLayeredCache_Delete_RedisOnly(t *testing.T) {
	cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	key := "redis-only-delete-key"
	value := "redis-only-delete-value"

	// 设置数据
	err = cache.Set(ctx, key, value)
	if err != nil {
		t.Errorf("Set() error = %v", err)
		return
	}

	// 验证数据存在
	validateKeyExists(t, cache, key)

	// 删除数据
	err = cache.Delete(ctx, key)
	if err != nil {
		t.Errorf("Delete() error = %v", err)
		return
	}

	// 验证数据已删除
	validateDeleteInAdapters(t, cache, key)
}

func TestLayeredCache_Delete_BothCaches(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	key := "both-caches-delete-key"
	value := TestUser{ID: 456, Name: "DeleteUser", Email: "delete@example.com"}

	// 设置数据
	err = cache.Set(ctx, key, value)
	if err != nil {
		t.Errorf("Set() error = %v", err)
		return
	}

	// 验证数据存在
	validateKeyExists(t, cache, key)

	// 删除数据
	err = cache.Delete(ctx, key)
	if err != nil {
		t.Errorf("Delete() error = %v", err)
		return
	}

	// 验证数据已删除
	validateDeleteInAdapters(t, cache, key)
}

func TestLayeredCache_Delete_MultipleKeys(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()

	// 设置多个键值对
	keys := []string{"key1", "key2", "key3", "key4", "key5"}
	values := []string{"value1", "value2", "value3", "value4", "value5"}

	for i, key := range keys {
		err = cache.Set(ctx, key, values[i])
		if err != nil {
			t.Errorf("Set() error for key %s = %v", key, err)
			return
		}
	}

	// 验证所有键存在
	for _, key := range keys {
		validateKeyExists(t, cache, key)
	}

	// 删除所有键
	for _, key := range keys {
		err = cache.Delete(ctx, key)
		if err != nil {
			t.Errorf("Delete() error for key %s = %v", key, err)
			return
		}

		// 验证键已被删除
		validateDeleteInAdapters(t, cache, key)
	}
}

func TestLayeredCache_Delete_ComplexTypes(t *testing.T) {
	// 为复杂类型测试使用更大的内存适配器
	largeMemoryAdapter, err := storage.NewOtter(10240) // 10KB内存限制
	if err != nil {
		t.Fatalf("NewOtter() error = %v", err)
	}

	cache, err := NewCache(
		WithConfigMemory(largeMemoryAdapter),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()

	testCases := []struct {
		name  string
		key   string
		value any
	}{
		{
			name:  "删除结构体",
			key:   "struct-delete-key",
			value: TestUser{ID: 999, Name: "DeleteStruct", Email: "struct@example.com"},
		},
		{
			name:  "删除数组",
			key:   "array-delete-key",
			value: []int{1, 2, 3, 4, 5},
		},
		{
			name: "删除映射",
			key:  "map-delete-key",
			value: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "删除嵌套结构",
			key:  "nested-delete-key",
			value: TestNestedStruct{
				User: TestUser{ID: 888, Name: "NestedDelete", Email: "nested@example.com"},
				Tags: []string{"delete", "test"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 设置数据
			err := cache.Set(ctx, tc.key, tc.value)
			if err != nil {
				t.Errorf("Set() error = %v", err)
				return
			}

			// 验证数据存在
			validateKeyExists(t, cache, tc.key)

			// 删除数据
			err = cache.Delete(ctx, tc.key)
			if err != nil {
				t.Errorf("Delete() error = %v", err)
				return
			}

			// 验证数据已删除
			validateDeleteInAdapters(t, cache, tc.key)
		})
	}
}

func TestLayeredCache_Delete_ContextCancellation(t *testing.T) {
	cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	// 设置数据
	ctx := context.Background()
	key := "context-cancel-delete-key"
	value := "context-cancel-delete-value"

	err = cache.Set(ctx, key, value)
	if err != nil {
		t.Errorf("Set() error = %v", err)
		return
	}

	// 验证数据存在
	validateKeyExists(t, cache, key)

	// 创建一个已取消的上下文
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	// 尝试删除数据
	err = cache.Delete(cancelCtx, key)
	if err != nil {
		t.Logf("Delete() with cancelled context returned error: %v", err)
		// 上下文取消应该返回错误，这是正常的
	}
}

func TestLayeredCache_Delete_AfterMSet(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()

	// 批量设置数据
	keyValues := map[string]any{
		"mset-key-1": "mset-value-1",
		"mset-key-2": TestUser{ID: 111, Name: "MSetUser", Email: "mset@example.com"},
		"mset-key-3": []string{"item1", "item2", "item3"},
	}

	err = cache.MSet(ctx, keyValues)
	if err != nil {
		t.Errorf("MSet() error = %v", err)
		return
	}

	// 验证所有键存在
	for key := range keyValues {
		validateKeyExists(t, cache, key)
	}

	// 删除所有键
	for key := range keyValues {
		err = cache.Delete(ctx, key)
		if err != nil {
			t.Errorf("Delete() error for key %s = %v", key, err)
			return
		}

		// 验证键已被删除
		validateDeleteInAdapters(t, cache, key)
	}
}

func TestLayeredCache_Get(t *testing.T) {
	tests := []struct {
		name         string
		setupCache   func(t *testing.T) Cache
		setupData    func(t *testing.T, cache Cache) // 预设数据
		key          string
		target       any
		options      []GetOption
		wantErr      error
		wantValue    any
		validateFunc func(t *testing.T, cache Cache, key string, target any)
	}{
		{
			name: "成功获取内存缓存中存在的值 - 字符串",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				ctx := context.Background()
				err := cache.Set(ctx, "memory-string-key", "memory-string-value")
				if err != nil {
					t.Fatalf("Set() error = %v", err)
				}
			},
			key:       "memory-string-key",
			target:    new(string),
			wantErr:   nil,
			wantValue: "memory-string-value",
		},
		{
			name: "成功获取内存缓存中存在的值 - 结构体",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				ctx := context.Background()
				user := TestUser{ID: 123, Name: "Alice", Email: "alice@example.com"}
				err := cache.Set(ctx, "memory-user-key", user)
				if err != nil {
					t.Fatalf("Set() error = %v", err)
				}
			},
			key:       "memory-user-key",
			target:    new(TestUser),
			wantErr:   nil,
			wantValue: TestUser{ID: 123, Name: "Alice", Email: "alice@example.com"},
		},
		{
			name: "成功获取内存缓存中存在的值 - 字节数组",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				ctx := context.Background()
				data := []byte("binary-data")
				err := cache.Set(ctx, "memory-bytes-key", data)
				if err != nil {
					t.Fatalf("Set() error = %v", err)
				}
			},
			key:       "memory-bytes-key",
			target:    new([]byte),
			wantErr:   nil,
			wantValue: []byte("binary-data"),
		},
		{
			name: "成功获取内存缓存不存在，Redis存在的值 - 字符串",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 直接向Redis适配器设置数据，避免写入内存
				layeredCache := cache.(*LayeredCache)
				ctx := context.Background()
				value := "redis-only-value"
				data, err := layeredCache.Marshal(value)
				if err != nil {
					t.Fatalf("Marshal() error = %v", err)
				}
				err = layeredCache.remote.Set(ctx, "redis-only-key", data, 24*time.Hour)
				if err != nil {
					t.Fatalf("Redis Set() error = %v", err)
				}
			},
			key:       "redis-only-key",
			target:    new(string),
			wantErr:   nil,
			wantValue: "redis-only-value",
			validateFunc: func(t *testing.T, cache Cache, key string, target any) {
				// 验证数据已经从Redis回写到内存缓存
				layeredCache := cache.(*LayeredCache)
				if layeredCache.memory != nil {
					if data, exists := layeredCache.memory.Get(key); !exists {
						t.Errorf("数据未从Redis回写到内存缓存")
					} else {
						var result string
						err := layeredCache.Unmarshal(data, &result)
						if err != nil {
							t.Errorf("内存缓存反序列化失败: %v", err)
						} else if result != "redis-only-value" {
							t.Errorf("内存缓存数据 = %v, want %v", result, "redis-only-value")
						}
					}
				}
			},
		},
		{
			name: "成功获取内存缓存不存在，Redis存在的值 - 复杂结构",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 直接向Redis适配器设置数据
				layeredCache := cache.(*LayeredCache)
				ctx := context.Background()
				value := TestUser{ID: 456, Name: "Bob", Email: "bob@example.com"}
				data, err := layeredCache.Marshal(value)
				if err != nil {
					t.Fatalf("Marshal() error = %v", err)
				}
				err = layeredCache.remote.Set(ctx, "redis-user-key", data, 24*time.Hour)
				if err != nil {
					t.Fatalf("Redis Set() error = %v", err)
				}
			},
			key:       "redis-user-key",
			target:    new(TestUser),
			wantErr:   nil,
			wantValue: TestUser{ID: 456, Name: "Bob", Email: "bob@example.com"},
		},
		{
			name: "获取内存与Redis都不存在，没有loader时 - 返回NotFound",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			key:     "non-existent-key",
			target:  new(string),
			wantErr: errors.ErrNotFound,
		},
		{
			name: "获取内存与Redis都不存在，有loader，loader成功返回 - 字符串",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			key:    "loader-success-key",
			target: new(string),
			options: []GetOption{
				WithLoader(func(ctx context.Context, key string) (any, error) {
					return "loaded-value", nil
				}),
			},
			wantErr:   nil,
			wantValue: "loaded-value",
			validateFunc: func(t *testing.T, cache Cache, key string, target any) {
				// 验证loader加载的数据已缓存到内存和Redis
				validateKeyExists(t, cache, key)
			},
		},
		{
			name: "获取内存与Redis都不存在，有loader，loader成功返回 - 结构体",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			key:    "loader-user-key",
			target: new(TestUser),
			options: []GetOption{
				WithLoader(func(ctx context.Context, key string) (any, error) {
					return TestUser{ID: 789, Name: "LoadedUser", Email: "loaded@example.com"}, nil
				}),
			},
			wantErr:   nil,
			wantValue: TestUser{ID: 789, Name: "LoadedUser", Email: "loaded@example.com"},
		},
		{
			name: "获取内存与Redis都不存在，有loader，loader返回自定义error",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			key:    "loader-error-key",
			target: new(string),
			options: []GetOption{
				WithLoader(func(ctx context.Context, key string) (any, error) {
					return nil, errors.New("custom loader error")
				}),
			},
			wantErr: errors.New("custom loader error"),
		},
		{
			name: "获取内存与Redis都不存在，有loader，loader返回NotFound，没有空值缓存",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			key:    "loader-notfound-key",
			target: new(string),
			options: []GetOption{
				WithLoader(func(ctx context.Context, key string) (any, error) {
					return nil, errors.ErrNotFound
				}),
				WithCacheNotFound(false, 30*time.Second),
			},
			wantErr: errors.ErrNotFound,
			validateFunc: func(t *testing.T, cache Cache, key string, target any) {
				// 验证没有缓存空值
				layeredCache := cache.(*LayeredCache)
				if layeredCache.memory != nil {
					if _, exists := layeredCache.memory.Get(key); exists {
						t.Errorf("不应该缓存空值，但在内存中找到了键: %s", key)
					}
				}
				if layeredCache.remote != nil {
					if _, err := layeredCache.remote.Get(context.Background(), key); err == nil {
						t.Errorf("不应该缓存空值，但在Redis中找到了键: %s", key)
					}
				}
			},
		},
		{
			name: "获取内存与Redis都不存在，有loader，loader返回NotFound，有空值缓存",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			key:    "loader-notfound-cached-key",
			target: new(string),
			options: []GetOption{
				WithLoader(func(ctx context.Context, key string) (any, error) {
					return nil, errors.ErrNotFound
				}),
				WithCacheNotFound(true, 30*time.Second),
			},
			wantErr: errors.ErrNotFound,
			validateFunc: func(t *testing.T, cache Cache, key string, target any) {
				// 验证已经缓存了空值
				layeredCache := cache.(*LayeredCache)
				if layeredCache.memory != nil {
					if data, exists := layeredCache.memory.Get(key); exists {
						// 反序列化检查是否是空值占位符
						var result interface{}
						if err := layeredCache.Unmarshal(data, &result); err != nil {
							t.Errorf("内存缓存反序列化失败: %v", err)
						} else if !bytes.Equal(result.([]byte), notFoundPlaceholder) {
							t.Errorf("内存缓存的空值不正确: got %v, want %v", result, notFoundPlaceholder)
						}
					} else {
						t.Errorf("内存缓存中未找到空值")
					}
				}
				if layeredCache.remote != nil {
					if data, err := layeredCache.remote.Get(context.Background(), key); err == nil {
						// 反序列化检查是否是空值占位符
						var result interface{}
						if err := layeredCache.Unmarshal(data, &result); err != nil {
							t.Errorf("Redis缓存反序列化失败: %v", err)
						} else if !bytes.Equal(result.([]byte), notFoundPlaceholder) {
							t.Errorf("Redis缓存的空值不正确: got %v, want %v", result, notFoundPlaceholder)
						}
					} else {
						t.Errorf("Redis缓存中未找到空值: %v", err)
					}
				}
			},
		},
		{
			name: "获取内存缓存中存在的空值缓存",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 直接在内存缓存中设置空值占位符
				layeredCache := cache.(*LayeredCache)
				layeredCache.memory.Set("cached-notfound-key", notFoundPlaceholder, 5*time.Minute)
			},
			key:     "cached-notfound-key",
			target:  new(string),
			wantErr: errors.ErrNotFound,
		},
		{
			name: "获取Redis缓存中存在的空值缓存",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 直接在Redis缓存中设置空值占位符
				layeredCache := cache.(*LayeredCache)
				ctx := context.Background()
				err := layeredCache.remote.Set(ctx, "redis-cached-notfound-key", notFoundPlaceholder, time.Hour)
				if err != nil {
					t.Fatalf("Redis Set() error = %v", err)
				}
			},
			key:     "redis-cached-notfound-key",
			target:  new(string),
			wantErr: errors.ErrNotFound,
		},
		{
			name: "获取内存与Redis都不存在，有loader，loader返回nil值",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			key:    "loader-nil-key",
			target: new(string),
			options: []GetOption{
				WithLoader(func(ctx context.Context, key string) (any, error) {
					return nil, nil // 返回nil值
				}),
				WithCacheNotFound(false, 30*time.Second),
			},
			wantErr: errors.ErrNotFound,
		},
		{
			name: "获取内存与Redis都不存在，有loader，loader返回nil值，有空值缓存",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			key:    "loader-nil-cached-key",
			target: new(string),
			options: []GetOption{
				WithLoader(func(ctx context.Context, key string) (any, error) {
					return nil, nil // 返回nil值
				}),
				WithCacheNotFound(true, 30*time.Second),
			},
			wantErr: errors.ErrNotFound,
		},
		{
			name: "获取内存与Redis都不存在，有loader，自定义TTL",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			key:    "loader-custom-ttl-key",
			target: new(string),
			options: []GetOption{
				WithLoader(func(ctx context.Context, key string) (any, error) {
					return "custom-ttl-value", nil
				}),
				WithTTL(2*time.Minute, 10*time.Minute),
			},
			wantErr:   nil,
			wantValue: "custom-ttl-value",
		},
		{
			name: "失败 - 无效的内存TTL",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			key:    "invalid-memory-ttl-key",
			target: new(string),
			options: []GetOption{
				WithLoader(func(ctx context.Context, key string) (any, error) {
					return "test-value", nil
				}),
				WithTTL(0, time.Hour),
			},
			wantErr: errors.ErrInvalidMemoryExpireTime,
		},
		{
			name: "失败 - 无效的Redis TTL",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			key:    "invalid-redis-ttl-key",
			target: new(string),
			options: []GetOption{
				WithLoader(func(ctx context.Context, key string) (any, error) {
					return "test-value", nil
				}),
				WithTTL(time.Hour, 0),
			},
			wantErr: errors.ErrInvalidRedisExpireTime,
		},
		{
			name: "失败 - 无效的空值缓存TTL",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			key:    "invalid-cache-notfound-ttl-key",
			target: new(string),
			options: []GetOption{
				WithLoader(func(ctx context.Context, key string) (any, error) {
					return nil, errors.ErrNotFound
				}),
				WithCacheNotFound(true, 0),
			},
			wantErr: errors.ErrInvalidCacheNotFondTTL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := tt.setupCache(t)
			tt.setupData(t, cache)

			ctx := context.Background()
			err := cache.Get(ctx, tt.key, tt.target, tt.options...)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("Get() expected error %v, got nil", tt.wantErr)
					return
				}
				// 对于预定义的错误，使用 errors.Is 比较
				if errors.Is(tt.wantErr, errors.ErrNotFound) ||
					errors.Is(tt.wantErr, errors.ErrInvalidMemoryExpireTime) ||
					errors.Is(tt.wantErr, errors.ErrInvalidRedisExpireTime) ||
					errors.Is(tt.wantErr, errors.ErrInvalidCacheNotFondTTL) {
					if !errors.Is(err, tt.wantErr) {
						t.Errorf("Get() error = %v, want %v", err, tt.wantErr)
					}
				} else {
					// 对于自定义错误，使用字符串比较
					if err.Error() != tt.wantErr.Error() {
						t.Errorf("Get() error = %v, want %v", err, tt.wantErr)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("Get() unexpected error = %v", err)
				return
			}

			// 验证返回值
			if tt.wantValue != nil {
				validateGetResult(t, tt.target, tt.wantValue)
			}

			// 执行自定义验证
			if tt.validateFunc != nil {
				tt.validateFunc(t, cache, tt.key, tt.target)
			}
		})
	}
}

func TestLayeredCache_Get_MemoryOnly(t *testing.T) {
	cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	key := "memory-only-get-key"
	value := "memory-only-get-value"

	// 设置数据
	err = cache.Set(ctx, key, value)
	if err != nil {
		t.Errorf("Set() error = %v", err)
		return
	}

	// 获取数据
	var result string
	err = cache.Get(ctx, key, &result)
	if err != nil {
		t.Errorf("Get() error = %v", err)
		return
	}

	if result != value {
		t.Errorf("Get() result = %v, want %v", result, value)
	}
}

func TestLayeredCache_Get_RedisOnly(t *testing.T) {
	cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	key := "redis-only-get-key"
	value := "redis-only-get-value"

	// 设置数据
	err = cache.Set(ctx, key, value)
	if err != nil {
		t.Errorf("Set() error = %v", err)
		return
	}

	// 获取数据
	var result string
	err = cache.Get(ctx, key, &result)
	if err != nil {
		t.Errorf("Get() error = %v", err)
		return
	}

	if result != value {
		t.Errorf("Get() result = %v, want %v", result, value)
	}
}

func TestLayeredCache_Get_BothCaches(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	key := "both-caches-get-key"
	value := TestUser{ID: 999, Name: "GetUser", Email: "get@example.com"}

	// 设置数据
	err = cache.Set(ctx, key, value)
	if err != nil {
		t.Errorf("Set() error = %v", err)
		return
	}

	// 获取数据
	var result TestUser
	err = cache.Get(ctx, key, &result)
	if err != nil {
		t.Errorf("Get() error = %v", err)
		return
	}

	if result != value {
		t.Errorf("Get() result = %v, want %v", result, value)
	}
}

func TestLayeredCache_Get_ComplexTypes(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name      string
		key       string
		setValue  any
		getTarget any
		wantValue any
	}{
		{
			name:      "数组类型",
			key:       "array-get-key",
			setValue:  []int{1, 2, 3, 4, 5},
			getTarget: new([]int),
			wantValue: []int{1, 2, 3, 4, 5},
		},
		{
			name: "映射类型",
			key:  "map-get-key",
			setValue: map[string]int{
				"one":   1,
				"two":   2,
				"three": 3,
			},
			getTarget: new(map[string]int),
			wantValue: map[string]int{
				"one":   1,
				"two":   2,
				"three": 3,
			},
		},
		{
			name: "嵌套结构",
			key:  "nested-get-key",
			setValue: TestNestedStruct{
				User: TestUser{ID: 111, Name: "NestedGet", Email: "nested@example.com"},
				Tags: []string{"get", "nested"},
			},
			getTarget: new(TestNestedStruct),
			wantValue: TestNestedStruct{
				User: TestUser{ID: 111, Name: "NestedGet", Email: "nested@example.com"},
				Tags: []string{"get", "nested"},
			},
		},
		{
			name:      "字节数组",
			key:       "bytes-get-key",
			setValue:  []byte("binary-get-data"),
			getTarget: new([]byte),
			wantValue: []byte("binary-get-data"),
		},
		{
			name:      "空字符串",
			key:       "empty-string-get-key",
			setValue:  "",
			getTarget: new(string),
			wantValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置数据
			err := cache.Set(ctx, tt.key, tt.setValue)
			if err != nil {
				t.Errorf("Set() error = %v", err)
				return
			}

			// 获取数据
			err = cache.Get(ctx, tt.key, tt.getTarget)
			if err != nil {
				t.Errorf("Get() error = %v", err)
				return
			}

			// 验证结果
			validateGetResult(t, tt.getTarget, tt.wantValue)
		})
	}
}

func TestLayeredCache_Get_WithLoader_Success(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	key := "loader-success-get-key"
	expectedValue := "loaded-success-value"

	// 使用loader获取数据
	var result string
	err = cache.Get(ctx, key, &result, WithLoader(func(ctx context.Context, key string) (any, error) {
		return expectedValue, nil
	}))
	if err != nil {
		t.Errorf("Get() error = %v", err)
		return
	}

	if result != expectedValue {
		t.Errorf("Get() result = %v, want %v", result, expectedValue)
	}

	// 验证数据已缓存
	validateKeyExists(t, cache, key)

	// 再次获取，应该从缓存中获取
	var cachedResult string
	err = cache.Get(ctx, key, &cachedResult) // 没有loader
	if err != nil {
		t.Errorf("Get() from cache error = %v", err)
		return
	}

	if cachedResult != expectedValue {
		t.Errorf("Get() cached result = %v, want %v", cachedResult, expectedValue)
	}
}

func TestLayeredCache_Get_WithLoader_Error(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	key := "loader-error-get-key"
	expectedError := errors.New("loader custom error")

	// 使用loader获取数据
	var result string
	err = cache.Get(ctx, key, &result, WithLoader(func(ctx context.Context, key string) (any, error) {
		return nil, expectedError
	}))

	if err == nil {
		t.Error("Get() expected error, got nil")
		return
	}

	if err.Error() != expectedError.Error() {
		t.Errorf("Get() error = %v, want %v", err, expectedError)
	}
}

func TestLayeredCache_Get_WithLoader_NotFound(t *testing.T) {
	tests := []struct {
		name          string
		cacheNotFound bool
		expectCached  bool
	}{
		{
			name:          "不缓存空值",
			cacheNotFound: false,
			expectCached:  false,
		},
		{
			name:          "缓存空值",
			cacheNotFound: true,
			expectCached:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := NewCache(
				WithConfigMemory(createMemoryAdapter(t)),
				WithConfigRemote(createRemoteAdapter(t)),
			)
			if err != nil {
				t.Fatalf("NewCache() error = %v", err)
			}

			ctx := context.Background()
			key := fmt.Sprintf("loader-notfound-get-key-%v", tt.cacheNotFound)

			// 使用loader获取数据
			var result string
			err = cache.Get(ctx, key, &result, WithLoader(func(ctx context.Context, key string) (any, error) {
				return nil, errors.ErrNotFound
			}), WithCacheNotFound(tt.cacheNotFound, 30*time.Second))

			if !errors.Is(err, errors.ErrNotFound) {
				t.Errorf("Get() error = %v, want %v", err, errors.ErrNotFound)
				return
			}

			// 验证缓存状态
			layeredCache := cache.(*LayeredCache)
			if tt.expectCached {
				// 应该缓存空值
				if layeredCache.memory != nil {
					if data, exists := layeredCache.memory.Get(key); !exists {
						t.Error("空值应该被缓存到内存，但未找到")
					} else {
						if !bytes.Equal(data, notFoundPlaceholder) {
							t.Errorf("内存缓存的空值不正确: got %v, want %v", result, notFoundPlaceholder)
						}
					}
				}
				if layeredCache.remote != nil {
					if data, err := layeredCache.remote.Get(ctx, key); err != nil {
						t.Errorf("空值应该被缓存到Redis，但未找到: %v", err)
					} else {
						if !bytes.Equal(data, notFoundPlaceholder) {
							t.Errorf("Redis缓存的空值不正确: got %v, want %v", result, notFoundPlaceholder)
						}
					}
				}
			} else {
				// 不应该缓存空值
				if layeredCache.memory != nil {
					if _, exists := layeredCache.memory.Get(key); exists {
						t.Error("空值不应该被缓存到内存，但找到了")
					}
				}
				if layeredCache.remote != nil {
					if _, err := layeredCache.remote.Get(ctx, key); err == nil {
						t.Error("空值不应该被缓存到Redis，但找到了")
					}
				}
			}
		})
	}
}

func TestLayeredCache_Get_ContextCancellation(t *testing.T) {
	cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	// 创建已取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	key := "context-cancel-get-key"
	var result string

	err = cache.Get(ctx, key, &result)
	if err == nil {
		t.Error("Get() with cancelled context expected error, got nil")
	}
}

func TestLayeredCache_Get_SingleFlight(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	key := "singleflight-get-key"
	expectedValue := "singleflight-value"

	// 计数器，用于检测loader调用次数
	var loaderCallCount int32
	loader := func(ctx context.Context, key string) (any, error) {
		// 使用原子操作增加计数器
		count := atomic.AddInt32(&loaderCallCount, 1)

		// 模拟耗时操作
		time.Sleep(100 * time.Millisecond)

		return fmt.Sprintf("%s-%d", expectedValue, count), nil
	}

	// 并发调用Get方法
	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make([]string, numGoroutines)
	errors := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			var result string
			err := cache.Get(ctx, key, &result, WithLoader(loader))
			results[index] = result
			errors[index] = err
		}(i)
	}

	wg.Wait()

	// 验证所有调用都成功
	for i, err := range errors {
		if err != nil {
			t.Errorf("Get() goroutine %d error = %v", i, err)
		}
	}

	// 验证所有结果都相同（singleflight生效）
	firstResult := results[0]
	for i, result := range results {
		if result != firstResult {
			t.Errorf("Get() goroutine %d result = %v, want %v", i, result, firstResult)
		}
	}

	// 验证loader只被调用一次
	finalCount := atomic.LoadInt32(&loaderCallCount)
	if finalCount != 1 {
		t.Errorf("Loader called %d times, want 1", finalCount)
	}
}

func TestLayeredCache_Get_WriteBackFromRedis(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	key := "writeback-get-key"
	value := "writeback-get-value"

	// 只在Redis中设置数据
	layeredCache := cache.(*LayeredCache)
	data, err := layeredCache.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	err = layeredCache.remote.Set(ctx, key, data, time.Hour)
	if err != nil {
		t.Fatalf("Redis Set() error = %v", err)
	}

	// 验证内存中没有数据
	if _, exists := layeredCache.memory.Get(key); exists {
		t.Error("内存中不应该有数据")
	}

	// 获取数据
	var result string
	err = cache.Get(ctx, key, &result)
	if err != nil {
		t.Errorf("Get() error = %v", err)
		return
	}

	if result != value {
		t.Errorf("Get() result = %v, want %v", result, value)
	}

	// 验证数据已经写回内存
	if memData, exists := layeredCache.memory.Get(key); !exists {
		t.Error("数据应该写回内存，但未找到")
	} else {
		var memResult string
		err = layeredCache.Unmarshal(memData, &memResult)
		if err != nil {
			t.Errorf("内存数据反序列化失败: %v", err)
		} else if memResult != value {
			t.Errorf("内存数据 = %v, want %v", memResult, value)
		}
	}
}

func TestLayeredCache_Get_CustomTTL(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	key := "custom-ttl-get-key"
	expectedValue := "custom-ttl-value"

	// 使用自定义TTL的loader获取数据
	var result string
	err = cache.Get(ctx, key, &result,
		WithLoader(func(ctx context.Context, key string) (any, error) {
			return expectedValue, nil
		}),
		WithTTL(2*time.Minute, 10*time.Minute),
	)
	if err != nil {
		t.Errorf("Get() error = %v", err)
		return
	}

	if result != expectedValue {
		t.Errorf("Get() result = %v, want %v", result, expectedValue)
	}

	// 验证数据已缓存
	validateKeyExists(t, cache, key)

	// 验证Redis TTL（这里只能验证TTL存在且合理）
	layeredCache := cache.(*LayeredCache)
	if layeredCache.remote != nil {
		ttl, err := layeredCache.remote.TTL(ctx, key)
		if err != nil {
			t.Errorf("TTL() error = %v", err)
		} else if ttl <= 0 || ttl > 10*time.Minute {
			t.Errorf("TTL = %v, want > 0 and <= 10m", ttl)
		}
	}
}

// validateGetResult 验证Get方法的结果
func validateGetResult(t *testing.T, target any, expected any) {
	t.Helper()

	// 使用反射获取target的实际值
	targetVal := reflect.ValueOf(target)
	if targetVal.Kind() != reflect.Ptr {
		t.Errorf("Target must be a pointer, got %T", target)
		return
	}
	actualVal := targetVal.Elem().Interface()

	// 使用深度比较
	if !reflect.DeepEqual(actualVal, expected) {
		t.Errorf("Get result = %v, want %v", actualVal, expected)
	}
}

func TestLayeredCache_MGet(t *testing.T) {
	tests := []struct {
		name         string
		setupCache   func(t *testing.T) Cache
		setupData    func(t *testing.T, cache Cache) // 预设数据
		keys         []string
		target       any
		options      []GetOption
		wantErr      error
		wantResult   map[string]any
		validateFunc func(t *testing.T, cache Cache, keys []string, target any)
	}{
		{
			name: "成功从内存缓存批量获取 - 字符串",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				ctx := context.Background()
				keyValues := map[string]any{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				}
				err := cache.MSet(ctx, keyValues)
				if err != nil {
					t.Fatalf("MSet() error = %v", err)
				}
			},
			keys:   []string{"key1", "key2", "key3"},
			target: &map[string]string{},
			wantResult: map[string]any{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
		},
		{
			name: "成功从内存缓存批量获取 - 结构体",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				ctx := context.Background()
				keyValues := map[string]any{
					"user1": TestUser{ID: 1, Name: "User1", Email: "user1@example.com"},
					"user2": TestUser{ID: 2, Name: "User2", Email: "user2@example.com"},
				}
				err := cache.MSet(ctx, keyValues)
				if err != nil {
					t.Fatalf("MSet() error = %v", err)
				}
			},
			keys:   []string{"user1", "user2"},
			target: &map[string]TestUser{},
			wantResult: map[string]any{
				"user1": TestUser{ID: 1, Name: "User1", Email: "user1@example.com"},
				"user2": TestUser{ID: 2, Name: "User2", Email: "user2@example.com"},
			},
		},
		{
			name: "成功从内存缓存部分获取 - 部分键存在",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				ctx := context.Background()
				keyValues := map[string]any{
					"existing1": "value1",
					"existing2": "value2",
				}
				err := cache.MSet(ctx, keyValues)
				if err != nil {
					t.Fatalf("MSet() error = %v", err)
				}
			},
			keys:   []string{"existing1", "missing", "existing2"},
			target: &map[string]string{},
			wantResult: map[string]any{
				"existing1": "value1",
				"existing2": "value2",
				// "missing" 不应该出现在结果中
			},
		},
		{
			name: "成功从Redis缓存批量获取并回写内存",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 直接在Redis中设置数据，避免写入内存
				layeredCache := cache.(*LayeredCache)
				ctx := context.Background()
				keyValues := map[string]any{
					"redis1": "redis-value1",
					"redis2": "redis-value2",
				}
				serializedData := make(map[string][]byte)
				for key, value := range keyValues {
					data, err := layeredCache.Marshal(value)
					if err != nil {
						t.Fatalf("Marshal() error = %v", err)
					}
					serializedData[key] = data
				}
				err := layeredCache.remote.MSet(ctx, serializedData, 24*time.Hour)
				if err != nil {
					t.Fatalf("Redis MSet() error = %v", err)
				}
			},
			keys:   []string{"redis1", "redis2"},
			target: &map[string]string{},
			wantResult: map[string]any{
				"redis1": "redis-value1",
				"redis2": "redis-value2",
			},
			validateFunc: func(t *testing.T, cache Cache, keys []string, target any) {
				// 验证数据已经从Redis回写到内存缓存
				layeredCache := cache.(*LayeredCache)
				if layeredCache.memory != nil {
					for _, key := range keys {
						if _, exists := layeredCache.memory.Get(key); !exists {
							t.Errorf("键 %s 未从Redis回写到内存缓存", key)
						}
					}
				}
			},
		},
		{
			name: "成功混合从内存和Redis批量获取",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				ctx := context.Background()
				// 在内存中设置一些数据
				memoryData := map[string]any{
					"memory1": "memory-value1",
					"memory2": "memory-value2",
				}
				err := cache.MSet(ctx, memoryData)
				if err != nil {
					t.Fatalf("MSet() error = %v", err)
				}

				// 直接在Redis中设置一些数据
				layeredCache := cache.(*LayeredCache)
				redisData := map[string]any{
					"redis1": "redis-value1",
					"redis2": "redis-value2",
				}
				serializedData := make(map[string][]byte)
				for key, value := range redisData {
					data, err := layeredCache.Marshal(value)
					if err != nil {
						t.Fatalf("Marshal() error = %v", err)
					}
					serializedData[key] = data
				}
				err = layeredCache.remote.MSet(ctx, serializedData, 24*time.Hour)
				if err != nil {
					t.Fatalf("Redis MSet() error = %v", err)
				}
			},
			keys:   []string{"memory1", "redis1", "memory2", "redis2"},
			target: &map[string]string{},
			wantResult: map[string]any{
				"memory1": "memory-value1",
				"memory2": "memory-value2",
				"redis1":  "redis-value1",
				"redis2":  "redis-value2",
			},
		},
		{
			name: "成功使用batchLoader批量加载",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何缓存数据
			},
			keys:   []string{"load1", "load2", "load3"},
			target: &map[string]string{},
			options: []GetOption{
				WithBatchLoader(func(ctx context.Context, keys []string) (map[string]any, error) {
					result := make(map[string]any)
					for _, key := range keys {
						result[key] = "loaded-" + key
					}
					return result, nil
				}),
			},
			wantResult: map[string]any{
				"load1": "loaded-load1",
				"load2": "loaded-load2",
				"load3": "loaded-load3",
			},
			validateFunc: func(t *testing.T, cache Cache, keys []string, target any) {
				// 验证batchLoader加载的数据已缓存
				for _, key := range keys {
					validateKeyExists(t, cache, key)
				}
			},
		},
		{
			name: "成功混合缓存和batchLoader",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				ctx := context.Background()
				// 在缓存中设置部分数据
				cachedData := map[string]any{
					"cached1": "cached-value1",
					"cached2": "cached-value2",
				}
				err := cache.MSet(ctx, cachedData)
				if err != nil {
					t.Fatalf("MSet() error = %v", err)
				}
			},
			keys:   []string{"cached1", "load1", "cached2", "load2"},
			target: &map[string]string{},
			options: []GetOption{
				WithBatchLoader(func(ctx context.Context, keys []string) (map[string]any, error) {
					result := make(map[string]any)
					for _, key := range keys {
						result[key] = "loaded-" + key
					}
					return result, nil
				}),
			},
			wantResult: map[string]any{
				"cached1": "cached-value1",
				"cached2": "cached-value2",
				"load1":   "loaded-load1",
				"load2":   "loaded-load2",
			},
		},
		{
			name: "成功处理空键列表",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			keys:       []string{},
			target:     &map[string]string{},
			wantResult: map[string]any{},
		},
		{
			name: "成功处理全部不存在的键",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			keys:       []string{"missing1", "missing2", "missing3"},
			target:     &map[string]string{},
			wantResult: map[string]any{},
		},
		{
			name: "成功处理batchLoader返回部分键",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			keys:   []string{"load1", "load2", "missing"},
			target: &map[string]string{},
			options: []GetOption{
				WithBatchLoader(func(ctx context.Context, keys []string) (map[string]any, error) {
					result := make(map[string]any)
					for _, key := range keys {
						if key != "missing" {
							result[key] = "loaded-" + key
						}
					}
					return result, nil
				}),
			},
			wantResult: map[string]any{
				"load1": "loaded-load1",
				"load2": "loaded-load2",
				// "missing" 不应该出现在结果中
			},
		},
		{
			name: "成功处理batchLoader返回nil值",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			keys:   []string{"load1", "nil-key", "load2"},
			target: &map[string]string{},
			options: []GetOption{
				WithBatchLoader(func(ctx context.Context, keys []string) (map[string]any, error) {
					result := make(map[string]any)
					for _, key := range keys {
						if key == "nil-key" {
							result[key] = nil
						} else {
							result[key] = "loaded-" + key
						}
					}
					return result, nil
				}),
				WithCacheNotFound(false, 30*time.Second),
			},
			wantResult: map[string]any{
				"load1": "loaded-load1",
				"load2": "loaded-load2",
				// "nil-key" 不应该出现在结果中（因为不缓存空值）
			},
		},
		{
			name: "成功处理batchLoader返回nil值并缓存空值",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			keys:   []string{"load1", "nil-key", "load2"},
			target: &map[string]string{},
			options: []GetOption{
				WithBatchLoader(func(ctx context.Context, keys []string) (map[string]any, error) {
					result := make(map[string]any)
					for _, key := range keys {
						if key == "nil-key" {
							result[key] = nil
						} else {
							result[key] = "loaded-" + key
						}
					}
					return result, nil
				}),
				WithCacheNotFound(true, 30*time.Second),
			},
			wantResult: map[string]any{
				"load1": "loaded-load1",
				"load2": "loaded-load2",
				// "nil-key" 不应该出现在结果中（即使缓存了空值）
			},
			validateFunc: func(t *testing.T, cache Cache, keys []string, target any) {
				// 验证空值已被缓存
				layeredCache := cache.(*LayeredCache)
				if layeredCache.memory != nil {
					if data, exists := layeredCache.memory.Get("nil-key"); !exists {
						t.Errorf("空值应该被缓存到内存，但未找到")
					} else if !bytes.Equal(data, notFoundPlaceholder) {
						t.Errorf("内存缓存的空值不正确")
					}
				}
			},
		},
		{
			name: "失败 - 无效的target类型（非指针）",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			keys:    []string{"key1"},
			target:  map[string]string{}, // 不是指针
			wantErr: errors.ErrInvalidMGetTarget,
		},
		{
			name: "失败 - 无效的target类型（非map）",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			keys:    []string{"key1"},
			target:  &[]string{}, // 不是map
			wantErr: errors.ErrInvalidMGetTarget,
		},
		{
			name: "失败 - 无效的target类型（map key不是string）",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			keys:    []string{"key1"},
			target:  &map[int]string{}, // key不是string
			wantErr: errors.ErrInvalidMGetTarget,
		},
		{
			name: "失败 - nil target",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			keys:    []string{"key1"},
			target:  nil,
			wantErr: errors.ErrInvalidMGetTarget,
		},
		{
			name: "失败 - batchLoader返回错误",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(
					WithConfigMemory(createMemoryAdapter(t)),
					WithConfigRemote(createRemoteAdapter(t)),
				)
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			keys:   []string{"key1", "key2"},
			target: &map[string]string{},
			options: []GetOption{
				WithBatchLoader(func(ctx context.Context, keys []string) (map[string]any, error) {
					return nil, errors.New("batchLoader custom error")
				}),
			},
			wantErr: errors.New("batchLoader custom error"),
		},
		{
			name: "失败 - 无效的内存TTL",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			keys:   []string{"key1"},
			target: &map[string]string{},
			options: []GetOption{
				WithBatchLoader(func(ctx context.Context, keys []string) (map[string]any, error) {
					return map[string]any{"key1": "value1"}, nil
				}),
				WithTTL(0, time.Hour),
			},
			wantErr: errors.ErrInvalidMemoryExpireTime,
		},
		{
			name: "失败 - 无效的Redis TTL",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			keys:   []string{"key1"},
			target: &map[string]string{},
			options: []GetOption{
				WithBatchLoader(func(ctx context.Context, keys []string) (map[string]any, error) {
					return map[string]any{"key1": "value1"}, nil
				}),
				WithTTL(time.Hour, 0),
			},
			wantErr: errors.ErrInvalidRedisExpireTime,
		},
		{
			name: "失败 - 无效的空值缓存TTL",
			setupCache: func(t *testing.T) Cache {
				cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
				if err != nil {
					t.Fatalf("NewCache() error = %v", err)
				}
				return cache
			},
			setupData: func(t *testing.T, cache Cache) {
				// 不设置任何数据
			},
			keys:   []string{"key1"},
			target: &map[string]string{},
			options: []GetOption{
				WithBatchLoader(func(ctx context.Context, keys []string) (map[string]any, error) {
					return map[string]any{"key1": nil}, nil
				}),
				WithCacheNotFound(true, 0),
			},
			wantErr: errors.ErrInvalidCacheNotFondTTL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := tt.setupCache(t)
			tt.setupData(t, cache)

			ctx := context.Background()
			err := cache.MGet(ctx, tt.keys, tt.target, tt.options...)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("MGet() expected error %v, got nil", tt.wantErr)
					return
				}
				// 对于预定义的错误，使用 errors.Is 比较
				if errors.Is(tt.wantErr, errors.ErrInvalidMGetTarget) ||
					errors.Is(tt.wantErr, errors.ErrInvalidMemoryExpireTime) ||
					errors.Is(tt.wantErr, errors.ErrInvalidRedisExpireTime) ||
					errors.Is(tt.wantErr, errors.ErrInvalidCacheNotFondTTL) {
					if !errors.Is(err, tt.wantErr) {
						t.Errorf("MGet() error = %v, want %v", err, tt.wantErr)
					}
				} else {
					// 对于自定义错误，使用字符串比较
					if err.Error() != tt.wantErr.Error() {
						t.Errorf("MGet() error = %v, want %v", err, tt.wantErr)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("MGet() unexpected error = %v", err)
				return
			}

			// 验证返回结果
			if tt.wantResult != nil {
				validateMGetResult(t, tt.target, tt.wantResult)
			}

			// 执行自定义验证
			if tt.validateFunc != nil {
				tt.validateFunc(t, cache, tt.keys, tt.target)
			}
		})
	}
}

func TestLayeredCache_MGet_MemoryOnly(t *testing.T) {
	cache, err := NewCache(WithConfigMemory(createMemoryAdapter(t)))
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	keyValues := map[string]any{
		"memory-key-1": "memory-value-1",
		"memory-key-2": "memory-value-2",
		"memory-key-3": "memory-value-3",
	}

	// 设置数据
	err = cache.MSet(ctx, keyValues)
	if err != nil {
		t.Errorf("MSet() error = %v", err)
		return
	}

	// 获取数据
	keys := []string{"memory-key-1", "memory-key-2", "memory-key-3"}
	var result map[string]string
	err = cache.MGet(ctx, keys, &result)
	if err != nil {
		t.Errorf("MGet() error = %v", err)
		return
	}

	expected := map[string]string{
		"memory-key-1": "memory-value-1",
		"memory-key-2": "memory-value-2",
		"memory-key-3": "memory-value-3",
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("MGet() result = %v, want %v", result, expected)
	}
}

func TestLayeredCache_MGet_RedisOnly(t *testing.T) {
	cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	keyValues := map[string]any{
		"redis-key-1": "redis-value-1",
		"redis-key-2": "redis-value-2",
		"redis-key-3": "redis-value-3",
	}

	// 设置数据
	err = cache.MSet(ctx, keyValues)
	if err != nil {
		t.Errorf("MSet() error = %v", err)
		return
	}

	// 获取数据
	keys := []string{"redis-key-1", "redis-key-2", "redis-key-3"}
	var result map[string]string
	err = cache.MGet(ctx, keys, &result)
	if err != nil {
		t.Errorf("MGet() error = %v", err)
		return
	}

	expected := map[string]string{
		"redis-key-1": "redis-value-1",
		"redis-key-2": "redis-value-2",
		"redis-key-3": "redis-value-3",
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("MGet() result = %v, want %v", result, expected)
	}
}

func TestLayeredCache_MGet_BothCaches(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	keyValues := map[string]any{
		"both-key-1": TestUser{ID: 1, Name: "User1", Email: "user1@example.com"},
		"both-key-2": TestUser{ID: 2, Name: "User2", Email: "user2@example.com"},
		"both-key-3": TestUser{ID: 3, Name: "User3", Email: "user3@example.com"},
	}

	// 设置数据
	err = cache.MSet(ctx, keyValues)
	if err != nil {
		t.Errorf("MSet() error = %v", err)
		return
	}

	// 获取数据
	keys := []string{"both-key-1", "both-key-2", "both-key-3"}
	var result map[string]TestUser
	err = cache.MGet(ctx, keys, &result)
	if err != nil {
		t.Errorf("MGet() error = %v", err)
		return
	}

	expected := map[string]TestUser{
		"both-key-1": TestUser{ID: 1, Name: "User1", Email: "user1@example.com"},
		"both-key-2": TestUser{ID: 2, Name: "User2", Email: "user2@example.com"},
		"both-key-3": TestUser{ID: 3, Name: "User3", Email: "user3@example.com"},
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("MGet() result = %v, want %v", result, expected)
	}
}

func TestLayeredCache_MGet_ComplexTypes(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name      string
		keyValues map[string]any
		keys      []string
		target    any
		expected  any
	}{
		{
			name: "数组类型",
			keyValues: map[string]any{
				"array1": []int{1, 2, 3},
				"array2": []int{4, 5, 6},
			},
			keys:   []string{"array1", "array2"},
			target: &map[string][]int{},
			expected: map[string][]int{
				"array1": {1, 2, 3},
				"array2": {4, 5, 6},
			},
		},
		{
			name: "映射类型",
			keyValues: map[string]any{
				"map1": map[string]int{"a": 1, "b": 2},
				"map2": map[string]int{"c": 3, "d": 4},
			},
			keys:   []string{"map1", "map2"},
			target: &map[string]map[string]int{},
			expected: map[string]map[string]int{
				"map1": {"a": 1, "b": 2},
				"map2": {"c": 3, "d": 4},
			},
		},
		{
			name: "嵌套结构",
			keyValues: map[string]any{
				"nested1": TestNestedStruct{
					User: TestUser{ID: 1, Name: "Nested1", Email: "nested1@example.com"},
					Tags: []string{"tag1", "tag2"},
				},
				"nested2": TestNestedStruct{
					User: TestUser{ID: 2, Name: "Nested2", Email: "nested2@example.com"},
					Tags: []string{"tag3", "tag4"},
				},
			},
			keys:   []string{"nested1", "nested2"},
			target: &map[string]TestNestedStruct{},
			expected: map[string]TestNestedStruct{
				"nested1": {
					User: TestUser{ID: 1, Name: "Nested1", Email: "nested1@example.com"},
					Tags: []string{"tag1", "tag2"},
				},
				"nested2": {
					User: TestUser{ID: 2, Name: "Nested2", Email: "nested2@example.com"},
					Tags: []string{"tag3", "tag4"},
				},
			},
		},
		{
			name: "字节数组",
			keyValues: map[string]any{
				"bytes1": []byte("binary-data-1"),
				"bytes2": []byte("binary-data-2"),
			},
			keys:   []string{"bytes1", "bytes2"},
			target: &map[string][]byte{},
			expected: map[string][]byte{
				"bytes1": []byte("binary-data-1"),
				"bytes2": []byte("binary-data-2"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置数据
			err := cache.MSet(ctx, tt.keyValues)
			if err != nil {
				t.Errorf("MSet() error = %v", err)
				return
			}

			// 获取数据
			err = cache.MGet(ctx, tt.keys, tt.target)
			if err != nil {
				t.Errorf("MGet() error = %v", err)
				return
			}

			// 验证结果
			validateMGetResult(t, tt.target, tt.expected)
		})
	}
}

func TestLayeredCache_MGet_WithBatchLoader(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	keys := []string{"load-key-1", "load-key-2", "load-key-3"}

	// 使用batchLoader获取数据
	var result map[string]string
	err = cache.MGet(ctx, keys, &result, WithBatchLoader(func(ctx context.Context, keys []string) (map[string]any, error) {
		data := make(map[string]any)
		for _, key := range keys {
			data[key] = "loaded-" + key
		}
		return data, nil
	}))
	if err != nil {
		t.Errorf("MGet() error = %v", err)
		return
	}

	expected := map[string]string{
		"load-key-1": "loaded-load-key-1",
		"load-key-2": "loaded-load-key-2",
		"load-key-3": "loaded-load-key-3",
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("MGet() result = %v, want %v", result, expected)
	}

	// 验证数据已缓存
	for _, key := range keys {
		validateKeyExists(t, cache, key)
	}

	// 再次获取，应该从缓存中获取
	var cachedResult map[string]string
	err = cache.MGet(ctx, keys, &cachedResult) // 没有batchLoader
	if err != nil {
		t.Errorf("MGet() from cache error = %v", err)
		return
	}

	if !reflect.DeepEqual(cachedResult, expected) {
		t.Errorf("MGet() cached result = %v, want %v", cachedResult, expected)
	}
}

func TestLayeredCache_MGet_ContextCancellation(t *testing.T) {
	cache, err := NewCache(WithConfigRemote(createRemoteAdapter(t)))
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	// 创建已取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	keys := []string{"context-cancel-key-1", "context-cancel-key-2"}
	var result map[string]string

	err = cache.MGet(ctx, keys, &result)
	if err == nil {
		t.Error("MGet() with cancelled context expected error, got nil")
	}
}

func TestLayeredCache_MGet_SingleFlight(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	keys := []string{"sf-key-1", "sf-key-2", "sf-key-3"}

	// 计数器，用于检测batchLoader调用次数
	var batchLoaderCallCount int32
	batchLoader := func(ctx context.Context, keys []string) (map[string]any, error) {
		// 使用原子操作增加计数器
		count := atomic.AddInt32(&batchLoaderCallCount, 1)

		// 模拟耗时操作
		time.Sleep(100 * time.Millisecond)

		result := make(map[string]any)
		for _, key := range keys {
			result[key] = fmt.Sprintf("loaded-%s-%d", key, count)
		}
		return result, nil
	}

	// 并发调用MGet方法
	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make([]map[string]string, numGoroutines)
	errorList := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			var result map[string]string
			err := cache.MGet(ctx, keys, &result, WithBatchLoader(batchLoader))
			results[index] = result
			errorList[index] = err
		}(i)
	}

	wg.Wait()

	// 验证所有调用都成功
	for i, err := range errorList {
		if err != nil {
			t.Errorf("MGet() goroutine %d error = %v", i, err)
		}
	}

	// 验证所有结果都相同（singleflight生效）
	firstResult := results[0]
	for i, result := range results {
		if !reflect.DeepEqual(result, firstResult) {
			t.Errorf("MGet() goroutine %d result = %v, want %v", i, result, firstResult)
		}
	}

	// 验证batchLoader只被调用一次
	finalCount := atomic.LoadInt32(&batchLoaderCallCount)
	if finalCount != 1 {
		t.Errorf("BatchLoader called %d times, want 1", finalCount)
	}
}

func TestLayeredCache_MGet_PartialHit(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()

	// 在内存中设置部分数据
	memoryData := map[string]any{
		"memory-key-1": "memory-value-1",
		"memory-key-2": "memory-value-2",
	}
	err = cache.MSet(ctx, memoryData)
	if err != nil {
		t.Errorf("MSet() error = %v", err)
		return
	}

	// 直接在Redis中设置其他数据
	layeredCache := cache.(*LayeredCache)
	redisData := map[string]any{
		"redis-key-1": "redis-value-1",
		"redis-key-2": "redis-value-2",
	}
	serializedData := make(map[string][]byte)
	for key, value := range redisData {
		data, err := layeredCache.Marshal(value)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		serializedData[key] = data
	}
	err = layeredCache.remote.MSet(ctx, serializedData, 24*time.Hour)
	if err != nil {
		t.Fatalf("Redis MSet() error = %v", err)
	}

	// 获取混合数据（包括需要batchLoader的键）
	keys := []string{"memory-key-1", "redis-key-1", "load-key-1", "memory-key-2", "redis-key-2", "load-key-2"}
	var result map[string]string
	err = cache.MGet(ctx, keys, &result, WithBatchLoader(func(ctx context.Context, keys []string) (map[string]any, error) {
		data := make(map[string]any)
		for _, key := range keys {
			data[key] = "loaded-" + key
		}
		return data, nil
	}))
	if err != nil {
		t.Errorf("MGet() error = %v", err)
		return
	}

	expected := map[string]string{
		"memory-key-1": "memory-value-1",
		"memory-key-2": "memory-value-2",
		"redis-key-1":  "redis-value-1",
		"redis-key-2":  "redis-value-2",
		"load-key-1":   "loaded-load-key-1",
		"load-key-2":   "loaded-load-key-2",
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("MGet() result = %v, want %v", result, expected)
	}

	// 验证Redis数据已回写到内存
	for key := range redisData {
		if _, exists := layeredCache.memory.Get(key); !exists {
			t.Errorf("Redis数据 %s 未回写到内存", key)
		}
	}
}

func TestLayeredCache_MGet_CustomTTL(t *testing.T) {
	cache, err := NewCache(
		WithConfigMemory(createMemoryAdapter(t)),
		WithConfigRemote(createRemoteAdapter(t)),
	)
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}

	ctx := context.Background()
	keys := []string{"custom-ttl-key-1", "custom-ttl-key-2"}

	// 使用自定义TTL的batchLoader获取数据
	var result map[string]string
	err = cache.MGet(ctx, keys, &result,
		WithBatchLoader(func(ctx context.Context, keys []string) (map[string]any, error) {
			data := make(map[string]any)
			for _, key := range keys {
				data[key] = "custom-ttl-" + key
			}
			return data, nil
		}),
		WithTTL(2*time.Minute, 10*time.Minute),
	)
	if err != nil {
		t.Errorf("MGet() error = %v", err)
		return
	}

	expected := map[string]string{
		"custom-ttl-key-1": "custom-ttl-custom-ttl-key-1",
		"custom-ttl-key-2": "custom-ttl-custom-ttl-key-2",
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("MGet() result = %v, want %v", result, expected)
	}

	// 验证数据已缓存
	for _, key := range keys {
		validateKeyExists(t, cache, key)
	}

	// 验证Redis TTL（这里只能验证TTL存在且合理）
	layeredCache := cache.(*LayeredCache)
	if layeredCache.remote != nil {
		for _, key := range keys {
			ttl, err := layeredCache.remote.TTL(ctx, key)
			if err != nil {
				t.Errorf("TTL() error for key %s = %v", key, err)
			} else if ttl <= 0 || ttl > 10*time.Minute {
				t.Errorf("TTL for key %s = %v, want > 0 and <= 10m", key, ttl)
			}
		}
	}
}

// validateMGetResult 验证MGet方法的结果
func validateMGetResult(t *testing.T, target any, expected any) {
	t.Helper()

	// 使用反射获取target的实际值
	targetVal := reflect.ValueOf(target)
	if targetVal.Kind() != reflect.Ptr {
		t.Errorf("Target must be a pointer, got %T", target)
		return
	}
	actualVal := targetVal.Elem().Interface()

	// 检查预期结果的类型
	expectedMap, ok := expected.(map[string]any)
	if !ok {
		// 如果不是 map[string]any 类型，直接使用深度比较
		if !reflect.DeepEqual(actualVal, expected) {
			t.Errorf("MGet result = %v, want %v", actualVal, expected)
		}
		return
	}

	// 处理 map[string]any 类型的预期结果
	actualMapVal := reflect.ValueOf(actualVal)
	if actualMapVal.Kind() != reflect.Map {
		t.Errorf("Actual result is not a map, got %T", actualVal)
		return
	}

	// 检查长度
	if actualMapVal.Len() != len(expectedMap) {
		t.Errorf("MGet result length = %d, want %d", actualMapVal.Len(), len(expectedMap))
		return
	}

	// 逐个比较键值对
	for expectedKey, expectedValue := range expectedMap {
		actualValue := actualMapVal.MapIndex(reflect.ValueOf(expectedKey))
		if !actualValue.IsValid() {
			t.Errorf("MGet result missing key %s", expectedKey)
			continue
		}

		// 比较值
		if !reflect.DeepEqual(actualValue.Interface(), expectedValue) {
			t.Errorf("MGet result[%s] = %v, want %v", expectedKey, actualValue.Interface(), expectedValue)
		}
	}

	// 检查是否有额外的键
	for _, key := range actualMapVal.MapKeys() {
		keyStr, ok := key.Interface().(string)
		if !ok {
			t.Errorf("MGet result key is not string: %v", key.Interface())
			continue
		}
		if _, exists := expectedMap[keyStr]; !exists {
			t.Errorf("MGet result contains unexpected key %s", keyStr)
		}
	}
}
