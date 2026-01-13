package cdp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
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
	"github.com/mafredri/cdp/rpcc"
)

// Manager 负责管理一个会话下的所有浏览器 page 目标
//
// 设计要点：
//   - 一个 Manager 对应一个 DevTools 端点（一个浏览器实例）。
//   - 同一会话内可以同时附加多个 page，每个 page 对应一个独立的 targetSession。
//   - 不再自动发现 / 自动跟随前台页面，所有需要拦截的 page 由调用方显式 Attach / Detach。
//   - 默认行为：当调用 AttachTarget 且未指定 targetID 时，自动选择第一个 Type=="page" 的目标。
//   - 当某个 page 被用户关闭或连接异常导致拦截流终止时，自动清理对应的 targetSession。
//
// 这样可以在保证能力的前提下，大幅降低隐式状态和出错概率。

type Manager struct {
	devtoolsURL string
	log         logger.Logger

	// 规则与运行时配置
	engine            *rules.Engine
	bodySizeThreshold int64
	processTimeoutMS  int

	// 并发控制（在会话维度共享）
	pool *workerPool

	// 事件通道（由 service 层创建并传入，会话级共享）
	events  chan model.Event
	pending chan model.PendingItem

	// Pause 审批通道
	approvalsMu sync.Mutex
	approvals   map[string]chan *rulespec.Rewrite

	// 已附加的 targets
	targetsMu sync.Mutex
	targets   map[model.TargetID]*targetSession

	// 拦截开关（会话级）
	stateMu sync.RWMutex
	enabled bool
}

// targetSession 表示一个已附加并可拦截的 page 目标
// 每个目标拥有独立的 CDP 连接与上下文，但共享同一个规则引擎与 worker pool。

type targetSession struct {
	id     model.TargetID
	conn   *rpcc.Conn
	client *cdp.Client
	ctx    context.Context
	cancel context.CancelFunc
}

// New 创建并返回一个管理器，用于管理 CDP 连接与拦截流程
func New(devtoolsURL string, events chan model.Event, pending chan model.PendingItem, l logger.Logger) *Manager {
	if l == nil {
		l = logger.NewNoopLogger()
	}
	return &Manager{
		devtoolsURL: devtoolsURL,
		log:         l,
		events:      events,
		pending:     pending,
		approvals:   make(map[string]chan *rulespec.Rewrite),
		targets:     make(map[model.TargetID]*targetSession),
	}
}

// setEnabled 设置拦截开关
func (m *Manager) setEnabled(v bool) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	m.enabled = v
}

// isEnabled 获取当前拦截开关状态
func (m *Manager) isEnabled() bool {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.enabled
}

// AttachTarget 附加到指定浏览器目标并建立 CDP 会话。
//
// 语义：
//   - 如果 target 为空，自动选择第一个 Type=="page" 的目标。
//   - 如果指定的 target 已经附加，则本调用是幂等的，直接返回。
//   - 不会影响其他已附加的目标，可以多次调用以附加多个 page。
func (m *Manager) AttachTarget(target model.TargetID) error {
	m.targetsMu.Lock()
	defer m.targetsMu.Unlock()

	if m.devtoolsURL == "" {
		return fmt.Errorf("devtools url empty")
	}

	// 已附加则幂等返回
	if target != "" {
		if _, ok := m.targets[target]; ok {
			return nil
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	selected, err := m.selectTarget(ctx, target)
	if err != nil {
		cancel()
		return err
	}
	if selected == nil {
		cancel()
		m.log.Error("未找到可附加的浏览器目标")
		return fmt.Errorf("no target")
	}

	conn, err := rpcc.DialContext(ctx, selected.WebSocketDebuggerURL)
	if err != nil {
		cancel()
		m.log.Error("连接浏览器 DevTools 失败", "error", err)
		return err
	}

	client := cdp.NewClient(conn)
	ts := &targetSession{
		id:     model.TargetID(selected.ID),
		conn:   conn,
		client: client,
		ctx:    ctx,
		cancel: cancel,
	}

	m.targets[ts.id] = ts
	m.log.Info("附加浏览器目标成功", "target", string(ts.id))

	// 如果会话已经启用拦截，则对新目标立即启用
	if m.isEnabled() {
		if err := m.enableTarget(ts); err != nil {
			m.log.Error("为新目标启用拦截失败", "target", string(ts.id), "error", err)
		}
	}

	return nil
}

// Detach 断开目标连接并释放资源。
// target 为空时，表示断开所有已附加目标。
func (m *Manager) Detach(target model.TargetID) error {
	m.targetsMu.Lock()
	defer m.targetsMu.Unlock()

	if target == "" {
		for id, ts := range m.targets {
			m.closeTargetSession(ts)
			delete(m.targets, id)
		}
		return nil
	}

	ts, ok := m.targets[target]
	if !ok {
		return nil
	}
	m.closeTargetSession(ts)
	delete(m.targets, target)
	return nil
}

// closeTargetSession 关闭单个 targetSession
func (m *Manager) closeTargetSession(ts *targetSession) {
	if ts == nil {
		return
	}
	if ts.cancel != nil {
		ts.cancel()
	}
	if ts.conn != nil {
		_ = ts.conn.Close()
	}
}

// Enable 启用 Fetch/Network 拦截功能并开始消费事件
func (m *Manager) Enable() error {
	m.targetsMu.Lock()
	defer m.targetsMu.Unlock()

	if len(m.targets) == 0 {
		return fmt.Errorf("no targets attached")
	}

	m.log.Info("开始启用拦截功能")
	m.setEnabled(true)

	for id, ts := range m.targets {
		if err := m.enableTarget(ts); err != nil {
			m.log.Error("为目标启用拦截失败", "target", string(id), "error", err)
		}
	}

	m.log.Info("拦截功能启用完成")
	return nil
}

// enableTarget 为单个目标启用 Network/Fetch 并启动事件消费
func (m *Manager) enableTarget(ts *targetSession) error {
	if ts == nil || ts.client == nil {
		return fmt.Errorf("target client not initialized")
	}

	if err := ts.client.Network.Enable(ts.ctx, nil); err != nil {
		return err
	}

	p := "*"
	patterns := []fetch.RequestPattern{
		{URLPattern: &p, RequestStage: fetch.RequestStageRequest},
		{URLPattern: &p, RequestStage: fetch.RequestStageResponse},
	}
	if err := ts.client.Fetch.Enable(ts.ctx, &fetch.EnableArgs{Patterns: patterns}); err != nil {
		return err
	}

	// 如果已配置 worker pool 且未启动，现在启动
	if m.pool != nil && m.pool.sem != nil {
		m.pool.setLogger(m.log)
		m.pool.start(ts.ctx)
	}

	go m.consume(ts)
	return nil
}

// Disable 停止拦截功能但保留连接
func (m *Manager) Disable() error {
	m.targetsMu.Lock()
	defer m.targetsMu.Unlock()

	if len(m.targets) == 0 {
		m.setEnabled(false)
		return nil
	}

	m.setEnabled(false)

	for id, ts := range m.targets {
		if ts.client == nil {
			continue
		}
		if err := ts.client.Fetch.Disable(ts.ctx); err != nil {
			m.log.Error("停用目标拦截失败", "target", string(id), "error", err)
		}
	}

	return nil
}

// buildRuleContext 构造规则匹配上下文
func (m *Manager) buildRuleContext(ts *targetSession, ev *fetch.RequestPausedReply, stage string) rules.Ctx {
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
			ctx2, cancel := context.WithTimeout(ts.ctx, 500*time.Millisecond)
			defer cancel()
			rb, err := ts.client.Fetch.GetResponseBody(ctx2, &fetch.GetResponseBodyArgs{RequestID: ev.RequestID})
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

	return rules.Ctx{
		URL:         ev.Request.URL,
		Method:      ev.Request.Method,
		Headers:     h,
		Query:       q,
		Cookies:     ck,
		Body:        bodyText,
		ContentType: ctype,
		Stage:       stage,
	}
}

// decide 构造规则上下文并进行匹配决策
func (m *Manager) decide(ts *targetSession, ev *fetch.RequestPausedReply, stage string) *rules.Result {
	if m.engine == nil {
		return nil
	}
	ctx := m.buildRuleContext(ts, ev, stage)
	res := m.engine.Eval(ctx)
	if res == nil {
		return nil
	}
	return res
}

// selectTarget 根据传入的 targetID 或默认策略选择目标
func (m *Manager) selectTarget(ctx context.Context, target model.TargetID) (*devtool.Target, error) {
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

	// 默认选择第一个 page 目标，不做 URL 过滤
	for i := range targets {
		if targets[i] == nil {
			continue
		}
		if targets[i].Type != "page" {
			continue
		}
		return targets[i], nil
	}

	return nil, nil
}

// ListTargets 列出当前浏览器中的所有 page 目标，并标记哪些已附加
func (m *Manager) ListTargets(ctx context.Context) ([]model.TargetInfo, error) {
	if m.devtoolsURL == "" {
		return nil, fmt.Errorf("devtools url empty")
	}
	dt := devtool.New(m.devtoolsURL)
	targets, err := dt.List(ctx)
	if err != nil {
		return nil, err
	}

	m.targetsMu.Lock()
	defer m.targetsMu.Unlock()

	out := make([]model.TargetInfo, 0, len(targets))
	for i := range targets {
		if targets[i] == nil {
			continue
		}
		if targets[i].Type != "page" {
			continue
		}
		id := model.TargetID(targets[i].ID)
		info := model.TargetInfo{
			ID:        id,
			Type:      string(targets[i].Type),
			URL:       targets[i].URL,
			Title:     targets[i].Title,
			IsCurrent: m.targets[id] != nil, // 语义：当前会话是否已附加该目标
			IsUser:    isUserPageURL(targets[i].URL),
		}
		out = append(out, info)
	}
	return out, nil
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
		case ch <- &mutations:
		default:
		}
	}
}

// Reject 拒绝审批项，使请求失败
func (m *Manager) Reject(itemID string) {
	m.approvalsMu.Lock()
	ch, ok := m.approvals[itemID]
	m.approvalsMu.Unlock()
	if ok {
		select {
		case ch <- nil: // nil 表示拒绝
		default:
		}
	}
}

// SetConcurrency 配置拦截处理的并发工作协程数
func (m *Manager) SetConcurrency(n int) {
	m.pool = newWorkerPool(n)
	if m.pool != nil && m.pool.sem != nil {
		m.pool.setLogger(m.log)
		m.log.Info("并发工作池已配置", "workers", n, "queueCap", m.pool.queueCap)
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
