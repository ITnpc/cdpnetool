package traffic

import (
	"net/http"
	"strings"
)

// Header 封装通用的头部操作
type Header map[string]string

// Get 获取指定 Header 的值（大小写不敏感）
func (h Header) Get(key string) string {
	if h == nil {
		return ""
	}
	return h[strings.ToLower(key)]
}

// Set 设置指定 Header 的值（自动转换为小写）
func (h Header) Set(key, value string) {
	h[strings.ToLower(key)] = value
}

// Del 删除指定 Header
func (h Header) Del(key string) {
	delete(h, strings.ToLower(key))
}

// Request 中立的请求模型
type Request struct {
	ID           string            // 事务唯一ID
	URL          string            // 完整URL
	Method       string            // HTTP方法
	Headers      Header            // 请求头
	Body         []byte            // 请求体原始数据
	ResourceType string            // 资源类型 (如 Document, XHR)
	Query        map[string]string // 预解析的查询参数
	Cookies      map[string]string // 预解析的Cookie
}

// Response 中立的响应模型
type Response struct {
	StatusCode int    // 状态码
	Headers    Header // 响应头
	Body       []byte // 响应体数据
}

// NewRequest 创建初始化请求对象
func NewRequest() *Request {
	return &Request{
		Headers: make(Header),
		Query:   make(map[string]string),
		Cookies: make(map[string]string),
	}
}

// NewResponse 创建初始化响应对象
func NewResponse() *Response {
	return &Response{
		StatusCode: http.StatusOK,
		Headers:    make(Header),
	}
}
