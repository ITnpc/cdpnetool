package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"cdpnetool/internal/storage/model"
	"cdpnetool/pkg/rulespec"

	"gorm.io/gorm"
)

// ConfigRepo 配置仓库
type ConfigRepo struct {
	BaseRepository[model.ConfigRecord]
}

// NewConfigRepo 创建配置仓库实例
func NewConfigRepo(db *gorm.DB) *ConfigRepo {
	return &ConfigRepo{
		BaseRepository: *NewBaseRepository[model.ConfigRecord](db),
	}
}

// Create 创建新配置
func (r *ConfigRepo) Create(ctx context.Context, cfg *rulespec.Config) (*model.ConfigRecord, error) {
	// 校验配置 ID
	if err := rulespec.ValidateConfigID(cfg.ID); err != nil {
		return nil, err
	}

	// 校验规则 ID
	if err := r.validateRuleIDs(cfg.Rules); err != nil {
		return nil, err
	}

	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}

	record := &model.ConfigRecord{
		ConfigID:   cfg.ID,
		Name:       cfg.Name,
		Version:    cfg.Version,
		ConfigJSON: string(configJSON),
		IsActive:   false,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := r.Db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

// Update 更新配置（按数据库 ID）
func (r *ConfigRepo) Update(ctx context.Context, dbID uint, cfg *rulespec.Config) error {
	// 校验配置 ID
	if err := rulespec.ValidateConfigID(cfg.ID); err != nil {
		return err
	}

	// 校验规则 ID
	if err := r.validateRuleIDs(cfg.Rules); err != nil {
		return err
	}

	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	return r.Db.WithContext(ctx).Model(&model.ConfigRecord{}).Where("id = ?", dbID).Updates(map[string]any{
		"config_id":   cfg.ID,
		"name":        cfg.Name,
		"version":     cfg.Version,
		"config_json": string(configJSON),
		"updated_at":  time.Now(),
	}).Error
}

// GetByConfigID 根据配置业务 ID 获取配置
func (r *ConfigRepo) GetByConfigID(ctx context.Context, configID string) (*model.ConfigRecord, error) {
	var record model.ConfigRecord
	if err := r.Db.WithContext(ctx).Where("config_id = ?", configID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

// List 列出所有配置
func (r *ConfigRepo) List(ctx context.Context) ([]model.ConfigRecord, error) {
	var records []model.ConfigRecord
	if err := r.Db.WithContext(ctx).Order("id DESC").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// SetActive 设置激活的配置（只能有一个激活）
func (r *ConfigRepo) SetActive(ctx context.Context, id uint) error {
	return r.Db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先取消所有激活
		if err := tx.Model(&model.ConfigRecord{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
			return err
		}
		// 激活指定配置
		if err := tx.Model(&model.ConfigRecord{}).Where("id = ?", id).Update("is_active", true).Error; err != nil {
			return err
		}
		return nil
	})
}

// GetActive 获取当前激活的配置
func (r *ConfigRepo) GetActive(ctx context.Context) (*model.ConfigRecord, error) {
	var record model.ConfigRecord
	if err := r.Db.WithContext(ctx).Where("is_active = ?", true).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

// ToRulespecConfig 将记录转换为 rulespec.Config
func (r *ConfigRepo) ToRulespecConfig(record *model.ConfigRecord) (*rulespec.Config, error) {
	if record == nil || record.ConfigJSON == "" {
		return nil, nil
	}

	var cfg rulespec.Config
	if err := json.Unmarshal([]byte(record.ConfigJSON), &cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	return &cfg, nil
}

// Save 保存配置（根据数据库 ID 判断新增或更新）
func (r *ConfigRepo) Save(ctx context.Context, dbID uint, cfg *rulespec.Config) (*model.ConfigRecord, error) {
	if dbID == 0 {
		return r.Create(ctx, cfg)
	}
	if err := r.Update(ctx, dbID, cfg); err != nil {
		return nil, err
	}
	return r.FindOne(ctx, dbID)
}

// Upsert 导入配置（根据配置业务 ID 判断覆盖或新增）
func (r *ConfigRepo) Upsert(ctx context.Context, cfg *rulespec.Config) (*model.ConfigRecord, error) {
	if err := rulespec.ValidateConfigID(cfg.ID); err != nil {
		return nil, err
	}

	if err := r.validateRuleIDs(cfg.Rules); err != nil {
		return nil, err
	}

	existing, err := r.GetByConfigID(ctx, cfg.ID)
	if err != nil {
		return nil, err
	}

	if existing != nil {
		if err := r.Update(ctx, existing.ID, cfg); err != nil {
			return nil, err
		}
		return r.FindOne(ctx, existing.ID)
	}

	return r.Create(ctx, cfg)
}

// Rename 重命名配置
func (r *ConfigRepo) Rename(ctx context.Context, id uint, newName string) error {
	record, err := r.FindOne(ctx, id)
	if err != nil {
		return err
	}

	cfg, err := r.ToRulespecConfig(record)
	if err != nil {
		return err
	}

	cfg.Name = newName
	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	return r.Db.WithContext(ctx).Model(&model.ConfigRecord{}).Where("id = ?", id).Updates(map[string]any{
		"name":        newName,
		"config_json": string(configJSON),
		"updated_at":  time.Now(),
	}).Error
}

// validateRuleIDs 校验规则 ID 格式和唯一性
func (r *ConfigRepo) validateRuleIDs(rules []rulespec.Rule) error {
	seen := make(map[string]bool)
	for _, rule := range rules {
		if err := rulespec.ValidateRuleID(rule.ID); err != nil {
			return fmt.Errorf("规则 '%s': %w", rule.Name, err)
		}
		if seen[rule.ID] {
			return fmt.Errorf("规则 ID '%s' 重复", rule.ID)
		}
		seen[rule.ID] = true
	}
	return nil
}
