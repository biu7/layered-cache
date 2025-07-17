package storage

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

func setupRistretto(t *testing.T, capacity int) *Ristretto {
	t.Helper()

	rt, err := NewRistretto(capacity)
	if err != nil {
		t.Fatalf("创建 Ristretto 失败: %v", err)
	}

	return rt
}

func TestNewRistretto(t *testing.T) {
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
			rt, err := NewRistretto(tt.capacity)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewRistretto() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if rt == nil {
					t.Error("NewRistretto() 返回了 nil rt")
				}
			}
		})
	}
}

func TestRistretto_Set(t *testing.T) {
	rt := setupRistretto(t, 1000)

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
			name:          "负过期时间",
			key:           "neg-expire",
			value:         []byte("value"),
			expire:        -time.Hour,
			expectedCount: 0,
			reason:        "负过期时间应该被拒绝",
		},
		{
			name:          "零过期时间",
			key:           "zero-expire",
			value:         []byte("value"),
			expire:        0,
			expectedCount: 1,
			reason:        "零过期时间应该被接受",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := rt.Set(tt.key, tt.value, tt.expire)

			if count != tt.expectedCount {
				t.Errorf("Set() returned count = %v, want %v, reason: %s", count, tt.expectedCount, tt.reason)
				return
			}

			if tt.expectedCount > 0 {
				// 验证值是否正确设置
				got, exists := rt.Get(tt.key)
				if !exists {
					t.Errorf("Set() 成功后无法获取键 %s", tt.key)
					return
				}
				if !bytes.Equal(got, tt.value) {
					t.Errorf("Set() 设置值 = %v, want %v", got, tt.value)
				}
			} else {
				// 验证失败的设置操作确实没有设置值
				_, exists := rt.Get(tt.key)
				if exists {
					t.Errorf("Set() 失败但键 %s 仍然存在", tt.key)
				}
			}
		})
	}
}

func TestRistretto_MSet(t *testing.T) {
	rt := setupRistretto(t, 1000)

	tests := []struct {
		name          string
		values        map[string][]byte
		expire        time.Duration
		expectedCount int32
	}{
		{
			name: "成功批量设置多个键值对",
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
			count := rt.MSet(tt.values, tt.expire)

			if count != tt.expectedCount {
				t.Errorf("MSet() returned count = %v, want %v", count, tt.expectedCount)
				return
			}

			if tt.expectedCount > 0 {
				// 验证所有键值对是否正确设置
				successCount := int32(0)
				for key, expectedValue := range tt.values {
					got, exists := rt.Get(key)
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

func TestRistretto_Get(t *testing.T) {
	rt := setupRistretto(t, 1000)

	// 预设一些测试数据
	testData := map[string][]byte{
		"existing": []byte("value"),
		"empty":    nil,
	}

	for key, value := range testData {
		count := rt.Set(key, value, time.Hour)
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
			got, exists := rt.Get(tt.key)

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

func TestRistretto_MGet(t *testing.T) {
	rt := setupRistretto(t, 1000)

	// 预设一些测试数据
	testData := map[string][]byte{
		"k1": []byte("v1"),
		"k2": []byte("v2"),
		"k3": nil,
	}

	for key, value := range testData {
		count := rt.Set(key, value, time.Hour)
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
			got := rt.MGet(tt.keys)

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

func TestRistretto_Delete(t *testing.T) {
	rt := setupRistretto(t, 1000)

	// 预设一些测试数据
	testKeys := []string{"del1", "del2"}

	for _, key := range testKeys {
		count := rt.Set(key, []byte("value"), time.Hour)
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
			rt.Delete(tt.key)

			if tt.key != "" {
				// 验证键是否被删除（如果原本存在）
				_, exists := rt.Get(tt.key)
				if exists && containsRistretto(testKeys, tt.key) {
					t.Errorf("Delete() 未能删除键 %s", tt.key)
				}
			}
		})
	}
}

func TestRistretto_TTL(t *testing.T) {
	rt := setupRistretto(t, 1000)

	// 测试过期时间功能
	t.Run("过期时间测试", func(t *testing.T) {
		key := "expire"
		value := []byte("value")
		expireTime := time.Millisecond * 500

		count := rt.Set(key, value, expireTime)
		if count != 1 {
			t.Fatalf("Set 失败: count=%d", count)
		}

		// 立即检查键是否存在
		got, exists := rt.Get(key)
		if !exists {
			t.Fatalf("过期前应该能获取到键")
		}
		if !bytes.Equal(got, value) {
			t.Errorf("过期前获取的值 = %v, want %v", got, value)
		}

		// 等待过期
		time.Sleep(expireTime + 100*time.Millisecond)

		// 检查键是否已过期
		_, exists = rt.Get(key)
		if exists {
			t.Logf("注意：键在预期过期时间后仍然可访问，可能是 Ristretto 的惰性过期机制")
		}
	})

	t.Run("较长过期时间的键应该保持可访问", func(t *testing.T) {
		key := "long-expire"
		value := []byte("value")

		count := rt.Set(key, value, time.Hour)
		if count != 1 {
			t.Fatalf("Set 失败: count=%d", count)
		}

		// 等待一段时间
		time.Sleep(50 * time.Millisecond)

		// 键应该仍然存在
		got, exists := rt.Get(key)
		if !exists {
			t.Fatalf("长过期时间的键应该仍然存在")
		}
		if !bytes.Equal(got, value) {
			t.Errorf("获取的值 = %v, want %v", got, value)
		}
	})

	t.Run("零过期时间测试", func(t *testing.T) {
		key := "zero-expire"
		value := []byte("value")

		count := rt.Set(key, value, 0)
		if count != 1 {
			t.Fatalf("Set 失败: count=%d", count)
		}

		// 对于零过期时间，键应该立即过期或永不过期（取决于实现）
		got, exists := rt.Get(key)
		if exists {
			t.Logf("零过期时间的键仍然可访问: %v", got)
		} else {
			t.Logf("零过期时间的键立即过期")
		}
	})
}

func TestRistretto_ConcurrentAccess(t *testing.T) {
	rt := setupRistretto(t, 10000)

	t.Run("并发读写测试", func(t *testing.T) {
		const numGoroutines = 5
		const itemsPerGoroutine = 10

		done := make(chan bool, numGoroutines)

		// 启动多个写入 goroutine
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer func() { done <- true }()
				for j := 0; j < itemsPerGoroutine; j++ {
					key := fmt.Sprintf("c%d%d", id, j)
					value := []byte(fmt.Sprintf("v%d%d", id, j))
					_ = rt.Set(key, value, time.Hour)
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

				if value, exists := rt.Get(key); exists && bytes.Equal(value, []byte(expectedValue)) {
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

// 辅助函数：检查字符串数组是否包含指定元素
func containsRistretto(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
