package main

import (
	"fmt"
	"sync"

	api "cdpnetool/pkg/api"
	"cdpnetool/pkg/model"
	"cdpnetool/pkg/rulespec"
)

// App 是 GUI 应用的核心状态与业务逻辑封装
type App struct {
	mu  sync.RWMutex
	svc api.Service

	sessions       []SessionItem
	currentSession int

	targets       []TargetItem
	currentTarget int

	rules []RuleItem
}

// SessionItem 表示会话列表项
type SessionItem struct {
	ID          string
	DevToolsURL string
	Enabled     bool
}

// TargetItem 表示目标列表项
type TargetItem struct {
	ID       string
	Title    string
	URL      string
	Type     string
	Attached bool
	IsUser   bool
}

// RuleItem 表示规则列表项
type RuleItem struct {
	ID       string
	Name     string
	Priority int
	Mode     string
}

// NewApp 创建应用实例
func NewApp() *App {
	return &App{
		svc:            api.NewService(),
		currentSession: -1,
		currentTarget:  -1,
	}
}

// StartSession 创建新会话
func (a *App) StartSession(devToolsURL string) error {
	if devToolsURL == "" {
		devToolsURL = "http://127.0.0.1:9222"
	}
	cfg := model.SessionConfig{DevToolsURL: devToolsURL}
	id, err := a.svc.StartSession(cfg)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessions = append(a.sessions, SessionItem{
		ID:          string(id),
		DevToolsURL: devToolsURL,
		Enabled:     false,
	})
	if a.currentSession == -1 {
		a.currentSession = 0
	}
	return nil
}

// SetCurrentSession 设置当前活跃会话
func (a *App) SetCurrentSession(idx int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if idx < 0 || idx >= len(a.sessions) {
		a.currentSession = -1
		return
	}
	a.currentSession = idx
}

// GetSessions 获取会话列表
func (a *App) GetSessions() []SessionItem {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]SessionItem, len(a.sessions))
	copy(result, a.sessions)
	return result
}

// GetCurrentSessionID 获取当前会话 ID
func (a *App) GetCurrentSessionID() (model.SessionID, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.currentSession < 0 || a.currentSession >= len(a.sessions) {
		return "", false
	}
	return model.SessionID(a.sessions[a.currentSession].ID), true
}

// EnableInterception 启用拦截
func (a *App) EnableInterception() error {
	id, ok := a.GetCurrentSessionID()
	if !ok {
		return fmt.Errorf("no session selected")
	}
	if err := a.svc.EnableInterception(id); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.currentSession >= 0 && a.currentSession < len(a.sessions) {
		a.sessions[a.currentSession].Enabled = true
	}
	return nil
}

// DisableInterception 停用拦截
func (a *App) DisableInterception() error {
	id, ok := a.GetCurrentSessionID()
	if !ok {
		return fmt.Errorf("no session selected")
	}
	if err := a.svc.DisableInterception(id); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.currentSession >= 0 && a.currentSession < len(a.sessions) {
		a.sessions[a.currentSession].Enabled = false
	}
	return nil
}

// RefreshTargets 刷新目标列表
func (a *App) RefreshTargets() error {
	id, ok := a.GetCurrentSessionID()
	if !ok {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.targets = nil
		a.currentTarget = -1
		return nil
	}
	targets, err := a.svc.ListTargets(id)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.targets = a.targets[:0]
	for _, t := range targets {
		a.targets = append(a.targets, TargetItem{
			ID:       string(t.ID),
			Title:    t.Title,
			URL:      t.URL,
			Type:     t.Type,
			Attached: t.IsCurrent,
			IsUser:   t.IsUser,
		})
	}
	a.currentTarget = -1
	return nil
}

// GetTargets 获取目标列表
func (a *App) GetTargets() []TargetItem {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]TargetItem, len(a.targets))
	copy(result, a.targets)
	return result
}

// SetCurrentTarget 设置当前选中目标
func (a *App) SetCurrentTarget(idx int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if idx < 0 || idx >= len(a.targets) {
		a.currentTarget = -1
		return
	}
	a.currentTarget = idx
}

// AttachSelectedTarget 附加选中目标
func (a *App) AttachSelectedTarget() error {
	id, ok := a.GetCurrentSessionID()
	if !ok {
		return fmt.Errorf("no session selected")
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.currentTarget < 0 || a.currentTarget >= len(a.targets) {
		return fmt.Errorf("no target selected")
	}
	t := a.targets[a.currentTarget]
	return a.svc.AttachTarget(id, model.TargetID(t.ID))
}

// DetachSelectedTarget 移除选中目标
func (a *App) DetachSelectedTarget() error {
	id, ok := a.GetCurrentSessionID()
	if !ok {
		return fmt.Errorf("no session selected")
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.currentTarget < 0 || a.currentTarget >= len(a.targets) {
		return fmt.Errorf("no target selected")
	}
	t := a.targets[a.currentTarget]
	return a.svc.DetachTarget(id, model.TargetID(t.ID))
}

// AttachDefaultTarget 附加默认目标
func (a *App) AttachDefaultTarget() error {
	id, ok := a.GetCurrentSessionID()
	if !ok {
		return fmt.Errorf("no session selected")
	}
	return a.svc.AttachTarget(id, "")
}

// LoadRules 加载规则集
func (a *App) LoadRules(rs rulespec.RuleSet) error {
	id, ok := a.GetCurrentSessionID()
	if !ok {
		return fmt.Errorf("no session selected")
	}
	if err := a.svc.LoadRules(id, rs); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rules = a.rules[:0]
	for _, r := range rs.Rules {
		a.rules = append(a.rules, RuleItem{
			ID:       string(r.ID),
			Name:     r.Name,
			Priority: r.Priority,
			Mode:     string(r.Mode),
		})
	}
	return nil
}

// GetRules 获取规则列表
func (a *App) GetRules() []RuleItem {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]RuleItem, len(a.rules))
	copy(result, a.rules)
	return result
}
