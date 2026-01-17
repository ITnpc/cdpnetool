package cdp

import (
	"context"
	"time"

	"github.com/mafredri/cdp/protocol/fetch"

	"cdpnetool/internal/rules"
	"cdpnetool/pkg/model"
	"cdpnetool/pkg/rulespec"
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

	// 判断阶段
	stage := rulespec.StageRequest
	statusCode := 0
	if ev.ResponseStatusCode != nil {
		stage = rulespec.StageResponse
		statusCode = *ev.ResponseStatusCode
	}

	// 事件：拦截开始
	m.sendEvent(model.Event{
		Type:       "intercepted",
		Target:     ts.id,
		URL:        ev.Request.URL,
		Method:     ev.Request.Method,
		Stage:      string(stage),
		StatusCode: statusCode,
	})

	m.log.Debug("开始处理拦截事件", "stage", stage, "url", ev.Request.URL, "method", ev.Request.Method)

	// 构建评估上下文（基于请求信息）
	evalCtx := m.buildEvalContext(ts, ev)

	// 评估匹配规则
	if m.engine == nil {
		m.executor.ContinueRequest(ctx, ts, ev)
		return
	}
	matchedRules := m.engine.EvalForStage(evalCtx, stage)
	if len(matchedRules) == 0 {
		if stage == rulespec.StageRequest {
			m.executor.ContinueRequest(ctx, ts, ev)
		} else {
			m.executor.ContinueResponse(ctx, ts, ev)
		}
		m.log.Debug("拦截事件处理完成，无匹配规则", "stage", stage, "duration", time.Since(start))
		return
	}

	// 执行所有匹配规则的行为（aggregate 模式）
	if stage == rulespec.StageRequest {
		m.executeRequestStage(ctx, ts, ev, matchedRules, start)
	} else {
		m.executeResponseStage(ctx, ts, ev, matchedRules, start)
	}
}

// executeRequestStage 执行请求阶段的行为
func (m *Manager) executeRequestStage(ctx context.Context, ts *targetSession, ev *fetch.RequestPausedReply, matchedRules []*rules.MatchedRule, start time.Time) {
	var aggregatedMut *RequestMutation

	for _, matched := range matchedRules {
		rule := matched.Rule
		if len(rule.Actions) == 0 {
			continue
		}

		// 执行当前规则的所有行为
		mut := m.executor.ExecuteRequestActions(rule.Actions, ev)
		if mut == nil {
			continue
		}

		// 检查是否是终结性行为（block）
		if mut.Block != nil {
			// 使用 ActionExecutor 的 ApplyRequestMutation 来应用 block
			m.executor.ApplyRequestMutation(ctx, ts, ev, mut)
			m.sendEvent(model.Event{
				Type:   "blocked",
				Rule:   (*model.RuleID)(&rule.ID),
				Target: ts.id,
				URL:    ev.Request.URL,
				Method: ev.Request.Method,
				Stage:  string(rulespec.StageRequest),
			})
			m.log.Info("请求被阻止", "rule", rule.ID, "url", ev.Request.URL)
			return
		}

		// 聚合变更
		if aggregatedMut == nil {
			aggregatedMut = mut
		} else {
			mergeRequestMutation(aggregatedMut, mut)
		}
	}

	// 应用聚合后的变更
	if aggregatedMut != nil && hasRequestMutation(aggregatedMut) {
		m.executor.ApplyRequestMutation(ctx, ts, ev, aggregatedMut)
		m.sendEvent(model.Event{
			Type:   "mutated",
			Target: ts.id,
			URL:    ev.Request.URL,
			Method: ev.Request.Method,
			Stage:  string(rulespec.StageRequest),
		})
	} else {
		m.executor.ContinueRequest(ctx, ts, ev)
	}
	m.log.Debug("请求阶段处理完成", "duration", time.Since(start))
}

// executeResponseStage 执行响应阶段的行为
func (m *Manager) executeResponseStage(ctx context.Context, ts *targetSession, ev *fetch.RequestPausedReply, matchedRules []*rules.MatchedRule, start time.Time) {
	// 获取响应体
	responseBody, _ := m.executor.FetchResponseBody(ctx, ts, ev.RequestID)
	var aggregatedMut *ResponseMutation

	for _, matched := range matchedRules {
		rule := matched.Rule
		if len(rule.Actions) == 0 {
			continue
		}

		// 执行当前规则的所有行为
		mut := m.executor.ExecuteResponseActions(rule.Actions, ev, responseBody)
		if mut == nil {
			continue
		}

		// 聚合变更
		if aggregatedMut == nil {
			aggregatedMut = mut
		} else {
			mergeResponseMutation(aggregatedMut, mut)
		}

		// 更新 responseBody 供后续规则使用
		if mut.Body != nil {
			responseBody = *mut.Body
		}
	}

	// 应用聚合后的变更
	if aggregatedMut != nil && hasResponseMutation(aggregatedMut) {
		// 确保 Body 是最新的
		if aggregatedMut.Body == nil && responseBody != "" {
			aggregatedMut.Body = &responseBody
		}
		m.executor.ApplyResponseMutation(ctx, ts, ev, aggregatedMut)
		m.sendEvent(model.Event{
			Type:       "mutated",
			Target:     ts.id,
			URL:        ev.Request.URL,
			Method:     ev.Request.Method,
			Stage:      string(rulespec.StageResponse),
			StatusCode: getStatusCode(ev),
		})
	} else {
		m.executor.ContinueResponse(ctx, ts, ev)
	}
	m.log.Debug("响应阶段处理完成", "duration", time.Since(start))
}

// mergeRequestMutation 合并请求变更
func mergeRequestMutation(dst, src *RequestMutation) {
	if src.URL != nil {
		dst.URL = src.URL
	}
	if src.Method != nil {
		dst.Method = src.Method
	}
	for k, v := range src.Headers {
		if dst.Headers == nil {
			dst.Headers = make(map[string]string)
		}
		dst.Headers[k] = v
	}
	for k, v := range src.Query {
		if dst.Query == nil {
			dst.Query = make(map[string]string)
		}
		dst.Query[k] = v
	}
	for k, v := range src.Cookies {
		if dst.Cookies == nil {
			dst.Cookies = make(map[string]string)
		}
		dst.Cookies[k] = v
	}
	dst.RemoveHeaders = append(dst.RemoveHeaders, src.RemoveHeaders...)
	dst.RemoveQuery = append(dst.RemoveQuery, src.RemoveQuery...)
	dst.RemoveCookies = append(dst.RemoveCookies, src.RemoveCookies...)
	if src.Body != nil {
		dst.Body = src.Body
	}
}

// mergeResponseMutation 合并响应变更
func mergeResponseMutation(dst, src *ResponseMutation) {
	if src.StatusCode != nil {
		dst.StatusCode = src.StatusCode
	}
	for k, v := range src.Headers {
		if dst.Headers == nil {
			dst.Headers = make(map[string]string)
		}
		dst.Headers[k] = v
	}
	dst.RemoveHeaders = append(dst.RemoveHeaders, src.RemoveHeaders...)
	if src.Body != nil {
		dst.Body = src.Body
	}
}

// hasRequestMutation 检查请求变更是否有效
func hasRequestMutation(m *RequestMutation) bool {
	return m.URL != nil || m.Method != nil ||
		len(m.Headers) > 0 || len(m.Query) > 0 || len(m.Cookies) > 0 ||
		len(m.RemoveHeaders) > 0 || len(m.RemoveQuery) > 0 || len(m.RemoveCookies) > 0 ||
		m.Body != nil
}

// hasResponseMutation 检查响应变更是否有效
func hasResponseMutation(m *ResponseMutation) bool {
	return m.StatusCode != nil || len(m.Headers) > 0 || len(m.RemoveHeaders) > 0 || m.Body != nil
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
		m.log.Err(err, "订阅拦截事件流失败", "target", string(ts.id))
		m.handleTargetStreamClosed(ts, err)
		return
	}
	defer rp.Close()

	m.log.Info("开始消费拦截事件流", "target", string(ts.id))
	for {
		ev, err := rp.Recv()
		if err != nil {
			m.log.Err(err, "接收拦截事件失败", "target", string(ts.id))
			m.handleTargetStreamClosed(ts, err)
			return
		}
		m.dispatchPaused(ts, ev)
	}
}

// handleTargetStreamClosed 处理单个目标的拦截流终止
func (m *Manager) handleTargetStreamClosed(ts *targetSession, err error) {
	if !m.isEnabled() {
		m.log.Info("拦截已禁用，停止目标事件消费", "target", string(ts.id))
		return
	}

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
	m.executor.ContinueRequest(ctx, ts, ev)
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

// getStatusCode 获取响应状态码
func getStatusCode(ev *fetch.RequestPausedReply) int {
	if ev.ResponseStatusCode != nil {
		return *ev.ResponseStatusCode
	}
	return 0
}
