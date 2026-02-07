package repo

import (
	"context"
	"time"

	"cdpnetool/internal/config"
	"cdpnetool/internal/storage/model"

	"gorm.io/gorm"
)

// SettingsRepo 设置仓库
type SettingsRepo struct {
	BaseRepository[model.Setting]
}

// NewSettingsRepo 创建设置仓库实例
func NewSettingsRepo(db *gorm.DB) *SettingsRepo {
	return &SettingsRepo{
		BaseRepository: *NewBaseRepository[model.Setting](db),
	}
}

// Get 获取设置值
func (r *SettingsRepo) Get(ctx context.Context, key string) (string, error) {
	var setting model.Setting
	result := r.Db.WithContext(ctx).Where("key = ?", key).First(&setting)
	if result.Error != nil {
		return "", result.Error
	}
	return setting.Value, nil
}

// GetWithDefault 获取设置值，不存在时返回默认值
func (r *SettingsRepo) GetWithDefault(ctx context.Context, key, defaultValue string) string {
	val, err := r.Get(ctx, key)
	if err != nil {
		return defaultValue
	}
	return val
}

// Set 设置值（存在则更新，不存在则创建）
func (r *SettingsRepo) Set(ctx context.Context, key, value string) error {
	setting := model.Setting{
		Key:       key,
		Value:     value,
		UpdatedAt: time.Now(),
	}
	return r.Db.WithContext(ctx).Save(&setting).Error
}

// DeleteByKey 根据 key 删除设置
func (r *SettingsRepo) DeleteByKey(ctx context.Context, key string) error {
	return r.Db.WithContext(ctx).Delete(&model.Setting{}, "key = ?", key).Error
}

// GetAll 获取所有设置
func (r *SettingsRepo) GetAll(ctx context.Context) (map[string]string, error) {
	var settings []model.Setting
	if err := r.Db.WithContext(ctx).Find(&settings).Error; err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

// SetMultiple 批量设置
func (r *SettingsRepo) SetMultiple(ctx context.Context, kvs map[string]string) error {
	return r.Db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		for key, value := range kvs {
			setting := model.Setting{
				Key:       key,
				Value:     value,
				UpdatedAt: now,
			}
			if err := tx.Save(&setting).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetTheme 获取主题
func (r *SettingsRepo) GetTheme(ctx context.Context) string {
	return r.GetWithDefault(ctx, model.SettingKeyTheme, "system")
}

// SetTheme 设置主题
func (r *SettingsRepo) SetTheme(ctx context.Context, theme string) error {
	return r.Set(ctx, model.SettingKeyTheme, theme)
}

// GetLastConfigID 获取上次使用的配置 ID
func (r *SettingsRepo) GetLastConfigID(ctx context.Context) string {
	return r.GetWithDefault(ctx, model.SettingKeyLastConfigID, "")
}

// SetLastConfigID 设置上次使用的配置 ID
func (r *SettingsRepo) SetLastConfigID(ctx context.Context, id string) error {
	return r.Set(ctx, model.SettingKeyLastConfigID, id)
}

// GetAllWithDefaults 获取所有设置（不存在的使用默认值）
func (r *SettingsRepo) GetAllWithDefaults(ctx context.Context) (map[string]string, error) {
	settings, err := r.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	defaults := config.GetDefaultSettings()
	result := map[string]string{
		model.SettingKeyLanguage:    defaults.Language,
		model.SettingKeyTheme:       defaults.Theme,
		model.SettingKeyBrowserArgs: defaults.BrowserArgs,
		model.SettingKeyBrowserPath: defaults.BrowserPath,
	}

	// 用数据库中的值覆盖默认值
	for k, v := range settings {
		result[k] = v
	}

	return result, nil
}

// GetLanguage 获取语言设置
func (r *SettingsRepo) GetLanguage(ctx context.Context) string {
	return r.GetWithDefault(ctx, model.SettingKeyLanguage, config.GetDefaultSettings().Language)
}

// SetLanguage 设置语言
func (r *SettingsRepo) SetLanguage(ctx context.Context, lang string) error {
	return r.Set(ctx, model.SettingKeyLanguage, lang)
}

// GetBrowserArgs 获取浏览器启动参数
func (r *SettingsRepo) GetBrowserArgs(ctx context.Context) string {
	return r.GetWithDefault(ctx, model.SettingKeyBrowserArgs, config.GetDefaultSettings().BrowserArgs)
}

// SetBrowserArgs 设置浏览器启动参数
func (r *SettingsRepo) SetBrowserArgs(ctx context.Context, args string) error {
	return r.Set(ctx, model.SettingKeyBrowserArgs, args)
}

// GetBrowserPath 获取浏览器可执行文件路径
func (r *SettingsRepo) GetBrowserPath(ctx context.Context) string {
	return r.GetWithDefault(ctx, model.SettingKeyBrowserPath, config.GetDefaultSettings().BrowserPath)
}

// SetBrowserPath 设置浏览器可执行文件路径
func (r *SettingsRepo) SetBrowserPath(ctx context.Context, path string) error {
	return r.Set(ctx, model.SettingKeyBrowserPath, path)
}
