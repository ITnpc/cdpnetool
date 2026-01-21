package db

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Options 数据库配置选项
type Options struct {
	// Name 数据库文件名
	Name string
	// Prefix 表前缀
	Prefix string
	// Logger GORM 日志实现
	Logger logger.Interface
}

// New 创建并初始化数据库连接
func New(opts Options) (*gorm.DB, error) {
	dbPath, err := GetDefaultPath(opts.Name)
	if err != nil {
		return nil, err
	}

	// 确保数据库目录存在
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	// 打开数据库连接
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: opts.Logger,
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   opts.Prefix,
			SingularTable: true,
		},
	})
	if err != nil {
		return nil, err
	}

	// 配置连接池（对于 SQLite 主要是控制并发）
	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
	}

	return db, nil
}

// Migrate 执行数据库自动迁移
func Migrate(db *gorm.DB, models ...any) error {
	return db.AutoMigrate(models...)
}

// GetDefaultPath 获取平台相关的默认数据库文件路径
func GetDefaultPath(dbName string) (string, error) {
	var baseDir string

	switch runtime.GOOS {
	case "windows":
		// %APPDATA%/cdpnetool/
		baseDir = os.Getenv("APPDATA")
		if baseDir == "" {
			baseDir = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
	case "darwin":
		// ~/Library/Application Support/cdpnetool/
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		baseDir = filepath.Join(home, "Library", "Application Support")
	default:
		// Linux: ~/.local/share/cdpnetool/
		baseDir = os.Getenv("XDG_DATA_HOME")
		if baseDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			baseDir = filepath.Join(home, ".local", "share")
		}
	}

	return filepath.Join(baseDir, "cdpnetool", dbName), nil
}
