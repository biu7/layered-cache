package cache

import (
	"context"
	"time"

	"github.com/biu7/layered-cache/errors"
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
	memoryTTL *time.Duration

	// remoteTTL 加载后写入Redis缓存的过期时间
	remoteTTL *time.Duration

	// cacheNotFound 是否缓存缺失值（防止缓存穿透）
	cacheNotFound *bool

	// cacheNotFoundTTL 缺失值的缓存过期时间
	cacheNotFoundTTL *time.Duration
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
	remoteTTL time.Duration
}

func (w withTTL) applyGet(cfg *getOptions) {
	cfg.memoryTTL = &w.memoryTTL
	cfg.remoteTTL = &w.remoteTTL
}

func (w withTTL) applySet(cfg *setOptions) {
	cfg.memoryTTL = &w.memoryTTL
	cfg.remoteTTL = &w.remoteTTL
}

// WithTTL 设置缓存过期时间（通用选项，可用于Get和Set操作）
func WithTTL(memoryTTL, remoteTTL time.Duration) interface {
	GetOption
	SetOption
} {
	return withTTL{memoryTTL: memoryTTL, remoteTTL: remoteTTL}
}

type withMemoryTTL struct {
	memoryTTL time.Duration
}

func (w withMemoryTTL) applyGet(cfg *getOptions) {
	cfg.memoryTTL = &w.memoryTTL
}

func (w withMemoryTTL) applySet(cfg *setOptions) {
	cfg.memoryTTL = &w.memoryTTL
}

// WithMemoryTTL 设置缓存过期时间（通用选项，可用于Get和Set操作）
func WithMemoryTTL(memoryTTL time.Duration) interface {
	GetOption
	SetOption
} {
	return withMemoryTTL{memoryTTL: memoryTTL}
}

type withRedisTTL struct {
	remoteTTL time.Duration
}

func (w withRedisTTL) applyGet(cfg *getOptions) {
	cfg.remoteTTL = &w.remoteTTL
}

func (w withRedisTTL) applySet(cfg *setOptions) {
	cfg.remoteTTL = &w.remoteTTL
}

// WithRemoteTTL 设置缓存过期时间（通用选项，可用于Get和Set操作）
func WithRemoteTTL(remoteTTL time.Duration) interface {
	GetOption
	SetOption
} {
	return withRedisTTL{remoteTTL: remoteTTL}
}

// withCacheNotFound 设置是否缓存缺失值
type withCacheNotFound struct {
	cacheNotFound    bool
	cacheNotFoundTTL time.Duration
}

func (w withCacheNotFound) applyGet(cfg *getOptions) {
	cfg.cacheNotFound = &w.cacheNotFound
	cfg.cacheNotFoundTTL = &w.cacheNotFoundTTL
}

// WithCacheNotFound 设置是否缓存缺失值（防止缓存穿透）
// cacheNotFound: 是否启用缺失值缓存
// cacheNotFoundTTL: 缺失值的缓存过期时间，如果小于0则使用默认值
func WithCacheNotFound(cacheNotFound bool, cacheNotFoundTTL time.Duration) GetOption {
	return withCacheNotFound{cacheNotFound: cacheNotFound, cacheNotFoundTTL: cacheNotFoundTTL}
}

// applyGetOptions 应用Get选项到配置
func applyGetOptions(cfg *getOptions, opts ...GetOption) error {
	for _, opt := range opts {
		opt.applyGet(cfg)
	}
	return validateGetOptions(cfg)
}

// newGetOptions 创建默认Get配置
func newGetOptions() *getOptions {
	return &getOptions{}
}

func validateGetOptions(cfg *getOptions) error {
	if cfg.memoryTTL != nil && *cfg.memoryTTL <= 0 {
		return errors.ErrInvalidMemoryExpireTime
	}

	if cfg.remoteTTL != nil && *cfg.remoteTTL <= 0 {
		return errors.ErrInvalidRedisExpireTime
	}

	if cfg.cacheNotFoundTTL != nil && *cfg.cacheNotFoundTTL <= 0 {
		return errors.ErrInvalidCacheNotFondTTL
	}
	return nil
}

// SetOption Set操作的选项配置
type SetOption interface {
	applySet(*setOptions)
}

// setOptions Set操作的内部配置
type setOptions struct {
	// memoryTTL 内存缓存过期时间
	memoryTTL *time.Duration

	// remoteTTL Redis缓存过期时间
	remoteTTL *time.Duration
}

// applySetOptions 应用Set选项到配置
func applySetOptions(cfg *setOptions, opts ...SetOption) error {
	for _, opt := range opts {
		opt.applySet(cfg)
	}
	return validateSetOptions(cfg)
}

// newSetOptions 创建默认Set配置
func newSetOptions() *setOptions {
	return &setOptions{}
}

func validateSetOptions(cfg *setOptions) error {
	if cfg.memoryTTL != nil && *cfg.memoryTTL <= 0 {
		return errors.ErrInvalidMemoryExpireTime
	}

	if cfg.remoteTTL != nil && *cfg.remoteTTL <= 0 {
		return errors.ErrInvalidRedisExpireTime
	}
	return nil
}
