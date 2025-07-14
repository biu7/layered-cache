package errors

import "errors"

var (
	Is  = errors.Is
	New = errors.New
)

var (
	ErrAdapterRequired = errors.New("adapter is required")

	ErrNotFound = errors.New("key not found")

	// ErrInvalidMemoryExpireTime 无效的过期时间
	ErrInvalidMemoryExpireTime = errors.New("invalid memory expire time")
	ErrInvalidRedisExpireTime  = errors.New("invalid redis expire time")
	ErrInvalidCacheNotFondTTL  = errors.New("invalid cache not found ttl")

	// ErrInvalidTarget 无效的目标类型
	ErrInvalidTarget = errors.New("invalid target type, must be a pointer to map[string]T")
)
