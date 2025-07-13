package main

import (
	"context"
	"time"
)

// LoaderFunc 单个键的加载函数
type LoaderFunc func(ctx context.Context, key string) (any, error)

// BatchLoaderFunc 批量键的加载函数
type BatchLoaderFunc func(ctx context.Context, keys []string) (map[string]any, error)

// GetOption Get操作的选项配置
type GetOption interface {
	applyGet(*getOptions)
}

// getOptions Get操作的内部配置
type getOptions struct {
	// loader 缓存未命中时的加载函数
	loader LoaderFunc

	// batchLoader 缓存未命中时的批量加载函数
	batchLoader BatchLoaderFunc

	// memoryTTL 加载后写入内存缓存的过期时间
	memoryTTL time.Duration

	// redisTTL 加载后写入Redis缓存的过期时间
	redisTTL time.Duration

	// cacheMissing 是否缓存缺失值（防止缓存穿透）
	cacheMissing *bool

	// missingTTL 缺失值的缓存过期时间
	missingTTL time.Duration
}

// withLoader 设置缓存未命中时的加载函数
type withLoader struct {
	loader LoaderFunc
}

func (w withLoader) applyGet(cfg *getOptions) {
	cfg.loader = w.loader
}

// WithLoader 设置缓存未命中时的加载函数
func WithLoader(loader LoaderFunc) GetOption {
	return withLoader{loader: loader}
}

// withBatchLoader 设置缓存未命中时的批量加载函数
type withBatchLoader struct {
	batchLoader BatchLoaderFunc
}

func (w withBatchLoader) applyGet(cfg *getOptions) {
	cfg.batchLoader = w.batchLoader
}

// WithBatchLoader 设置缓存未命中时的批量加载函数
func WithBatchLoader(batchLoader BatchLoaderFunc) GetOption {
	return withBatchLoader{batchLoader: batchLoader}
}

// withTTL TTL选项的通用实现
type withTTL struct {
	memoryTTL time.Duration
	redisTTL  time.Duration
}

func (w withTTL) applyGet(cfg *getOptions) {
	cfg.memoryTTL = w.memoryTTL
	cfg.redisTTL = w.redisTTL
}

func (w withTTL) applySet(cfg *setOptions) {
	cfg.memoryTTL = w.memoryTTL
	cfg.redisTTL = w.redisTTL
}

// WithTTL 设置缓存过期时间（通用选项，可用于Get和Set操作）
func WithTTL(memoryTTL, redisTTL time.Duration) interface {
	GetOption
	SetOption
} {
	return withTTL{memoryTTL: memoryTTL, redisTTL: redisTTL}
}

// withCacheMissing 设置是否缓存缺失值
type withCacheMissing struct {
	cacheMissing bool
	missingTTL   time.Duration
}

func (w withCacheMissing) applyGet(cfg *getOptions) {
	cfg.cacheMissing = &w.cacheMissing
	cfg.missingTTL = w.missingTTL
}

// WithCacheMissing 设置是否缓存缺失值（防止缓存穿透）
// cacheMissing: 是否启用缺失值缓存
// missingTTL: 缺失值的缓存过期时间，如果小于0则使用默认值
func WithCacheMissing(cacheMissing bool, missingTTL time.Duration) GetOption {
	return withCacheMissing{cacheMissing: cacheMissing, missingTTL: missingTTL}
}

// applyGetOptions 应用Get选项到配置
func applyGetOptions(cfg *getOptions, opts ...GetOption) {
	for _, opt := range opts {
		opt.applyGet(cfg)
	}
}

// newGetOptions 创建默认Get配置
func newGetOptions() *getOptions {
	return &getOptions{}
}

// SetOption Set操作的选项配置
type SetOption interface {
	applySet(*setOptions)
}

// setOptions Set操作的内部配置
type setOptions struct {
	// memoryTTL 内存缓存过期时间
	memoryTTL time.Duration

	// redisTTL Redis缓存过期时间
	redisTTL time.Duration
}

// applySetOptions 应用Set选项到配置
func applySetOptions(cfg *setOptions, opts ...SetOption) {
	for _, opt := range opts {
		opt.applySet(cfg)
	}
}

// newSetOptions 创建默认Set配置
func newSetOptions() *setOptions {
	return &setOptions{}
}
