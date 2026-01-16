package storage

import (
	"time"
)

// Setting 用户设置表
type Setting struct {
	Key       string    `gorm:"primaryKey" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// 预定义的设置 Key
const (
	SettingKeyDevToolsURL   = "devtools_url"
	SettingKeyTheme         = "theme"
	SettingKeyWindowBounds  = "window_bounds"
	SettingKeyLastRuleSetID = "last_ruleset_id"
)

// RuleSetRecord 规则集表
type RuleSetRecord struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"uniqueIndex;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"` // 配置描述
	Version     string    `json:"version"`
	RulesJSON   string    `gorm:"type:text" json:"rulesJson"` // JSON 序列化的规则数组
	IsActive    bool      `gorm:"default:false" json:"isActive"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// InterceptEventRecord 拦截事件历史表
type InterceptEventRecord struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	SessionID  string    `gorm:"index" json:"sessionId"`
	TargetID   string    `json:"targetId"`
	Type       string    `gorm:"index" json:"type"` // matched, rewritten, failed, rejected...
	URL        string    `json:"url"`
	Method     string    `json:"method"`
	Stage      string    `json:"stage"`
	StatusCode int       `json:"statusCode"`
	RuleID     *string   `json:"ruleId"`
	Error      string    `json:"error"`
	Timestamp  int64     `gorm:"index" json:"timestamp"`
	CreatedAt  time.Time `json:"createdAt"`
}
