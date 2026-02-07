package config

// DefaultSettings 定义所有设置的默认值
type DefaultSettings struct {
	Language    string
	Theme       string
	BrowserArgs string
	BrowserPath string
}

// GetDefaultSettings 返回默认设置
func GetDefaultSettings() DefaultSettings {
	return DefaultSettings{
		Language:    "zh",
		Theme:       "system",
		BrowserArgs: "",
		BrowserPath: "",
	}
}
