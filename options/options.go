package options

import (
	"context"
	"time"
)

// setConfig Set操作的内部配置
type setConfig struct {
	// memoryTTL 内存缓存过期时间
	memoryTTL time.Duration

	// redisTTL Redis缓存过期时间
	redisTTL time.Duration

	// onlyMemory 只写入内存缓存
	onlyMemory bool

	// onlyRedis 只写入Redis缓存
	onlyRedis bool
}

// WithSkipLayers 跳过指定的缓存层级
type withSkipLayers struct {
	skipMemory bool
	skipRedis  bool
}

func (w withSkipLayers) applyGet(cfg *getConfig) {
	cfg.skipMemory = w.skipMemory
	cfg.skipRedis = w.skipRedis
}

func WithSkipLayers(skipMemory, skipRedis bool) GetOption {
	return withSkipLayers{skipMemory: skipMemory, skipRedis: skipRedis}
}

// WithRefreshAsync 设置异步刷新缓存
type withRefreshAsync struct {
	async bool
}

func (w withRefreshAsync) applyGet(cfg *getConfig) {
	cfg.refreshAsync = w.async
}

func WithRefreshAsync(async bool) GetOption {
	return withRefreshAsync{async: async}
}

// SetOption 实现

// WithTTL 设置缓存过期时间
type withTTL struct {
	memoryTTL time.Duration
	redisTTL  time.Duration
}

func (w withTTL) applySet(cfg *setConfig) {
	cfg.memoryTTL = w.memoryTTL
	cfg.redisTTL = w.redisTTL
}

func WithTTL(memoryTTL, redisTTL time.Duration) SetOption {
	return withTTL{memoryTTL: memoryTTL, redisTTL: redisTTL}
}

// WithMemoryTTL 只设置内存缓存过期时间
type withMemoryTTL struct {
	ttl time.Duration
}

func (w withMemoryTTL) applySet(cfg *setConfig) {
	cfg.memoryTTL = w.ttl
}

func WithMemoryTTL(ttl time.Duration) SetOption {
	return withMemoryTTL{ttl: ttl}
}

// WithRedisTTL 只设置Redis缓存过期时间
type withRedisTTL struct {
	ttl time.Duration
}

func (w withRedisTTL) applySet(cfg *setConfig) {
	cfg.redisTTL = w.ttl
}

func WithRedisTTL(ttl time.Duration) SetOption {
	return withRedisTTL{ttl: ttl}
}

// WithOnlyMemory 只写入内存缓存
type withOnlyMemory struct{}

func (w withOnlyMemory) applySet(cfg *setConfig) {
	cfg.onlyMemory = true
}

func WithOnlyMemory() SetOption {
	return withOnlyMemory{}
}

// WithOnlyRedis 只写入Redis缓存
type withOnlyRedis struct{}

func (w withOnlyRedis) applySet(cfg *setConfig) {
	cfg.onlyRedis = true
}

func WithOnlyRedis() SetOption {
	return withOnlyRedis{}
}
