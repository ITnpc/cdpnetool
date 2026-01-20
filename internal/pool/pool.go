package pool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cdpnetool/internal/logger"
)

// Pool 并发工作池，通过信号量控制活跃协程数，并提供阻塞/丢弃机制的任务队列
type Pool struct {
	sem         chan struct{} // 信号量通道，容量即为最大并发数
	queue       chan func()   // 任务缓冲队列
	queueCap    int           // 队列最大容量
	log         logger.Logger // 日志接口
	totalSubmit int64         // 累计提交任务数
	totalDrop   int64         // 累计丢弃任务数
	mu          sync.Mutex    // 保护统计字段的互斥锁
	stopMonitor chan struct{} // 停止监控协程的信号通道
}

// New 创建并发工作池实例
// size: 最大并发协程数；queueCap: 缓冲队列容量（若为0则默认为 size * 8）
func New(size int, queueCap int) *Pool {
	if size <= 0 {
		return &Pool{}
	}

	if queueCap <= 0 {
		queueCap = size * 8
	}

	return &Pool{
		sem:      make(chan struct{}, size),
		queue:    make(chan func(), queueCap),
		queueCap: queueCap,
	}
}

// SetLogger 设置日志记录器
func (p *Pool) SetLogger(l logger.Logger) {
	p.log = l
}

// Start 启动工作池，创建固定数量的 worker 协程并开启状态监控
func (p *Pool) Start(ctx context.Context) {
	if p.sem == nil {
		return
	}
	// 启动 worker 协程群
	for i := 0; i < cap(p.sem); i++ {
		go p.worker(ctx)
	}
	p.stopMonitor = make(chan struct{})
	go p.monitor(ctx)
}

// Stop 停止监控协程
func (p *Pool) Stop() {
	if p.stopMonitor != nil {
		close(p.stopMonitor)
	}
}

// monitor 定期输出工作池状态监控日志
func (p *Pool) monitor(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopMonitor:
			return
		case <-ticker.C:
			qLen, qCap, submit, drop := p.Stats()
			if p.log != nil && submit > 0 {
				usage := float64(qLen) / float64(qCap) * 100
				dropRate := float64(drop) / float64(submit) * 100
				p.log.Info("工作池状态监控", "queueLen", qLen, "queueCap", qCap, "usage", fmt.Sprintf("%.1f%%", usage), "totalSubmit", submit, "totalDrop", drop, "dropRate", fmt.Sprintf("%.2f%%", dropRate))
			}
		}
	}
}

// worker 工作协程，从队列中取任务并执行
func (p *Pool) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case fn := <-p.queue:
			if fn != nil {
				fn()
			}
		}
	}
}

// Submit 提交任务到工作池
// 如果池未启用限制，则直接启动新协程执行
// 如果队列已满，则增加丢弃计数并返回 false
func (p *Pool) Submit(fn func()) bool {
	if p.sem == nil {
		go fn()
		return true
	}
	p.mu.Lock()
	p.totalSubmit++
	p.mu.Unlock()
	select {
	case p.queue <- fn:
		return true
	default:
		p.mu.Lock()
		p.totalDrop++
		drop := p.totalDrop
		submit := p.totalSubmit
		p.mu.Unlock()
		if p.log != nil {
			p.log.Warn("工作池队列已满，任务被丢弃", "queueCap", p.queueCap, "totalSubmit", submit, "totalDrop", drop)
		}
		return false
	}
}

// Stats 返回工作池统计信息
func (p *Pool) Stats() (queueLen, queueCap, totalSubmit, totalDrop int64) {
	if p.sem == nil {
		return 0, 0, 0, 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return int64(len(p.queue)), int64(p.queueCap), p.totalSubmit, p.totalDrop
}

// GetQueueCap 返回队列容量
func (p *Pool) GetQueueCap() int {
	return p.queueCap
}

// IsEnabled 检查工作池是否已启用并发限制
func (p *Pool) IsEnabled() bool {
	return p.sem != nil
}
