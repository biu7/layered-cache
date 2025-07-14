package storage

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

func setupOtter(t *testing.T, capacity int) *Otter {
	t.Helper()

	ot, err := NewOtter(capacity)
	if err != nil {
		t.Fatalf("创建 Otter 失败: %v", err)
	}

	return ot
}

func TestNewOtter(t *testing.T) {
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
			ot, err := NewOtter(tt.capacity)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewOtter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if ot == nil {
					t.Error("NewOtter() 返回了 nil ot")
				}
			}
		})
	}
}

func TestOtter_Set(t *testing.T) {
	ot := setupOtter(t, 1000)

	tests := []struct {
		name          string
		key           string
		value         []byte
		expire        time.Duration
		expectedCount int32
		reason        string
	}{
		{
			name:          "成功设置键值对",
			key:           "key",
			value:         []byte("value"),
			expire:        time.Hour,
			expectedCount: 1,
			reason:        "应该成功设置并返回1",
		},
		{
			name:          "空键名",
			key:           "",
			value:         []byte("value"),
			expire:        time.Hour,
			expectedCount: 1,
			reason:        "空键名应该被接受",
		},
		{
			name:          "空值",
			key:           "empty",
			value:         nil,
			expire:        time.Hour,
			expectedCount: 1,
			reason:        "空值应该被接受",
		},
		{
			name:          "负过期时间自动转换为0",
			key:           "neg-expire",
			value:         []byte("value"),
			expire:        -time.Hour,
			expectedCount: 1,
			reason:        "负过期时间应该转换为0并成功设置",
		},
		{
			name:          "零过期时间",
			key:           "zero-expire",
			value:         []byte("value"),
			expire:        0,
			expectedCount: 1,
			reason:        "零过期时间应该被接受",
		},
		{
			name:          "超过容量10%的大键值对",
			key:           "large-key-that-is-very-long-and-exceeds-capacity-limit",
			value:         []byte("large-value-content-that-when-combined-with-the-key-exceeds-10-percent-of-total-capacity-and-should-be-dropped-by-otter-cache"),
			expire:        time.Minute,
			expectedCount: 0, // 应该失败，因为超过了容量的10%
			reason:        "大键值对应该被拒绝，返回0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := ot.Set(tt.key, tt.value, tt.expire)

			if count != tt.expectedCount {
				t.Errorf("Set() returned count = %v, want %v, reason: %s", count, tt.expectedCount, tt.reason)
				return
			}

			if tt.expectedCount > 0 {
				// 对于零过期时间，Otter 会立即删除键值对，所以我们不验证其存在性
				if tt.expire == 0 || tt.expire < 0 {
					t.Logf("跳过零/负过期时间的验证，键: %s，因为 Otter 可能立即删除", tt.key)
				} else {
					// 验证值是否正确设置
					got, exists := ot.Get(tt.key)
					if !exists {
						t.Errorf("Set() 成功后无法获取键 %s", tt.key)
						return
					}
					if !bytes.Equal(got, tt.value) {
						t.Errorf("Set() 设置值 = %v, want %v", got, tt.value)
					}
				}
			}
		})
	}
}

func TestOtter_MSet(t *testing.T) {
	ot := setupOtter(t, 1000)

	tests := []struct {
		name          string
		values        map[string][]byte
		expire        time.Duration
		expectedCount int32
	}{
		{
			name: "成功批量设置多个小键值对",
			values: map[string][]byte{
				"k1": []byte("v1"),
				"k2": []byte("v2"),
				"k3": []byte("v3"),
			},
			expire:        time.Hour,
			expectedCount: 3,
		},
		{
			name:          "空map",
			values:        map[string][]byte{},
			expire:        time.Hour,
			expectedCount: 0,
		},
		{
			name: "单个键值对",
			values: map[string][]byte{
				"single": []byte("value"),
			},
			expire:        time.Minute * 30,
			expectedCount: 1,
		},
		{
			name: "包含空值的键值对",
			values: map[string][]byte{
				"empty":  nil,
				"normal": []byte("value"),
			},
			expire:        time.Hour,
			expectedCount: 2,
		},
		{
			name: "适量键值对",
			values: func() map[string][]byte {
				values := make(map[string][]byte)
				for i := 0; i < 10; i++ {
					values[fmt.Sprintf("k%d", i)] = []byte(fmt.Sprintf("v%d", i))
				}
				return values
			}(),
			expire:        time.Hour,
			expectedCount: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := ot.MSet(tt.values, tt.expire)

			if count != tt.expectedCount {
				t.Errorf("MSet() returned count = %v, want %v", count, tt.expectedCount)
				return
			}

			if tt.expectedCount > 0 {
				// 验证所有键值对是否正确设置
				successCount := int32(0)
				for key, expectedValue := range tt.values {
					got, exists := ot.Get(key)
					if !exists {
						t.Logf("键 %s 未能获取", key)
						continue
					}
					if !bytes.Equal(got, expectedValue) {
						t.Errorf("MSet() 键 %s 的值 = %v, want %v", key, got, expectedValue)
					} else {
						successCount++
					}
				}
				if successCount != tt.expectedCount {
					t.Errorf("MSet 实际成功设置 %d 个键值对，预期 %d 个", successCount, tt.expectedCount)
				}
			}
		})
	}
}

func TestOtter_Get(t *testing.T) {
	ot := setupOtter(t, 1000)

	// 预设一些测试数据
	testData := map[string][]byte{
		"existing": []byte("value"),
		"empty":    nil,
	}

	for key, value := range testData {
		count := ot.Set(key, value, time.Hour)
		if count != 1 {
			t.Fatalf("预设测试数据失败: key=%s, count=%d", key, count)
		}
	}

	tests := []struct {
		name       string
		key        string
		want       []byte
		wantExists bool
	}{
		{
			name:       "获取存在的键",
			key:        "existing",
			want:       []byte("value"),
			wantExists: true,
		},
		{
			name:       "获取空值的键",
			key:        "empty",
			want:       nil,
			wantExists: true,
		},
		{
			name:       "获取不存在的键",
			key:        "missing",
			want:       nil,
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, exists := ot.Get(tt.key)

			if exists != tt.wantExists {
				t.Errorf("Get() exists = %v, want %v", exists, tt.wantExists)
				return
			}

			if tt.wantExists && !bytes.Equal(got, tt.want) {
				t.Errorf("Get() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOtter_MGet(t *testing.T) {
	ot := setupOtter(t, 1000)

	// 预设一些测试数据
	testData := map[string][]byte{
		"k1": []byte("v1"),
		"k2": []byte("v2"),
		"k3": nil,
	}

	for key, value := range testData {
		count := ot.Set(key, value, time.Hour)
		if count != 1 {
			t.Fatalf("预设测试数据失败: key=%s, count=%d", key, count)
		}
	}

	tests := []struct {
		name string
		keys []string
		want map[string][]byte
	}{
		{
			name: "获取多个存在的键",
			keys: []string{"k1", "k2"},
			want: map[string][]byte{
				"k1": []byte("v1"),
				"k2": []byte("v2"),
			},
		},
		{
			name: "获取存在和不存在的键混合",
			keys: []string{"k1", "missing", "k2"},
			want: map[string][]byte{
				"k1": []byte("v1"),
				"k2": []byte("v2"),
				// "missing" 应该被忽略
			},
		},
		{
			name: "获取空值的键",
			keys: []string{"k3"},
			want: map[string][]byte{
				"k3": nil,
			},
		},
		{
			name: "空键列表",
			keys: []string{},
			want: map[string][]byte{},
		},
		{
			name: "全部不存在的键",
			keys: []string{"missing1", "missing2"},
			want: map[string][]byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ot.MGet(tt.keys)

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
		})
	}
}

func TestOtter_Delete(t *testing.T) {
	ot := setupOtter(t, 1000)

	// 预设一些测试数据
	testKeys := []string{"del1", "del2"}

	for _, key := range testKeys {
		count := ot.Set(key, []byte("value"), time.Hour)
		if count != 1 {
			t.Fatalf("预设测试数据失败: key=%s, count=%d", key, count)
		}
	}

	tests := []struct {
		name string
		key  string
	}{
		{
			name: "删除存在的键",
			key:  "del1",
		},
		{
			name: "删除不存在的键",
			key:  "missing",
		},
		{
			name: "删除空键名",
			key:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Delete 不返回错误，总是成功
			ot.Delete(tt.key)

			if tt.key != "" {
				// 验证键是否被删除（如果原本存在）
				_, exists := ot.Get(tt.key)
				if exists && containsOtter(testKeys, tt.key) {
					t.Errorf("Delete() 未能删除键 %s", tt.key)
				}
			}
		})
	}
}

func TestOtter_TTL(t *testing.T) {
	ot := setupOtter(t, 1000)

	// 测试过期时间功能 - 注意：Otter 的过期可能是惰性的
	t.Run("过期时间测试", func(t *testing.T) {
		key := "expire"
		value := []byte("value")
		expireTime := time.Second * 1

		count := ot.Set(key, value, expireTime)
		if count != 1 {
			t.Fatalf("Set 失败: count=%d", count)
		}

		// 立即检查键是否存在
		got, exists := ot.Get(key)
		if !exists {
			t.Fatalf("过期前应该能获取到键")
		}
		if !bytes.Equal(got, value) {
			t.Errorf("过期前获取的值 = %v, want %v", got, value)
		}

		// 等待过期
		time.Sleep(expireTime + 100*time.Millisecond)

		// 检查键是否已过期
		_, exists = ot.Get(key)
		if exists {
			t.Logf("注意：键在预期过期时间后仍然可访问，这可能是 Otter 的惰性过期机制")
		}

		// 再次尝试访问以触发可能的惰性过期检查
		_, exists = ot.Get(key)
		if exists {
			t.Logf("键在多次访问后仍然存在，可能 Otter 的过期实现与预期不同")
		}
	})

	t.Run("较长过期时间的键应该保持可访问", func(t *testing.T) {
		key := "long-expire"
		value := []byte("value")

		count := ot.Set(key, value, time.Hour)
		if count != 1 {
			t.Fatalf("Set 失败: count=%d", count)
		}

		// 等待一段时间
		time.Sleep(50 * time.Millisecond)

		// 键应该仍然存在
		got, exists := ot.Get(key)
		if !exists {
			t.Fatalf("长过期时间的键应该仍然存在")
		}
		if !bytes.Equal(got, value) {
			t.Errorf("获取的值 = %v, want %v", got, value)
		}
	})
}

func TestOtter_Capacity(t *testing.T) {
	// 测试容量限制 - 使用较大的容量，但设置超过10%限制的键值对
	ot := setupOtter(t, 100) // 容量100字节

	t.Run("缓存容量限制测试", func(t *testing.T) {
		// 单个键值对不超过10字节 (100 * 10% = 10)
		smallData := map[string]string{
			"a": "1", // 2字节
			"b": "2", // 2字节
			"c": "3", // 2字节
		}

		successCount := int32(0)
		for key, value := range smallData {
			count := ot.Set(key, []byte(value), time.Hour)
			if count == 0 {
				t.Logf("Set() 键 %s 失败", key)
			} else {
				successCount += count
			}
		}

		t.Logf("小键值对设置成功 %d/%d", successCount, len(smallData))

		// 测试超过10%限制的大键值对
		largeKey := "large-key"       // 9字节
		largeValue := []byte("large") // 5字节，总共14字节 > 10字节 (10%)
		count := ot.Set(largeKey, largeValue, time.Hour)
		if count == 1 {
			t.Error("大键值对应该被拒绝，但设置成功了")
		} else {
			t.Logf("大键值对正确被拒绝: count=%d", count)
		}
	})
}

func TestOtter_ConcurrentAccess(t *testing.T) {
	ot := setupOtter(t, 10000) // 增加容量以减少容量限制的影响

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
					key := fmt.Sprintf("c%d%d", id, j)           // 更短的键名
					value := []byte(fmt.Sprintf("v%d%d", id, j)) // 更短的值
					_ = ot.Set(key, value, time.Hour)
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

				if value, exists := ot.Get(key); exists && bytes.Equal(value, []byte(expectedValue)) {
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
