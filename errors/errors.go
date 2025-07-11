package errors

import "errors"

var (
	Is  = errors.Is
	As  = errors.As
	New = errors.New
)

var (
	// ErrKeyNotFound 请求的键不存在
	ErrKeyNotFound = errors.New("key not found")
	// ErrInvalidExpireTime 无效的过期时间
	ErrInvalidExpireTime = errors.New("invalid expire time")

	// ErrExpired 表示键已过期
	// 当缓存项过期时返回此错误
	ErrExpired = errors.New("key expired")

	// ErrInvalidKey 表示键格式无效
	// 当提供的键不符合要求时返回此错误
	ErrInvalidKey = errors.New("invalid key")

	// ErrInvalidValue 表示值格式无效
	// 当提供的值不符合要求时返回此错误
	ErrInvalidValue = errors.New("invalid value")

	// ErrCacheFull 表示缓存已满
	// 主要用于内存缓存，当无法添加新项时返回
	ErrCacheFull = errors.New("cache is full")

	// ErrConnectionFailed 表示连接失败
	// 当无法连接到缓存服务时返回此错误
	ErrConnectionFailed = errors.New("connection failed")

	// ErrOperationTimeout 表示操作超时
	// 当缓存操作超过指定时间时返回此错误
	ErrOperationTimeout = errors.New("operation timeout")
)
