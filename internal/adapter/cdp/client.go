package cdp

import (
	"context"
	"fmt"
	"sync"

	"cdpnetool/internal/logger"
	"cdpnetool/pkg/domain"

	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/devtool"
	"github.com/mafredri/cdp/rpcc"
)

// TargetSession 代表一个已附着的浏览器目标会话
type TargetSession struct {
	ID     domain.TargetID
	Client *cdp.Client
	Conn   *rpcc.Conn
	Ctx    context.Context    // 会话级上下文
	Cancel context.CancelFunc // 取消函数
}

// ClientManager 负责管理与浏览器的 CDP 连接
type ClientManager struct {
	devtoolsURL string
	log         logger.Logger
	mu          sync.RWMutex
	sessions    map[domain.TargetID]*TargetSession
}

// NewClientManager 创建 CDP 客户端管理器
func NewClientManager(url string, l logger.Logger) *ClientManager {
	if l == nil {
		l = logger.NewNop()
	}
	return &ClientManager{
		devtoolsURL: url,
		log:         l,
		sessions:    make(map[domain.TargetID]*TargetSession),
	}
}

// TestConnection 测试与浏览器的连通性
func (m *ClientManager) TestConnection(ctx context.Context) error {
	dt := devtool.New(m.devtoolsURL)
	_, err := dt.List(ctx)
	return err
}

// ListTargets 获取浏览器当前所有的标签页目标（仅返回 type == "page"）
func (m *ClientManager) ListTargets(ctx context.Context) ([]domain.TargetInfo, error) {
	dt := devtool.New(m.devtoolsURL)
	targets, err := dt.List(ctx)
	if err != nil {
		return nil, err
	}

	res := make([]domain.TargetInfo, 0)
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, t := range targets {
		if t == nil {
			continue
		}
		// 仅返回 page 类型的目标
		if t.Type != "page" {
			continue
		}
		id := domain.TargetID(t.ID)
		_, attached := m.sessions[id]
		res = append(res, domain.TargetInfo{
			ID:        id,
			Type:      string(t.Type),
			URL:       t.URL,
			Title:     t.Title,
			IsCurrent: attached,
		})
	}
	return res, nil
}

// AttachTarget 附着到一个指定的目标
func (m *ClientManager) AttachTarget(ctx context.Context, id domain.TargetID) (*TargetSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.sessions[id]; ok {
		m.log.Info("Target 已存在，复用现有会话", "targetID", string(id))
		return s, nil
	}

	dt := devtool.New(m.devtoolsURL)
	targets, err := dt.List(ctx)
	if err != nil {
		m.log.Err(err, "获取 Target 列表失败")
		return nil, err
	}

	var target *devtool.Target
	for _, t := range targets {
		if string(t.ID) == string(id) {
			target = t
			break
		}
	}

	if target == nil {
		m.log.Warn("Target 未找到", "targetID", string(id))
		return nil, fmt.Errorf("cdp: target not found: %s", id)
	}

	// 派生 Session 级 Context
	sessionCtx, sessionCancel := context.WithCancel(ctx)

	// 使用与旧版一致的连接配置：压缩 + 大写缓冲
	conn, err := rpcc.DialContext(sessionCtx, target.WebSocketDebuggerURL,
		rpcc.WithWriteBufferSize(16*1024*1024),
		rpcc.WithCompression())
	if err != nil {
		sessionCancel()
		m.log.Err(err, "CDP 连接建立失败", "targetID", string(id), "wsURL", target.WebSocketDebuggerURL)
		return nil, err
	}

	s := &TargetSession{
		ID:     id,
		Client: cdp.NewClient(conn),
		Conn:   conn,
		Ctx:    sessionCtx,
		Cancel: sessionCancel,
	}
	m.sessions[id] = s
	m.log.Info("Target 附着成功", "targetID", string(id), "url", target.URL)
	return s, nil
}

// DetachTarget 断开与目标的连接
func (m *ClientManager) DetachTarget(id domain.TargetID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.sessions[id]; ok {
		delete(m.sessions, id)
		// 先取消 context，再关闭连接
		if s.Cancel != nil {
			s.Cancel()
		}
		if s.Conn != nil {
			return s.Conn.Close()
		}
	}
	return nil
}

// GetSession 获取已存在的会话
func (m *ClientManager) GetSession(id domain.TargetID) (*TargetSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}
