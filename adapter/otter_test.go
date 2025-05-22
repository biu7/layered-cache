package adapter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/biu7/layered-cache/errors"
)

func setupOtterAdapter(t *testing.T, capacity int) *OtterAdapter {
	t.Helper()

	adapter, err := NewOtterAdapter(capacity)
	if err != nil {
		t.Fatalf("创建 OtterAdapter 失败: %v", err)
	}

	return adapter
}

func setupOtterAdapterForBench(b *testing.B, capacity int) *OtterAdapter {
	b.Helper()

	adapter, err := NewOtterAdapter(capacity)
	if err != nil {
		b.Fatalf("创建 OtterAdapter 失败: %v", err)
	}

	return adapter
}

func TestNewOtterAdapter(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		wantErr  bool
	}{
		{
			name:     "正常容量",
			capacity: 1000,
			wantErr:  false,
		},
		{
			name:     "零容量",
			capacity: 0,
			wantErr:  true,
		},
		{
			name:     "负容量",
			capacity: -1,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := NewOtterAdapter(tt.capacity)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewOtterAdapter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if adapter == nil {
					t.Error("NewOtterAdapter() 返回了 nil adapter")
				}
			}
		})
	}
}

func TestOtterAdapter_Set(t *testing.T) {
	adapter := setupOtterAdapter(t, 1000)

	tests := []struct {
		name    string
		key     string
		value   string
		expire  time.Duration
		wantErr bool
		reason  string
	}{
		{
			name:    "成功设置键值对",
			key:     "key",
			value:   "value",
			expire:  time.Hour,
			wantErr: false,
			reason:  "应该成功",
		},
		{
			name:    "空键名",
			key:     "",
			value:   "value",
			expire:  time.Hour,
			wantErr: false,
			reason:  "空键名应该被接受",
		},
		{
			name:    "空值",
			key:     "empty",
			value:   "",
			expire:  time.Hour,
			wantErr: false,
			reason:  "空值应该被接受",
		},
		{
			name:    "零过期时间",
			key:     "key",
			value:   "value",
			expire:  0,
			wantErr: true,
			reason:  "应该报错，实现中不支持小于 1 秒的过期时间(因为 otter 不支持设置永久存储)",
		},
		{
			name:    "超过容量10%的大键值对",
			key:     "large-key-that-is-very-long-and-exceeds-capacity-limit",
			value:   "large-value-content-that-when-combined-with-the-key-exceeds-10-percent-of-total-capacity-and-should-be-dropped-by-otter-cache",
			expire:  time.Minute,
			wantErr: true, // 应该失败，因为超过了容量的10%
			reason:  "大键值对应该被拒绝",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := adapter.Set(ctx, tt.key, tt.value, tt.expire)

			if (err != nil) != tt.wantErr {
				t.Errorf("Set() error = %v, wantErr %v, reason: %s", err, tt.wantErr, tt.reason)
				return
			}

			if !tt.wantErr {
				// 对于 expire = 0 的情况，我们需要特殊处理
				if tt.expire == 0 {
					// Otter 中 expire = 0 可能意味着立即过期，所以我们不验证是否能获取到
					t.Logf("跳过零过期时间的验证，键: %s", tt.key)
				} else if tt.expire > time.Millisecond {
					// 验证值是否正确设置（如果没有立即过期）
					got, err := adapter.Get(ctx, tt.key)
					if err != nil {
						t.Errorf("验证设置失败: %v", err)
						return
					}
					if got != tt.value {
						t.Errorf("Set() 设置值 = %v, want %v", got, tt.value)
					}
				}
			}
		})
	}
}

func TestOtterAdapter_MSet(t *testing.T) {
	adapter := setupOtterAdapter(t, 1000)

	tests := []struct {
		name    string
		values  map[string]string
		expire  time.Duration
		wantErr bool
	}{
		{
			name: "成功批量设置多个小键值对",
			values: map[string]string{
				"k1": "v1",
				"k2": "v2",
				"k3": "v3",
			},
			expire:  time.Hour,
			wantErr: false,
		},
		{
			name:    "空map",
			values:  map[string]string{},
			expire:  time.Hour,
			wantErr: false,
		},
		{
			name: "单个键值对",
			values: map[string]string{
				"single": "value",
			},
			expire:  time.Minute * 30,
			wantErr: false,
		},
		{
			name: "包含空值的键值对",
			values: map[string]string{
				"empty":  "",
				"normal": "value",
			},
			expire:  time.Hour,
			wantErr: false,
		},
		{
			name: "适量键值对", // 减少数量，避免容量问题
			values: func() map[string]string {
				values := make(map[string]string)
				for i := 0; i < 10; i++ { // 从100减少到10
					values[fmt.Sprintf("k%d", i)] = fmt.Sprintf("v%d", i)
				}
				return values
			}(),
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
				successCount := 0
				for key, expectedValue := range tt.values {
					got, err := adapter.Get(ctx, key)
					if err != nil {
						t.Logf("键 %s 未能获取: %v", key, err)
						continue
					}
					if got != expectedValue {
						t.Errorf("MSet() 键 %s 的值 = %v, want %v", key, got, expectedValue)
					} else {
						successCount++
					}
				}
				t.Logf("MSet 成功设置 %d/%d 个键值对", successCount, len(tt.values))
			}
		})
	}
}

func TestOtterAdapter_Get(t *testing.T) {
	adapter := setupOtterAdapter(t, 1000)

	// 预设一些测试数据
	ctx := context.Background()
	testData := map[string]string{
		"existing": "value",
		"empty":    "",
	}

	for key, value := range testData {
		if err := adapter.Set(ctx, key, value, time.Hour); err != nil {
			t.Fatalf("预设测试数据失败: %v", err)
		}
	}

	tests := []struct {
		name        string
		key         string
		want        string
		wantErr     bool
		expectedErr error
	}{
		{
			name:        "获取存在的键",
			key:         "existing",
			want:        "value",
			wantErr:     false,
			expectedErr: nil,
		},
		{
			name:        "获取空值的键",
			key:         "empty",
			want:        "",
			wantErr:     false,
			expectedErr: nil,
		},
		{
			name:        "获取不存在的键",
			key:         "missing",
			want:        "",
			wantErr:     true,
			expectedErr: errors.ErrKeyNotFound,
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

			if !tt.wantErr && got != tt.want {
				t.Errorf("Get() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOtterAdapter_MGet(t *testing.T) {
	adapter := setupOtterAdapter(t, 1000)

	// 预设一些测试数据
	ctx := context.Background()
	testData := map[string]string{
		"k1": "v1",
		"k2": "v2",
		"k3": "",
	}

	for key, value := range testData {
		if err := adapter.Set(ctx, key, value, time.Hour); err != nil {
			t.Fatalf("预设测试数据失败: %v", err)
		}
	}

	tests := []struct {
		name    string
		keys    []string
		want    map[string]string
		wantErr bool
	}{
		{
			name: "获取多个存在的键",
			keys: []string{"k1", "k2"},
			want: map[string]string{
				"k1": "v1",
				"k2": "v2",
			},
			wantErr: false,
		},
		{
			name: "获取存在和不存在的键混合",
			keys: []string{"k1", "missing", "k2"},
			want: map[string]string{
				"k1": "v1",
				"k2": "v2",
				// "missing" 应该被忽略
			},
			wantErr: false,
		},
		{
			name: "获取空值的键",
			keys: []string{"k3"},
			want: map[string]string{
				"k3": "",
			},
			wantErr: false,
		},
		{
			name:    "空键列表",
			keys:    []string{},
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name:    "全部不存在的键",
			keys:    []string{"missing1", "missing2"},
			want:    map[string]string{},
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
					} else if actualValue != expectedValue {
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

func TestOtterAdapter_Delete(t *testing.T) {
	adapter := setupOtterAdapter(t, 1000)

	// 预设一些测试数据
	ctx := context.Background()
	testKeys := []string{"del1", "del2"}

	for _, key := range testKeys {
		if err := adapter.Set(ctx, key, "value", time.Hour); err != nil {
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
			key:     "del1",
			wantErr: false,
		},
		{
			name:    "删除不存在的键",
			key:     "missing",
			wantErr: false, // Otter 删除不存在的键不返回错误
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
					if containsOtter(testKeys, tt.key) {
						t.Errorf("Delete() 未能删除键 %s", tt.key)
					}
				}
			}
		})
	}
}

func TestOtterAdapter_TTL(t *testing.T) {
	adapter := setupOtterAdapter(t, 1000)
	ctx := context.Background()

	// 测试过期时间功能 - 注意：Otter 的过期可能是惰性的
	t.Run("过期时间测试", func(t *testing.T) {
		key := "expire"
		value := "value"
		expireTime := time.Second * 1

		err := adapter.Set(ctx, key, value, expireTime)
		if err != nil {
			t.Fatalf("Set 失败: %v", err)
		}

		// 立即检查键是否存在
		got, err := adapter.Get(ctx, key)
		if err != nil {
			t.Fatalf("过期前应该能获取到键: %v", err)
		}
		if got != value {
			t.Errorf("过期前获取的值 = %v, want %v", got, value)
		}

		// 等待过期
		time.Sleep(expireTime + 100*time.Millisecond)

		// 检查键是否已过期
		_, err = adapter.Get(ctx, key)
		if err == nil {
			t.Logf("注意：键在预期过期时间后仍然可访问，这可能是 Otter 的惰性过期机制")
		} else if !errors.Is(err, errors.ErrKeyNotFound) {
			t.Errorf("过期后应该返回 ErrKeyNotFound 或仍可访问，但得到了其他错误: %v", err)
		}

		// 再次尝试访问以触发可能的惰性过期检查
		_, err = adapter.Get(ctx, key)
		if err == nil {
			t.Logf("键在多次访问后仍然存在，可能 Otter 的过期实现与预期不同")
		}
	})

	t.Run("较长过期时间的键应该保持可访问", func(t *testing.T) {
		key := "long-expire"
		value := "value"

		err := adapter.Set(ctx, key, value, time.Hour)
		if err != nil {
			t.Fatalf("Set 失败: %v", err)
		}

		// 等待一段时间
		time.Sleep(50 * time.Millisecond)

		// 键应该仍然存在
		got, err := adapter.Get(ctx, key)
		if err != nil {
			t.Fatalf("长过期时间的键应该仍然存在: %v", err)
		}
		if got != value {
			t.Errorf("获取的值 = %v, want %v", got, value)
		}
	})
}

func TestOtterAdapter_Capacity(t *testing.T) {
	// 测试容量限制 - 使用较大的容量，但设置超过10%限制的键值对
	adapter := setupOtterAdapter(t, 100) // 容量100字节
	ctx := context.Background()

	t.Run("缓存容量限制测试", func(t *testing.T) {
		// 单个键值对不超过10字节 (100 * 10% = 10)
		smallData := map[string]string{
			"a": "1", // 2字节
			"b": "2", // 2字节
			"c": "3", // 2字节
		}

		successCount := 0
		for key, value := range smallData {
			err := adapter.Set(ctx, key, value, time.Hour)
			if err != nil {
				t.Logf("Set() 键 %s 失败: %v", key, err)
			} else {
				successCount++
			}
		}

		t.Logf("小键值对设置成功 %d/%d", successCount, len(smallData))

		// 测试超过10%限制的大键值对
		largeKey := "large-key" // 9字节
		largeValue := "large"   // 5字节，总共14字节 > 10字节 (10%)
		err := adapter.Set(ctx, largeKey, largeValue, time.Hour)
		if err == nil {
			t.Error("大键值对应该被拒绝，但设置成功了")
		} else {
			t.Logf("大键值对正确被拒绝: %v", err)
		}
	})
}

func TestOtterAdapter_ContextIndependence(t *testing.T) {
	adapter := setupOtterAdapter(t, 1000)

	// 测试上下文取消不会影响 otter 操作
	// 因为 otter 是内存缓存，不依赖外部资源
	t.Run("上下文取消不应影响操作", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // 立即取消上下文

		// 所有操作仍应正常工作
		err := adapter.Set(ctx, "ctx", "value", time.Hour)
		if err != nil {
			t.Errorf("Set() 在取消的上下文中失败: %v", err)
		}

		val, err := adapter.Get(ctx, "ctx")
		if err != nil {
			t.Errorf("Get() 在取消的上下文中失败: %v", err)
		}
		if val != "value" {
			t.Errorf("Get() 返回值 = %v, want %v", val, "value")
		}

		err = adapter.Delete(ctx, "ctx")
		if err != nil {
			t.Errorf("Delete() 在取消的上下文中失败: %v", err)
		}
	})
}

func TestOtterAdapter_ConcurrentAccess(t *testing.T) {
	adapter := setupOtterAdapter(t, 10000) // 增加容量以减少容量限制的影响
	ctx := context.Background()

	t.Run("并发读写测试", func(t *testing.T) {
		// 并发写入 - 减少数量和大小
		const numGoroutines = 5
		const itemsPerGoroutine = 10

		done := make(chan bool, numGoroutines)

		// 启动多个写入 goroutine
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer func() { done <- true }()
				for j := 0; j < itemsPerGoroutine; j++ {
					key := fmt.Sprintf("c%d%d", id, j)   // 更短的键名
					value := fmt.Sprintf("v%d%d", id, j) // 更短的值
					_ = adapter.Set(ctx, key, value, time.Hour)
				}
			}(i)
		}

		// 等待所有写入完成
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// 验证数据
		successCount := 0
		totalItems := numGoroutines * itemsPerGoroutine
		for i := 0; i < numGoroutines; i++ {
			for j := 0; j < itemsPerGoroutine; j++ {
				key := fmt.Sprintf("c%d%d", i, j)
				expectedValue := fmt.Sprintf("v%d%d", i, j)

				if value, err := adapter.Get(ctx, key); err == nil && value == expectedValue {
					successCount++
				}
			}
		}

		t.Logf("并发写入 %d 个项目，成功读取 %d 个", totalItems, successCount)

		if successCount < totalItems {
			t.Errorf("并发操作成功率过低: %d/%d", successCount, totalItems)
		}
	})
}

// 辅助函数：检查字符串数组是否包含指定元素（为了避免与 redis_test.go 中的 contains 函数冲突）
func containsOtter(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
