package rules

import (
	"sort"
	"strings"
	"sync"

	"cdpnetool/internal/regexutil"
	"cdpnetool/pkg/rulespec"

	"github.com/tidwall/gjson"
)

// Engine 规则引擎
type Engine struct {
	config  *rulespec.Config
	mu      sync.RWMutex
	total   int64
	matched int64
	byRule  map[string]int64
	cache   *regexutil.Cache
}

// New 创建规则引擎实例，并初始化正则缓存
func New(config *rulespec.Config) *Engine {
	return &Engine{
		config: config,
		byRule: make(map[string]int64),
		cache:  regexutil.New(),
	}
}

// Update 线程安全地更新规则配置
func (e *Engine) Update(config *rulespec.Config) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config = config
}

// GetConfig 线程安全地获取当前生效的配置
func (e *Engine) GetConfig() *rulespec.Config {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config
}

// EvalContext 评估上下文
type EvalContext struct {
	URL          string            // 请求 URL
	Method       string            // HTTP 方法
	Headers      map[string]string // 请求头
	Query        map[string]string // 查询参数
	Cookies      map[string]string // Cookie
	Body         string            // 请求体
	ResourceType string            // 资源类型
}

// MatchedRule 匹配的规则
type MatchedRule struct {
	Rule *rulespec.Rule
}

// EvalForStage 评估指定阶段（请求或响应）的匹配规则
// 返回按优先级（Priority）从大到小排序的匹配规则列表
func (e *Engine) EvalForStage(ctx *EvalContext, stage rulespec.Stage) []*MatchedRule {
	e.mu.Lock()
	e.total++
	config := e.config
	e.mu.Unlock()

	if config == nil || len(config.Rules) == 0 {
		return nil
	}

	var matched []*MatchedRule
	for i := range config.Rules {
		rule := &config.Rules[i]
		// 跳过禁用的规则
		if !rule.Enabled {
			continue
		}
		// 跳过不匹配阶段的规则
		if rule.Stage != stage {
			continue
		}
		// 评估匹配条件
		if e.matchRule(ctx, &rule.Match) {
			matched = append(matched, &MatchedRule{Rule: rule})
		}
	}

	if len(matched) == 0 {
		return nil
	}

	// 按优先级从大到小排序
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Rule.Priority > matched[j].Rule.Priority
	})

	// 更新统计
	e.mu.Lock()
	e.matched++
	for _, m := range matched {
		e.byRule[m.Rule.ID]++
	}
	e.mu.Unlock()

	return matched
}

// matchRule 评估匹配规则
func (e *Engine) matchRule(ctx *EvalContext, m *rulespec.Match) bool {
	// allOf: 所有条件都必须满足
	if len(m.AllOf) > 0 {
		for i := range m.AllOf {
			if !e.evalCondition(ctx, &m.AllOf[i]) {
				return false
			}
		}
	}
	// anyOf: 任一条件满足即可
	if len(m.AnyOf) > 0 {
		anyMatch := false
		for i := range m.AnyOf {
			if e.evalCondition(ctx, &m.AnyOf[i]) {
				anyMatch = true
				break
			}
		}
		if !anyMatch {
			return false
		}
	}
	return true
}

// evalCondition 评估单个条件
func (e *Engine) evalCondition(ctx *EvalContext, c *rulespec.Condition) bool {
	switch c.Type {
	// URL 条件
	case rulespec.ConditionURLEquals:
		return ctx.URL == c.Value
	case rulespec.ConditionURLPrefix:
		return strings.HasPrefix(ctx.URL, c.Value)
	case rulespec.ConditionURLSuffix:
		return strings.HasSuffix(ctx.URL, c.Value)
	case rulespec.ConditionURLContains:
		return strings.Contains(ctx.URL, c.Value)
	case rulespec.ConditionURLRegex:
		return e.matchRegex(ctx.URL, c.Pattern)

	// Method 条件
	case rulespec.ConditionMethod:
		for _, v := range c.Values {
			if strings.EqualFold(ctx.Method, v) {
				return true
			}
		}
		return false

	// ResourceType 条件
	case rulespec.ConditionResourceType:
		for _, v := range c.Values {
			if strings.EqualFold(ctx.ResourceType, v) {
				return true
			}
		}
		return false

	// Header 条件
	case rulespec.ConditionHeaderExists:
		_, ok := e.getHeader(ctx.Headers, c.Name)
		return ok
	case rulespec.ConditionHeaderNotExists:
		_, ok := e.getHeader(ctx.Headers, c.Name)
		return !ok
	case rulespec.ConditionHeaderEquals:
		v, ok := e.getHeader(ctx.Headers, c.Name)
		return ok && v == c.Value
	case rulespec.ConditionHeaderContains:
		v, ok := e.getHeader(ctx.Headers, c.Name)
		return ok && strings.Contains(v, c.Value)
	case rulespec.ConditionHeaderRegex:
		v, ok := e.getHeader(ctx.Headers, c.Name)
		return ok && e.matchRegex(v, c.Pattern)

	// Query 条件（key 统一小写匹配）
	case rulespec.ConditionQueryExists:
		_, ok := ctx.Query[strings.ToLower(c.Name)]
		return ok
	case rulespec.ConditionQueryNotExists:
		_, ok := ctx.Query[strings.ToLower(c.Name)]
		return !ok
	case rulespec.ConditionQueryEquals:
		v, ok := ctx.Query[strings.ToLower(c.Name)]
		return ok && v == c.Value
	case rulespec.ConditionQueryContains:
		v, ok := ctx.Query[strings.ToLower(c.Name)]
		return ok && strings.Contains(v, c.Value)
	case rulespec.ConditionQueryRegex:
		v, ok := ctx.Query[strings.ToLower(c.Name)]
		return ok && e.matchRegex(v, c.Pattern)

	// Cookie 条件（name 统一小写匹配）
	case rulespec.ConditionCookieExists:
		_, ok := ctx.Cookies[strings.ToLower(c.Name)]
		return ok
	case rulespec.ConditionCookieNotExists:
		_, ok := ctx.Cookies[strings.ToLower(c.Name)]
		return !ok
	case rulespec.ConditionCookieEquals:
		v, ok := ctx.Cookies[strings.ToLower(c.Name)]
		return ok && v == c.Value
	case rulespec.ConditionCookieContains:
		v, ok := ctx.Cookies[strings.ToLower(c.Name)]
		return ok && strings.Contains(v, c.Value)
	case rulespec.ConditionCookieRegex:
		v, ok := ctx.Cookies[strings.ToLower(c.Name)]
		return ok && e.matchRegex(v, c.Pattern)

	// Body 条件
	case rulespec.ConditionBodyContains:
		return strings.Contains(ctx.Body, c.Value)
	case rulespec.ConditionBodyRegex:
		return e.matchRegex(ctx.Body, c.Pattern)
	case rulespec.ConditionBodyJsonPath:
		val, ok := e.evalJsonPath(ctx.Body, c.Path)
		return ok && val == c.Value

	default:
		return false
	}
}

// getHeader 获取 HTTP 头部值（支持不区分大小写的键名匹配）
func (e *Engine) getHeader(headers map[string]string, name string) (string, bool) {
	// 先尝试精确匹配
	if v, ok := headers[name]; ok {
		return v, true
	}
	// 不区分大小写匹配
	nameLower := strings.ToLower(name)
	for k, v := range headers {
		if strings.ToLower(k) == nameLower {
			return v, true
		}
	}
	return "", false
}

// evalJsonPath 评估 JSON Path，使用 gjson 支持完整语法
func (e *Engine) evalJsonPath(body, path string) (string, bool) {
	if body == "" || path == "" {
		return "", false
	}

	searchPath := path
	if strings.HasPrefix(path, "$.") {
		searchPath = path[2:]
	}

	result := gjson.Get(body, searchPath)
	if !result.Exists() {
		return "", false
	}

	return result.String(), true
}

// matchRegex 使用缓存的正则进行匹配
func (e *Engine) matchRegex(s, pattern string) bool {
	re, err := e.cache.Get(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(s)
}

// Stats 返回统计信息
type Stats struct {
	Total   int64
	Matched int64
	ByRule  map[string]int64
}

// GetStats 获取统计信息
func (e *Engine) GetStats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	byRule := make(map[string]int64, len(e.byRule))
	for k, v := range e.byRule {
		byRule[k] = v
	}
	return Stats{
		Total:   e.total,
		Matched: e.matched,
		ByRule:  byRule,
	}
}

// ResetStats 重置统计信息
func (e *Engine) ResetStats() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.total = 0
	e.matched = 0
	e.byRule = make(map[string]int64)
}
