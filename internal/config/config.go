package config

// Config 配置文件结构体
type Config struct {
	Version string `yaml:"version"`

	Sqlite struct {
		Dsn    string `yaml:"dsn"`
		Prefix string `yaml:"prefix"`
	} `yaml:"sqlite"`

	Log struct {
		Level  string   `yaml:"level"`
		Writer []string `yaml:"writer"`
	} `yaml:"log"`
}

// NewConfig 创建默认配置
func NewConfig() *Config {
	return &Config{
		Version: "1.0.0",
		Sqlite: struct {
			Dsn    string `yaml:"dsn"`
			Prefix string `yaml:"prefix"`
		}{
			Dsn:    "db.sqlite3",
			Prefix: "cdpnetool_",
		},
		Log: struct {
			Level  string   `yaml:"level"`
			Writer []string `yaml:"writer"`
		}{
			Level:  "debug",
			Writer: []string{"console", "file"},
		},
	}
}
