package tracker_test

import (
	"testing"
	"time"

	"cdpnetool/internal/logger"
	"cdpnetool/internal/tracker"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		want    time.Duration
	}{
		{"正常超时", 30 * time.Second, 30 * time.Second},
		{"零超时使用默认值", 0, 60 * time.Second},
		{"负超时使用默认值", -1 * time.Second, 60 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tracker.New(tt.timeout, logger.NewNop())
			defer tr.Stop()

			if tr == nil {
				t.Error("New() returned nil")
			}
		})
	}
}

func TestSetAndGet(t *testing.T) {
	tr := tracker.New(5*time.Second, logger.NewNop())
	defer tr.Stop()

	type testData struct {
		Name string
		Age  int
	}

	data := &testData{Name: "test", Age: 18}
	tr.Set("id1", data)

	got, ok := tr.Get("id1")
	if !ok {
		t.Error("Get() returned false")
	}

	gotData, ok := got.(*testData)
	if !ok {
		t.Error("type assertion failed")
	}

	if gotData.Name != data.Name || gotData.Age != data.Age {
		t.Errorf("got %+v, want %+v", gotData, data)
	}

	// 第二次Get应该失败（已被删除）
	_, ok = tr.Get("id1")
	if ok {
		t.Error("Get() should return false after first call")
	}
}

func TestPeek(t *testing.T) {
	tr := tracker.New(5*time.Second, logger.NewNop())
	defer tr.Stop()

	data := "test-data"
	tr.Set("id1", data)

	// 第一次Peek
	got, ok := tr.Peek("id1")
	if !ok {
		t.Error("Peek() returned false")
	}
	if got != data {
		t.Errorf("got %v, want %v", got, data)
	}

	// 第二次Peek仍应成功
	got, ok = tr.Peek("id1")
	if !ok {
		t.Error("Peek() should not delete data")
	}
	if got != data {
		t.Errorf("got %v, want %v", got, data)
	}
}

func TestDelete(t *testing.T) {
	tr := tracker.New(5*time.Second, logger.NewNop())
	defer tr.Stop()

	tr.Set("id1", "data")
	tr.Delete("id1")

	_, ok := tr.Peek("id1")
	if ok {
		t.Error("Delete() did not remove data")
	}
}

func TestCleanup(t *testing.T) {
	tr := tracker.New(100*time.Millisecond, logger.NewNop())
	defer tr.Stop()

	tr.Set("id1", "data")

	// 等待超时清理（cleanupLoop每30秒执行一次，加上超时时间）
	time.Sleep(35 * time.Second)

	_, ok := tr.Peek("id1")
	if ok {
		t.Error("cleanup did not remove expired data")
	}
}

func TestStop(t *testing.T) {
	tr := tracker.New(5*time.Second, logger.NewNop())
	tr.Stop()

	// 多次调用Stop应该安全
	tr.Stop()
	tr.Stop()
}

func TestGetNotExists(t *testing.T) {
	tr := tracker.New(5*time.Second, logger.NewNop())
	defer tr.Stop()

	_, ok := tr.Get("not-exist")
	if ok {
		t.Error("Get() should return false for non-existent id")
	}
}
