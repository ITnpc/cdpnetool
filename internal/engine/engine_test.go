package engine_test

import (
	"testing"

	"cdpnetool/internal/engine"
	"cdpnetool/pkg/rulespec"
	"cdpnetool/pkg/traffic"
)

func TestNew(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	eng := engine.New(cfg)
	if eng == nil {
		t.Error("New() returned nil")
	}
}

func TestUpdate(t *testing.T) {
	cfg1 := rulespec.NewConfig("test1")
	cfg2 := rulespec.NewConfig("test2")

	eng := engine.New(cfg1)
	eng.Update(cfg2)

	// 验证更新后配置生效
	req := &traffic.Request{
		ID:     "req1",
		URL:    "https://example.com",
		Method: "GET",
	}
	matched := eng.Eval(req, rulespec.StageRequest)
	if matched != nil {
		t.Error("Eval() should return nil for empty rules")
	}
}

func TestEval_NoConfig(t *testing.T) {
	eng := engine.New(nil)
	req := &traffic.Request{
		ID:     "req1",
		URL:    "https://example.com",
		Method: "GET",
	}
	matched := eng.Eval(req, rulespec.StageRequest)
	if matched != nil {
		t.Errorf("got %v, want nil", matched)
	}
}

func TestEval_NoMatch(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:      "rule1",
			Name:    "test rule",
			Enabled: true,
			Stage:   rulespec.StageRequest,
			Match: rulespec.Match{
				AllOf: []rulespec.Condition{
					{Type: rulespec.ConditionURLContains, Value: "notfound.com"},
				},
			},
		},
	}

	eng := engine.New(cfg)
	req := &traffic.Request{
		ID:     "req1",
		URL:    "https://example.com",
		Method: "GET",
	}
	matched := eng.Eval(req, rulespec.StageRequest)
	if matched != nil {
		t.Errorf("got %v, want nil", matched)
	}
}

func TestEval_URLContains(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:      "rule1",
			Name:    "test rule",
			Enabled: true,
			Stage:   rulespec.StageRequest,
			Match: rulespec.Match{
				AllOf: []rulespec.Condition{
					{Type: rulespec.ConditionURLContains, Value: "example.com"},
				},
			},
		},
	}

	eng := engine.New(cfg)
	req := &traffic.Request{
		ID:     "req1",
		URL:    "https://example.com/path",
		Method: "GET",
	}
	matched := eng.Eval(req, rulespec.StageRequest)
	if len(matched) != 1 {
		t.Errorf("got %d matches, want 1", len(matched))
	}
	if matched[0].Rule.ID != "rule1" {
		t.Errorf("got rule ID %v, want rule1", matched[0].Rule.ID)
	}
}

func TestEval_URLEquals(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:      "rule1",
			Name:    "test rule",
			Enabled: true,
			Stage:   rulespec.StageRequest,
			Match: rulespec.Match{
				AllOf: []rulespec.Condition{
					{Type: rulespec.ConditionURLEquals, Value: "https://example.com"},
				},
			},
		},
	}

	eng := engine.New(cfg)
	req := &traffic.Request{
		ID:     "req1",
		URL:    "https://example.com",
		Method: "GET",
	}
	matched := eng.Eval(req, rulespec.StageRequest)
	if len(matched) != 1 {
		t.Errorf("got %d matches, want 1", len(matched))
	}
}

func TestEval_URLPrefix(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:      "rule1",
			Name:    "test rule",
			Enabled: true,
			Stage:   rulespec.StageRequest,
			Match: rulespec.Match{
				AllOf: []rulespec.Condition{
					{Type: rulespec.ConditionURLPrefix, Value: "https://example"},
				},
			},
		},
	}

	eng := engine.New(cfg)
	req := &traffic.Request{
		ID:     "req1",
		URL:    "https://example.com/path",
		Method: "GET",
	}
	matched := eng.Eval(req, rulespec.StageRequest)
	if len(matched) != 1 {
		t.Errorf("got %d matches, want 1", len(matched))
	}
}

func TestEval_URLSuffix(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:      "rule1",
			Name:    "test rule",
			Enabled: true,
			Stage:   rulespec.StageRequest,
			Match: rulespec.Match{
				AllOf: []rulespec.Condition{
					{Type: rulespec.ConditionURLSuffix, Value: ".json"},
				},
			},
		},
	}

	eng := engine.New(cfg)
	req := &traffic.Request{
		ID:     "req1",
		URL:    "https://example.com/data.json",
		Method: "GET",
	}
	matched := eng.Eval(req, rulespec.StageRequest)
	if len(matched) != 1 {
		t.Errorf("got %d matches, want 1", len(matched))
	}
}

func TestEval_URLRegex(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:      "rule1",
			Name:    "test rule",
			Enabled: true,
			Stage:   rulespec.StageRequest,
			Match: rulespec.Match{
				AllOf: []rulespec.Condition{
					{Type: rulespec.ConditionURLRegex, Pattern: `^https://example\.com/\d+$`},
				},
			},
		},
	}

	eng := engine.New(cfg)
	req := &traffic.Request{
		ID:     "req1",
		URL:    "https://example.com/123",
		Method: "GET",
	}
	matched := eng.Eval(req, rulespec.StageRequest)
	if len(matched) != 1 {
		t.Errorf("got %d matches, want 1", len(matched))
	}
}

func TestEval_Method(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:      "rule1",
			Name:    "test rule",
			Enabled: true,
			Stage:   rulespec.StageRequest,
			Match: rulespec.Match{
				AllOf: []rulespec.Condition{
					{Type: rulespec.ConditionMethod, Values: []string{"POST", "PUT"}},
				},
			},
		},
	}

	eng := engine.New(cfg)
	req := &traffic.Request{
		ID:     "req1",
		URL:    "https://example.com",
		Method: "POST",
	}
	matched := eng.Eval(req, rulespec.StageRequest)
	if len(matched) != 1 {
		t.Errorf("got %d matches, want 1", len(matched))
	}
}

func TestEval_ResourceType(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:      "rule1",
			Name:    "test rule",
			Enabled: true,
			Stage:   rulespec.StageRequest,
			Match: rulespec.Match{
				AllOf: []rulespec.Condition{
					{Type: rulespec.ConditionResourceType, Values: []string{"XHR", "Fetch"}},
				},
			},
		},
	}

	eng := engine.New(cfg)
	req := &traffic.Request{
		ID:           "req1",
		URL:          "https://example.com",
		Method:       "GET",
		ResourceType: "xhr",
	}
	matched := eng.Eval(req, rulespec.StageRequest)
	if len(matched) != 1 {
		t.Errorf("got %d matches, want 1", len(matched))
	}
}

func TestEval_HeaderExists(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:      "rule1",
			Name:    "test rule",
			Enabled: true,
			Stage:   rulespec.StageRequest,
			Match: rulespec.Match{
				AllOf: []rulespec.Condition{
					{Type: rulespec.ConditionHeaderExists, Name: "Authorization"},
				},
			},
		},
	}

	eng := engine.New(cfg)
	req := &traffic.Request{
		ID:      "req1",
		URL:     "https://example.com",
		Method:  "GET",
		Headers: make(traffic.Header),
	}
	req.Headers.Set("Authorization", "Bearer token")

	matched := eng.Eval(req, rulespec.StageRequest)
	if len(matched) != 1 {
		t.Errorf("got %d matches, want 1", len(matched))
	}
}

func TestEval_HeaderNotExists(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:      "rule1",
			Name:    "test rule",
			Enabled: true,
			Stage:   rulespec.StageRequest,
			Match: rulespec.Match{
				AllOf: []rulespec.Condition{
					{Type: rulespec.ConditionHeaderNotExists, Name: "X-Custom"},
				},
			},
		},
	}

	eng := engine.New(cfg)
	req := &traffic.Request{
		ID:      "req1",
		URL:     "https://example.com",
		Method:  "GET",
		Headers: make(traffic.Header),
	}

	matched := eng.Eval(req, rulespec.StageRequest)
	if len(matched) != 1 {
		t.Errorf("got %d matches, want 1", len(matched))
	}
}

func TestEval_QueryExists(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:      "rule1",
			Name:    "test rule",
			Enabled: true,
			Stage:   rulespec.StageRequest,
			Match: rulespec.Match{
				AllOf: []rulespec.Condition{
					{Type: rulespec.ConditionQueryExists, Name: "id"},
				},
			},
		},
	}

	eng := engine.New(cfg)
	req := &traffic.Request{
		ID:     "req1",
		URL:    "https://example.com?id=123",
		Method: "GET",
		Query:  map[string]string{"id": "123"},
	}

	matched := eng.Eval(req, rulespec.StageRequest)
	if len(matched) != 1 {
		t.Errorf("got %d matches, want 1", len(matched))
	}
}

func TestEval_CookieEquals(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:      "rule1",
			Name:    "test rule",
			Enabled: true,
			Stage:   rulespec.StageRequest,
			Match: rulespec.Match{
				AllOf: []rulespec.Condition{
					{Type: rulespec.ConditionCookieEquals, Name: "session", Value: "abc123"},
				},
			},
		},
	}

	eng := engine.New(cfg)
	req := &traffic.Request{
		ID:      "req1",
		URL:     "https://example.com",
		Method:  "GET",
		Cookies: map[string]string{"session": "abc123"},
	}

	matched := eng.Eval(req, rulespec.StageRequest)
	if len(matched) != 1 {
		t.Errorf("got %d matches, want 1", len(matched))
	}
}

func TestEval_BodyContains(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:      "rule1",
			Name:    "test rule",
			Enabled: true,
			Stage:   rulespec.StageRequest,
			Match: rulespec.Match{
				AllOf: []rulespec.Condition{
					{Type: rulespec.ConditionBodyContains, Value: "test"},
				},
			},
		},
	}

	eng := engine.New(cfg)
	req := &traffic.Request{
		ID:     "req1",
		URL:    "https://example.com",
		Method: "POST",
		Body:   []byte("test data"),
	}

	matched := eng.Eval(req, rulespec.StageRequest)
	if len(matched) != 1 {
		t.Errorf("got %d matches, want 1", len(matched))
	}
}

func TestEval_BodyJsonPath(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:      "rule1",
			Name:    "test rule",
			Enabled: true,
			Stage:   rulespec.StageRequest,
			Match: rulespec.Match{
				AllOf: []rulespec.Condition{
					{Type: rulespec.ConditionBodyJsonPath, Path: "$.name", Value: "test"},
				},
			},
		},
	}

	eng := engine.New(cfg)
	req := &traffic.Request{
		ID:     "req1",
		URL:    "https://example.com",
		Method: "POST",
		Body:   []byte(`{"name":"test","age":18}`),
	}

	matched := eng.Eval(req, rulespec.StageRequest)
	if len(matched) != 1 {
		t.Errorf("got %d matches, want 1", len(matched))
	}
}

func TestEval_Priority(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:       "rule1",
			Name:     "low priority",
			Enabled:  true,
			Priority: 1,
			Stage:    rulespec.StageRequest,
			Match: rulespec.Match{
				AllOf: []rulespec.Condition{
					{Type: rulespec.ConditionURLContains, Value: "example.com"},
				},
			},
		},
		{
			ID:       "rule2",
			Name:     "high priority",
			Enabled:  true,
			Priority: 10,
			Stage:    rulespec.StageRequest,
			Match: rulespec.Match{
				AllOf: []rulespec.Condition{
					{Type: rulespec.ConditionURLContains, Value: "example.com"},
				},
			},
		},
	}

	eng := engine.New(cfg)
	req := &traffic.Request{
		ID:     "req1",
		URL:    "https://example.com",
		Method: "GET",
	}

	matched := eng.Eval(req, rulespec.StageRequest)
	if len(matched) != 2 {
		t.Errorf("got %d matches, want 2", len(matched))
	}
	// 验证优先级排序
	if matched[0].Rule.ID != "rule2" {
		t.Errorf("first match should be rule2, got %v", matched[0].Rule.ID)
	}
	if matched[1].Rule.ID != "rule1" {
		t.Errorf("second match should be rule1, got %v", matched[1].Rule.ID)
	}
}

func TestEval_Disabled(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:      "rule1",
			Name:    "disabled rule",
			Enabled: false,
			Stage:   rulespec.StageRequest,
			Match: rulespec.Match{
				AllOf: []rulespec.Condition{
					{Type: rulespec.ConditionURLContains, Value: "example.com"},
				},
			},
		},
	}

	eng := engine.New(cfg)
	req := &traffic.Request{
		ID:     "req1",
		URL:    "https://example.com",
		Method: "GET",
	}

	matched := eng.Eval(req, rulespec.StageRequest)
	if matched != nil {
		t.Errorf("got %v matches, want nil", len(matched))
	}
}

func TestEval_WrongStage(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:      "rule1",
			Name:    "response rule",
			Enabled: true,
			Stage:   rulespec.StageResponse,
			Match: rulespec.Match{
				AllOf: []rulespec.Condition{
					{Type: rulespec.ConditionURLContains, Value: "example.com"},
				},
			},
		},
	}

	eng := engine.New(cfg)
	req := &traffic.Request{
		ID:     "req1",
		URL:    "https://example.com",
		Method: "GET",
	}

	matched := eng.Eval(req, rulespec.StageRequest)
	if matched != nil {
		t.Errorf("got %v matches, want nil", len(matched))
	}
}

func TestEval_AnyOf(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:      "rule1",
			Name:    "anyof rule",
			Enabled: true,
			Stage:   rulespec.StageRequest,
			Match: rulespec.Match{
				AnyOf: []rulespec.Condition{
					{Type: rulespec.ConditionURLContains, Value: "example.com"},
					{Type: rulespec.ConditionURLContains, Value: "test.com"},
				},
			},
		},
	}

	eng := engine.New(cfg)
	req := &traffic.Request{
		ID:     "req1",
		URL:    "https://example.com",
		Method: "GET",
	}

	matched := eng.Eval(req, rulespec.StageRequest)
	if len(matched) != 1 {
		t.Errorf("got %d matches, want 1", len(matched))
	}
}

func TestRecordStats(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:      "rule1",
			Name:    "test rule",
			Enabled: true,
			Stage:   rulespec.StageRequest,
			Match: rulespec.Match{
				AllOf: []rulespec.Condition{
					{Type: rulespec.ConditionURLContains, Value: "example.com"},
				},
			},
		},
	}

	eng := engine.New(cfg)
	req := &traffic.Request{
		ID:     "req1",
		URL:    "https://example.com",
		Method: "GET",
	}

	matched := eng.Eval(req, rulespec.StageRequest)
	eng.RecordStats(matched)

	total, matchedCount, byRule := eng.GetStats()
	if total != 1 {
		t.Errorf("got total %d, want 1", total)
	}
	if matchedCount != 1 {
		t.Errorf("got matched %d, want 1", matchedCount)
	}
	if byRule["rule1"] != 1 {
		t.Errorf("got rule1 count %d, want 1", byRule["rule1"])
	}
}

func TestGetStats(t *testing.T) {
	cfg := rulespec.NewConfig("test")
	eng := engine.New(cfg)

	total, matched, byRule := eng.GetStats()
	if total != 0 {
		t.Errorf("got total %d, want 0", total)
	}
	if matched != 0 {
		t.Errorf("got matched %d, want 0", matched)
	}
	if len(byRule) != 0 {
		t.Errorf("got byRule len %d, want 0", len(byRule))
	}
}
