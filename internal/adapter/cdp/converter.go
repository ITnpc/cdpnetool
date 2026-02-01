package cdp

import (
	"encoding/json"
	"strings"

	"cdpnetool/pkg/traffic"

	"github.com/mafredri/cdp/protocol/fetch"
)

// ToNeutralRequest 将 CDP 事件转换为中立 Request 模型
func ToNeutralRequest(ev *fetch.RequestPausedReply) *traffic.Request {
	req := traffic.NewRequest()
	req.ID = string(ev.RequestID)
	req.URL = ev.Request.URL
	req.Method = ev.Request.Method
	req.ResourceType = string(ev.ResourceType)

	// 处理 Header
	var headers map[string]string
	if len(ev.Request.Headers) > 0 {
		if err := json.Unmarshal(ev.Request.Headers, &headers); err == nil {
			for k, v := range headers {
				req.Headers.Set(k, v)
			}
		}
	}

	// 解析 Query 参数
	if idx := strings.Index(req.URL, "?"); idx != -1 {
		queryStr := req.URL[idx+1:]
		if queryStr != "" {
			for _, pair := range strings.Split(queryStr, "&") {
				if kv := strings.SplitN(pair, "=", 2); len(kv) == 2 {
					req.Query[strings.ToLower(kv[0])] = kv[1]
				}
			}
		}
	}

	// 解析 Cookie
	if cookieHeader := req.Headers.Get("cookie"); cookieHeader != "" {
		for _, pair := range strings.Split(cookieHeader, ";") {
			pair = strings.TrimSpace(pair)
			if kv := strings.SplitN(pair, "=", 2); len(kv) == 2 {
				req.Cookies[strings.ToLower(kv[0])] = kv[1]
			}
		}
	}

	return req
}

// ToNeutralResponse 将 CDP 事件转换为中立 Response 模型
func ToNeutralResponse(ev *fetch.RequestPausedReply, body []byte) *traffic.Response {
	res := traffic.NewResponse()
	if ev.ResponseStatusCode != nil {
		res.StatusCode = *ev.ResponseStatusCode
	}
	for _, h := range ev.ResponseHeaders {
		res.Headers.Set(h.Name, h.Value)
	}
	res.Body = body
	return res
}

// ToHeaderEntries 将中立 Header 转换为 CDP Header 条目
func ToHeaderEntries(h traffic.Header) []fetch.HeaderEntry {
	entries := make([]fetch.HeaderEntry, 0, len(h))
	for k, v := range h {
		entries = append(entries, fetch.HeaderEntry{Name: k, Value: v})
	}
	return entries
}
