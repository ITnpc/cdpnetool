package tracker

import (
	"cdpnetool/internal/logger"
	"sync"
	"time"
)

// Entry 事务追踪条目
type Entry struct {
	ID        string    // 事务唯一ID
	StartTime time.Time // 事务开始时间
	Data      any       // 关联的业务数据
}

// Tracker 事务追踪器，负责管理请求/响应生命周期内的上下文
type Tracker struct {
	pool    sync.Map
	timeout time.Duration
	log     logger.Logger
	done    chan struct{}
}

// New 创建一个新的事务追踪器
func New(timeout time.Duration, l logger.Logger) *Tracker {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	if l == nil {
		l = logger.NewNop()
	}
	t := &Tracker{
		timeout: timeout,
		log:     l,
		done:    make(chan struct{}),
	}
	go t.cleanupLoop()
	return t
}

// Set 存入事务关联数据
func (t *Tracker) Set(id string, data any) {
	t.pool.Store(id, &Entry{
		ID:        id,
		StartTime: time.Now(),
		Data:      data,
	})
}

// Get 获取并移除事务数据
func (t *Tracker) Get(id string) (any, bool) {
	val, ok := t.pool.LoadAndDelete(id)
	if !ok {
		return nil, false
	}
	return val.(*Entry).Data, true
}

// Peek 仅获取事务数据而不移除
func (t *Tracker) Peek(id string) (any, bool) {
	val, ok := t.pool.Load(id)
	if !ok {
		return nil, false
	}
	return val.(*Entry).Data, true
}

// Delete 手动删除事务数据
func (t *Tracker) Delete(id string) {
	t.pool.Delete(id)
}

// Stop 停止追踪器，释放资源
func (t *Tracker) Stop() {
	select {
	case <-t.done:
		return
	default:
		close(t.done)
	}
}

// cleanupLoop 定期清理过期事务的后台协程
func (t *Tracker) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-t.done:
			return
		case <-ticker.C:
			now := time.Now()
			t.pool.Range(func(key, value any) bool {
				entry := value.(*Entry)
				if now.Sub(entry.StartTime) > t.timeout {
					t.pool.Delete(key)
					t.log.Debug("清理过期事务数据", "id", key, "startTime", entry.StartTime)
				}
				return true
			})
		}
	}
}
