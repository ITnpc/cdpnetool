package logger

import (
	"cdpnetool/internal/config"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogLevel 定义日志级别
type LogLevel int

const (
	// LogLevelDebug 调试级别
	LogLevelDebug LogLevel = iota

	// LogLevelInfo 信息级别
	LogLevelInfo

	// LogLevelWarn 警告级别
	LogLevelWarn

	// LogLevelError 错误级别
	LogLevelError

	// LogLevelNone 禁用日志
	LogLevelNone
)

// String 返回日志级别的字符串表示
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	case LogLevelNone:
		return "NONE"
	default:
		return "UNKNOWN"
	}
}

// Logger 定义日志接口
type Logger interface {
	// Debug 记录调试信息
	Debug(format string, args ...any)

	// Info 记录一般信息
	Info(format string, args ...any)

	// Warn 记录警告信息
	Warn(format string, args ...any)

	// Error 记录错误信息
	Error(format string, args ...any)

	// Err 记录错误信息
	Err(err error, msg string, fields ...any)
}

// DefaultLogger 默认日志实现
type DefaultLogger struct {
	level  LogLevel
	logger *log.Logger
}

// NewDefaultLogger 创建默认日志记录器
func NewDefaultLogger(cfg *config.Config) *DefaultLogger {
	// 初始化日志级别
	logLevel := LogLevelDebug
	switch cfg.Log.Level {
	case "info":
		logLevel = LogLevelInfo
	case "warn":
		logLevel = LogLevelWarn
	case "error":
		logLevel = LogLevelError
	}

	// 根据配置添加写入器
	writers := make([]io.Writer, 0)
	for _, writer := range cfg.Log.Writer {
		switch writer {
		case "console":
			writers = append(writers, os.Stderr)
		case "file":
			filename, _ := getLogPath()
			writers = append(writers, &lumberjack.Logger{
				Filename:   filename,
				MaxSize:    1,
				MaxAge:     30,
				MaxBackups: 3,
				LocalTime:  true,
				Compress:   false,
			})
		}
	}

	if len(writers) == 0 {
		return &DefaultLogger{
			level:  LogLevelNone,
			logger: log.New(io.Discard, "", 0),
		}
	}

	multiWriter := io.MultiWriter(writers...)

	return &DefaultLogger{
		level:  logLevel,
		logger: log.New(multiWriter, "", 0), // 不使用标准库的前缀,我们自己格式化
	}
}

// Debug 记录调试信息
func (l *DefaultLogger) Debug(format string, args ...any) {
	if l.level <= LogLevelDebug {
		l.log(LogLevelDebug, format, args...)
	}
}

// Info 记录一般信息
func (l *DefaultLogger) Info(format string, args ...any) {
	if l.level <= LogLevelInfo {
		l.log(LogLevelInfo, format, args...)
	}
}

// Warn 记录警告信息
func (l *DefaultLogger) Warn(format string, args ...any) {
	if l.level <= LogLevelWarn {
		l.log(LogLevelWarn, format, args...)
	}
}

// Error 记录错误信息
func (l *DefaultLogger) Error(format string, args ...any) {
	if l.level <= LogLevelError {
		l.log(LogLevelError, format, args...)
	}
}

// Err 记录错误信息
func (l *DefaultLogger) Err(err error, msg string, fields ...any) {
	if l.level <= LogLevelError {
		l.log(LogLevelError, fmt.Sprintf("%s: %s", msg, err), fields...)
	}
}

// log 内部日志方法
func (l *DefaultLogger) log(level LogLevel, message string, args ...any) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")

	if len(args)%2 != 0 {
		args = append(args, "MISSING")
	}

	// 添加键值对
	var others strings.Builder
	for i := 0; i < len(args); i += 2 {
		key := fmt.Sprintf("%v", args[i])
		value := args[i+1]
		fmt.Fprintf(&others, " %s=%v", key, value)
	}

	l.logger.Printf("[%s] [%s] \"%s\" %s", timestamp, level.String(), message, others.String())
}

// NoopLogger 空日志实现,不输出任何日志
type NoopLogger struct{}

// NewNoopLogger 创建空日志记录器
func NewNoopLogger() *NoopLogger {
	return &NoopLogger{}
}

// Debug 不执行任何操作
func (l *NoopLogger) Debug(format string, args ...any) {}

// Info 不执行任何操作
func (l *NoopLogger) Info(format string, args ...any) {}

// Warn 不执行任何操作
func (l *NoopLogger) Warn(format string, args ...any) {}

// Error 不执行任何操作
func (l *NoopLogger) Error(format string, args ...any) {}

// Err 记录错误信息
func (l *NoopLogger) Err(err error, msg string, fields ...any) {}

// ZeroLogger 日志组件
type ZeroLogger struct {
	logger   zerolog.Logger
	logLevel zerolog.Level
}

// NewZeroLogger 创建日志组件
func NewZeroLogger(cfg *config.Config) *ZeroLogger {
	if cfg == nil {
		return &ZeroLogger{
			logger:   zerolog.Nop(),
			logLevel: zerolog.Disabled,
		}
	}

	// 初始化日志级别
	logLevel := zerolog.DebugLevel
	switch cfg.Log.Level {
	case "info":
		logLevel = zerolog.InfoLevel
	case "warn":
		logLevel = zerolog.WarnLevel
	case "error":
		logLevel = zerolog.ErrorLevel
	}

	// 根据配置添加写入器
	writers := make([]io.Writer, 0)
	for _, writer := range cfg.Log.Writer {
		switch writer {
		case "console":
			writers = append(writers, os.Stderr)
		case "file":
			filename, _ := getLogPath()
			writers = append(writers, &lumberjack.Logger{
				Filename:   filename,
				MaxSize:    1,
				MaxAge:     30,
				MaxBackups: 3,
				LocalTime:  true,
				Compress:   false,
			})
		}
	}

	if len(writers) == 0 {
		return Nop()
	}

	multiWriter := io.MultiWriter(writers...)
	zerolog.TimeFieldFormat = "2006-01-02 15:04:05"
	logger := zerolog.New(multiWriter).
		With().
		Caller().
		Timestamp().
		Logger().
		Level(logLevel)

	return &ZeroLogger{logger: logger, logLevel: logLevel}
}

// Nop 创建一个空的日志记录器
func Nop() *ZeroLogger { return &ZeroLogger{logger: zerolog.Nop()} }

// Info 记录信息
func (z *ZeroLogger) Info(msg string, fields ...any) {
	z.logger.Info().CallerSkipFrame(1).Fields(fields).Msg(msg)
}

// Error 记录错误
func (z *ZeroLogger) Error(msg string, fields ...any) {
	z.logger.Error().CallerSkipFrame(1).Fields(fields).Msg(msg)
}

// Debug 记录调试信息
func (z *ZeroLogger) Debug(msg string, fields ...any) {
	z.logger.Debug().CallerSkipFrame(1).Fields(fields).Msg(msg)
}

// Warn 记录警告
func (z *ZeroLogger) Warn(msg string, fields ...any) {
	z.logger.Warn().CallerSkipFrame(1).Fields(fields).Msg(msg)
}

// Err 记录错误信息
func (z *ZeroLogger) Err(err error, msg string, fields ...any) {
	z.logger.Err(err).CallerSkipFrame(1).Fields(fields).Msg(msg)
}

// getLogPath 获取日志目录
func getLogPath() (string, error) {
	var baseDir string

	switch runtime.GOOS {
	case "windows":
		// %APPDATA%/cdpnetool/data.db
		baseDir = os.Getenv("APPDATA")
		if baseDir == "" {
			baseDir = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
	case "darwin":
		// ~/Library/Application Support/cdpnetool/data.db
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		baseDir = filepath.Join(home, "Library", "Application Support")
	default:
		// Linux: ~/.local/share/cdpnetool/data.db
		baseDir = os.Getenv("XDG_DATA_HOME")
		if baseDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			baseDir = filepath.Join(home, ".local", "share")
		}
	}

	return filepath.Join(baseDir, "cdpnetool", "logs", "app.log"), nil
}
