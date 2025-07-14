package main

import (
	"time"

	"github.com/biu7/layered-cache/adapter"
	"github.com/biu7/layered-cache/serializer"
)

type Option interface {
	apply(*options)
}

// options 缓存配置
type options struct {
	// MemoryAdapter 内存缓存适配器
	memoryAdapter adapter.MemoryAdapter

	// redisAdapter Redis 缓存适配器
	redisAdapter adapter.RemoteAdapter

	// serializer 序列化器
	serializer serializer.Serializer

	// defaultMemoryTTL 默认内存缓存过期时间
	defaultMemoryTTL time.Duration

	// defaultRedisTTL 默认Redis缓存过期时间
	defaultRedisTTL time.Duration

	// defaultCacheMissing 默认是否缓存缺失值（防止缓存穿透）
	defaultCacheMissing bool

	// defaultMissingTTL 默认缺失值的缓存过期时间
	defaultMissingTTL time.Duration
}

type memoryAdapterOption struct {
	adapter adapter.MemoryAdapter
}

func (m memoryAdapterOption) apply(opts *options) {
	opts.memoryAdapter = m.adapter
}

func WithMemory(adp adapter.MemoryAdapter) Option {
	return memoryAdapterOption{adapter: adp}
}

type redisAdapterOption struct {
	adapter adapter.RemoteAdapter
}

func (r redisAdapterOption) apply(opts *options) {
	opts.redisAdapter = r.adapter
}

func WithRedis(adp adapter.RemoteAdapter) Option {
	return redisAdapterOption{adapter: adp}
}

type serializerOption struct {
	serializer serializer.Serializer
}

func (s serializerOption) apply(opts *options) {
	opts.serializer = s.serializer
}

func WithSerializer(srl serializer.Serializer) Option {
	return serializerOption{serializer: srl}
}

type defaultTTLOption struct {
	memoryTTL time.Duration
	redisTTL  time.Duration
}

func (d defaultTTLOption) apply(opts *options) {
	opts.defaultMemoryTTL = d.memoryTTL
	opts.defaultRedisTTL = d.redisTTL
}

func WithDefaultTTL(memoryTTL, redisTTL time.Duration) Option {
	return defaultTTLOption{memoryTTL: memoryTTL, redisTTL: redisTTL}
}

// withDefaultCacheMissing 设置默认缺失值缓存选项
type withDefaultCacheMissing struct {
	cacheMissing bool
	missingTTL   time.Duration
}

func (w withDefaultCacheMissing) apply(opts *options) {
	opts.defaultCacheMissing = w.cacheMissing
	opts.defaultMissingTTL = w.missingTTL
}

// WithDefaultCacheMissing 设置默认缺失值缓存选项
func WithDefaultCacheMissing(cacheMissing bool, missingTTL time.Duration) Option {
	return withDefaultCacheMissing{cacheMissing: cacheMissing, missingTTL: missingTTL}
}

// applyOptions 应用选项到配置
func applyOptions(opts *options, options ...Option) {
	for _, option := range options {
		option.apply(opts)
	}
}

// newOptions 创建默认配置
func newOptions() *options {
	return &options{
		serializer:          serializer.NewSonicJson(), // 默认使用SonicJson序列化
		defaultMemoryTTL:    5 * time.Minute,           // 默认内存缓存5分钟
		defaultRedisTTL:     14 * 24 * time.Hour,       // 默认Redis缓存14天
		defaultCacheMissing: false,                     // 默认不缓存缺失值
		defaultMissingTTL:   time.Minute,               // 默认缺失值缓存1分钟
	}
}
