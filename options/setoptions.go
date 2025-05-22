package options

import (
	"context"
	"time"
)

// GetOption Get操作的选项配置
type GetOption interface {
	applyGet(*getConfig)
}

type LoaderFunc func(ctx context.Context, key string) (any, error)
type BatchLoaderFunc func(ctx context.Context, keys []string) (map[string]any, error)

// getConfig Get操作的内部配置
type getConfig struct {
	// loader 缓存未命中时的加载函数
	loader LoaderFunc

	// batchLoader 批量时缓存未命中时的加载函数
	batchLoader BatchLoaderFunc

	// memoryTTL 加载后写入内存缓存的过期时间
	memoryTTL time.Duration

	// redisTTL 加载后写入Redis缓存的过期时间
	redisTTL time.Duration

	// skipMemory 是否跳过内存缓存
	skipMemory bool

	// skipRedis 是否跳过Redis缓存
	skipRedis bool
}

// WithLoader 设置缓存未命中时的加载函数
type withLoader struct {
	loader LoaderFunc
}

func (w withLoader) applyGet(cfg *getConfig) {
	cfg.loader = w.loader
}

func WithLoader(loader LoaderFunc) GetOption {
	return withLoader{loader: loader}
}

// WithBatchLoader 设置缓存未命中时的批量加载函数
type withBatchLoader struct {
	batchLoader BatchLoaderFunc
}

func (w withBatchLoader) applyGet(cfg *getConfig) {
	cfg.batchLoader = w.batchLoader
}

func WithBatchLoader(batchLoader BatchLoaderFunc) GetOption {
	return withBatchLoader{batchLoader: batchLoader}
}

// WithLoaderTTL 设置加载后的缓存过期时间
type withLoaderTTL struct {
	memoryTTL time.Duration
	redisTTL  time.Duration
}

func (w withLoaderTTL) applyGet(cfg *getConfig) {
	cfg.memoryTTL = w.memoryTTL
	cfg.redisTTL = w.redisTTL
}

func WithLoaderTTL(memoryTTL, redisTTL time.Duration) GetOption {
	return withLoaderTTL{memoryTTL: memoryTTL, redisTTL: redisTTL}
}
