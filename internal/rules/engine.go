package rules

import (
	"strings"

	"cdpnetool/pkg/model"
)

type Engine struct {
	rs model.RuleSet
}

func New(rs model.RuleSet) *Engine { return &Engine{rs: rs} }

func (e *Engine) Update(rs model.RuleSet) { e.rs = rs }

type Ctx struct {
	URL     string
	Method  string
	Headers map[string]string
	Query   map[string]string
	Cookies map[string]string
	Stage   string
}

type Result struct {
	RuleID *model.RuleID
	Action *model.Action
}

func (e *Engine) Eval(ctx Ctx) *Result {
	if len(e.rs.Rules) == 0 {
		return nil
	}
	var chosen *model.Rule
	for i := range e.rs.Rules {
		r := &e.rs.Rules[i]
		if matchRule(ctx, r.Match) {
			if chosen == nil || r.Priority > chosen.Priority {
				chosen = r
				if r.Mode == "short_circuit" {
					break
				}
			}
		}
	}
	if chosen == nil {
		return nil
	}
	rid := chosen.ID
	return &Result{RuleID: &rid, Action: &chosen.Action}
}

func matchRule(ctx Ctx, m model.Match) bool {
	ok := true
	if len(m.AllOf) > 0 {
		ok = ok && allOf(ctx, m.AllOf)
	}
	if len(m.AnyOf) > 0 {
		ok = ok && anyOf(ctx, m.AnyOf)
	}
	if len(m.NoneOf) > 0 {
		ok = ok && noneOf(ctx, m.NoneOf)
	}
	return ok
}

func allOf(ctx Ctx, cs []model.Condition) bool {
	for i := range cs {
		if !cond(ctx, cs[i]) {
			return false
		}
	}
	return true
}

func anyOf(ctx Ctx, cs []model.Condition) bool {
	for i := range cs {
		if cond(ctx, cs[i]) {
			return true
		}
	}
	return false
}

func noneOf(ctx Ctx, cs []model.Condition) bool { return !anyOf(ctx, cs) }

func cond(ctx Ctx, c model.Condition) bool {
	switch c.Type {
	case "url":
		switch c.Mode {
		case "prefix":
			return strings.HasPrefix(ctx.URL, c.Pattern)
		case "regex":
			return matchRegex(ctx.URL, c.Pattern)
		case "exact":
			return ctx.URL == c.Pattern
		default:
			return glob(ctx.URL, c.Pattern)
		}
	case "method":
		for _, v := range c.Values {
			if strings.EqualFold(ctx.Method, v) {
				return true
			}
		}
		return false
	case "header":
		v, ok := ctx.Headers[c.Key]
		if !ok {
			return false
		}
		switch c.Op {
		case "equals":
			return v == c.Value
		case "contains":
			return strings.Contains(v, c.Value)
		case "regex":
			return matchRegex(v, c.Value)
		default:
			return true
		}
	case "query":
		v, ok := ctx.Query[c.Key]
		if !ok {
			return false
		}
		switch c.Op {
		case "equals":
			return v == c.Value
		case "contains":
			return strings.Contains(v, c.Value)
		case "regex":
			return matchRegex(v, c.Value)
		default:
			return true
		}
	case "cookie":
		v, ok := ctx.Cookies[c.Key]
		if !ok {
			return false
		}
		switch c.Op {
		case "equals":
			return v == c.Value
		case "contains":
			return strings.Contains(v, c.Value)
		case "regex":
			return matchRegex(v, c.Value)
		default:
			return true
		}
	default:
		return false
	}
}

func matchRegex(s, pattern string) bool {
	re, err := regexCache.Get(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(s)
}

func glob(s, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(s, strings.TrimPrefix(pattern, "*")) {
		return true
	}
	if strings.HasSuffix(pattern, "*") && strings.HasPrefix(s, strings.TrimSuffix(pattern, "*")) {
		return true
	}
	return s == pattern
}
