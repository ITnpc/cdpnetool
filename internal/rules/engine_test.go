package rules_test

import (
	"testing"

	"cdpnetool/internal/rules"
	"cdpnetool/pkg/rulespec"
)

// TestEngine_Conditions 测试各种匹配条件的评估逻辑
func TestEngine_Conditions(t *testing.T) {
	tests := []struct {
		name      string
		condition rulespec.Condition
		ctx       *rules.EvalContext
		wantMatch bool
	}{
		// --- URL 匹配测试 ---
		{"URL Equals Pass", rulespec.Condition{Type: rulespec.ConditionURLEquals, Value: "https://a.com"}, &rules.EvalContext{URL: "https://a.com"}, true},
		{"URL Equals Fail", rulespec.Condition{Type: rulespec.ConditionURLEquals, Value: "https://a.com"}, &rules.EvalContext{URL: "https://b.com"}, false},
		{"URL Prefix Pass", rulespec.Condition{Type: rulespec.ConditionURLPrefix, Value: "https://a.com/api"}, &rules.EvalContext{URL: "https://a.com/api/v1"}, true},
		{"URL Suffix Pass", rulespec.Condition{Type: rulespec.ConditionURLSuffix, Value: ".js"}, &rules.EvalContext{URL: "https://a.com/script.js"}, true},
		{"URL Contains Pass", rulespec.Condition{Type: rulespec.ConditionURLContains, Value: "google"}, &rules.EvalContext{URL: "https://www.google.com/search"}, true},
		{"URL Regex Pass", rulespec.Condition{Type: rulespec.ConditionURLRegex, Pattern: `user/\d+`}, &rules.EvalContext{URL: "https://a.com/user/123"}, true},

		// --- Method & ResourceType ---
		{"Method Pass", rulespec.Condition{Type: rulespec.ConditionMethod, Values: []string{"GET", "POST"}}, &rules.EvalContext{Method: "POST"}, true},
		{"Method Case Insensitive", rulespec.Condition{Type: rulespec.ConditionMethod, Values: []string{"GET"}}, &rules.EvalContext{Method: "get"}, true},
		{"ResourceType Pass", rulespec.Condition{Type: rulespec.ConditionResourceType, Values: []string{"script", "xhr"}}, &rules.EvalContext{ResourceType: "xhr"}, true},

		// --- Header 匹配 (含大小写不敏感测试) ---
		{"Header Exists Pass", rulespec.Condition{Type: rulespec.ConditionHeaderExists, Name: "X-Token"}, &rules.EvalContext{Headers: map[string]string{"X-Token": "abc"}}, true},
		{"Header NotExists Pass", rulespec.Condition{Type: rulespec.ConditionHeaderNotExists, Name: "X-Auth"}, &rules.EvalContext{Headers: map[string]string{"X-Token": "abc"}}, true},
		{"Header Equals Case Insensitive", rulespec.Condition{Type: rulespec.ConditionHeaderEquals, Name: "Content-Type", Value: "application/json"}, &rules.EvalContext{Headers: map[string]string{"content-type": "application/json"}}, true},
		{"Header Regex Pass", rulespec.Condition{Type: rulespec.ConditionHeaderRegex, Name: "User-Agent", Pattern: "Chrome/.*"}, &rules.EvalContext{Headers: map[string]string{"User-Agent": "Mozilla/5.0 Chrome/120.0"}}, true},

		// --- Query 匹配 ---
		{"Query Exists Pass", rulespec.Condition{Type: rulespec.ConditionQueryExists, Name: "id"}, &rules.EvalContext{Query: map[string]string{"id": "1"}}, true},
		{"Query Equals Pass", rulespec.Condition{Type: rulespec.ConditionQueryEquals, Name: "name", Value: "qoder"}, &rules.EvalContext{Query: map[string]string{"name": "qoder"}}, true},
		{"Query Regex Pass", rulespec.Condition{Type: rulespec.ConditionQueryRegex, Name: "token", Pattern: "^[a-f0-9]{32}$"}, &rules.EvalContext{Query: map[string]string{"token": "5f35230224d048e7884841b83d8e059b"}}, true},

		// --- Cookie 匹配 ---
		{"Cookie Exists Pass", rulespec.Condition{Type: rulespec.ConditionCookieExists, Name: "session"}, &rules.EvalContext{Cookies: map[string]string{"session": "xyz"}}, true},
		{"Cookie Equals Pass", rulespec.Condition{Type: rulespec.ConditionCookieEquals, Name: "lang", Value: "zh"}, &rules.EvalContext{Cookies: map[string]string{"lang": "zh"}}, true},

		// --- Body 匹配 ---
		{"Body Contains Pass", rulespec.Condition{Type: rulespec.ConditionBodyContains, Value: "success"}, &rules.EvalContext{Body: `{"status": "success"}`}, true},
		{"Body Regex Pass", rulespec.Condition{Type: rulespec.ConditionBodyRegex, Pattern: `id":\d+`}, &rules.EvalContext{Body: `{"id":123}`}, true},
		{"Body JSON Path Pass", rulespec.Condition{Type: rulespec.ConditionBodyJsonPath, Path: "$.data.items.#", Value: "2"}, &rules.EvalContext{Body: `{"data":{"items":[1,2]}}`}, true},
		{"Body JSON Path Deep", rulespec.Condition{Type: rulespec.ConditionBodyJsonPath, Path: "user.profile.name", Value: "tom"}, &rules.EvalContext{Body: `{"user":{"profile":{"name":"tom"}}}`}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 为每个用例创建独立的规则配置
			config := &rulespec.Config{
				Rules: []rulespec.Rule{
					{
						ID:      "test-rule",
						Enabled: true,
						Stage:   rulespec.StageRequest,
						Match:   rulespec.Match{AllOf: []rulespec.Condition{tt.condition}},
					},
				},
			}
			// 支持响应阶段测试
			if tt.condition.Type == rulespec.ConditionBodyJsonPath || tt.condition.Type == rulespec.ConditionBodyContains || tt.condition.Type == rulespec.ConditionBodyRegex {
				config.Rules[0].Stage = rulespec.StageResponse
			}

			engine := rules.New(config)
			matched := engine.EvalForStage(tt.ctx, config.Rules[0].Stage)

			if tt.wantMatch && len(matched) == 0 {
				t.Errorf("期望匹配但未匹配成功")
			}
			if !tt.wantMatch && len(matched) > 0 {
				t.Errorf("期望不匹配但意外匹配成功")
			}
		})
	}
}

// TestEngine_ComplexLogic 测试多条件的复合逻辑（AllOf 和 AnyOf）
func TestEngine_ComplexLogic(t *testing.T) {
	config := &rulespec.Config{
		Rules: []rulespec.Rule{
			{
				ID:      "combined-rule",
				Enabled: true,
				Stage:   rulespec.StageRequest,
				Match: rulespec.Match{
					AllOf: []rulespec.Condition{
						{Type: rulespec.ConditionMethod, Values: []string{"GET"}},
						{Type: rulespec.ConditionURLContains, Value: "api"},
					},
					AnyOf: []rulespec.Condition{
						{Type: rulespec.ConditionHeaderExists, Name: "X-Admin"},
						{Type: rulespec.ConditionQueryExists, Name: "debug"},
					},
				},
			},
		},
	}
	engine := rules.New(config)

	tests := []struct {
		name      string
		ctx       *rules.EvalContext
		wantMatch bool
	}{
		{"Full Match (Admin Header)", &rules.EvalContext{Method: "GET", URL: "/api/v1", Headers: map[string]string{"X-Admin": "1"}}, true},
		{"Full Match (Debug Query)", &rules.EvalContext{Method: "GET", URL: "/api/v1", Query: map[string]string{"debug": "true"}}, true},
		{"Fail AllOf (Wrong Method)", &rules.EvalContext{Method: "POST", URL: "/api/v1", Query: map[string]string{"debug": "true"}}, false},
		{"Fail AnyOf (Missing both)", &rules.EvalContext{Method: "GET", URL: "/api/v1"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := engine.EvalForStage(tt.ctx, rulespec.StageRequest)
			if (len(matched) > 0) != tt.wantMatch {
				t.Errorf("匹配结果期望 %v, 实际 %v", tt.wantMatch, len(matched) > 0)
			}
		})
	}
}

// TestEngine_PriorityAndStats 测试规则匹配的优先级排序和统计信息记录
func TestEngine_PriorityAndStats(t *testing.T) {
	config := &rulespec.Config{
		Rules: []rulespec.Rule{
			{ID: "low", Priority: 1, Enabled: true, Stage: rulespec.StageRequest, Match: rulespec.Match{AllOf: []rulespec.Condition{{Type: rulespec.ConditionURLEquals, Value: "/a"}}}},
			{ID: "high", Priority: 100, Enabled: true, Stage: rulespec.StageRequest, Match: rulespec.Match{AllOf: []rulespec.Condition{{Type: rulespec.ConditionURLEquals, Value: "/a"}}}},
		},
	}
	engine := rules.New(config)

	// 1. 验证优先级排序
	matched := engine.EvalForStage(&rules.EvalContext{URL: "/a"}, rulespec.StageRequest)
	if len(matched) != 2 || matched[0].Rule.ID != "high" {
		t.Errorf("优先级排序错误: 第一位应该是 high, 实际是 %s", matched[0].Rule.ID)
	}

	// 2. 验证统计信息
	stats := engine.GetStats()
	if stats.Matched != 1 || stats.Total != 1 {
		t.Errorf("基础统计错误: Matched=%d, Total=%d", stats.Matched, stats.Total)
	}
	if stats.ByRule["high"] != 1 || stats.ByRule["low"] != 1 {
		t.Errorf("单条规则统计错误: %+v", stats.ByRule)
	}

	// 3. 验证统计重置
	engine.ResetStats()
	stats2 := engine.GetStats()
	if stats2.Total != 0 || len(stats2.ByRule) != 0 {
		t.Error("统计重置失败")
	}
}
