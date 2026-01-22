package handler

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"sync"
	"time"

	"cdpnetool/internal/executor"
	"cdpnetool/internal/logger"
	"cdpnetool/internal/protocol"
	"cdpnetool/internal/rules"
	"cdpnetool/pkg/domain"
	"cdpnetool/pkg/rulespec"

	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/protocol/fetch"
)

// Handler 事件处理器，负责协调规则匹配、行为执行和全周期事件合并
type Handler struct {
	engine           *rules.Engine
	executor         *executor.Executor
	events           chan domain.NetworkEvent
	processTimeoutMS int
	log              logger.Logger
	collectUnmatched bool     // 是否收集未匹配的请求
	pendingPool      sync.Map // 在途请求池: map[RequestID]*PendingRequest
}

// PendingRequest 暂存在内存中的请求阶段信息
type PendingRequest struct {
	TraceID      string
	StartTime    time.Time
	RequestInfo  domain.RequestInfo
	MatchedRules []domain.RuleMatch
	IsMatched    bool
}

// Config 配置选项
type Config struct {
	Engine           *rules.Engine
	Executor         *executor.Executor
	Events           chan domain.NetworkEvent
	ProcessTimeoutMS int
	Logger           logger.Logger
	CollectUnmatched bool
}

// New 创建事件处理器并启动清理协程
func New(cfg Config) *Handler {
	h := &Handler{
		engine:           cfg.Engine,
		executor:         cfg.Executor,
		events:           cfg.Events,
		processTimeoutMS: cfg.ProcessTimeoutMS,
		log:              cfg.Logger,
		collectUnmatched: cfg.CollectUnmatched,
	}
	go h.cleanupLoop()
	return h
}

// SetCollectUnmatched 动态设置是否收集未匹配请求
func (h *Handler) SetCollectUnmatched(collect bool) {
	h.collectUnmatched = collect
}

// cleanupLoop 定期清理内存池中的孤儿请求（防止由于浏览器异常导致的数据残留）
func (h *Handler) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		h.pendingPool.Range(func(key, value any) bool {
			req, ok := value.(*PendingRequest)
			if ok && now.Sub(req.StartTime) > 60*time.Second {
				h.pendingPool.Delete(key)
				h.log.Debug("清理过期请求记录", "requestID", key, "traceID", req.TraceID)
			}
			return true
		})
	}
}

// SetEngine 设置规则引擎
func (h *Handler) SetEngine(engine *rules.Engine) {
	h.engine = engine
}

// SetProcessTimeout 设置处理超时时间
func (h *Handler) SetProcessTimeout(timeoutMS int) {
	h.processTimeoutMS = timeoutMS
}

// HandleRequest 处理请求拦截
func (h *Handler) HandleRequest(
	ctx context.Context,
	targetID domain.TargetID,
	client *cdp.Client,
	ev *fetch.RequestPausedReply,
	l logger.Logger,
	traceID string,
) {
	evalCtx := h.buildEvalContext(ev)
	if h.engine == nil {
		if h.collectUnmatched {
			h.saveToPool(ev, nil, nil, false, traceID)
		}
		h.executor.ContinueRequest(ctx, client, ev)
		return
	}

	start := time.Now()
	matchedRules := h.engine.EvalForStage(evalCtx, rulespec.StageRequest)

	if len(matchedRules) == 0 {
		if h.collectUnmatched {
			h.saveToPool(ev, nil, nil, false, traceID)
		}
		h.executor.ContinueRequest(ctx, client, ev)
		return
	}

	// 1. 计算变更
	mutation, blockRule, ruleMatches := h.computeRequestMutation(ev, matchedRules)

	// 2. 执行修改
	if blockRule != nil {
		h.executor.ApplyRequestMutation(ctx, client, ev, mutation)
		originalInfo := h.captureRequestData(ev)
		h.emitRequestEvent(targetID, "blocked", ruleMatches, originalInfo, mutation, start, l)
		return
	}

	if mutation != nil && hasRequestMutation(mutation) {
		h.executor.ApplyRequestMutation(ctx, client, ev, mutation)
	} else {
		h.executor.ContinueRequest(ctx, client, ev)
	}

	// 3. 入池暂存
	h.saveToPool(ev, mutation, ruleMatches, true, traceID)
}

// saveToPool 将请求信息存入待处理池
func (h *Handler) saveToPool(
	ev *fetch.RequestPausedReply,
	mut *executor.RequestMutation,
	matches []domain.RuleMatch,
	isMatched bool,
	traceID string,
) {
	original := h.captureRequestData(ev)
	finalRequest := original
	if mut != nil {
		finalRequest = h.captureModifiedRequestData(original, mut)
	}

	h.pendingPool.Store(ev.RequestID, &PendingRequest{
		TraceID:      traceID,
		StartTime:    time.Now(),
		RequestInfo:  finalRequest,
		MatchedRules: matches,
		IsMatched:    isMatched,
	})
}

// HandleResponse 处理响应拦截
func (h *Handler) HandleResponse(
	client *cdp.Client,
	ctx context.Context,
	targetID domain.TargetID,
	ev *fetch.RequestPausedReply,
	l logger.Logger,
	traceID string,
) {
	// 1. 从池中检索关联的请求信息
	val, ok := h.pendingPool.Load(ev.RequestID)
	if !ok {
		h.executor.ContinueResponse(ctx, client, ev)
		return
	}
	pending := val.(*PendingRequest)
	defer h.pendingPool.Delete(ev.RequestID)

	start := pending.StartTime
	l = l.With("traceID", pending.TraceID)

	// 2. 无论是否匹配，强制采集响应体（满足用户完整性需求）
	originalReqInfo := pending.RequestInfo
	_, originalResInfo := h.captureResponseData(client, ctx, ev)

	// 3. 计算并应用响应阶段的变更
	evalCtx := h.buildEvalContext(ev)
	matchedRules := h.engine.EvalForStage(evalCtx, rulespec.StageResponse)

	mutation, ruleMatches, finalBody := h.computeResponseMutation(ev, matchedRules, originalResInfo.Body)

	var finalResult string
	if mutation != nil && hasResponseMutation(mutation) {
		if mutation.Body == nil && finalBody != "" {
			mutation.Body = &finalBody
		}
		h.executor.ApplyResponseMutation(ctx, client, ev, mutation)
		finalResult = "modified"
	} else {
		h.executor.ContinueResponse(ctx, client, ev)
		finalResult = "passed"
		if pending.IsMatched {
			finalResult = "matched"
		}
	}

	// 4. 发送原子化全周期事件
	allMatches := append(pending.MatchedRules, ruleMatches...)
	h.emitResponseEvent(targetID, finalResult, allMatches, originalReqInfo, originalResInfo, mutation, finalBody, start, l)
}

// computeRequestMutation 计算请求阶段的所有变更
func (h *Handler) computeRequestMutation(ev *fetch.RequestPausedReply, matchedRules []*rules.MatchedRule) (*executor.RequestMutation, *rules.MatchedRule, []domain.RuleMatch) {
	var aggregated *executor.RequestMutation
	ruleMatches := buildRuleMatches(matchedRules)

	for _, matched := range matchedRules {
		if len(matched.Rule.Actions) == 0 {
			continue
		}

		mut := h.executor.ExecuteRequestActions(matched.Rule.Actions, ev)
		if mut == nil {
			continue
		}

		// 处理阻止行为
		if mut.Block != nil {
			return mut, matched, ruleMatches
		}

		// 聚合
		if aggregated == nil {
			aggregated = mut
		} else {
			mergeRequestMutation(aggregated, mut)
		}
	}
	return aggregated, nil, ruleMatches
}

// computeResponseMutation 计算响应阶段的所有变更
func (h *Handler) computeResponseMutation(ev *fetch.RequestPausedReply, matchedRules []*rules.MatchedRule, originalBody string) (*executor.ResponseMutation, []domain.RuleMatch, string) {
	var aggregated *executor.ResponseMutation
	currentBody := originalBody
	ruleMatches := buildRuleMatches(matchedRules)

	for _, matched := range matchedRules {
		if len(matched.Rule.Actions) == 0 {
			continue
		}

		mut := h.executor.ExecuteResponseActions(matched.Rule.Actions, ev, currentBody)
		if mut == nil {
			continue
		}

		if aggregated == nil {
			aggregated = mut
		} else {
			mergeResponseMutation(aggregated, mut)
		}

		if mut.Body != nil {
			currentBody = *mut.Body
		}
	}
	return aggregated, ruleMatches, currentBody
}

// emitRequestEvent 组装并发送请求事件
func (h *Handler) emitRequestEvent(
	targetID domain.TargetID,
	result string,
	matches []domain.RuleMatch,
	original domain.RequestInfo,
	mut *executor.RequestMutation,
	start time.Time,
	l logger.Logger,
) {
	modifiedInfo := original
	if result == "modified" && mut != nil {
		modifiedInfo = h.captureModifiedRequestData(original, mut)
	}

	h.sendMatchedEvent(targetID, result, matches, modifiedInfo, domain.ResponseInfo{})
	l.Debug("请求处理完成", "result", result, "duration", time.Since(start))
}

// emitResponseEvent 组装并发送响应事件
func (h *Handler) emitResponseEvent(
	targetID domain.TargetID,
	result string,
	matches []domain.RuleMatch,
	originalReq domain.RequestInfo,
	originalRes domain.ResponseInfo,
	mut *executor.ResponseMutation,
	finalBody string,
	start time.Time,
	l logger.Logger,
) {
	modifiedResInfo := originalRes
	// 只要有变更或匹配，且有 mutation 对象，就尝试渲染修改后的数据
	if (result == "modified" || result == "matched") && mut != nil {
		modifiedResInfo = h.captureModifiedResponseData(originalRes, mut, finalBody)
	}

	h.sendMatchedEvent(targetID, result, matches, originalReq, modifiedResInfo)
	l.Debug("全周期处理完成", "result", result, "duration", time.Since(start))
}

// buildEvalContext 构造规则匹配上下文
func (h *Handler) buildEvalContext(ev *fetch.RequestPausedReply) *rules.EvalContext {
	headers := map[string]string{}
	query := map[string]string{}
	cookies := map[string]string{}
	var bodyText string
	var resourceType string

	if ev.ResourceType != "" {
		resourceType = string(ev.ResourceType)
	}

	_ = json.Unmarshal(ev.Request.Headers, &headers)
	if len(headers) > 0 {
		normalized := make(map[string]string, len(headers))
		for k, v := range headers {
			normalized[strings.ToLower(k)] = v
		}
		headers = normalized
	}

	if ev.Request.URL != "" {
		if u, err := url.Parse(ev.Request.URL); err == nil {
			for key, vals := range u.Query() {
				if len(vals) > 0 {
					query[strings.ToLower(key)] = vals[0]
				}
			}
		}
	}

	if v, ok := headers["cookie"]; ok {
		for name, val := range protocol.ParseCookie(v) {
			cookies[strings.ToLower(name)] = val
		}
	}

	bodyText = protocol.GetRequestBody(ev)

	return &rules.EvalContext{
		URL:          ev.Request.URL,
		Method:       ev.Request.Method,
		ResourceType: resourceType,
		Headers:      headers,
		Query:        query,
		Cookies:      cookies,
		Body:         bodyText,
	}
}

// sendMatchedEvent 统一发送网络事件
func (h *Handler) sendMatchedEvent(
	targetID domain.TargetID,
	finalResult string,
	matchedRules []domain.RuleMatch,
	requestInfo domain.RequestInfo,
	responseInfo domain.ResponseInfo,
) {
	if h.events == nil {
		return
	}

	// 核心逻辑：是否有任何规则匹配
	isMatched := len(matchedRules) > 0

	evt := domain.NetworkEvent{
		Session:      "", // 会在上层填充
		Target:       targetID,
		Timestamp:    time.Now().UnixMilli(),
		IsMatched:    isMatched,
		Request:      requestInfo,
		Response:     responseInfo,
		FinalResult:  finalResult,
		MatchedRules: matchedRules,
	}

	select {
	case h.events <- evt:
	default:
	}
}

// captureRequestData 捕获原始请求数据
func (h *Handler) captureRequestData(ev *fetch.RequestPausedReply) domain.RequestInfo {
	requestInfo := domain.RequestInfo{
		URL:          ev.Request.URL,
		Method:       ev.Request.Method,
		Headers:      make(map[string]string),
		ResourceType: string(ev.ResourceType),
	}
	_ = json.Unmarshal(ev.Request.Headers, &requestInfo.Headers)
	requestInfo.Body = protocol.GetRequestBody(ev)
	return requestInfo
}

// captureResponseData 捕获原始请求/响应数据
func (h *Handler) captureResponseData(
	client *cdp.Client,
	ctx context.Context,
	ev *fetch.RequestPausedReply,
) (domain.RequestInfo, domain.ResponseInfo) {
	requestInfo := h.captureRequestData(ev)

	responseInfo := domain.ResponseInfo{
		Headers: make(map[string]string),
	}

	if ev.ResponseStatusCode != nil {
		responseInfo.StatusCode = *ev.ResponseStatusCode
	}
	for _, h := range ev.ResponseHeaders {
		responseInfo.Headers[h.Name] = h.Value
	}
	// 响应体需要单独获取
	body, _ := h.executor.FetchResponseBody(ctx, client, ev.RequestID)
	responseInfo.Body = body

	return requestInfo, responseInfo
}

// captureModifiedRequestData 捕获修改后的请求数据
func (h *Handler) captureModifiedRequestData(original domain.RequestInfo, mut *executor.RequestMutation) domain.RequestInfo {
	modified := domain.RequestInfo{
		URL:          original.URL,
		Method:       original.Method,
		ResourceType: original.ResourceType,
		Headers:      make(map[string]string),
		Body:         original.Body,
	}

	for k, v := range original.Headers {
		modified.Headers[k] = v
	}

	if mut.URL != nil {
		modified.URL = *mut.URL
	}

	for _, h := range mut.RemoveHeaders {
		delete(modified.Headers, h)
	}
	for k, v := range mut.Headers {
		modified.Headers[k] = v
	}

	if mut.Body != nil {
		modified.Body = *mut.Body
	}

	return modified
}

// captureModifiedResponseData 捕获修改后的响应数据
func (h *Handler) captureModifiedResponseData(original domain.ResponseInfo, mut *executor.ResponseMutation, finalBody string) domain.ResponseInfo {
	modified := domain.ResponseInfo{
		StatusCode: original.StatusCode,
		Headers:    make(map[string]string),
		Body:       finalBody,
	}

	for k, v := range original.Headers {
		modified.Headers[k] = v
	}

	if mut.StatusCode != nil {
		modified.StatusCode = *mut.StatusCode
	}

	for _, h := range mut.RemoveHeaders {
		delete(modified.Headers, h)
	}
	for k, v := range mut.Headers {
		modified.Headers[k] = v
	}

	return modified
}

// buildRuleMatches 构建规则匹配信息列表
func buildRuleMatches(matchedRules []*rules.MatchedRule) []domain.RuleMatch {
	matches := make([]domain.RuleMatch, len(matchedRules))
	for i, mr := range matchedRules {
		actionTypes := make([]string, 0, len(mr.Rule.Actions))
		for _, action := range mr.Rule.Actions {
			actionTypes = append(actionTypes, string(action.Type))
		}
		matches[i] = domain.RuleMatch{
			RuleID:   mr.Rule.ID,
			RuleName: mr.Rule.Name,
			Actions:  actionTypes,
		}
	}
	return matches
}

// mergeRequestMutation 合并请求变更
func mergeRequestMutation(dst, src *executor.RequestMutation) {
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
func mergeResponseMutation(dst, src *executor.ResponseMutation) {
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
func hasRequestMutation(m *executor.RequestMutation) bool {
	return m.URL != nil || m.Method != nil ||
		len(m.Headers) > 0 || len(m.Query) > 0 || len(m.Cookies) > 0 ||
		len(m.RemoveHeaders) > 0 || len(m.RemoveQuery) > 0 || len(m.RemoveCookies) > 0 ||
		m.Body != nil
}

// hasResponseMutation 检查响应变更是否有效
func hasResponseMutation(m *executor.ResponseMutation) bool {
	return m.StatusCode != nil || len(m.Headers) > 0 || len(m.RemoveHeaders) > 0 || m.Body != nil
}
