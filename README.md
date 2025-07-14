# LayeredCache

[English](#english) | [中文](#中文)

## 中文

LayeredCache 是一个 Go 语言的分层缓存库，支持内存缓存和 Redis 缓存。

### 特性

- **分层缓存**：内存缓存（Otter）+ Redis 缓存
- **泛型支持**：提供 `TypedCache[T]` 类型安全的缓存操作
- **防穿透**：支持缓存空值，避免缓存穿透
- **防并发**：使用 singleflight 防止并发重复请求
- **多序列化器**：支持 JSON、MessagePack 等序列化方式
- **灵活配置**：支持独立配置内存和 Redis 的 TTL

### 安装

```bash
go get github.com/biu7/layered-cache
```

### 快速开始

```go
package main

import (
    "context"
    "time"
    
    "github.com/biu7/layered-cache"
    "github.com/biu7/layered-cache/adapter"
    "github.com/redis/go-redis/v9"
)

type User struct {
    ID   int64  `json:"id"`
    Name string `json:"name"`
}

func main() {
    // 创建内存适配器
    memory, _ := adapter.NewOtterAdapter(100_000) // 100KB
    
    // 创建 Redis 适配器
    rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    redisCache := adapter.NewRedisAdapterWithClient(rdb)
    
    // 创建缓存实例
    c, _ := cache.NewCache(
        cache.WithMemory(memory),
        cache.WithRedis(redisCache),
        cache.WithDefaultTTL(time.Minute, 24*time.Hour),
        cache.WithDefaultCacheNotFound(true, time.Minute),
    )
    
    // 使用泛型缓存
    userCache := cache.Typed[User](c)
    
    ctx := context.Background()
    user := User{ID: 1, Name: "Alice"}
    
    // 设置缓存
    userCache.Set(ctx, "user:1", user)
    
    // 获取缓存
    cachedUser, err := userCache.Get(ctx, "user:1", nil)
    if err != nil {
        panic(err)
    }
    
    // 带 loader 的获取
    user, err = userCache.Get(ctx, "user:2", func(ctx context.Context, key string) (User, error) {
        // 从数据库获取
        return User{ID: 2, Name: "Bob"}, nil
    })
}
```

### 主要API

- `Set/MSet`: 设置缓存
- `Get/MGet`: 获取缓存，支持 loader 函数
- `Delete`: 删除缓存
- `Typed[T]`: 创建类型安全的缓存包装器

---

## English

LayeredCache is a Go caching library that supports both memory and Redis caching layers.

### Features

- **Layered Caching**: Memory cache (Otter) + Redis cache
- **Generic Support**: Type-safe cache operations with `TypedCache[T]`
- **Cache Penetration Protection**: Support for caching null values
- **Concurrency Protection**: Uses singleflight to prevent duplicate concurrent requests
- **Multiple Serializers**: Support for JSON, MessagePack, and other serialization formats
- **Flexible Configuration**: Independent TTL configuration for memory and Redis

### Installation

```bash
go get github.com/biu7/layered-cache
```

### Quick Start

```go
package main

import (
    "context"
    "time"
    
    "github.com/biu7/layered-cache"
    "github.com/biu7/layered-cache/adapter"
    "github.com/redis/go-redis/v9"
)

type User struct {
    ID   int64  `json:"id"`
    Name string `json:"name"`
}

func main() {
    // Create memory adapter
    memory, _ := adapter.NewOtterAdapter(100_000) // 100KB
    
    // Create Redis adapter
    rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    redisCache := adapter.NewRedisAdapterWithClient(rdb)
    
    // Create cache instance
    c, _ := cache.NewCache(
        cache.WithMemory(memory),
        cache.WithRedis(redisCache),
        cache.WithDefaultTTL(time.Minute, 24*time.Hour),
        cache.WithDefaultCacheNotFound(true, time.Minute),
    )
    
    // Use typed cache
    userCache := cache.Typed[User](c)
    
    ctx := context.Background()
    user := User{ID: 1, Name: "Alice"}
    
    // Set cache
    userCache.Set(ctx, "user:1", user)
    
    // Get cache
    cachedUser, err := userCache.Get(ctx, "user:1", nil)
    if err != nil {
        panic(err)
    }
    
    // Get with loader
    user, err = userCache.Get(ctx, "user:2", func(ctx context.Context, key string) (User, error) {
        // Load from database
        return User{ID: 2, Name: "Bob"}, nil
    })
}
```

### Main APIs

- `Set/MSet`: Set cache values
- `Get/MGet`: Get cache values with optional loader functions
- `Delete`: Delete cache entries
- `Typed[T]`: Create type-safe cache wrapper

### License

MIT 