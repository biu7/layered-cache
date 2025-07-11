package main

import (
	"context"
	"time"

	"github.com/biu7/layered-cache/options"
)

type Cache interface {
	Get(ctx context.Context, key string, target interface{}, opts ...options.GetOption) error
	Set(ctx context.Context, key string, value interface{}, opts ...SetOption) error
	Delete(ctx context.Context, key string) error
}

func


// TypedGetOption 泛型Get操作的选项配置
type TypedGetOption[T any] interface {
	applyTypedGet(*typedGetConfig[T])
}

// Serializer 序列化器接口，用于内存缓存的类型转换

// CacheConfig 缓存配置
type CacheConfig struct {
	// MemoryAdapter 内存缓存适配器
	MemoryAdapter CacheAdapter

	// RedisAdapter Redis缓存适配器
	RedisAdapter CacheAdapter

	// Serializer 序列化器，用于Redis缓存
	Serializer Serializer

	// DefaultMemoryTTL 默认内存缓存过期时间
	DefaultMemoryTTL time.Duration

	// DefaultRedisTTL 默认Redis缓存过期时间
	DefaultRedisTTL time.Duration

	// EnableMemory 是否启用内存缓存
	EnableMemory bool

	// EnableRedis 是否启用Redis缓存
	EnableRedis bool
}
