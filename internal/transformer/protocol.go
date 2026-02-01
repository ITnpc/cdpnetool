package transformer

import (
	"strings"
)

// ParseCookies 解析 Cookie 字符串为映射
func ParseCookies(cookieStr string) map[string]string {
	cookies := make(map[string]string)
	if cookieStr == "" {
		return cookies
	}

	parts := strings.Split(cookieStr, ";")
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			cookies[kv[0]] = kv[1]
		}
	}
	return cookies
}

// BuildCookieString 将映射重新构建为 Cookie 字符串
func BuildCookieString(cookies map[string]string) string {
	if len(cookies) == 0 {
		return ""
	}
	var parts []string
	for k, v := range cookies {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, "; ")
}

// IsBinaryContentType 判断是否为二进制内容类型
func IsBinaryContentType(contentType string) bool {
	ct := strings.ToLower(contentType)
	binaryPrefixes := []string{"image/", "video/", "audio/", "application/octet-stream", "font/"}
	for _, prefix := range binaryPrefixes {
		if strings.HasPrefix(ct, prefix) {
			return true
		}
	}
	return false
}
