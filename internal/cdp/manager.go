package cdp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	ilog "cdpnetool/internal/log"
	"cdpnetool/internal/rules"
	"cdpnetool/pkg/model"

	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/devtool"
	"github.com/mafredri/cdp/protocol/fetch"
	"github.com/mafredri/cdp/protocol/network"
	"github.com/mafredri/cdp/rpcc"
)

type Manager struct {
	devtoolsURL       string
	conn              *rpcc.Conn
	client            *cdp.Client
	ctx               context.Context
	cancel            context.CancelFunc
	events            chan model.Event
	pending           chan any
	engine            *rules.Engine
	approvals         map[string]chan model.Rewrite
	workers           int
	bodySizeThreshold int64
	processTimeoutMS  int
}

func New(devtoolsURL string, events chan model.Event, pending chan any) *Manager {
	return &Manager{devtoolsURL: devtoolsURL, events: events, pending: pending, approvals: make(map[string]chan model.Rewrite)}
}

func (m *Manager) AttachTarget(target model.TargetID) error {
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	m.cancel = cancel
	dt := devtool.New(m.devtoolsURL)
	targets, err := dt.List(ctx)
	if err != nil {
		return err
	}
	var sel *devtool.Target
	for i := range targets {
		if string(targets[i].ID) == string(target) || target == "" {
			sel = targets[i]
			if target == "" {
				break
			}
		}
	}
	if sel == nil {
		return fmt.Errorf("no target")
	}
	conn, err := rpcc.DialContext(ctx, sel.WebSocketDebuggerURL)
	if err != nil {
		return err
	}
	m.conn = conn
	m.client = cdp.NewClient(conn)
	return nil
}

func (m *Manager) Detach() error {
	if m.cancel != nil {
		m.cancel()
	}
	if m.conn != nil {
		return m.conn.Close()
	}
	return nil
}

func (m *Manager) Enable() error {
	if m.client == nil {
		return fmt.Errorf("not attached")
	}
	err := m.client.Network.Enable(m.ctx, nil)
	if err != nil {
		return err
	}
	p := "*"
	patterns := []fetch.RequestPattern{
		{URLPattern: &p, RequestStage: fetch.RequestStageRequest},
		{URLPattern: &p, RequestStage: fetch.RequestStageResponse},
	}
	err = m.client.Fetch.Enable(m.ctx, &fetch.EnableArgs{Patterns: patterns})
	if err != nil {
		return err
	}
	go m.consume()
	return nil
}

func (m *Manager) Disable() error {
	if m.client == nil {
		return fmt.Errorf("not attached")
	}
	return m.client.Fetch.Disable(m.ctx)
}

func (m *Manager) consume() {
	rp, err := m.client.Fetch.RequestPaused(m.ctx)
	if err != nil {
		return
	}
	defer rp.Close()
	var sem chan struct{}
	if m.workers > 0 {
		sem = make(chan struct{}, m.workers)
	}
	for {
		ev, err := rp.Recv()
		if err != nil {
			return
		}
		if sem != nil {
			sem <- struct{}{}
			go func(e *fetch.RequestPausedReply) {
				defer func() { <-sem }()
				m.handle(e)
			}(ev)
		} else {
			go m.handle(ev)
		}
	}
}

func (m *Manager) handle(ev *fetch.RequestPausedReply) {
	to := m.processTimeoutMS
	if to <= 0 {
		to = 3000
	}
	ctx, cancel := context.WithTimeout(m.ctx, time.Duration(to)*time.Millisecond)
	defer cancel()
	start := time.Now()
	m.events <- model.Event{Type: "intercepted"}
	stg := "request"
	if ev.ResponseStatusCode != nil {
		stg = "response"
	}
	res := m.decide(ev, stg)
	if res == nil || res.Action == nil {
		m.applyContinue(ctx, ev, stg)
		return
	}
	a := res.Action
	if a.DelayMS > 0 {
		time.Sleep(time.Duration(a.DelayMS) * time.Millisecond)
	}
	if time.Since(start) > time.Duration(to)*time.Millisecond {
		m.applyContinue(ctx, ev, stg)
		m.events <- model.Event{Type: "degraded"}
		return
	}
	if a.Pause != nil {
		m.applyPause(ctx, ev, a.Pause, stg)
		return
	}
	if a.Fail != nil {
		m.applyFail(ctx, ev, a.Fail)
		m.events <- model.Event{Type: "failed", Rule: res.RuleID}
		return
	}
	if a.Respond != nil {
		m.applyRespond(ctx, ev, a.Respond, stg)
		m.events <- model.Event{Type: "fulfilled", Rule: res.RuleID}
		return
	}
	if a.Rewrite != nil {
		m.applyRewrite(ctx, ev, a.Rewrite, stg)
		m.events <- model.Event{Type: "mutated", Rule: res.RuleID}
		return
	}
	m.applyContinue(ctx, ev, stg)
}

func (m *Manager) decide(ev *fetch.RequestPausedReply, stage string) *rules.Result {
	if m.engine == nil {
		return nil
	}
	h := map[string]string{}
	q := map[string]string{}
	ck := map[string]string{}
	var bodyText string
	var ctype string
	if stage == "response" {
		if len(ev.ResponseHeaders) > 0 {
			for i := range ev.ResponseHeaders {
				k := ev.ResponseHeaders[i].Name
				v := ev.ResponseHeaders[i].Value
				h[strings.ToLower(k)] = v
				if strings.EqualFold(k, "set-cookie") {
					name, val := parseSetCookie(v)
					if name != "" {
						ck[strings.ToLower(name)] = val
					}
				}
				if strings.EqualFold(k, "content-type") {
					ctype = v
				}
			}
		}
		var clen int64
		if v, ok := h["content-length"]; ok {
			if n, err := parseInt64(v); err == nil {
				clen = n
			}
		}
		if shouldGetBody(ctype, clen, m.bodySizeThreshold) {
			ctx2, cancel := context.WithTimeout(m.ctx, 500*time.Millisecond)
			defer cancel()
			rb, err := m.client.Fetch.GetResponseBody(ctx2, &fetch.GetResponseBodyArgs{RequestID: ev.RequestID})
			if err == nil && rb != nil {
				if rb.Base64Encoded {
					if b, err := base64.StdEncoding.DecodeString(rb.Body); err == nil {
						bodyText = string(b)
					}
				} else {
					bodyText = rb.Body
				}
			}
		}
	} else {
		_ = json.Unmarshal(ev.Request.Headers, &h)
		if len(h) > 0 {
			m2 := make(map[string]string, len(h))
			for k, v := range h {
				m2[strings.ToLower(k)] = v
			}
			h = m2
		}
		if ev.Request.URL != "" {
			if u, err := url.Parse(ev.Request.URL); err == nil {
				for key, vals := range u.Query() {
					if len(vals) > 0 {
						q[strings.ToLower(key)] = vals[0]
					}
				}
			}
		}
		if v, ok := h["cookie"]; ok {
			for name, val := range parseCookie(v) {
				ck[strings.ToLower(name)] = val
			}
		}
		if v, ok := h["content-type"]; ok {
			ctype = v
		}
		if ev.Request.PostData != nil {
			bodyText = *ev.Request.PostData
		}
	}
	res := m.engine.Eval(rules.Ctx{URL: ev.Request.URL, Method: ev.Request.Method, Headers: h, Query: q, Cookies: ck, Body: bodyText, ContentType: ctype, Stage: stage})
	if res == nil {
		return nil
	}
	return res
}

func parseCookie(s string) map[string]string {
	out := make(map[string]string)
	parts := strings.Split(s, ";")
	for _, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) == 2 {
			out[kv[0]] = kv[1]
		}
	}
	return out
}

func parseSetCookie(s string) (string, string) {
	// CookieName=CookieValue; Attr=...
	p := strings.SplitN(s, ";", 2)
	first := strings.TrimSpace(p[0])
	kv := strings.SplitN(first, "=", 2)
	if len(kv) == 2 {
		return kv[0], kv[1]
	}
	return "", ""
}

func urlParse(raw string, qpatch map[string]*string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for k, v := range qpatch {
		if v == nil {
			q.Del(k)
		} else {
			q.Set(k, *v)
		}
	}
	u.RawQuery = q.Encode()
	return u, nil
}

func shouldGetBody(ctype string, clen int64, thr int64) bool {
	if thr <= 0 {
		thr = 4 * 1024 * 1024
	}
	if clen > 0 && clen > thr {
		return false
	}
	lc := strings.ToLower(ctype)
	if strings.HasPrefix(lc, "text/") {
		return true
	}
	if strings.HasPrefix(lc, "application/json") {
		return true
	}
	return false
}

func parseInt64(s string) (int64, error) {
	var n int64
	var mul int64 = 1
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid")
		}
		n = n*10 + int64(c-'0')
	}
	return n * mul, nil
}

func (m *Manager) applyContinue(ctx context.Context, ev *fetch.RequestPausedReply, stage string) {
	if stage == "response" {
		m.client.Fetch.ContinueResponse(ctx, &fetch.ContinueResponseArgs{RequestID: ev.RequestID})
		ilog.L().Debug("continue_response")
	} else {
		m.client.Fetch.ContinueRequest(ctx, &fetch.ContinueRequestArgs{RequestID: ev.RequestID})
		ilog.L().Debug("continue_request")
	}
}

func (m *Manager) applyFail(ctx context.Context, ev *fetch.RequestPausedReply, f *model.Fail) {
	m.client.Fetch.FailRequest(ctx, &fetch.FailRequestArgs{RequestID: ev.RequestID, ErrorReason: network.ErrorReasonFailed})
}

func (m *Manager) applyRespond(ctx context.Context, ev *fetch.RequestPausedReply, r *model.Respond, stage string) {
	if stage == "response" && len(r.Body) == 0 {
		// 仅修改响应码/头，继续响应
		args := &fetch.ContinueResponseArgs{RequestID: ev.RequestID}
		if r.Status != 0 {
			args.ResponseCode = &r.Status
		}
		if len(r.Headers) > 0 {
			args.ResponseHeaders = toHeaderEntries(r.Headers)
		}
		m.client.Fetch.ContinueResponse(ctx, args)
		return
	}
	// fulfill 完整响应
	args := &fetch.FulfillRequestArgs{RequestID: ev.RequestID, ResponseCode: r.Status}
	if len(r.Headers) > 0 {
		args.ResponseHeaders = toHeaderEntries(r.Headers)
	}
	if len(r.Body) > 0 {
		args.Body = r.Body
	}
	m.client.Fetch.FulfillRequest(ctx, args)
}

func (m *Manager) applyRewrite(ctx context.Context, ev *fetch.RequestPausedReply, rw *model.Rewrite, stage string) {
	var url, method *string
	if rw.URL != nil {
		url = rw.URL
	}
	if rw.Method != nil {
		method = rw.Method
	}
	var hdrs []fetch.HeaderEntry
	if rw.Headers != nil {
		for k, v := range rw.Headers {
			if v != nil {
				hdrs = append(hdrs, fetch.HeaderEntry{Name: k, Value: *v})
			}
		}
	}
	if stage == "response" {
		// mutate response headers by patching existing headers then continue response
		if rw.Headers != nil {
			cur := make(map[string]string, len(ev.ResponseHeaders))
			for i := range ev.ResponseHeaders {
				cur[strings.ToLower(ev.ResponseHeaders[i].Name)] = ev.ResponseHeaders[i].Value
			}
			for k, v := range rw.Headers {
				lk := strings.ToLower(k)
				if v == nil {
					delete(cur, lk)
				} else {
					cur[lk] = *v
				}
			}
			var out []fetch.HeaderEntry
			for k, v := range cur {
				out = append(out, fetch.HeaderEntry{Name: k, Value: v})
			}
			m.client.Fetch.ContinueResponse(ctx, &fetch.ContinueResponseArgs{RequestID: ev.RequestID, ResponseHeaders: out})
			return
		}
		m.client.Fetch.ContinueResponse(ctx, &fetch.ContinueResponseArgs{RequestID: ev.RequestID})
		return
	}
	if rw.Cookies != nil {
		h := map[string]string{}
		_ = json.Unmarshal(ev.Request.Headers, &h)
		var cookie string
		for k, v := range h {
			if strings.EqualFold(k, "cookie") {
				cookie = v
				break
			}
		}
		cm := parseCookie(cookie)
		for name, val := range rw.Cookies {
			if val == nil {
				delete(cm, name)
			} else {
				cm[name] = *val
			}
		}
		if len(cm) > 0 {
			var b strings.Builder
			first := true
			for k, v := range cm {
				if !first {
					b.WriteString("; ")
				}
				first = false
				b.WriteString(k)
				b.WriteString("=")
				b.WriteString(v)
			}
			hdrs = append(hdrs, fetch.HeaderEntry{Name: "Cookie", Value: b.String()})
		}
	}
	var post []byte
	if rw.Body != nil {
		switch rw.Body.Type {
		case "base64":
			if len(rw.Body.Ops) > 0 {
				if s, ok := rw.Body.Ops[0].(string); ok {
					b, err := base64.StdEncoding.DecodeString(s)
					if err == nil {
						post = b
					}
				}
			}
		case "text_regex":
			if ev.Request.PostData != nil {
				src := *ev.Request.PostData
				if len(rw.Body.Ops) >= 2 {
					p, pOk := rw.Body.Ops[0].(string)
					r, rOk := rw.Body.Ops[1].(string)
					if pOk && rOk {
						re, err := regexp.Compile(p)
						if err == nil {
							post = []byte(re.ReplaceAllString(src, r))
						}
					}
				}
			}
		case "json_patch":
			var src string
			if ev.Request.PostData != nil {
				src = *ev.Request.PostData
			}
			if out, ok := applyJSONPatch(src, rw.Body.Ops); ok {
				post = []byte(out)
			}
		}
	}
	args := &fetch.ContinueRequestArgs{RequestID: ev.RequestID, URL: url, Method: method, Headers: hdrs}
	if rw.Query != nil && url == nil {
		if u, err := urlParse(ev.Request.URL, rw.Query); err == nil {
			us := u.String()
			args.URL = &us
		}
	}
	if len(post) > 0 {
		args.PostData = post
	}
	m.client.Fetch.ContinueRequest(ctx, args)
}

func applyJSONPatch(doc string, ops []any) (string, bool) {
	var v any
	if doc == "" {
		v = make(map[string]any)
	} else {
		if err := json.Unmarshal([]byte(doc), &v); err != nil {
			return "", false
		}
	}
	for _, op := range ops {
		m, ok := op.(map[string]any)
		if !ok {
			continue
		}
		typ, _ := m["op"].(string)
		path, _ := m["path"].(string)
		val := m["value"]
		switch typ {
		case "add", "replace":
			v = setByPtr(v, path, val, typ == "replace")
		case "remove":
			v = removeByPtr(v, path)
		}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", false
	}
	return string(b), true
}

func setByPtr(cur any, ptr string, val any, replace bool) any {
	if ptr == "" || ptr[0] != '/' {
		return cur
	}
	tokens := splitPtr(ptr)
	return setRec(cur, tokens, val)
}

func setRec(cur any, tokens []string, val any) any {
	if len(tokens) == 0 {
		return val
	}
	t := tokens[0]
	switch c := cur.(type) {
	case map[string]any:
		child, ok := c[t]
		if !ok {
			child = make(map[string]any)
		}
		c[t] = setRec(child, tokens[1:], val)
		return c
	case []any:
		idx, ok := toIndex(t)
		if !ok || idx < 0 || idx >= len(c) {
			return c
		}
		c[idx] = setRec(c[idx], tokens[1:], val)
		return c
	default:
		if len(tokens) == 1 {
			return val
		}
		return cur
	}
}

func removeByPtr(cur any, ptr string) any {
	if ptr == "" || ptr[0] != '/' {
		return cur
	}
	tokens := splitPtr(ptr)
	return removeRec(cur, tokens)
}

func removeRec(cur any, tokens []string) any {
	if len(tokens) == 0 {
		return cur
	}
	t := tokens[0]
	switch c := cur.(type) {
	case map[string]any:
		if len(tokens) == 1 {
			delete(c, t)
			return c
		}
		child, ok := c[t]
		if !ok {
			return c
		}
		c[t] = removeRec(child, tokens[1:])
		return c
	case []any:
		idx, ok := toIndex(t)
		if !ok || idx < 0 || idx >= len(c) {
			return c
		}
		if len(tokens) == 1 {
			nc := append(c[:idx], c[idx+1:]...)
			return nc
		}
		c[idx] = removeRec(c[idx], tokens[1:])
		return c
	default:
		return cur
	}
}

func splitPtr(p string) []string {
	var out []string
	i := 1
	for i < len(p) {
		j := i
		for j < len(p) && p[j] != '/' {
			j++
		}
		tok := p[i:j]
		tok = strings.ReplaceAll(tok, "~1", "/")
		tok = strings.ReplaceAll(tok, "~0", "~")
		out = append(out, tok)
		i = j + 1
	}
	return out
}

func toIndex(s string) (int, bool) {
	n := 0
	if len(s) == 0 {
		return 0, false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

func toHeaderEntries(h map[string]string) []fetch.HeaderEntry {
	out := make([]fetch.HeaderEntry, 0, len(h))
	for k, v := range h {
		out = append(out, fetch.HeaderEntry{Name: k, Value: v})
	}
	return out
}

func (m *Manager) applyPause(ctx context.Context, ev *fetch.RequestPausedReply, p *model.Pause, stage string) {
	id := string(ev.RequestID)
	ch := make(chan model.Rewrite, 1)
	m.approvals[id] = ch
	if m.pending != nil {
		select {
		case m.pending <- struct{ ID string }{ID: id}:
		default:
			switch p.DefaultAction.Type {
			case "fulfill":
				m.applyRespond(ctx, ev, &model.Respond{Status: p.DefaultAction.Status}, stage)
			case "fail":
				m.applyFail(ctx, ev, &model.Fail{Reason: p.DefaultAction.Reason})
			case "continue_mutated":
				m.applyContinue(ctx, ev, stage)
			default:
				m.applyContinue(ctx, ev, stage)
			}
			m.events <- model.Event{Type: "degraded"}
			delete(m.approvals, id)
			return
		}
	}
	t := time.NewTimer(time.Duration(p.TimeoutMS) * time.Millisecond)
	select {
	case mut := <-ch:
		_ = mut
		m.applyContinue(ctx, ev, stage)
	case <-t.C:
		switch p.DefaultAction.Type {
		case "fulfill":
			m.applyRespond(ctx, ev, &model.Respond{Status: p.DefaultAction.Status}, stage)
		case "fail":
			m.applyFail(ctx, ev, &model.Fail{Reason: p.DefaultAction.Reason})
		case "continue_mutated":
			m.applyContinue(ctx, ev, stage)
		default:
			m.applyContinue(ctx, ev, stage)
		}
	}
	delete(m.approvals, id)
}

func (m *Manager) SetRules(rs model.RuleSet) { m.engine = rules.New(rs) }

func (m *Manager) UpdateRules(rs model.RuleSet) {
	if m.engine == nil {
		m.engine = rules.New(rs)
	} else {
		m.engine.Update(rs)
	}
}

func (m *Manager) Approve(itemID string, mutations model.Rewrite) {
	if ch, ok := m.approvals[itemID]; ok {
		ch <- mutations
	}
}

func (m *Manager) SetConcurrency(n int) { m.workers = n }

func (m *Manager) SetRuntime(bodySizeThreshold int64, processTimeoutMS int) {
	m.bodySizeThreshold = bodySizeThreshold
	m.processTimeoutMS = processTimeoutMS
}
