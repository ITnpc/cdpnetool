package cdp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	logger "cdpnetool/internal/logger"
	"cdpnetool/internal/rules"
	"cdpnetool/pkg/model"
	"cdpnetool/pkg/rulespec"

	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/devtool"
	"github.com/mafredri/cdp/protocol/fetch"
	"github.com/mafredri/cdp/protocol/network"
	"github.com/mafredri/cdp/rpcc"
)

type workspaceMode int

const (
	workspaceModeAutoFollow workspaceMode = iota
	workspaceModeFixed
)

type Manager struct {
	devtoolsURL       string
	conn              *rpcc.Conn
	client            *cdp.Client
	ctx               context.Context
	cancel            context.CancelFunc
	events            chan model.Event
	pending           chan model.PendingItem
	engine            *rules.Engine
	approvalsMu       sync.Mutex
	approvals         map[string]chan rulespec.Rewrite
	pool              *workerPool
	bodySizeThreshold int64
	processTimeoutMS  int
	log               logger.Logger
	attachMu          sync.Mutex
	currentTarget     model.TargetID
	fixedTarget       model.TargetID
	workspaceStop     chan struct{}
	mode              workspaceMode
	watchersMu        sync.Mutex
	watchers          map[model.TargetID]*targetWatcher
}

type workerPool struct {
	sem         chan struct{}
	queue       chan func()
	queueCap    int
	log         logger.Logger
	totalSubmit int64
	totalDrop   int64
	mu          sync.Mutex
	stopMonitor chan struct{}
}

func newWorkerPool(size int) *workerPool {
	if size <= 0 {
		return &workerPool{}
	}
	return &workerPool{
		sem:      make(chan struct{}, size),
		queue:    make(chan func(), size*2),
		queueCap: size * 2,
	}
}

func (p *workerPool) setLogger(l logger.Logger) {
	p.log = l
}

func (p *workerPool) start(ctx context.Context) {
	if p.sem == nil {
		return
	}
	for i := 0; i < cap(p.sem); i++ {
		go p.worker(ctx)
	}
	p.stopMonitor = make(chan struct{})
	go p.monitor(ctx)
}

func (p *workerPool) stop() {
	if p.stopMonitor != nil {
		close(p.stopMonitor)
	}
}

func (p *workerPool) monitor(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopMonitor:
			return
		case <-ticker.C:
			qLen, qCap, submit, drop := p.stats()
			if p.log != nil && submit > 0 {
				usage := float64(qLen) / float64(qCap) * 100
				dropRate := float64(drop) / float64(submit) * 100
				p.log.Info("工作池状态监控", "queueLen", qLen, "queueCap", qCap, "usage", fmt.Sprintf("%.1f%%", usage), "totalSubmit", submit, "totalDrop", drop, "dropRate", fmt.Sprintf("%.2f%%", dropRate))
			}
		}
	}
}

func (p *workerPool) worker(ctx context.Context) {
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

func (p *workerPool) submit(fn func()) bool {
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

func (p *workerPool) stats() (queueLen, queueCap, totalSubmit, totalDrop int64) {
	if p.sem == nil {
		return 0, 0, 0, 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return int64(len(p.queue)), int64(p.queueCap), p.totalSubmit, p.totalDrop
}

type targetWatcher struct {
	id     model.TargetID
	conn   *rpcc.Conn
	client *cdp.Client
	cancel context.CancelFunc
}

// New 创建并返回一个管理器，用于管理CDP连接与拦截流程
func New(devtoolsURL string, events chan model.Event, pending chan model.PendingItem, l logger.Logger) *Manager {
	if l == nil {
		l = logger.NewNoopLogger()
	}
	return &Manager{
		devtoolsURL: devtoolsURL,
		events:      events,
		pending:     pending,
		approvals:   make(map[string]chan rulespec.Rewrite),
		log:         l,
		mode:        workspaceModeAutoFollow,
		watchers:    make(map[model.TargetID]*targetWatcher),
	}
}

// AttachTarget 附着到指定浏览器目标并建立CDP会话
func (m *Manager) AttachTarget(target model.TargetID) error {
	m.attachMu.Lock()
	defer m.attachMu.Unlock()
	m.log.Info("开始附加浏览器目标", "devtools", m.devtoolsURL, "target", string(target))
	if target != "" {
		m.fixedTarget = target
		m.mode = workspaceModeFixed
	} else {
		m.fixedTarget = ""
		m.mode = workspaceModeAutoFollow
	}
	if m.cancel != nil {
		m.cancel()
	}
	if m.conn != nil {
		_ = m.conn.Close()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	m.cancel = cancel
	sel, err := m.resolveTarget(ctx, target)
	if err != nil {
		return err
	}
	if sel == nil {
		m.log.Error("未找到可附加的浏览器目标")
		return fmt.Errorf("no target")
	}
	conn, err := rpcc.DialContext(ctx, sel.WebSocketDebuggerURL)
	if err != nil {
		m.log.Error("连接浏览器 DevTools 失败", "error", err)
		return err
	}
	m.conn = conn
	m.client = cdp.NewClient(conn)
	m.currentTarget = model.TargetID(sel.ID)
	m.log.Info("附加浏览器目标成功", "target", string(m.currentTarget))
	if target == "" {
		m.startWorkspaceWatcher()
	} else {
		m.stopWorkspaceWatcher()
	}
	return nil
}

// Detach 断开当前会话连接并释放资源
func (m *Manager) Detach() error {
	m.attachMu.Lock()
	defer m.attachMu.Unlock()
	if m.cancel != nil {
		m.cancel()
	}
	if m.pool != nil {
		m.pool.stop()
	}
	m.stopWorkspaceWatcher()
	if m.conn != nil {
		return m.conn.Close()
	}
	return nil
}

// Enable 启用Fetch/Network拦截功能并开始消费事件
func (m *Manager) Enable() error {
	if m.client == nil {
		return fmt.Errorf("not attached")
	}
	m.log.Info("开始启用拦截功能")
	err := m.client.Network.Enable(m.ctx, nil)
	if err != nil {
		return err
	}
	p := "*"
	patterns := []fetch.RequestPattern{
		{URLPattern: &p, RequestStage: fetch.RequestStageRequest},
		{URLPattern: &p, RequestStage: fetch.RequestStageResponse},
	}
	err = m.client.Fetch.Enable(m.ctx, &fetch.EnableArgs{Patterns: patterns})
	if err != nil {
		return err
	}
	// 如果已配置 worker pool 且未启动，现在启动
	if m.pool != nil && m.pool.sem != nil && m.ctx != nil {
		m.pool.start(m.ctx)
	}
	go m.consume()
	m.log.Info("拦截功能启用完成")
	return nil
}

// Disable 停止拦截功能但保留连接
func (m *Manager) Disable() error {
	if m.client == nil {
		return fmt.Errorf("not attached")
	}
	return m.client.Fetch.Disable(m.ctx)
}

// consume 持续接收拦截事件并按并发限制分发处理
func (m *Manager) consume() {
	rp, err := m.client.Fetch.RequestPaused(m.ctx)
	if err != nil {
		m.log.Error("订阅拦截事件流失败", "error", err)
		m.handleStreamError(err)
		return
	}
	defer rp.Close()
	m.log.Info("开始消费拦截事件流")
	for {
		ev, err := rp.Recv()
		if err != nil {
			m.log.Error("接收拦截事件失败", "error", err)
			m.handleStreamError(err)
			return
		}
		m.dispatchPaused(ev)
	}
}

// dispatchPaused 根据并发配置调度单次拦截事件处理
func (m *Manager) dispatchPaused(ev *fetch.RequestPausedReply) {
	if m.pool == nil {
		go m.handle(ev)
		return
	}
	submitted := m.pool.submit(func() {
		m.handle(ev)
	})
	if !submitted {
		m.degradeAndContinue(ev, "并发队列已满")
	}
}

func (m *Manager) handleStreamError(err error) {
	if m.ctx == nil {
		return
	}
	if m.ctx.Err() != nil {
		return
	}
	m.log.Warn("拦截流被中断，尝试自动重连", "error", err)
	var target model.TargetID
	if m.fixedTarget != "" {
		target = m.fixedTarget
	}
	auto := m.fixedTarget == ""
	if err := m.attachAndEnable(target, auto); err != nil {
		m.log.Error("重连附加浏览器目标失败", "error", err)
	}
}

func (m *Manager) startWorkspaceWatcher() {
	m.log.Debug("开始工作区轮询", "func", "startWorkspaceWatcher")
	if m.workspaceStop != nil {
		return
	}
	ch := make(chan struct{})
	m.workspaceStop = ch
	go m.workspaceLoop(ch)
}

func (m *Manager) stopWorkspaceWatcher() {
	if m.workspaceStop != nil {
		close(m.workspaceStop)
		m.workspaceStop = nil
	}
	m.stopAllWatchers()
}

func (m *Manager) workspaceLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			m.checkWorkspace()
		}
	}
}

func (m *Manager) checkWorkspace() {
	m.log.Debug("开始工作区轮询", "func", "checkWorkspace")
	if m.devtoolsURL == "" {
		return
	}
	if m.fixedTarget != "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	dt := devtool.New(m.devtoolsURL)
	targets, err := dt.List(ctx)
	if err != nil {
		m.log.Debug("工作区轮询获取目标列表失败", "error", err)
		return
	}
	m.refreshWatchers(ctx, targets)
	sel := selectAutoTarget(targets)
	if sel == nil {
		return
	}
	candidate := model.TargetID(sel.ID)
	if candidate == "" {
		return
	}
	if m.currentTarget != "" && string(m.currentTarget) == string(candidate) {
		return
	}
	if err := m.attachAndEnable(candidate, true); err != nil {
		m.log.Error("自动切换浏览器目标失败", "error", err)
	}
}

func (m *Manager) attachAndEnable(target model.TargetID, auto bool) error {
	var err error
	if auto {
		err = m.attachAuto(target)
	} else {
		err = m.AttachTarget(target)
	}
	if err != nil {
		return err
	}
	if err := m.Enable(); err != nil {
		return err
	}
	return nil
}

func (m *Manager) attachAuto(target model.TargetID) error {
	m.attachMu.Lock()
	defer m.attachMu.Unlock()
	m.log.Info("自动附加浏览器目标", "devtools", m.devtoolsURL, "target", string(target))
	if m.cancel != nil {
		m.cancel()
	}
	if m.conn != nil {
		_ = m.conn.Close()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	m.cancel = cancel
	sel, err := m.resolveTarget(ctx, target)
	if err != nil {
		return err
	}
	if sel == nil {
		m.log.Error("未找到可附加的浏览器目标")
		return fmt.Errorf("no target")
	}
	conn, err := rpcc.DialContext(ctx, sel.WebSocketDebuggerURL)
	if err != nil {
		m.log.Error("连接浏览器 DevTools 失败", "error", err)
		return err
	}
	m.conn = conn
	m.client = cdp.NewClient(conn)
	m.currentTarget = model.TargetID(sel.ID)
	m.log.Info("自动附加浏览器目标成功", "target", string(m.currentTarget))
	return nil
}

// handle 处理一次拦截事件并根据规则执行相应动作
func (m *Manager) handle(ev *fetch.RequestPausedReply) {
	to := m.processTimeoutMS
	if to <= 0 {
		to = 3000
	}
	ctx, cancel := context.WithTimeout(m.ctx, time.Duration(to)*time.Millisecond)
	defer cancel()
	start := time.Now()
	m.events <- model.Event{Type: "intercepted"}
	stg := "request"
	if ev.ResponseStatusCode != nil {
		stg = "response"
	}
	m.log.Debug("开始处理拦截事件", "stage", stg, "url", ev.Request.URL, "method", ev.Request.Method)
	res := m.decide(ev, stg)
	if res == nil || res.Action == nil {
		m.applyContinue(ctx, ev, stg)
		return
	}
	a := res.Action
	if a.DropRate > 0 {
		if rand.Float64() < a.DropRate {
			m.applyContinue(ctx, ev, stg)
			m.events <- model.Event{Type: "degraded"}
			m.log.Warn("触发丢弃概率降级", "stage", stg)
			return
		}
	}
	if a.DelayMS > 0 {
		time.Sleep(time.Duration(a.DelayMS) * time.Millisecond)
	}
	elapsed := time.Since(start)
	if elapsed > time.Duration(to)*time.Millisecond {
		m.applyContinue(ctx, ev, stg)
		m.events <- model.Event{Type: "degraded"}
		m.log.Warn("拦截处理超时自动降级", "stage", stg, "elapsed", elapsed, "timeout", to)
		return
	}
	if a.Pause != nil {
		m.log.Info("应用暂停审批动作", "stage", stg)
		m.applyPause(ctx, ev, a.Pause, stg, res.RuleID)
		return
	}
	if a.Fail != nil {
		if m.log != nil {
			m.log.Info("apply_fail", "stage", stg)
		}
		m.applyFail(ctx, ev, a.Fail)
		m.events <- model.Event{Type: "failed", Rule: res.RuleID}
		return
	}
	if a.Respond != nil {
		m.log.Info("应用自定义响应动作", "stage", stg)
		m.applyRespond(ctx, ev, a.Respond, stg)
		m.events <- model.Event{Type: "fulfilled", Rule: res.RuleID}
		return
	}
	if a.Rewrite != nil {
		m.log.Info("应用请求响应重写动作", "stage", stg)
		m.applyRewrite(ctx, ev, a.Rewrite, stg)
		m.events <- model.Event{Type: "mutated", Rule: res.RuleID}
		m.log.Debug("拦截事件处理完成", "stage", stg, "duration", time.Since(start))
		return
	}
	m.applyContinue(ctx, ev, stg)
	m.log.Debug("拦截事件处理完成", "stage", stg, "duration", time.Since(start))
}

// decide 构造规则上下文并进行匹配决策
func (m *Manager) decide(ev *fetch.RequestPausedReply, stage string) *rules.Result {
	if m.engine == nil {
		return nil
	}
	ctx := m.buildRuleContext(ev, stage)
	res := m.engine.Eval(ctx)
	if res == nil {
		return nil
	}
	return res
}

// buildRuleContext 从 CDP 拦截事件构造规则引擎上下文
func (m *Manager) buildRuleContext(ev *fetch.RequestPausedReply, stage string) rules.Ctx {
	h := map[string]string{}
	q := map[string]string{}
	ck := map[string]string{}
	var bodyText string
	var ctype string

	if stage == "response" {
		if len(ev.ResponseHeaders) > 0 {
			for i := range ev.ResponseHeaders {
				k := ev.ResponseHeaders[i].Name
				v := ev.ResponseHeaders[i].Value
				h[strings.ToLower(k)] = v
				if strings.EqualFold(k, "set-cookie") {
					name, val := parseSetCookie(v)
					if name != "" {
						ck[strings.ToLower(name)] = val
					}
				}
				if strings.EqualFold(k, "content-type") {
					ctype = v
				}
			}
		}
		var clen int64
		if v, ok := h["content-length"]; ok {
			if n, err := parseInt64(v); err == nil {
				clen = n
			}
		}
		if shouldGetBody(ctype, clen, m.bodySizeThreshold) {
			ctx2, cancel := context.WithTimeout(m.ctx, 500*time.Millisecond)
			defer cancel()
			rb, err := m.client.Fetch.GetResponseBody(ctx2, &fetch.GetResponseBodyArgs{RequestID: ev.RequestID})
			if err == nil && rb != nil {
				if rb.Base64Encoded {
					if b, err := base64.StdEncoding.DecodeString(rb.Body); err == nil {
						bodyText = string(b)
					}
				} else {
					bodyText = rb.Body
				}
			}
		}
	} else {
		_ = json.Unmarshal(ev.Request.Headers, &h)
		if len(h) > 0 {
			m2 := make(map[string]string, len(h))
			for k, v := range h {
				m2[strings.ToLower(k)] = v
			}
			h = m2
		}
		if ev.Request.URL != "" {
			if u, err := url.Parse(ev.Request.URL); err == nil {
				for key, vals := range u.Query() {
					if len(vals) > 0 {
						q[strings.ToLower(key)] = vals[0]
					}
				}
			}
		}
		if v, ok := h["cookie"]; ok {
			for name, val := range parseCookie(v) {
				ck[strings.ToLower(name)] = val
			}
		}
		if v, ok := h["content-type"]; ok {
			ctype = v
		}
		if ev.Request.PostData != nil {
			bodyText = *ev.Request.PostData
		}
	}

	return rules.Ctx{URL: ev.Request.URL, Method: ev.Request.Method, Headers: h, Query: q, Cookies: ck, Body: bodyText, ContentType: ctype, Stage: stage}
}

// parseCookie 解析Cookie头为键值对映射
func parseCookie(s string) map[string]string {
	out := make(map[string]string)
	parts := strings.Split(s, ";")
	for _, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) == 2 {
			out[kv[0]] = kv[1]
		}
	}
	return out
}

// parseSetCookie 解析Set-Cookie的首个键值
func parseSetCookie(s string) (string, string) {
	// CookieName=CookieValue; Attr=...
	p := strings.SplitN(s, ";", 2)
	first := strings.TrimSpace(p[0])
	kv := strings.SplitN(first, "=", 2)
	if len(kv) == 2 {
		return kv[0], kv[1]
	}
	return "", ""
}

// urlParse 解析并按补丁修改查询参数后返回URL
func urlParse(raw string, qpatch map[string]*string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for k, v := range qpatch {
		if v == nil {
			q.Del(k)
		} else {
			q.Set(k, *v)
		}
	}
	u.RawQuery = q.Encode()
	return u, nil
}

// shouldGetBody 判断是否需要获取响应体以用于匹配或重写
func shouldGetBody(ctype string, clen int64, thr int64) bool {
	if thr <= 0 {
		thr = 4 * 1024 * 1024
	}
	if clen > 0 && clen > thr {
		return false
	}
	lc := strings.ToLower(ctype)
	if strings.HasPrefix(lc, "text/") {
		return true
	}
	if strings.HasPrefix(lc, "application/json") {
		return true
	}
	return false
}

// parseInt64 将数字字符串解析为int64
func parseInt64(s string) (int64, error) {
	var n int64
	var mul int64 = 1
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid")
		}
		n = n*10 + int64(c-'0')
	}
	return n * mul, nil
}

// applyContinue 继续原请求或响应不做修改
func (m *Manager) applyContinue(ctx context.Context, ev *fetch.RequestPausedReply, stage string) {
	if stage == "response" {
		m.client.Fetch.ContinueResponse(ctx, &fetch.ContinueResponseArgs{RequestID: ev.RequestID})
		m.log.Debug("继续原始响应")
	} else {
		m.client.Fetch.ContinueRequest(ctx, &fetch.ContinueRequestArgs{RequestID: ev.RequestID})
		m.log.Debug("继续原始请求")
	}
}

// applyFail 使请求失败并返回错误原因
func (m *Manager) applyFail(ctx context.Context, ev *fetch.RequestPausedReply, f *rulespec.Fail) {
	m.client.Fetch.FailRequest(ctx, &fetch.FailRequestArgs{RequestID: ev.RequestID, ErrorReason: network.ErrorReasonFailed})
}

// applyRespond 返回自定义响应（可只改头或完整替换）
func (m *Manager) applyRespond(ctx context.Context, ev *fetch.RequestPausedReply, r *rulespec.Respond, stage string) {
	if stage == "response" && len(r.Body) == 0 {
		// 仅修改响应码/头，继续响应
		m.continueResponseWithModifications(ctx, ev, r)
		return
	}
	// fulfill 完整响应
	m.fulfillRequest(ctx, ev, r)
}

// continueResponseWithModifications 继续响应并修改状态码/头部
func (m *Manager) continueResponseWithModifications(ctx context.Context, ev *fetch.RequestPausedReply, r *rulespec.Respond) {
	args := &fetch.ContinueResponseArgs{RequestID: ev.RequestID}
	if r.Status != 0 {
		args.ResponseCode = &r.Status
	}
	if len(r.Headers) > 0 {
		args.ResponseHeaders = toHeaderEntries(r.Headers)
	}
	m.client.Fetch.ContinueResponse(ctx, args)
}

// fulfillRequest 完整响应请求
func (m *Manager) fulfillRequest(ctx context.Context, ev *fetch.RequestPausedReply, r *rulespec.Respond) {
	args := &fetch.FulfillRequestArgs{RequestID: ev.RequestID, ResponseCode: r.Status}
	if len(r.Headers) > 0 {
		args.ResponseHeaders = toHeaderEntries(r.Headers)
	}
	if len(r.Body) > 0 {
		args.Body = r.Body
	}
	m.client.Fetch.FulfillRequest(ctx, args)
}

// applyRewrite 根据规则对请求或响应进行重写
func (m *Manager) applyRewrite(ctx context.Context, ev *fetch.RequestPausedReply, rw *rulespec.Rewrite, stage string) {
	if stage == "response" {
		m.applyResponseRewrite(ctx, ev, rw)
	} else {
		m.applyRequestRewrite(ctx, ev, rw)
	}
}

// applyResponseRewrite 处理响应阶段的重写
func (m *Manager) applyResponseRewrite(ctx context.Context, ev *fetch.RequestPausedReply, rw *rulespec.Rewrite) {
	if rw.Body == nil {
		// 仅修改头部，不需要获取 Body
		if rw.Headers != nil {
			cur := m.getCurrentResponseHeaders(ev)
			cur = applyHeaderPatch(cur, rw.Headers)
			m.client.Fetch.ContinueResponse(ctx, &fetch.ContinueResponseArgs{RequestID: ev.RequestID, ResponseHeaders: toHeaderEntries(cur)})
			return
		}
		m.client.Fetch.ContinueResponse(ctx, &fetch.ContinueResponseArgs{RequestID: ev.RequestID})
		return
	}

	// 需要修改 Body
	ctype, clen := m.extractResponseMetadata(ev)
	if !shouldGetBody(ctype, clen, m.bodySizeThreshold) {
		m.client.Fetch.ContinueResponse(ctx, &fetch.ContinueResponseArgs{RequestID: ev.RequestID})
		return
	}

	bodyText, ok := m.fetchResponseBody(ctx, ev.RequestID)
	if !ok {
		m.client.Fetch.ContinueResponse(ctx, &fetch.ContinueResponseArgs{RequestID: ev.RequestID})
		return
	}

	newBody, ok := applyBodyPatch(bodyText, rw.Body)
	if !ok || len(newBody) == 0 {
		m.client.Fetch.ContinueResponse(ctx, &fetch.ContinueResponseArgs{RequestID: ev.RequestID})
		return
	}

	code := 200
	if ev.ResponseStatusCode != nil {
		code = *ev.ResponseStatusCode
	}
	cur := m.getCurrentResponseHeaders(ev)
	cur = applyHeaderPatch(cur, rw.Headers)
	args := &fetch.FulfillRequestArgs{
		RequestID:       ev.RequestID,
		ResponseCode:    code,
		ResponseHeaders: toHeaderEntries(cur),
		Body:            newBody,
	}
	m.client.Fetch.FulfillRequest(ctx, args)
}

// getCurrentResponseHeaders 获取当前响应头部映射
func (m *Manager) getCurrentResponseHeaders(ev *fetch.RequestPausedReply) map[string]string {
	cur := make(map[string]string, len(ev.ResponseHeaders))
	for i := range ev.ResponseHeaders {
		cur[strings.ToLower(ev.ResponseHeaders[i].Name)] = ev.ResponseHeaders[i].Value
	}
	return cur
}

// extractResponseMetadata 提取响应元数据（Content-Type, Content-Length）
func (m *Manager) extractResponseMetadata(ev *fetch.RequestPausedReply) (ctype string, clen int64) {
	for i := range ev.ResponseHeaders {
		k := ev.ResponseHeaders[i].Name
		v := ev.ResponseHeaders[i].Value
		if strings.EqualFold(k, "content-type") {
			ctype = v
		}
		if strings.EqualFold(k, "content-length") {
			if n, err := parseInt64(v); err == nil {
				clen = n
			}
		}
	}
	return
}

// fetchResponseBody 获取响应 Body 文本
func (m *Manager) fetchResponseBody(ctx context.Context, requestID fetch.RequestID) (string, bool) {
	ctx2, cancel := context.WithTimeout(m.ctx, 500*time.Millisecond)
	defer cancel()
	rb, err := m.client.Fetch.GetResponseBody(ctx2, &fetch.GetResponseBodyArgs{RequestID: requestID})
	if err != nil || rb == nil {
		return "", false
	}
	if rb.Base64Encoded {
		if b, err := base64.StdEncoding.DecodeString(rb.Body); err == nil {
			return string(b), true
		}
		return "", false
	}
	return rb.Body, true
}

// applyRequestRewrite 处理请求阶段的重写
func (m *Manager) applyRequestRewrite(ctx context.Context, ev *fetch.RequestPausedReply, rw *rulespec.Rewrite) {
	var url, method *string
	if rw.URL != nil {
		url = rw.URL
	}
	if rw.Method != nil {
		method = rw.Method
	}

	hdrs := m.buildRequestHeaders(rw, ev)
	post := m.buildRequestBody(rw, ev)

	args := &fetch.ContinueRequestArgs{
		RequestID: ev.RequestID,
		URL:       url,
		Method:    method,
		Headers:   hdrs,
	}

	if rw.Query != nil && url == nil {
		if u, err := urlParse(ev.Request.URL, rw.Query); err == nil {
			us := u.String()
			args.URL = &us
		}
	}

	if len(post) > 0 {
		args.PostData = post
	}

	m.client.Fetch.ContinueRequest(ctx, args)
}

// buildRequestHeaders 构建请求头部列表
func (m *Manager) buildRequestHeaders(rw *rulespec.Rewrite, ev *fetch.RequestPausedReply) []fetch.HeaderEntry {
	var hdrs []fetch.HeaderEntry
	if rw.Headers != nil {
		for k, v := range rw.Headers {
			if v != nil {
				hdrs = append(hdrs, fetch.HeaderEntry{Name: k, Value: *v})
			}
		}
	}

	if rw.Cookies != nil {
		h := map[string]string{}
		_ = json.Unmarshal(ev.Request.Headers, &h)
		var cookie string
		for k, v := range h {
			if strings.EqualFold(k, "cookie") {
				cookie = v
				break
			}
		}
		cm := parseCookie(cookie)
		for name, val := range rw.Cookies {
			if val == nil {
				delete(cm, name)
			} else {
				cm[name] = *val
			}
		}
		if len(cm) > 0 {
			var b strings.Builder
			first := true
			for k, v := range cm {
				if !first {
					b.WriteString("; ")
				}
				first = false
				b.WriteString(k)
				b.WriteString("=")
				b.WriteString(v)
			}
			hdrs = append(hdrs, fetch.HeaderEntry{Name: "Cookie", Value: b.String()})
		}
	}

	return hdrs
}

// buildRequestBody 构建请求 Body
func (m *Manager) buildRequestBody(rw *rulespec.Rewrite, ev *fetch.RequestPausedReply) []byte {
	if rw.Body == nil {
		return nil
	}
	var src string
	if ev.Request.PostData != nil {
		src = *ev.Request.PostData
	}
	if b, ok := applyBodyPatch(src, rw.Body); ok && len(b) > 0 {
		return b
	}
	return nil
}

// applyBodyPatch 根据 BodyPatch 对文本或二进制内容进行修改
func applyBodyPatch(src string, bp *rulespec.BodyPatch) ([]byte, bool) {
	if bp == nil {
		return nil, false
	}
	// Base64 覆盖：直接以配置的 Base64 内容替换原文
	if bp.Base64 != nil {
		b, err := base64.StdEncoding.DecodeString(bp.Base64.Value)
		if err != nil {
			return nil, false
		}
		return b, true
	}
	// 文本正则替换：基于原始字符串进行正则替换
	if bp.TextRegex != nil {
		re, err := regexp.Compile(bp.TextRegex.Pattern)
		if err != nil {
			return nil, false
		}
		return []byte(re.ReplaceAllString(src, bp.TextRegex.Replace)), true
	}
	// JSON Patch：按 RFC6902 对 JSON 文本进行补丁
	if len(bp.JSONPatch) > 0 {
		out, ok := applyJSONPatch(src, bp.JSONPatch)
		if !ok {
			return nil, false
		}
		return []byte(out), true
	}
	return nil, false
}

// applyJSONPatch 对JSON文档应用Patch操作并返回结果
func applyJSONPatch(doc string, ops []rulespec.JSONPatchOp) (string, bool) {
	var v any
	if doc == "" {
		v = make(map[string]any)
	} else {
		if err := json.Unmarshal([]byte(doc), &v); err != nil {
			return "", false
		}
	}
	for _, op := range ops {
		typ := string(op.Op)
		path := op.Path
		val := op.Value
		from := op.From
		switch typ {
		case string(rulespec.JSONPatchOpAdd), string(rulespec.JSONPatchOpReplace):
			v = setByPtr(v, path, val, typ == string(rulespec.JSONPatchOpReplace))
		case string(rulespec.JSONPatchOpRemove):
			v = removeByPtr(v, path)
		case string(rulespec.JSONPatchOpCopy):
			src, ok := getByPtr(v, from)
			if !ok {
				return "", false
			}
			v = setByPtr(v, path, src, true)
		case string(rulespec.JSONPatchOpMove):
			src, ok := getByPtr(v, from)
			if !ok {
				return "", false
			}
			v = removeByPtr(v, from)
			v = setByPtr(v, path, src, true)
		case string(rulespec.JSONPatchOpTest):
			cur, ok := getByPtr(v, path)
			if !ok {
				return "", false
			}
			if !deepEqual(cur, val) {
				return "", false
			}
		}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", false
	}
	return string(b), true
}

// setByPtr 依据JSON Pointer设置节点值
func setByPtr(cur any, ptr string, val any, replace bool) any {
	if ptr == "" || ptr[0] != '/' {
		return cur
	}
	tokens := splitPtr(ptr)
	return setRec(cur, tokens, val)
}

// setRec 递归设置节点值的内部实现
func setRec(cur any, tokens []string, val any) any {
	if len(tokens) == 0 {
		return val
	}
	t := tokens[0]
	switch c := cur.(type) {
	case map[string]any:
		child, ok := c[t]
		if !ok {
			child = make(map[string]any)
		}
		c[t] = setRec(child, tokens[1:], val)
		return c
	case []any:
		idx, ok := toIndex(t)
		if !ok || idx < 0 || idx >= len(c) {
			return c
		}
		c[idx] = setRec(c[idx], tokens[1:], val)
		return c
	default:
		if len(tokens) == 1 {
			return val
		}
		return cur
	}
}

// removeByPtr 依据JSON Pointer移除节点
func removeByPtr(cur any, ptr string) any {
	if ptr == "" || ptr[0] != '/' {
		return cur
	}
	tokens := splitPtr(ptr)
	return removeRec(cur, tokens)
}

// getByPtr 依据JSON Pointer读取节点值
func getByPtr(cur any, ptr string) (any, bool) {
	if ptr == "" || ptr[0] != '/' {
		return nil, false
	}
	tokens := splitPtr(ptr)
	x := cur
	for _, t := range tokens {
		switch c := x.(type) {
		case map[string]any:
			v, ok := c[t]
			if !ok {
				return nil, false
			}
			x = v
		case []any:
			idx, ok := toIndex(t)
			if !ok || idx < 0 || idx >= len(c) {
				return nil, false
			}
			x = c[idx]
		default:
			return nil, false
		}
	}
	return x, true
}

// deepEqual 深度比较两个值是否相等
func deepEqual(a, b any) bool { return reflect.DeepEqual(a, b) }

// removeRec 递归移除节点的内部实现
func removeRec(cur any, tokens []string) any {
	if len(tokens) == 0 {
		return cur
	}
	t := tokens[0]
	switch c := cur.(type) {
	case map[string]any:
		if len(tokens) == 1 {
			delete(c, t)
			return c
		}
		child, ok := c[t]
		if !ok {
			return c
		}
		c[t] = removeRec(child, tokens[1:])
		return c
	case []any:
		idx, ok := toIndex(t)
		if !ok || idx < 0 || idx >= len(c) {
			return c
		}
		if len(tokens) == 1 {
			nc := append(c[:idx], c[idx+1:]...)
			return nc
		}
		c[idx] = removeRec(c[idx], tokens[1:])
		return c
	default:
		return cur
	}
}

// splitPtr 将JSON Pointer切分为令牌序列
func splitPtr(p string) []string {
	var out []string
	i := 1
	for i < len(p) {
		j := i
		for j < len(p) && p[j] != '/' {
			j++
		}
		tok := p[i:j]
		tok = strings.ReplaceAll(tok, "~1", "/")
		tok = strings.ReplaceAll(tok, "~0", "~")
		out = append(out, tok)
		i = j + 1
	}
	return out
}

// toIndex 将字符串转换为数组索引
func toIndex(s string) (int, bool) {
	n := 0
	if len(s) == 0 {
		return 0, false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

// applyHeaderPatch 根据补丁对当前头部映射进行增删改
func applyHeaderPatch(cur map[string]string, patch map[string]*string) map[string]string {
	if patch == nil {
		return cur
	}
	for k, v := range patch {
		lk := strings.ToLower(k)
		if v == nil {
			delete(cur, lk)
		} else {
			cur[lk] = *v
		}
	}
	return cur
}

// toHeaderEntries 将头部映射转换为CDP头部条目
func toHeaderEntries(h map[string]string) []fetch.HeaderEntry {
	out := make([]fetch.HeaderEntry, 0, len(h))
	for k, v := range h {
		out = append(out, fetch.HeaderEntry{Name: k, Value: v})
	}
	return out
}

func isUserPageURL(raw string) bool {
	if raw == "" {
		return false
	}
	url := strings.ToLower(raw)
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return true
	}
	return false
}

func (m *Manager) resolveTarget(ctx context.Context, target model.TargetID) (*devtool.Target, error) {
	dt := devtool.New(m.devtoolsURL)
	targets, err := dt.List(ctx)
	if err != nil {
		m.log.Error("获取浏览器目标列表失败", "error", err)
		return nil, err
	}
	if len(targets) == 0 {
		return nil, nil
	}
	if target != "" {
		for i := range targets {
			if string(targets[i].ID) == string(target) {
				return targets[i], nil
			}
		}
		return nil, nil
	}
	return selectAutoTarget(targets), nil
}

func selectAutoTarget(targets []*devtool.Target) *devtool.Target {
	var sel *devtool.Target
	for i := len(targets) - 1; i >= 0; i-- {
		if targets[i].Type != "page" {
			continue
		}
		if !isUserPageURL(targets[i].URL) {
			continue
		}
		sel = targets[i]
		break
	}
	if sel == nil && len(targets) > 0 {
		return targets[0]
	}
	return sel
}

func (m *Manager) refreshWatchers(ctx context.Context, targets []*devtool.Target) {
	ids := make(map[model.TargetID]*devtool.Target)
	for i := range targets {
		if targets[i] == nil {
			continue
		}
		if targets[i].Type != "page" {
			continue
		}
		if !isUserPageURL(targets[i].URL) {
			continue
		}
		id := model.TargetID(targets[i].ID)
		if id == "" {
			continue
		}
		ids[id] = targets[i]
	}
	m.watchersMu.Lock()
	for id, w := range m.watchers {
		if _, ok := ids[id]; !ok {
			w.cancel()
			if w.conn != nil {
				_ = w.conn.Close()
			}
			delete(m.watchers, id)
		}
	}
	for id, t := range ids {
		if _, ok := m.watchers[id]; ok {
			continue
		}
		w, err := m.startWatcher(ctx, id, t.WebSocketDebuggerURL)
		if err != nil {
			m.log.Debug("创建目标可见性监听器失败", "target", string(id), "error", err)
			continue
		}
		m.watchers[id] = w
	}
	m.watchersMu.Unlock()
}

func (m *Manager) startWatcher(ctx context.Context, id model.TargetID, wsURL string) (*targetWatcher, error) {
	wctx, cancel := context.WithCancel(context.Background())
	conn, err := rpcc.DialContext(wctx, wsURL)
	if err != nil {
		cancel()
		return nil, err
	}
	client := cdp.NewClient(conn)
	if err := client.Page.Enable(wctx); err != nil {
		cancel()
		_ = conn.Close()
		return nil, err
	}
	stream, err := client.Page.LifecycleEvent(wctx)
	if err != nil {
		cancel()
		_ = conn.Close()
		return nil, err
	}
	w := &targetWatcher{id: id, conn: conn, client: client, cancel: cancel}
	go func() {
		defer stream.Close()
		for {
			ev, err := stream.Recv()
			if err != nil {
				break
			}
			if ev == nil {
				continue
			}
			name := ev.Name
			if name == "visible" {
				m.onTargetVisible(id)
			}
		}
		m.removeWatcher(id)
	}()
	return w, nil
}

func (m *Manager) onTargetVisible(id model.TargetID) {
	if id == "" {
		return
	}
	if m.mode != workspaceModeAutoFollow {
		return
	}
	if m.currentTarget != "" && m.currentTarget == id {
		return
	}
	if err := m.attachAndEnable(id, true); err != nil {
		m.log.Error("根据可见性切换浏览器目标失败", "target", string(id), "error", err)
	}
}

func (m *Manager) removeWatcher(id model.TargetID) {
	m.watchersMu.Lock()
	defer m.watchersMu.Unlock()
	if w, ok := m.watchers[id]; ok {
		w.cancel()
		if w.conn != nil {
			_ = w.conn.Close()
		}
		delete(m.watchers, id)
	}
}

func (m *Manager) stopAllWatchers() {
	m.watchersMu.Lock()
	defer m.watchersMu.Unlock()
	for id, w := range m.watchers {
		w.cancel()
		if w.conn != nil {
			_ = w.conn.Close()
		}
		delete(m.watchers, id)
	}
}

func (m *Manager) ListTargets(ctx context.Context) ([]model.TargetInfo, error) {
	if m.devtoolsURL == "" {
		return nil, fmt.Errorf("devtools url empty")
	}
	dt := devtool.New(m.devtoolsURL)
	targets, err := dt.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]model.TargetInfo, 0, len(targets))
	for i := range targets {
		if targets[i] == nil {
			continue
		}
		id := model.TargetID(targets[i].ID)
		info := model.TargetInfo{
			ID:        id,
			Type:      string(targets[i].Type),
			URL:       targets[i].URL,
			Title:     targets[i].Title,
			IsCurrent: m.currentTarget != "" && id == m.currentTarget,
			IsUser:    isUserPageURL(targets[i].URL),
		}
		out = append(out, info)
	}
	return out, nil
}

// applyPause 进入人工审批流程并按超时默认动作处理
func (m *Manager) applyPause(ctx context.Context, ev *fetch.RequestPausedReply, p *rulespec.Pause, stage string, ruleID *model.RuleID) {
	id := string(ev.RequestID)
	ch := m.registerApproval(id)

	if !m.sendPendingItem(id, stage, ev, ruleID, ctx, p) {
		return
	}

	mut := m.waitForApproval(ch, p.TimeoutMS)
	m.applyApprovalResult(ctx, ev, mut, p, stage)
	m.unregisterApproval(id)
}

// registerApproval 注册审批通道
func (m *Manager) registerApproval(id string) chan rulespec.Rewrite {
	ch := make(chan rulespec.Rewrite, 1)
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

// sendPendingItem 发送待审批项到 pending 通道，返回是否成功
func (m *Manager) sendPendingItem(id, stage string, ev *fetch.RequestPausedReply, ruleID *model.RuleID, ctx context.Context, p *rulespec.Pause) bool {
	if m.pending == nil {
		return true
	}
	item := model.PendingItem{
		ID:     id,
		Stage:  stage,
		URL:    ev.Request.URL,
		Method: ev.Request.Method,
		Target: m.currentTarget,
		Rule:   ruleID,
	}
	select {
	case m.pending <- item:
		return true
	default:
		m.handlePauseOverflow(id, ctx, ev, p, stage)
		return false
	}
}

// waitForApproval 等待审批结果或超时，返回变更内容（nil 表示超时）
func (m *Manager) waitForApproval(ch chan rulespec.Rewrite, timeoutMS int) *rulespec.Rewrite {
	t := time.NewTimer(time.Duration(timeoutMS) * time.Millisecond)
	defer t.Stop()
	select {
	case mut := <-ch:
		return &mut
	case <-t.C:
		return nil
	}
}

// applyApprovalResult 应用审批结果或默认动作
func (m *Manager) applyApprovalResult(ctx context.Context, ev *fetch.RequestPausedReply, mut *rulespec.Rewrite, p *rulespec.Pause, stage string) {
	if mut != nil {
		if hasEffectiveMutations(*mut) {
			m.applyRewrite(ctx, ev, mut, stage)
		} else {
			m.applyContinue(ctx, ev, stage)
		}
	} else {
		m.applyPauseDefaultAction(ctx, ev, p, stage)
	}
}

// hasEffectiveMutations 判断重写是否包含有效变更
func hasEffectiveMutations(mut rulespec.Rewrite) bool {
	return mut.Body != nil || mut.URL != nil || mut.Method != nil || len(mut.Headers) > 0 || len(mut.Query) > 0 || len(mut.Cookies) > 0
}

// applyPauseDefaultAction 应用 Pause 配置中的默认动作
func (m *Manager) applyPauseDefaultAction(ctx context.Context, ev *fetch.RequestPausedReply, p *rulespec.Pause, stage string) {
	switch p.DefaultAction.Type {
	case rulespec.PauseDefaultActionFulfill:
		m.applyRespond(ctx, ev, &rulespec.Respond{Status: p.DefaultAction.Status}, stage)
	case rulespec.PauseDefaultActionFail:
		m.applyFail(ctx, ev, &rulespec.Fail{Reason: p.DefaultAction.Reason})
	case rulespec.PauseDefaultActionContinueMutated:
		m.applyContinue(ctx, ev, stage)
	default:
		m.applyContinue(ctx, ev, stage)
	}
}

// handlePauseOverflow 处理待审批队列溢出的情况
func (m *Manager) handlePauseOverflow(id string, ctx context.Context, ev *fetch.RequestPausedReply, p *rulespec.Pause, stage string) {
	m.applyPauseDefaultAction(ctx, ev, p, stage)
	m.events <- model.Event{Type: "degraded"}
	m.approvalsMu.Lock()
	delete(m.approvals, id)
	m.approvalsMu.Unlock()
}

// degradeAndContinue 统一的降级处理：直接放行请求
func (m *Manager) degradeAndContinue(ev *fetch.RequestPausedReply, reason string) {
	m.log.Warn("执行降级策略：直接放行", "reason", reason, "requestID", ev.RequestID)
	ctx, cancel := context.WithTimeout(m.ctx, 1*time.Second)
	defer cancel()
	args := &fetch.ContinueRequestArgs{RequestID: ev.RequestID}
	if err := m.client.Fetch.ContinueRequest(ctx, args); err != nil {
		m.log.Error("降级放行请求失败", "error", err)
	}
	m.events <- model.Event{Type: "degraded"}
}

// SetRules 设置新的规则集并初始化引擎
func (m *Manager) SetRules(rs rulespec.RuleSet) { m.engine = rules.New(rs) }

// UpdateRules 更新已有规则集到引擎
func (m *Manager) UpdateRules(rs rulespec.RuleSet) {
	if m.engine == nil {
		m.engine = rules.New(rs)
	} else {
		m.engine.Update(rs)
	}
}

// Approve 根据审批ID应用外部提供的重写变更
func (m *Manager) Approve(itemID string, mutations rulespec.Rewrite) {
	m.approvalsMu.Lock()
	ch, ok := m.approvals[itemID]
	m.approvalsMu.Unlock()
	if ok {
		select {
		case ch <- mutations:
		default:
		}
	}
}

// SetConcurrency 配置拦截处理的并发工作协程数
func (m *Manager) SetConcurrency(n int) {
	m.pool = newWorkerPool(n)
	if m.pool != nil && m.pool.sem != nil {
		m.pool.setLogger(m.log)
		if m.ctx != nil {
			m.pool.start(m.ctx)
		}
		m.log.Info("并发工作池已启动", "workers", n, "queueCap", m.pool.queueCap)
	} else {
		m.log.Info("并发工作池未限制，使用无界模式")
	}
}

// SetRuntime 设置运行时阈值与处理超时时间
func (m *Manager) SetRuntime(bodySizeThreshold int64, processTimeoutMS int) {
	m.bodySizeThreshold = bodySizeThreshold
	m.processTimeoutMS = processTimeoutMS
}

// GetStats 返回规则引擎的命中统计信息
func (m *Manager) GetStats() model.EngineStats {
	if m.engine == nil {
		return model.EngineStats{ByRule: make(map[model.RuleID]int64)}
	}
	return m.engine.Stats()
}

// GetPoolStats 返回并发工作池的运行统计
func (m *Manager) GetPoolStats() (queueLen, queueCap, totalSubmit, totalDrop int64) {
	if m.pool == nil {
		return 0, 0, 0, 0
	}
	return m.pool.stats()
}
