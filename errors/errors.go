package errors

import "errors"

var (
	Is   = errors.Is
	As   = errors.As
	New  = errors.New
	Join = errors.Join
)

var (
	// ErrAdapterRequired 表示适配器是必需的
	ErrAdapterRequired = errors.New("adapter is required")
	// ErrNotFound 请求的键不存在
	ErrNotFound = errors.New("key not found")
	// ErrInvalidExpireTime 无效的过期时间
	ErrInvalidExpireTime = errors.New("invalid expire time")
)
