# LayeredCache

[English](#english) | [中文](#中文)

## English

LayeredCache is a Go caching library that supports both memory and Redis caching layers.

### Features

- **Layered Caching**: Memory cache (Otter) + Redis cache
- **Generic Support**: Type-safe cache operations with `TypedCache[ID, T]` supporting multiple ID types
- **Smart Key Building**: Automatically handles different ID types (string, int, int32, int64, etc.) to generate
  formatted cache keys
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
	"github.com/biu7/layered-cache/storage"
	"github.com/redis/go-redis/v9"
)

type User struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func main() {
	// Create memory adapter
	memory, _ := storage.NewOtter(100_000) // 100KB

	// Create Redis adapter
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	redisCache := storage.NewRedisWithClient(rdb)

	// Create cache instance
	c, _ := cache.NewCache(
		cache.WithConfigMemory(memory),
		cache.WithConfigRemote(redisCache),
		cache.WithConfigDefaultTTL(time.Minute, 24*time.Hour),
		cache.WithConfigDefaultCacheNotFound(true, time.Minute),
	)

	// Use typed cache - specify ID type and value type
	userCache := cache.Typed[int64, User](c)

	ctx := context.Background()
	user := User{ID: 1, Name: "Alice"}

	// Set cache - using keyPrefix and ID
	userCache.Set(ctx, "user", user.ID, user)

	// Get cache
	cachedUser, err := userCache.Get(ctx, "user", user.ID, nil)
	if err != nil {
		panic(err)
	}

	// Get with loader - loader now receives ID instead of full key
	user, err = userCache.Get(ctx, "user", 2, func(ctx context.Context, id int64) (User, error) {
		// Load from database using ID
		return User{ID: id, Name: "Bob"}, nil
	})

	// Batch operations
	users := map[int64]User{
		1: {ID: 1, Name: "Alice"},
		2: {ID: 2, Name: "Bob"},
		3: {ID: 3, Name: "Charlie"},
	}

	// Batch set
	userCache.MSet(ctx, "user", users)

	// Batch get
	ids := []int64{1, 2, 3}
	cachedUsers, err := userCache.MGet(ctx, "user", ids, nil)
}
```

### TypedCache API

#### Basic Operations

- `Set(ctx, keyPrefix, id, value, opts...)`: Set a single cache value
- `Get(ctx, keyPrefix, id, loader, opts...)`: Get a single cache value with optional loader function
- `MSet(ctx, keyPrefix, values, opts...)`: Batch set cache values
- `MGet(ctx, keyPrefix, ids, loader, opts...)`: Batch get cache values with optional batch loader function
- `Delete(ctx, keyPrefix, id)`: Delete a single cache value

#### Key Building Rules

TypedCache automatically combines keyPrefix and ID to generate the final cache key:

- Format: `keyPrefix + ":" + ID`
- Supported ID types: `string`, `int`, `int32`, `int64`, and other comparable types
- Example: `keyPrefix="user"`, `id=123` → final key is `"user:123"`

#### Loader Functions

- **Single loader**: `func(ctx context.Context, id ID) (T, error)`
- **Batch loader**: `func(ctx context.Context, ids []ID) (map[ID]T, error)`

### Different ID Type Examples

```go
// String ID
stringCache := cache.Typed[string, User](c)
stringCache.Set(ctx, "user", "alice", user)

// Integer ID  
intCache := cache.Typed[int, User](c)
intCache.Set(ctx, "user", 123, user)

// Custom ID type
type UserID int64
customCache := cache.Typed[UserID, User](c)
customCache.Set(ctx, "user", UserID(456), user)
```

### Acknowledgments

Thanks to [@mgtv-tech/jetcache-go](https://github.com/mgtv-tech/jetcache-go) for providing design inspiration and
reference implementation

### License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

### Third-party Licenses

Parts of this project are inspired by or adapted from:

- [jetcache-go](https://github.com/mgtv-tech/jetcache-go) - BSD-2-Clause License

---

## 中文

LayeredCache 是一个 Go 语言的分层缓存库，支持内存缓存和 Redis 缓存。

### 特性

- **分层缓存**：内存缓存（Otter）+ Redis 缓存
- **泛型支持**：提供 `TypedCache[ID, T]` 类型安全的缓存操作，支持多种ID类型
- **智能Key构建**：自动处理不同类型的ID（string、int、int32、int64等），生成格式化的cache key
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
	"github.com/biu7/layered-cache/storage"
	"github.com/redis/go-redis/v9"
)

type User struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func main() {
	// 创建内存适配器
	memory, _ := storage.NewOtter(100_000) // 100KB

	// 创建 Redis 适配器
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	redisCache := storage.NewRedisWithClient(rdb)

	// 创建缓存实例
	c, _ := cache.NewCache(
		cache.WithConfigMemory(memory),
		cache.WithConfigRemote(redisCache),
		cache.WithConfigDefaultTTL(time.Minute, 24*time.Hour),
		cache.WithConfigDefaultCacheNotFound(true, time.Minute),
	)

	// 使用泛型缓存 - 指定ID类型和值类型
	userCache := cache.Typed[int64, User](c)

	ctx := context.Background()
	user := User{ID: 1, Name: "Alice"}

	// 设置缓存 - 使用keyPrefix和ID
	userCache.Set(ctx, "user", user.ID, user)

	// 获取缓存
	cachedUser, err := userCache.Get(ctx, "user", user.ID, nil)
	if err != nil {
		panic(err)
	}

	// 带 loader 的获取 - loader现在接收ID而不是完整的key
	user, err = userCache.Get(ctx, "user", 2, func(ctx context.Context, id int64) (User, error) {
		// 从数据库获取，使用ID查询
		return User{ID: id, Name: "Bob"}, nil
	})

	// 批量操作
	users := map[int64]User{
		1: {ID: 1, Name: "Alice"},
		2: {ID: 2, Name: "Bob"},
		3: {ID: 3, Name: "Charlie"},
	}
	
	// 批量设置
	userCache.MSet(ctx, "user", users)
	
	// 批量获取
	ids := []int64{1, 2, 3}
	cachedUsers, err := userCache.MGet(ctx, "user", ids, nil)
}
```

### TypedCache API

#### 基本操作
- `Set(ctx, keyPrefix, id, value, opts...)`: 设置单个缓存值
- `Get(ctx, keyPrefix, id, loader, opts...)`: 获取单个缓存值，支持loader函数
- `MSet(ctx, keyPrefix, values, opts...)`: 批量设置缓存值
- `MGet(ctx, keyPrefix, ids, loader, opts...)`: 批量获取缓存值，支持批量loader函数
- `Delete(ctx, keyPrefix, id)`: 删除单个缓存值

#### Key构建规则
TypedCache会自动将keyPrefix和ID组合生成最终的cache key：
- 格式：`keyPrefix + ":" + ID`
- 支持的ID类型：`string`、`int`、`int32`、`int64`及其他comparable类型
- 例如：`keyPrefix="user"`, `id=123` → 最终key为`"user:123"`

#### Loader函数
- **单个loader**: `func(ctx context.Context, id ID) (T, error)`
- **批量loader**: `func(ctx context.Context, ids []ID) (map[ID]T, error)`

### 不同ID类型示例

```go
// 字符串ID
stringCache := cache.Typed[string, User](c)
stringCache.Set(ctx, "user", "alice", user)

// 整数ID  
intCache := cache.Typed[int, User](c)
intCache.Set(ctx, "user", 123, user)

// 自定义ID类型
type UserID int64
customCache := cache.Typed[UserID, User](c)
customCache.Set(ctx, "user", UserID(456), user)
```

### 致谢

感谢 [@mgtv-tech/jetcache-go](https://github.com/mgtv-tech/jetcache-go) 项目提供的设计思路和参考实现
