package cdp

import (
	"context"
	"math/rand"
	"time"

	"github.com/mafredri/cdp/protocol/fetch"

	"cdpnetool/pkg/model"
)

// handle 处理一次拦截事件并根据规则执行相应动作
func (m *Manager) handle(ts *targetSession, ev *fetch.RequestPausedReply) {
	to := m.processTimeoutMS
	if to <= 0 {
		to = 3000
	}
	ctx, cancel := context.WithTimeout(ts.ctx, time.Duration(to)*time.Millisecond)
	defer cancel()
	start := time.Now()

	stage := "request"
	statusCode := 0
	if ev.ResponseStatusCode != nil {
		stage = "response"
		statusCode = *ev.ResponseStatusCode
	}

	// 事件：拦截开始
	m.sendEvent(model.Event{
		Type:       "intercepted",
		Target:     ts.id,
		URL:        ev.Request.URL,
		Method:     ev.Request.Method,
		Stage:      stage,
		StatusCode: statusCode,
	})

	m.log.Debug("开始处理拦截事件", "stage", stage, "url", ev.Request.URL, "method", ev.Request.Method)

	res := m.decide(ts, ev, stage)
	if res == nil || res.Action == nil {
		m.applyContinue(ctx, ts, ev, stage)
		m.log.Debug("拦截事件处理完成", "stage", stage, "duration", time.Since(start))
		return
	}

	a := res.Action
	// DropRate：按概率降级直接放行
	if a.DropRate > 0 {
		if rand.Float64() < a.DropRate {
			m.applyContinue(ctx, ts, ev, stage)
			m.sendEvent(model.Event{Type: "degraded", Target: ts.id, URL: ev.Request.URL, Method: ev.Request.Method, Stage: stage})
			m.log.Warn("触发丢弃概率降级", "stage", stage)
			return
		}
	}

	// DelayMS：动作前注入固定延迟
	if a.DelayMS > 0 {
		time.Sleep(time.Duration(a.DelayMS) * time.Millisecond)
	}

	elapsed := time.Since(start)
	if elapsed > time.Duration(to)*time.Millisecond {
		m.applyContinue(ctx, ts, ev, stage)
		m.sendEvent(model.Event{Type: "degraded", Target: ts.id, URL: ev.Request.URL, Method: ev.Request.Method, Stage: stage})
		m.log.Warn("拦截处理超时自动降级", "stage", stage, "elapsed", elapsed, "timeout", to)
		return
	}

	// Pause：进入人工审批流程
	if a.Pause != nil {
		m.log.Info("应用暂停审批动作", "stage", stage)
		m.applyPause(ctx, ts, ev, a.Pause, stage, res.RuleID)
		return
	}

	// Fail：使请求失败
	if a.Fail != nil {
		m.log.Info("应用失败动作", "stage", stage)
		m.applyFail(ctx, ts, ev, a.Fail)
		m.sendEvent(model.Event{Type: "failed", Rule: res.RuleID, Target: ts.id, URL: ev.Request.URL, Method: ev.Request.Method, Stage: stage})
		m.log.Debug("拦截事件处理完成", "stage", stage, "duration", time.Since(start))
		return
	}

	// Respond：直接返回自定义响应
	if a.Respond != nil {
		m.log.Info("应用自定义响应动作", "stage", stage)
		m.applyRespond(ctx, ts, ev, a.Respond, stage)
		m.sendEvent(model.Event{Type: "fulfilled", Rule: res.RuleID, Target: ts.id, URL: ev.Request.URL, Method: ev.Request.Method, Stage: stage, StatusCode: a.Respond.Status})
		m.log.Debug("拦截事件处理完成", "stage", stage, "duration", time.Since(start))
		return
	}

	// Rewrite：重写请求/响应
	if a.Rewrite != nil {
		m.log.Info("应用请求响应重写动作", "stage", stage)
		m.applyRewrite(ctx, ts, ev, a.Rewrite, stage)
		m.sendEvent(model.Event{Type: "mutated", Rule: res.RuleID, Target: ts.id, URL: ev.Request.URL, Method: ev.Request.Method, Stage: stage, StatusCode: statusCode})
		m.log.Debug("拦截事件处理完成", "stage", stage, "duration", time.Since(start))
		return
	}

	// 默认：直接放行
	m.applyContinue(ctx, ts, ev, stage)
	m.log.Debug("拦截事件处理完成", "stage", stage, "duration", time.Since(start))
}

// dispatchPaused 根据并发配置调度单次拦截事件处理
func (m *Manager) dispatchPaused(ts *targetSession, ev *fetch.RequestPausedReply) {
	if m.pool == nil {
		go m.handle(ts, ev)
		return
	}
	submitted := m.pool.submit(func() {
		m.handle(ts, ev)
	})
	if !submitted {
		m.degradeAndContinue(ts, ev, "并发队列已满")
	}
}

// consume 持续接收拦截事件并按并发限制分发处理
func (m *Manager) consume(ts *targetSession) {
	rp, err := ts.client.Fetch.RequestPaused(ts.ctx)
	if err != nil {
		m.log.Error("订阅拦截事件流失败", "target", string(ts.id), "error", err)
		m.handleTargetStreamClosed(ts, err)
		return
	}
	defer rp.Close()

	m.log.Info("开始消费拦截事件流", "target", string(ts.id))
	for {
		ev, err := rp.Recv()
		if err != nil {
			m.log.Error("接收拦截事件失败", "target", string(ts.id), "error", err)
			m.handleTargetStreamClosed(ts, err)
			return
		}
		m.dispatchPaused(ts, ev)
	}
}

// handleTargetStreamClosed 处理单个目标的拦截流终止
func (m *Manager) handleTargetStreamClosed(ts *targetSession, err error) {
	// 如果是由于显式 Disable 导致的中断，则只停止消费，不移除目标
	if !m.isEnabled() {
		m.log.Info("拦截已禁用，停止目标事件消费", "target", string(ts.id))
		return
	}

	// 其他情况视为页面关闭或连接异常，自动移除该目标
	m.log.Warn("拦截流被中断，自动移除目标", "target", string(ts.id), "error", err)

	m.targetsMu.Lock()
	defer m.targetsMu.Unlock()

	if cur, ok := m.targets[ts.id]; ok && cur == ts {
		m.closeTargetSession(cur)
		delete(m.targets, ts.id)
	}
}

// degradeAndContinue 统一的降级处理：直接放行请求
func (m *Manager) degradeAndContinue(ts *targetSession, ev *fetch.RequestPausedReply, reason string) {
	m.log.Warn("执行降级策略：直接放行", "target", string(ts.id), "reason", reason, "requestID", ev.RequestID)
	ctx, cancel := context.WithTimeout(ts.ctx, 1*time.Second)
	defer cancel()
	args := &fetch.ContinueRequestArgs{RequestID: ev.RequestID}
	if err := ts.client.Fetch.ContinueRequest(ctx, args); err != nil {
		m.log.Error("降级放行请求失败", "target", string(ts.id), "error", err)
	}
	m.sendEvent(model.Event{Type: "degraded", Target: ts.id, URL: ev.Request.URL, Method: ev.Request.Method})
}

// sendEvent 安全发送事件到通道，自动添加时间戳
func (m *Manager) sendEvent(evt model.Event) {
	evt.Timestamp = time.Now().UnixMilli()
	select {
	case m.events <- evt:
	default:
	}
}
