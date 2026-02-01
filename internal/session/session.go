package session

import (
	"sync"

	"cdpnetool/pkg/domain"
	"cdpnetool/pkg/rulespec"
)

// Session 代表一个业务拦截会话
type Session struct {
	ID     domain.SessionID
	Config *rulespec.Config // 业务规则配置

	mu      sync.RWMutex
	targets map[domain.TargetID]struct{} // 属于该会话的浏览器目标 ID
}

// New 创建一个新的会话实例
func New(id domain.SessionID) *Session {
	return &Session{
		ID:      id,
		targets: make(map[domain.TargetID]struct{}),
	}
}

// AddTarget 关联一个浏览器目标
func (s *Session) AddTarget(id domain.TargetID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.targets[id] = struct{}{}
}

// RemoveTarget 移除关联
func (s *Session) RemoveTarget(id domain.TargetID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.targets, id)
}

// GetTargets 获取所有关联的目标 ID
func (s *Session) GetTargets() []domain.TargetID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]domain.TargetID, 0, len(s.targets))
	for id := range s.targets {
		ids = append(ids, id)
	}
	return ids
}

// UpdateConfig 更新规则配置
func (s *Session) UpdateConfig(cfg *rulespec.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Config = cfg
}
