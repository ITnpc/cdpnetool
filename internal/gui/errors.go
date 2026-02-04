package gui

import (
	"encoding/json"
	"errors"
	"strings"

	"cdpnetool/pkg/domain"
)

// 错误码常量
const (
	CodeSessionNotFound     = "SESSION_NOT_FOUND"
	CodeSessionStartFailed  = "SESSION_START_FAILED"
	CodeNoTargetAttached    = "NO_TARGET_ATTACHED"
	CodeTargetNotFound      = "TARGET_NOT_FOUND"
	CodeDevToolsUnreachable = "DEVTOOLS_UNREACHABLE"
	CodeNetworkError        = "NETWORK_ERROR"
	CodeInvalidConfig       = "INVALID_CONFIG"
	CodeConfigNotFound      = "CONFIG_NOT_FOUND"
	CodeBrowserNotRunning   = "BROWSER_NOT_RUNNING"
	CodeBrowserStartFailed  = "BROWSER_START_FAILED"
	CodeDatabaseError       = "DATABASE_ERROR"
	CodeUnknown             = "UNKNOWN_ERROR"
)

// 错误映射表（仅返回错误码，前端根据错误码进行国际化）
var errorMappings = map[error]string{
	domain.ErrSessionNotFound:        CodeSessionNotFound,
	domain.ErrDevToolsUnreachable:    CodeDevToolsUnreachable,
	domain.ErrNoTargetAttached:       CodeNoTargetAttached,
	domain.ErrBrowserNotRunning:      CodeBrowserNotRunning,
	domain.ErrBrowserStartFailed:     CodeBrowserStartFailed,
	domain.ErrInvalidConfig:          CodeInvalidConfig,
	domain.ErrConfigNotFound:         CodeConfigNotFound,
	domain.ErrDatabaseNotInitialized: CodeDatabaseError,
}

// translateError 将领域错误转换为错误码（前端根据错误码进行国际化）
func (a *App) translateError(err error) (code, message string) {
	if err == nil {
		return "", ""
	}

	// 尝试匹配已知的领域错误
	for domainErr, errorCode := range errorMappings {
		if errors.Is(err, domainErr) {
			a.log.Err(err, "业务错误", "code", errorCode)
			return errorCode, ""
		}
	}

	// 处理网络相关错误
	errStr := err.Error()
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "dial tcp") ||
		strings.Contains(errStr, "websocket: bad handshake") {
		a.log.Err(err, "网络连接错误")
		return CodeNetworkError, ""
	}

	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") {
		a.log.Err(err, "网络超时")
		return CodeNetworkError, ""
	}

	// 处理 JSON 解析错误
	var jsonErr *json.SyntaxError
	if errors.As(err, &jsonErr) {
		a.log.Err(err, "JSON解析错误")
		return CodeInvalidConfig, ""
	}

	// 未知错误
	a.log.Err(err, "未知错误")
	return CodeUnknown, err.Error()
}
