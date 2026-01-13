package cdp

import (
	"context"
	"time"

	"github.com/mafredri/cdp/protocol/fetch"

	"cdpnetool/pkg/model"
	"cdpnetool/pkg/rulespec"
)

// applyPause 进入人工审批流程并按超时默认动作处理
func (m *Manager) applyPause(ctx context.Context, ts *targetSession, ev *fetch.RequestPausedReply, p *rulespec.Pause, stage string, ruleID *model.RuleID) {
	id := string(ev.RequestID)
	ch := m.registerApproval(id)

	if !m.sendPendingItem(id, stage, ev, ruleID, ts) {
		return
	}

	mut, timeout := m.waitForApproval(ch, p.TimeoutMS)
	m.applyApprovalResult(ctx, ts, ev, mut, timeout, p, stage)
	m.unregisterApproval(id)
}

// registerApproval 注册审批通道
func (m *Manager) registerApproval(id string) chan *rulespec.Rewrite {
	ch := make(chan *rulespec.Rewrite, 1)
	m.approvalsMu.Lock()
	m.approvals[id] = ch
	m.approvalsMu.Unlock()
	return ch
}

// unregisterApproval 注销审批通道
func (m *Manager) unregisterApproval(id string) {
	m.approvalsMu.Lock()
	delete(m.approvals, id)
	m.approvalsMu.Unlock()
}

// waitForApproval 等待审批结果或超时
// 返回值: *Rewrite 表示通过（可能有修改），nil+false 表示拒绝，nil+true 表示超时
func (m *Manager) waitForApproval(ch chan *rulespec.Rewrite, timeoutMS int) (mut *rulespec.Rewrite, timeout bool) {
	if timeoutMS <= 0 {
		timeoutMS = 3000
	}
	t := time.NewTimer(time.Duration(timeoutMS) * time.Millisecond)
	defer t.Stop()
	select {
	case mut := <-ch:
		return mut, false // mut==nil 表示拒绝
	case <-t.C:
		return nil, true // 超时
	}
}

// sendPendingItem 发送待审批项到 pending 通道
func (m *Manager) sendPendingItem(id, stage string, ev *fetch.RequestPausedReply, ruleID *model.RuleID, ts *targetSession) bool {
	if m.pending == nil {
		return true
	}
	item := model.PendingItem{
		ID:     id,
		Stage:  stage,
		URL:    ev.Request.URL,
		Method: ev.Request.Method,
		Target: ts.id,
		Rule:   ruleID,
	}
	select {
	case m.pending <- item:
		return true
	default:
		m.handlePauseOverflow(id, ts, ev)
		return false
	}
}

// handlePauseOverflow 处理 Pause 审批项超出 pending 队列容量的情况
func (m *Manager) handlePauseOverflow(id string, ts *targetSession, ev *fetch.RequestPausedReply) {
	m.log.Warn("Pause 审批项超出 pending 队列容量，触发降级", "id", id)
	// 队列满时直接降级为放行请求
	m.degradeAndContinue(ts, ev, "pending queue full")
}

// applyApprovalResult 应用审批结果或超时默认动作
func (m *Manager) applyApprovalResult(ctx context.Context, ts *targetSession, ev *fetch.RequestPausedReply, mut *rulespec.Rewrite, timeout bool, p *rulespec.Pause, stage string) {
	if timeout {
		// 超时，执行默认动作
		m.applyPauseDefaultAction(ctx, ts, ev, p, stage)
		return
	}
	if mut == nil {
		// 拒绝，使请求失败
		m.applyFail(ctx, ts, ev, &rulespec.Fail{Reason: "Aborted"})
		m.sendEvent(model.Event{Type: "rejected", Target: ts.id, URL: ev.Request.URL, Method: ev.Request.Method, Stage: stage})
		return
	}
	// 通过，应用修改
	if hasEffectiveMutations(*mut) {
		m.applyRewrite(ctx, ts, ev, mut, stage)
	} else {
		m.applyContinue(ctx, ts, ev, stage)
	}
}

// applyPauseDefaultAction 应用 Pause 超时默认动作
func (m *Manager) applyPauseDefaultAction(ctx context.Context, ts *targetSession, ev *fetch.RequestPausedReply, p *rulespec.Pause, stage string) {
	switch p.DefaultAction.Type {
	case rulespec.PauseDefaultActionFulfill:
		m.applyRespond(ctx, ts, ev, &rulespec.Respond{Status: p.DefaultAction.Status}, stage)
	case rulespec.PauseDefaultActionFail:
		m.applyFail(ctx, ts, ev, &rulespec.Fail{Reason: p.DefaultAction.Reason})
	case rulespec.PauseDefaultActionContinueMutated:
		m.applyContinue(ctx, ts, ev, stage)
	default:
		m.applyContinue(ctx, ts, ev, stage)
	}
}

// hasEffectiveMutations 检查变更是否有实际效果
func hasEffectiveMutations(mut rulespec.Rewrite) bool {
	if mut.URL != nil || mut.Method != nil {
		return true
	}
	if len(mut.Headers) > 0 || len(mut.Query) > 0 || len(mut.Cookies) > 0 {
		return true
	}
	if mut.Body != nil {
		return true
	}
	return false
}
