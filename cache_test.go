package main

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/biu7/layered-cache/adapter"
	"github.com/biu7/layered-cache/errors"
	"github.com/biu7/layered-cache/serializer"
	"github.com/redis/go-redis/v9"
)

func createMemoryAdapter(t *testing.T) adapter.MemoryAdapter {
	t.Helper()

	otter, err := adapter.NewOtterAdapter(1024)
	if err != nil {
		panic(err)
	}
	return otter
}

func createRedisAdapter(t *testing.T) adapter.RemoteAdapter {
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

	return adapter.NewRedisAdapterWithClient(client)
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
				WithMemory(createMemoryAdapter(t)),
			},
			wantErr: nil,
		},
		{
			name: "成功创建缓存 - 仅Redis适配器",
			options: []Option{
				WithRedis(createRedisAdapter(t)),
			},
			wantErr: nil,
		},
		{
			name: "成功创建缓存 - 内存和Redis适配器",
			options: []Option{
				WithMemory(createMemoryAdapter(t)),
				WithRedis(createRedisAdapter(t)),
			},
			wantErr: nil,
		},
		{
			name: "成功创建缓存 - 自定义序列化器",
			options: []Option{
				WithMemory(createMemoryAdapter(t)),
				WithSerializer(createSerializer(t)),
			},
			wantErr: nil,
		},
		{
			name: "成功创建缓存 - 自定义TTL",
			options: []Option{
				WithMemory(createMemoryAdapter(t)),
				WithRedis(createRedisAdapter(t)),
				WithDefaultTTL(10*time.Minute, 24*time.Hour),
			},
			wantErr: nil,
		},
		{
			name: "成功创建缓存 - 启用缺失值缓存",
			options: []Option{
				WithMemory(createMemoryAdapter(t)),
				WithDefaultCacheNotFound(true, 30*time.Second),
			},
			wantErr: nil,
		},
		{
			name: "失败 - 没有适配器",
			options: []Option{
				WithSerializer(createSerializer(t)),
			},
			wantErr: errors.ErrAdapterRequired,
		},
		{
			name: "失败 - 无效的内存TTL",
			options: []Option{
				WithMemory(createMemoryAdapter(t)),
				WithDefaultTTL(0, 24*time.Hour),
			},
			wantErr: errors.ErrInvalidMemoryExpireTime,
		},
		{
			name: "失败 - 负的内存TTL",
			options: []Option{
				WithMemory(createMemoryAdapter(t)),
				WithDefaultTTL(-1*time.Minute, 24*time.Hour),
			},
			wantErr: errors.ErrInvalidMemoryExpireTime,
		},
		{
			name: "失败 - 无效的Redis TTL",
			options: []Option{
				WithRedis(createRedisAdapter(t)),
				WithDefaultTTL(5*time.Minute, 0),
			},
			wantErr: errors.ErrInvalidRedisExpireTime,
		},
		{
			name: "失败 - 负的Redis TTL",
			options: []Option{
				WithRedis(createRedisAdapter(t)),
				WithDefaultTTL(5*time.Minute, -1*time.Hour),
			},
			wantErr: errors.ErrInvalidRedisExpireTime,
		},
		{
			name: "失败 - 无效的缺失值缓存TTL",
			options: []Option{
				WithMemory(createMemoryAdapter(t)),
				WithDefaultCacheNotFound(true, 0),
			},
			wantErr: errors.ErrInvalidCacheNotFondTTL,
		},
		{
			name: "失败 - 负的缺失值缓存TTL",
			options: []Option{
				WithMemory(createMemoryAdapter(t)),
				WithDefaultCacheNotFound(true, -1*time.Second),
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
	cache, err := NewCache(WithMemory(createMemoryAdapter(t)))
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
	cache, err := NewCache(WithRedis(createRedisAdapter(t)))
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
		WithMemory(createMemoryAdapter(t)),
		WithRedis(createRedisAdapter(t)),
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
	redisTTL := 48 * time.Hour

	cache, err := NewCache(
		WithMemory(createMemoryAdapter(t)),
		WithRedis(createRedisAdapter(t)),
		WithDefaultTTL(memoryTTL, redisTTL),
	)
	if err != nil {
		t.Fatalf("NewCache() unexpected error = %v", err)
	}

	layeredCache := cache.(*LayeredCache)

	if layeredCache.defaultMemoryTTL != memoryTTL {
		t.Errorf("defaultMemoryTTL = %v, want %v", layeredCache.defaultMemoryTTL, memoryTTL)
	}

	if layeredCache.defaultRedisTTL != redisTTL {
		t.Errorf("defaultRedisTTL = %v, want %v", layeredCache.defaultRedisTTL, redisTTL)
	}
}

func TestNewCache_CustomCacheMissing(t *testing.T) {
	missingTTL := 45 * time.Second

	cache, err := NewCache(
		WithMemory(createMemoryAdapter(t)),
		WithDefaultCacheNotFound(true, missingTTL),
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
		WithMemory(createMemoryAdapter(t)),
		WithSerializer(customSerializer),
	)
	if err != nil {
		t.Fatalf("NewCache() unexpected error = %v", err)
	}

	layeredCache := cache.(*LayeredCache)

	if layeredCache.serializer == nil {
		t.Error("serializer is nil")
	}
}
