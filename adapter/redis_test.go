package adapter

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/biu7/layered-cache/errors"
	"github.com/redis/go-redis/v9"
)

func setupRedisAdapter(t *testing.T) (*RedisAdapter, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	adapter := NewRedisAdapterWithClient(client)
	return adapter, mr
}

func TestNewRedisAdapter(t *testing.T) {
	adapter, mr := setupRedisAdapter(t)
	defer mr.Close()

	if adapter == nil {
		t.Fatal("expected adapter to be non-nil")
	}

	if adapter.client == nil {
		t.Fatal("expected client to be non-nil")
	}
}

func TestRedisAdapter_Set(t *testing.T) {
	adapter, mr := setupRedisAdapter(t)
	defer mr.Close()

	tests := []struct {
		name    string
		key     string
		value   []byte
		expire  time.Duration
		wantErr bool
	}{
		{
			name:    "成功设置键值对",
			key:     "test-key",
			value:   []byte("test-value"),
			expire:  time.Hour,
			wantErr: false,
		},
		{
			name:    "设置键值对无过期时间",
			key:     "test-key-no-expire",
			value:   []byte("test-value"),
			expire:  0,
			wantErr: false,
		},
		{
			name:    "空键名",
			key:     "",
			value:   []byte("test-value"),
			expire:  time.Hour,
			wantErr: false, // Redis 允许空键名
		},
		{
			name:    "空值",
			key:     "test-empty-value",
			value:   nil,
			expire:  time.Hour,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := adapter.Set(ctx, tt.key, tt.value, tt.expire)

			if (err != nil) != tt.wantErr {
				t.Errorf("Set() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// 验证值是否正确设置
				got, err := adapter.Get(ctx, tt.key)
				if err != nil {
					t.Errorf("验证设置失败: %v", err)
					return
				}
				if !bytes.Equal(got, tt.value) {
					t.Errorf("Set() 设置值 = %v, want %v", got, tt.value)
				}

				// 验证过期时间
				if tt.expire > 0 {
					ttl := mr.TTL(tt.key)
					if ttl <= 0 || ttl > tt.expire {
						t.Errorf("Set() TTL = %v, expected between 0 and %v", ttl, tt.expire)
					}
				}
			}
		})
	}
}

func TestRedisAdapter_MSet(t *testing.T) {
	adapter, mr := setupRedisAdapter(t)
	defer mr.Close()

	tests := []struct {
		name    string
		values  map[string][]byte
		expire  time.Duration
		wantErr bool
	}{
		{
			name: "成功批量设置多个键值对",
			values: map[string][]byte{
				"key1": []byte("value1"),
				"key2": []byte("value2"),
				"key3": []byte("value3"),
			},
			expire:  time.Hour,
			wantErr: false,
		},
		{
			name:    "空map",
			values:  map[string][]byte{},
			expire:  time.Hour,
			wantErr: false,
		},
		{
			name: "单个键值对",
			values: map[string][]byte{
				"single-key": []byte("single-value"),
			},
			expire:  time.Minute * 30,
			wantErr: false,
		},
		{
			name: "包含空值的键值对",
			values: map[string][]byte{
				"key-with-empty": nil,
				"normal-key":     []byte("normal-value"),
			},
			expire:  time.Hour,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := adapter.MSet(ctx, tt.values, tt.expire)

			if (err != nil) != tt.wantErr {
				t.Errorf("MSet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(tt.values) > 0 {
				// 验证所有键值对是否正确设置
				for key, expectedValue := range tt.values {
					got, err := adapter.Get(ctx, key)
					if err != nil {
						t.Errorf("验证 MSet 设置失败，键 %s: %v", key, err)
						continue
					}
					if !bytes.Equal(got, expectedValue) {
						t.Errorf("MSet() 键 %s 的值 = %v, want %v", key, got, expectedValue)
					}

					// 验证过期时间
					if tt.expire > 0 {
						ttl := mr.TTL(key)
						if ttl <= 0 || ttl > tt.expire {
							t.Errorf("MSet() 键 %s 的 TTL = %v, expected between 0 and %v", key, ttl, tt.expire)
						}
					}
				}
			}
		})
	}
}

func TestRedisAdapter_Get(t *testing.T) {
	adapter, mr := setupRedisAdapter(t)
	defer mr.Close()

	// 预设一些测试数据
	ctx := context.Background()
	testData := map[string][]byte{
		"existing-key": []byte("existing-value"),
		"empty-value":  nil,
	}

	for key, value := range testData {
		if err := adapter.Set(ctx, key, value, time.Hour); err != nil {
			t.Fatalf("预设测试数据失败: %v", err)
		}
	}

	tests := []struct {
		name        string
		key         string
		want        []byte
		wantErr     bool
		expectedErr error
	}{
		{
			name:        "获取存在的键",
			key:         "existing-key",
			want:        []byte("existing-value"),
			wantErr:     false,
			expectedErr: nil,
		},
		{
			name:        "获取空值的键",
			key:         "empty-value",
			want:        nil,
			wantErr:     false,
			expectedErr: nil,
		},
		{
			name:        "获取不存在的键",
			key:         "non-existing-key",
			want:        nil,
			wantErr:     true,
			expectedErr: errors.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := adapter.Get(ctx, tt.key)

			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.expectedErr != nil {
				// 验证返回的是正确的错误类型
				if !errors.Is(err, tt.expectedErr) {
					t.Errorf("Get() error = %v, want %v", err, tt.expectedErr)
				}
			}

			if !tt.wantErr && !bytes.Equal(got, tt.want) {
				t.Errorf("Get() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRedisAdapter_MGet(t *testing.T) {
	adapter, mr := setupRedisAdapter(t)
	defer mr.Close()

	// 预设一些测试数据
	ctx := context.Background()
	testData := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": nil,
	}

	for key, value := range testData {
		if err := adapter.Set(ctx, key, value, time.Hour); err != nil {
			t.Fatalf("预设测试数据失败: %v", err)
		}
	}

	tests := []struct {
		name    string
		keys    []string
		want    map[string][]byte
		wantErr bool
	}{
		{
			name: "获取多个存在的键",
			keys: []string{"key1", "key2"},
			want: map[string][]byte{
				"key1": []byte("value1"),
				"key2": []byte("value2"),
			},
			wantErr: false,
		},
		{
			name: "获取存在和不存在的键混合",
			keys: []string{"key1", "non-existing", "key2"},
			want: map[string][]byte{
				"key1": []byte("value1"),
				"key2": []byte("value2"),
				// "non-existing" 应该被忽略
			},
			wantErr: false,
		},
		{
			name: "获取空值的键",
			keys: []string{"key3"},
			want: map[string][]byte{
				"key3": nil,
			},
			wantErr: false,
		},
		{
			name:    "空键列表",
			keys:    []string{},
			want:    map[string][]byte{},
			wantErr: false,
		},
		{
			name:    "全部不存在的键",
			keys:    []string{"non-existing1", "non-existing2"},
			want:    map[string][]byte{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := adapter.MGet(ctx, tt.keys)

			if (err != nil) != tt.wantErr {
				t.Errorf("MGet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("MGet() 返回键数量 = %v, want %v", len(got), len(tt.want))
					return
				}

				for key, expectedValue := range tt.want {
					if actualValue, exists := got[key]; !exists {
						t.Errorf("MGet() 缺少键 %s", key)
					} else if !bytes.Equal(actualValue, expectedValue) {
						t.Errorf("MGet() 键 %s 的值 = %v, want %v", key, actualValue, expectedValue)
					}
				}

				// 检查是否有额外的键
				for key := range got {
					if _, exists := tt.want[key]; !exists {
						t.Errorf("MGet() 包含意外的键 %s", key)
					}
				}
			}
		})
	}
}

func TestRedisAdapter_Delete(t *testing.T) {
	adapter, mr := setupRedisAdapter(t)
	defer mr.Close()

	// 预设一些测试数据
	ctx := context.Background()
	testKeys := []string{"to-delete-1", "to-delete-2"}

	for _, key := range testKeys {
		if err := adapter.Set(ctx, key, []byte("test-value"), time.Hour); err != nil {
			t.Fatalf("预设测试数据失败: %v", err)
		}
	}

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "删除存在的键",
			key:     "to-delete-1",
			wantErr: false,
		},
		{
			name:    "删除不存在的键",
			key:     "non-existing-key",
			wantErr: false, // Redis DEL 对不存在的键不返回错误
		},
		{
			name:    "删除空键名",
			key:     "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.Delete(ctx, tt.key)

			if (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.key != "" {
				// 验证键是否被删除（如果原本存在）
				_, err := adapter.Get(ctx, tt.key)
				if err == nil {
					// 如果键原本存在，现在应该获取不到
					if contains(testKeys, tt.key) {
						t.Errorf("Delete() 未能删除键 %s", tt.key)
					}
				}
			}
		})
	}
}

func TestRedisAdapter_ContextCancellation(t *testing.T) {
	adapter, mr := setupRedisAdapter(t)
	defer mr.Close()

	// 测试上下文取消时的行为
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消上下文

	// 所有操作都应该返回上下文取消错误
	err := adapter.Set(ctx, "test-key", []byte("test-value"), time.Hour)
	if err == nil {
		t.Error("Set() 在取消的上下文中应该返回错误")
	}

	_, err = adapter.Get(ctx, "test-key")
	if err == nil {
		t.Error("Get() 在取消的上下文中应该返回错误")
	}

	err = adapter.Delete(ctx, "test-key")
	if err == nil {
		t.Error("Delete() 在取消的上下文中应该返回错误")
	}

	_, err = adapter.MGet(ctx, []string{"test-key"})
	if err == nil {
		t.Error("MGet() 在取消的上下文中应该返回错误")
	}

	err = adapter.MSet(ctx, map[string][]byte{"test-key": []byte("test-value")}, time.Hour)
	if err == nil {
		t.Error("MSet() 在取消的上下文中应该返回错误")
	}
}

func TestRedisAdapter_ContextTimeout(t *testing.T) {
	adapter, mr := setupRedisAdapter(t)
	defer mr.Close()

	// 测试超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// 等待超时
	time.Sleep(1 * time.Millisecond)

	err := adapter.Set(ctx, "test-key", []byte("test-value"), time.Hour)
	if err == nil {
		t.Error("Set() 在超时的上下文中应该返回错误")
	}
}

// 辅助函数：检查字符串数组是否包含指定元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// 基准测试
func BenchmarkRedisAdapter_Set(b *testing.B) {
	mr := miniredis.RunT(b)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	adapter := NewRedisAdapterWithClient(client)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench-key-%d", i)
			value := []byte(fmt.Sprintf("bench-value-%d", i))
			_ = adapter.Set(ctx, key, value, time.Hour)
			i++
		}
	})
}

func BenchmarkRedisAdapter_Get(b *testing.B) {
	mr := miniredis.RunT(b)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	adapter := NewRedisAdapterWithClient(client)
	ctx := context.Background()

	// 预设数据
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("bench-key-%d", i)
		value := []byte(fmt.Sprintf("bench-value-%d", i))
		_ = adapter.Set(ctx, key, value, time.Hour)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench-key-%d", i%1000)
			_, _ = adapter.Get(ctx, key)
			i++
		}
	})
}

func BenchmarkRedisAdapter_MGet(b *testing.B) {
	mr := miniredis.RunT(b)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	adapter := NewRedisAdapterWithClient(client)
	ctx := context.Background()

	// 预设数据
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("bench-key-%d", i)
		value := []byte(fmt.Sprintf("bench-value-%d", i))
		_ = adapter.Set(ctx, key, value, time.Hour)
	}

	keys := make([]string, 10)
	for i := 0; i < 10; i++ {
		keys[i] = fmt.Sprintf("bench-key-%d", i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = adapter.MGet(ctx, keys)
		}
	})
}
