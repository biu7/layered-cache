package main

import (
	"context"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/biu7/layered-cache"
	"github.com/biu7/layered-cache/adapter"
	"github.com/redis/go-redis/v9"
)

type Repo struct {
	cache cache.Cache
}

type User struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func NewRepo() *Repo {
	memory, err := adapter.NewOtterAdapter(100_000) // 100Kb
	if err != nil {
		panic(err)
	}
	s, err := miniredis.Run()
	if err != nil {
		panic(err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	redisCache := adapter.NewRedisAdapterWithClient(rdb)

	c, err := cache.NewCache(
		cache.WithMemory(memory),                           // 添加内存缓存，可选
		cache.WithRedis(redisCache),                        // 添加 Redis 缓存，可选
		cache.WithDefaultTTL(time.Minute, 14*24*time.Hour), // 设置缓存默认过期时间，Set 时可覆盖
		cache.WithDefaultCacheNotFound(true, time.Minute),  // 查询无数据时是否缓存空值，防穿透
	)
	if err != nil {
		panic(err)
	}
	return &Repo{cache: c}
}

func (repo *Repo) Set() {
	ctx := context.Background()
	var user = User{
		ID:   1,
		Name: "test user",
	}

	// 泛型包装，不喜欢也可以直接 repo.cache.Get/Set
	userCache := cache.Typed[User](repo.cache)

	// 设置缓存
	err := userCache.Set(ctx, "user:1", user)
	if err != nil {
		panic(err)
	}

	// 设置缓存，覆盖内存过期时间
	err = userCache.Set(ctx, "user:1", user, cache.WithMemoryTTL(time.Minute))
	if err != nil {
		panic(err)
	}
	// 设置缓存，覆盖 Redis 过期时间
	err = userCache.Set(ctx, "user:1", user, cache.WithRedisTTL(time.Minute))
	if err != nil {
		panic(err)
	}
	// 设置缓存，覆盖内存与 Redis 过期时间
	err = userCache.Set(ctx, "user:1", user, cache.WithTTL(time.Minute, time.Hour))
	if err != nil {
		panic(err)
	}
}

func (repo *Repo) MSet() {
	ctx := context.Background()
	var user = User{
		ID:   1,
		Name: "test user",
	}

	// 泛型包装，不喜欢也可以直接 repo.cache.Get/Set
	userCache := cache.Typed[User](repo.cache)

	// 批量设置缓存，覆盖内存过期时间
	err := userCache.MSet(ctx, map[string]User{
		"user:1": user,
	}, cache.WithMemoryTTL(time.Minute))
	if err != nil {
		panic(err)
	}
	// 批量设置缓存，覆盖 Redis 过期时间
	err = userCache.MSet(ctx, map[string]User{
		"user:1": user,
	}, cache.WithRedisTTL(time.Minute))
	if err != nil {
		panic(err)
	}
	// 批量设置缓存，覆盖内存与 Redis 过期时间
	err = userCache.MSet(ctx, map[string]User{
		"user:1": user,
	}, cache.WithTTL(time.Minute, time.Hour))
	if err != nil {
		panic(err)
	}
}

func (repo *Repo) Delete() {
	repo.Set()

	ctx := context.Background()
	var user = User{
		ID:   1,
		Name: "test user",
	}

	// 泛型包装，不喜欢也可以直接 repo.cache.Get/Set
	userCache := cache.Typed[User](repo.cache)

	// 设置缓存
	err := userCache.Set(ctx, "user:1", user)
	if err != nil {
		panic(err)
	}

	// 删除缓存
	err = userCache.Delete(ctx, "user:1")
	if err != nil {
		panic(err)
	}
}

func (repo *Repo) Get() {
	ctx := context.Background()
	var user = User{
		ID:   1,
		Name: "test user",
	}

	// 泛型包装，不喜欢也可以直接 repo.cache.Get/Set
	userCache := cache.Typed[User](repo.cache)

	// 设置缓存
	err := userCache.Set(ctx, "user:1", user)
	if err != nil {
		panic(err)
	}

	// 直接获取缓存
	u, err := userCache.Get(ctx, "user:1", nil)
	if err != nil {
		panic(err)
	}
	if u.ID != user.ID || u.Name != user.Name {
		panic("invalid cache")
	}

	// 获取不存在的缓存
	u, err = userCache.Get(ctx, "user:2", nil)
	if err != nil && !cache.IsNotFound(err) {
		panic(err)
	}
	if !cache.IsNotFound(err) {
		panic("invalid cache")
	}

	// 不存在时从 loader 加载
	u, err = userCache.Get(ctx,
		"user:2",
		func(ctx context.Context, key string) (User, error) {
			return User{
				ID:   2,
				Name: "test user2",
			}, nil
		},
		cache.WithTTL(time.Minute, time.Hour), // 有 WithTTL WithMemoryTTL WithRedisTTL
	)
	if err != nil {
		panic(err)
	}
	if u.ID != 2 || u.Name != "test user2" {
		panic("invalid cache")
	}

	// 从 loader 加载也不存在时
	u, err = userCache.Get(ctx,
		"user:2",
		func(ctx context.Context, key string) (User, error) {
			return User{}, cache.ErrNotFound
		},
		cache.WithTTL(time.Minute, time.Hour),      // 有 WithTTL WithMemoryTTL WithRedisTTL
		cache.WithCacheNotFound(true, time.Minute), // 是否缓存数据不存在的状态，放穿透。可选，不设则使用默认值
	)
	if err != nil && !cache.IsNotFound(err) {
		panic(err)
	}
}

func (repo *Repo) MGet() {
	ctx := context.Background()
	var user = User{
		ID:   1,
		Name: "test user",
	}

	// 泛型包装，不喜欢也可以直接 repo.cache.Get/Set
	userCache := cache.Typed[User](repo.cache)

	// 设置缓存
	err := userCache.Set(ctx, "user:1", user)
	if err != nil {
		panic(err)
	}

	// 直接获取缓存
	u, err := userCache.MGet(ctx, []string{"user:1", "user:2"}, nil)
	if err != nil {
		panic(err)
	}

	if u["user:1"].ID != user.ID || u["user:1"].Name != user.Name {
		panic("invalid cache")
	}

	// 获取不存在的缓存
	u, err = userCache.MGet(ctx, []string{"user:100"}, nil)
	if err != nil {
		panic(err)
	}
	if len(u) != 0 {
		panic("invalid cache")
	}

	// 不存在时从 loader 加载
	u, err = userCache.MGet(ctx,
		[]string{"user:100"},
		func(ctx context.Context, keys []string) (map[string]User, error) {
			return map[string]User{
				"user:100": {
					ID:   100,
					Name: "test user100",
				},
			}, nil
		},
		cache.WithTTL(time.Minute, time.Hour), // 有 WithTTL WithMemoryTTL WithRedisTTL
	)
	if err != nil {
		panic(err)
	}
	if u["user:100"].ID != 100 || u["user:100"].Name != "test user100" {
		panic("invalid cache")
	}

	// 从 loader 加载也不存在时
	u, err = userCache.MGet(ctx,
		[]string{"user:200"},
		func(ctx context.Context, keys []string) (map[string]User, error) {
			return nil, cache.ErrNotFound
		},
		cache.WithTTL(time.Minute, time.Hour),      // 有 WithTTL WithMemoryTTL WithRedisTTL
		cache.WithCacheNotFound(true, time.Minute), // 是否缓存数据不存在的状态，放穿透。可选，不设则使用默认值
	)
	if err != nil && !cache.IsNotFound(err) {
		panic(err)
	}

	if len(u) != 0 {
		panic("invalid cache")
	}
}
