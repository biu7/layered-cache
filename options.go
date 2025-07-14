package cache

import (
	"time"

	"github.com/biu7/layered-cache/errors"
	"github.com/biu7/layered-cache/serializer"
	"github.com/biu7/layered-cache/storage"
)

type Option interface {
	apply(*options)
}

// options 缓存配置
type options struct {
	// Memory 内存缓存适配器
	memoryAdapter storage.Memory

	// Remote 缓存适配器
	remoteAdapter storage.Remote

	// serializer 序列化器
	serializer serializer.Serializer

	// defaultMemoryTTL 默认内存缓存过期时间
	defaultMemoryTTL time.Duration

	// defaultRemoteTTL 默认 Remote 缓存过期时间
	defaultRemoteTTL time.Duration

	// defaultCacheNotFound 默认是否缓存缺失值（防止缓存穿透）
	defaultCacheNotFound bool

	// defaultCacheNotFoundTTL 默认缺失值的缓存过期时间
	defaultCacheNotFoundTTL time.Duration
}

type memoryAdapterOption struct {
	adapter storage.Memory
}

func (m memoryAdapterOption) apply(opts *options) {
	opts.memoryAdapter = m.adapter
}

func WithConfigMemory(adp storage.Memory) Option {
	return memoryAdapterOption{adapter: adp}
}

type remoteAdapterOption struct {
	adapter storage.Remote
}

func (r remoteAdapterOption) apply(opts *options) {
	opts.remoteAdapter = r.adapter
}

func WithConfigRemote(adp storage.Remote) Option {
	return remoteAdapterOption{adapter: adp}
}

type serializerOption struct {
	serializer serializer.Serializer
}

func (s serializerOption) apply(opts *options) {
	opts.serializer = s.serializer
}

func WithConfigSerializer(srl serializer.Serializer) Option {
	return serializerOption{serializer: srl}
}

type defaultTTLOption struct {
	memoryTTL time.Duration
	remoteTTL time.Duration
}

func (d defaultTTLOption) apply(opts *options) {
	opts.defaultMemoryTTL = d.memoryTTL
	opts.defaultRemoteTTL = d.remoteTTL
}

func WithConfigDefaultTTL(memoryTTL, remoteTTL time.Duration) Option {
	return defaultTTLOption{memoryTTL: memoryTTL, remoteTTL: remoteTTL}
}

// defaultCacheNotFoundOption 设置默认缺失值缓存选项
type defaultCacheNotFoundOption struct {
	cacheNotFound    bool
	cacheNotFoundTTL time.Duration
}

func (w defaultCacheNotFoundOption) apply(opts *options) {
	opts.defaultCacheNotFound = w.cacheNotFound
	opts.defaultCacheNotFoundTTL = w.cacheNotFoundTTL
}

// WithConfigDefaultCacheNotFound 设置默认缺失值缓存选项
func WithConfigDefaultCacheNotFound(cacheNotFound bool, cacheNotFoundTTL time.Duration) Option {
	return defaultCacheNotFoundOption{cacheNotFound: cacheNotFound, cacheNotFoundTTL: cacheNotFoundTTL}
}

// applyOptions 应用选项到配置
func applyOptions(opts *options, options ...Option) error {
	for _, option := range options {
		option.apply(opts)
	}
	return validateOptions(opts)
}

// newOptions 创建默认配置
func newOptions() *options {
	return &options{
		serializer:              serializer.NewSonicJson(), // 默认使用SonicJson序列化
		defaultMemoryTTL:        5 * time.Minute,           // 默认内存缓存5分钟
		defaultRemoteTTL:        14 * 24 * time.Hour,       // 默认Remote缓存14天
		defaultCacheNotFound:    false,                     // 默认不缓存缺失值
		defaultCacheNotFoundTTL: time.Minute,               // 默认缺失值缓存1分钟
	}
}

func validateOptions(cfg *options) error {
	if cfg.memoryAdapter == nil && cfg.remoteAdapter == nil {
		return errors.ErrAdapterRequired
	}

	if cfg.memoryAdapter != nil {
		if err := validMemoryTTL(cfg.defaultMemoryTTL); err != nil {
			return err
		}
	}

	if cfg.remoteAdapter != nil {
		if err := validRemoteTTL(cfg.defaultRemoteTTL); err != nil {
			return err
		}
	}

	if cfg.defaultCacheNotFound {
		if err := validCacheMissTTL(cfg.defaultCacheNotFoundTTL); err != nil {
			return err
		}
	}

	return nil
}
