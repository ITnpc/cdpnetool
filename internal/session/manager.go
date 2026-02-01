package session

import (
	"sync"

	"cdpnetool/internal/logger"
	"cdpnetool/pkg/domain"
)

// Manager 全局会话管理器
type Manager struct {
	mu       sync.RWMutex
	sessions map[domain.SessionID]*Session
	log      logger.Logger
}

// NewManager 创建会话管理器
func NewManager(l logger.Logger) *Manager {
	if l == nil {
		l = logger.NewNop()
	}
	return &Manager{
		sessions: make(map[domain.SessionID]*Session),
		log:      l,
	}
}

// Create 创建并注册新会话
func (m *Manager) Create(id domain.SessionID) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	s := New(id)
	m.sessions[id] = s
	m.log.Info("创建业务会话", "sessionID", string(id))
	return s
}

// Get 获取会话
func (m *Manager) Get(id domain.SessionID) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

// Delete 销毁会话
func (m *Manager) Delete(id domain.SessionID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
	m.log.Info("销毁业务会话", "sessionID", string(id))
}

// List 返回所有活动会话
func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		list = append(list, s)
	}
	return list
}
