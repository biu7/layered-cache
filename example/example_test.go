package example

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/alicebob/miniredis/v2"
	cache "github.com/biu7/layered-cache"
	"github.com/biu7/layered-cache/storage"
	"github.com/redis/go-redis/v9"
)

// User 表示用户数据模型
type User struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Example_basic_usage 展示基本使用方式
func Example_basic_usage() {
	// 创建内存缓存存储
	memory, err := storage.NewOtter(100_000) // 100KB 内存限制
	if err != nil {
		log.Fatalf("Failed to create memory storage: %v", err)
	}

	// 创建 Redis 存储（使用 miniredis）
	redisServer, err := miniredis.Run()
	if err != nil {
		log.Fatalf("Failed to start Redis server: %v", err)
	}
	defer redisServer.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: redisServer.Addr(),
	})
	defer rdb.Close()

	redisCache := storage.NewRedisWithClient(rdb)

	// 创建分层缓存
	c, err := cache.NewCache(
		cache.WithConfigMemory(memory),                           // 内存缓存层
		cache.WithConfigRemote(redisCache),                       // Redis 缓存层
		cache.WithConfigDefaultTTL(time.Minute, 14*24*time.Hour), // 默认过期时间
		cache.WithConfigDefaultCacheNotFound(true, time.Minute),  // 缓存空值防穿透
	)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}

	// 使用泛型包装器
	userCache := cache.Typed[User](c)

	ctx := context.Background()
	user := User{ID: 1, Name: "Alice"}

	// 设置缓存
	if err := userCache.Set(ctx, "user:1", user); err != nil {
		log.Printf("Failed to set cache: %v", err)
		return
	}

	// 获取缓存
	retrievedUser, err := userCache.Get(ctx, "user:1", nil)
	if err != nil {
		log.Printf("Failed to get cache: %v", err)
		return
	}

	fmt.Printf("Retrieved user: %+v\n", retrievedUser)
	// Output: Retrieved user: {ID:1 Name:Alice}
}

// Example_with_loader 展示使用 loader 函数的方式
func Example_with_loader() {
	// 创建缓存（简化配置）
	memory, _ := storage.NewOtter(1000)
	c, _ := cache.NewCache(cache.WithConfigMemory(memory))
	userCache := cache.Typed[User](c)

	ctx := context.Background()

	// 定义数据加载器
	loader := func(ctx context.Context, key string) (User, error) {
		// 模拟从数据库加载数据
		if key == "user:2" {
			return User{ID: 2, Name: "Bob"}, nil
		}
		return User{}, cache.ErrNotFound
	}

	// 获取缓存，如果不存在则通过 loader 加载
	user, err := userCache.Get(ctx, "user:2", loader)
	if err != nil {
		log.Printf("Failed to get user: %v", err)
		return
	}

	fmt.Printf("Loaded user: %+v\n", user)
	// Output: Loaded user: {ID:2 Name:Bob}
}

// Example_batch_operations 展示批量操作
func Example_batch_operations() {
	memory, _ := storage.NewOtter(1000)
	c, _ := cache.NewCache(cache.WithConfigMemory(memory))
	userCache := cache.Typed[User](c)

	ctx := context.Background()

	// 批量设置
	users := map[string]User{
		"user:1": {ID: 1, Name: "Alice"},
		"user:2": {ID: 2, Name: "Bob"},
		"user:3": {ID: 3, Name: "Charlie"},
	}

	if err := userCache.MSet(ctx, users); err != nil {
		log.Printf("Failed to batch set: %v", err)
		return
	}

	// 批量获取
	keys := []string{"user:1", "user:2", "user:3"}
	retrievedUsers, err := userCache.MGet(ctx, keys, nil)
	if err != nil {
		log.Printf("Failed to batch get: %v", err)
		return
	}

	for key, user := range retrievedUsers {
		fmt.Printf("%s: %+v\n", key, user)
	}
	// Output:
	// user:1: {ID:1 Name:Alice}
	// user:2: {ID:2 Name:Bob}
	// user:3: {ID:3 Name:Charlie}
}

// Example_custom_ttl 展示自定义过期时间
func Example_custom_ttl() {
	memory, _ := storage.NewOtter(1000)
	c, _ := cache.NewCache(cache.WithConfigMemory(memory))
	userCache := cache.Typed[User](c)

	ctx := context.Background()
	user := User{ID: 1, Name: "Alice"}

	// 设置自定义过期时间
	err := userCache.Set(ctx, "user:1", user,
		cache.WithTTL(5*time.Minute, 30*time.Minute), // 内存5分钟，Redis 30分钟
	)
	if err != nil {
		log.Printf("Failed to set with custom TTL: %v", err)
		return
	}

	fmt.Println("User cached with custom TTL")
	// Output: User cached with custom TTL
}

// Example_cache_not_found 展示缓存空值防穿透
func Example_cache_not_found() {
	memory, _ := storage.NewOtter(1000)
	c, _ := cache.NewCache(cache.WithConfigMemory(memory))
	userCache := cache.Typed[User](c)

	ctx := context.Background()

	// 定义返回空值的 loader
	loader := func(ctx context.Context, key string) (User, error) {
		return User{}, cache.ErrNotFound
	}

	// 第一次获取，会缓存空值
	_, err := userCache.Get(ctx, "user:999", loader,
		cache.WithCacheNotFound(true, time.Minute), // 缓存空值1分钟
	)
	if err != nil && !cache.IsNotFound(err) {
		log.Printf("Unexpected error: %v", err)
		return
	}

	// 第二次获取，直接从缓存返回空值，不会调用 loader
	_, err = userCache.Get(ctx, "user:999", loader)
	if err != nil && !cache.IsNotFound(err) {
		log.Printf("Unexpected error: %v", err)
		return
	}

	fmt.Println("Cache miss handled with empty value caching")
	// Output: Cache miss handled with empty value caching
}

// Example_delete_operation 展示删除操作
func Example_delete_operation() {
	memory, _ := storage.NewOtter(1000)
	c, _ := cache.NewCache(cache.WithConfigMemory(memory))
	userCache := cache.Typed[User](c)

	ctx := context.Background()
	user := User{ID: 1, Name: "Alice"}

	// 设置缓存
	if err := userCache.Set(ctx, "user:1", user); err != nil {
		log.Printf("Failed to set cache: %v", err)
		return
	}

	// 删除缓存
	if err := userCache.Delete(ctx, "user:1"); err != nil {
		log.Printf("Failed to delete cache: %v", err)
		return
	}

	// 验证删除成功
	_, err := userCache.Get(ctx, "user:1", nil)
	if err == nil {
		log.Println("Cache should have been deleted")
		return
	}

	fmt.Println("Cache successfully deleted")
	// Output: Cache successfully deleted
}
