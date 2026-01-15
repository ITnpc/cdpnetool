package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB 数据库连接管理器
type DB struct {
	gormDB *gorm.DB
}

// NewDB 创建新的数据库连接实例并执行迁移
func NewDB() (*DB, error) {
	dbPath, err := getDBPath()
	fmt.Println("DB Path:", dbPath)
	if err != nil {
		return nil, err
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}

	// 打开数据库连接
	gormDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	db := &DB{gormDB: gormDB}

	// 自动迁移
	if err := db.autoMigrate(); err != nil {
		return nil, err
	}

	return db, nil
}

// GormDB 获取 gorm.DB 实例
func (d *DB) GormDB() *gorm.DB {
	return d.gormDB
}

// Close 关闭数据库连接
func (d *DB) Close() error {
	if d.gormDB == nil {
		return nil
	}
	sqlDB, err := d.gormDB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// getDBPath 获取跨平台的数据库文件路径
func getDBPath() (string, error) {
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

	return filepath.Join(baseDir, "cdpnetool", "data.db"), nil
}

// autoMigrate 自动迁移所有模型
func (d *DB) autoMigrate() error {
	return d.gormDB.AutoMigrate(
		&Setting{},
		&RuleSetRecord{},
		&InterceptEventRecord{},
	)
}

// GetDBPath 导出获取数据库路径的方法（用于调试）
func GetDBPath() (string, error) {
	return getDBPath()
}
