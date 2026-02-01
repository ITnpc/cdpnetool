package auditor_test

import (
	"testing"
	"time"

	"cdpnetool/internal/auditor"
	"cdpnetool/internal/logger"
	"cdpnetool/pkg/domain"
)

func TestNew(t *testing.T) {
	events := make(chan domain.NetworkEvent, 10)
	aud := auditor.New(events, logger.NewNop())
	if aud == nil {
		t.Error("New() returned nil")
	}
}

func TestNew_NilLogger(t *testing.T) {
	events := make(chan domain.NetworkEvent, 10)
	aud := auditor.New(events, nil)
	if aud == nil {
		t.Error("New() returned nil")
	}
}

func TestSetEnabled(t *testing.T) {
	events := make(chan domain.NetworkEvent, 10)
	aud := auditor.New(events, logger.NewNop())

	aud.SetEnabled(false)
	// 验证禁用后不记录事件
	req := &domain.Request{
		ID:     "req1",
		URL:    "https://example.com",
		Method: "GET",
	}
	aud.Record("session1", "target1", req, nil, "passed", nil)

	select {
	case <-events:
		t.Error("event should not be dispatched when disabled")
	case <-time.After(10 * time.Millisecond):
		// 正确：未收到事件
	}
}

func TestRecord_Basic(t *testing.T) {
	events := make(chan domain.NetworkEvent, 10)
	aud := auditor.New(events, logger.NewNop())

	req := &domain.Request{
		ID:           "req1",
		URL:          "https://example.com",
		Method:       "GET",
		Headers:      make(domain.Header),
		ResourceType: "xhr",
	}

	res := &domain.Response{
		StatusCode: 200,
		Headers:    make(domain.Header),
		Body:       []byte("response body"),
	}

	matchedRules := []domain.RuleMatch{
		{RuleID: "rule1", RuleName: "test rule"},
	}

	aud.Record("session1", "target1", req, res, "matched", matchedRules)

	select {
	case evt := <-events:
		if evt.ID != "req1" {
			t.Errorf("got ID %v, want req1", evt.ID)
		}
		if evt.Session != "session1" {
			t.Errorf("got Session %v, want session1", evt.Session)
		}
		if evt.Target != "target1" {
			t.Errorf("got Target %v, want target1", evt.Target)
		}
		if evt.FinalResult != "matched" {
			t.Errorf("got FinalResult %v, want matched", evt.FinalResult)
		}
		if !evt.IsMatched {
			t.Error("IsMatched should be true")
		}
		if len(evt.MatchedRules) != 1 {
			t.Errorf("got %d matched rules, want 1", len(evt.MatchedRules))
		}
		if evt.Request.URL != "https://example.com" {
			t.Errorf("got URL %v, want https://example.com", evt.Request.URL)
		}
		if evt.Response.StatusCode != 200 {
			t.Errorf("got StatusCode %v, want 200", evt.Response.StatusCode)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

func TestRecord_NilRequest(t *testing.T) {
	events := make(chan domain.NetworkEvent, 10)
	aud := auditor.New(events, logger.NewNop())

	aud.Record("session1", "target1", nil, nil, "passed", nil)

	select {
	case <-events:
		t.Error("event should not be dispatched for nil request")
	case <-time.After(10 * time.Millisecond):
		// 正确：未收到事件
	}
}

func TestRecord_NilResponse(t *testing.T) {
	events := make(chan domain.NetworkEvent, 10)
	aud := auditor.New(events, logger.NewNop())

	req := &domain.Request{
		ID:     "req1",
		URL:    "https://example.com",
		Method: "GET",
	}

	aud.Record("session1", "target1", req, nil, "passed", nil)

	select {
	case evt := <-events:
		if evt.ID != "req1" {
			t.Errorf("got ID %v, want req1", evt.ID)
		}
		// 验证 Response 为 nil
		if evt.Response != nil {
			t.Errorf("got Response %v, want nil", evt.Response)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

func TestRecord_NoMatchedRules(t *testing.T) {
	events := make(chan domain.NetworkEvent, 10)
	aud := auditor.New(events, logger.NewNop())

	req := &domain.Request{
		ID:     "req1",
		URL:    "https://example.com",
		Method: "GET",
	}

	aud.Record("session1", "target1", req, nil, "passed", nil)

	select {
	case evt := <-events:
		if evt.IsMatched {
			t.Error("IsMatched should be false")
		}
		if len(evt.MatchedRules) != 0 {
			t.Errorf("got %d matched rules, want 0", len(evt.MatchedRules))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

func TestDispatch_FullChannel(t *testing.T) {
	events := make(chan domain.NetworkEvent, 1)
	aud := auditor.New(events, logger.NewNop())

	req1 := &domain.Request{
		ID:     "req1",
		URL:    "https://example.com",
		Method: "GET",
	}
	req2 := &domain.Request{
		ID:     "req2",
		URL:    "https://example.com",
		Method: "GET",
	}

	// 填满通道
	aud.Record("session1", "target1", req1, nil, "passed", nil)
	// 第二个应该被丢弃
	aud.Record("session1", "target1", req2, nil, "passed", nil)

	// 读取第一个事件
	select {
	case evt := <-events:
		if evt.ID != "req1" {
			t.Errorf("got ID %v, want req1", evt.ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for first event")
	}

	// 确认第二个事件被丢弃
	select {
	case evt := <-events:
		t.Errorf("unexpected event with ID %v", evt.ID)
	case <-time.After(10 * time.Millisecond):
		// 正确：未收到第二个事件
	}
}

func TestDispatch_NilChannel(t *testing.T) {
	aud := auditor.New(nil, logger.NewNop())

	req := &domain.Request{
		ID:     "req1",
		URL:    "https://example.com",
		Method: "GET",
	}

	// 不应该 panic
	aud.Record("session1", "target1", req, nil, "passed", nil)
}

func TestRecord_MultipleEvents(t *testing.T) {
	events := make(chan domain.NetworkEvent, 10)
	aud := auditor.New(events, logger.NewNop())

	for i := 0; i < 3; i++ {
		req := &domain.Request{
			ID:     "req" + string(rune('1'+i)),
			URL:    "https://example.com",
			Method: "GET",
		}
		aud.Record("session1", "target1", req, nil, "passed", nil)
	}

	count := 0
	timeout := time.After(100 * time.Millisecond)
	for count < 3 {
		select {
		case <-events:
			count++
		case <-timeout:
			t.Errorf("got %d events, want 3", count)
			return
		}
	}

	if count != 3 {
		t.Errorf("got %d events, want 3", count)
	}
}
