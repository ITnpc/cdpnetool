package model

type SessionID string
type TargetID string
type RuleID string

type SessionConfig struct {
	DevToolsURL       string `json:"devToolsURL"`
	Concurrency       int    `json:"concurrency"`
	BodySizeThreshold int64  `json:"bodySizeThreshold"`
	PendingCapacity   int    `json:"pendingCapacity"`
	ProcessTimeoutMS  int    `json:"processTimeoutMS"`
}

// 规则相关类型已迁移至 pkg/rulespec

type EngineStats struct {
	Total   int64            `json:"total"`
	Matched int64            `json:"matched"`
	ByRule  map[RuleID]int64 `json:"byRule"`
}

type Event struct {
	Type    string    `json:"type"`
	Session SessionID `json:"session"`
	Target  TargetID  `json:"target"`
	Rule    *RuleID   `json:"rule"`
	Error   error     `json:"error"`
}

type PendingItem struct {
	ID     string   `json:"id"`
	Stage  string   `json:"stage"`
	URL    string   `json:"url"`
	Method string   `json:"method"`
	Target TargetID `json:"target"`
	Rule   *RuleID  `json:"rule"`
}

type TargetInfo struct {
	ID        TargetID `json:"id"`
	Type      string   `json:"type"`
	URL       string   `json:"url"`
	Title     string   `json:"title"`
	IsCurrent bool     `json:"isCurrent"`
	IsUser    bool     `json:"isUser"`
}

// InterceptedRequest 领域模型：被拦截的请求/响应
type InterceptedRequest struct {
	RequestID string
	Stage     string // "request" or "response"
	URL       string
	Method    string

	// 请求阶段
	RequestHeaders map[string]string
	PostData       *string

	// 响应阶段
	ResponseStatusCode *int
	ResponseHeaders    map[string]string
	ResponseBody       *string
}
