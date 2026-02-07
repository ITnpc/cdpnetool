package repo_test

import (
	"context"
	"testing"

	"cdpnetool/internal/storage/db"
	"cdpnetool/internal/storage/model"
	"cdpnetool/internal/storage/repo"
)

// setupSettingsTestDB 创建用于 SettingsRepo 测试的内存数据库。
func setupSettingsTestDB(t *testing.T) *repo.SettingsRepo {
	gdb, err := db.New(db.Options{
		Name:   ":memory:",
		Prefix: "test_",
	})
	if err != nil {
		t.Fatalf("创建内存数据库失败: %v", err)
	}

	err = db.Migrate(gdb, &model.Setting{})
	if err != nil {
		t.Fatalf("迁移数据库失败: %v", err)
	}

	return repo.NewSettingsRepo(gdb)
}

// TestSettingsRepo_SetAndGet 测试设置的保存与读取。
func TestSettingsRepo_SetAndGet(t *testing.T) {
	r := setupSettingsTestDB(t)

	key := "test_key"
	value := "test_value"

	err := r.Set(context.Background(), key, value)
	if err != nil {
		t.Fatalf("设置失败: %v", err)
	}

	retrieved, err := r.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("获取设置失败: %v", err)
	}

	if retrieved != value {
		t.Errorf("预期值为 %s，实际为 %s", value, retrieved)
	}
}

// TestSettingsRepo_GetWithDefault 测试不存在的键返回默认值。
func TestSettingsRepo_GetWithDefault(t *testing.T) {
	r := setupSettingsTestDB(t)

	defaultVal := "default_value"
	retrieved := r.GetWithDefault(context.Background(), "non_existent_key", defaultVal)

	if retrieved != defaultVal {
		t.Errorf("预期返回默认值 %s，实际返回 %s", defaultVal, retrieved)
	}
}

// TestSettingsRepo_SetMultiple 测试批量设置功能及事务一致性。
func TestSettingsRepo_SetMultiple(t *testing.T) {
	r := setupSettingsTestDB(t)

	kvs := map[string]string{
		"k1": "v1",
		"k2": "v2",
		"k3": "v3",
	}

	err := r.SetMultiple(context.Background(), kvs)
	if err != nil {
		t.Fatalf("批量设置失败: %v", err)
	}

	// 验证所有键值对是否正确保存
	for key, expectedVal := range kvs {
		actualVal, err := r.Get(context.Background(), key)
		if err != nil {
			t.Errorf("获取键 %s 失败: %v", key, err)
		}
		if actualVal != expectedVal {
			t.Errorf("键 %s 预期值 %s，实际值 %s", key, expectedVal, actualVal)
		}
	}
}

// TestSettingsRepo_DeleteByKey 测试按键删除功能。
func TestSettingsRepo_DeleteByKey(t *testing.T) {
	r := setupSettingsTestDB(t)

	key := "to_delete"
	r.Set(context.Background(), key, "some_value")

	err := r.DeleteByKey(context.Background(), key)
	if err != nil {
		t.Fatalf("删除失败: %v", err)
	}

	_, err = r.Get(context.Background(), key)
	if err == nil {
		t.Error("预期键已删除，但仍然能获取到值")
	}
}

// TestSettingsRepo_PresetKeys 测试预设置的键是否按预期工作。
func TestSettingsRepo_PresetKeys(t *testing.T) {
	r := setupSettingsTestDB(t)

	// 测试 Theme
	expectedTheme := "dark"
	r.SetTheme(context.Background(), expectedTheme)
	actualTheme := r.GetTheme(context.Background())
	if actualTheme != expectedTheme {
		t.Errorf("Theme 预期 %s，实际 %s", expectedTheme, actualTheme)
	}

	// 测试 Language
	expectedLang := "en"
	r.SetLanguage(context.Background(), expectedLang)
	actualLang := r.GetLanguage(context.Background())
	if actualLang != expectedLang {
		t.Errorf("Language 预期 %s，实际 %s", expectedLang, actualLang)
	}

	// 测试 BrowserPath
	expectedPath := "/usr/bin/chromium"
	r.SetBrowserPath(context.Background(), expectedPath)
	actualPath := r.GetBrowserPath(context.Background())
	if actualPath != expectedPath {
		t.Errorf("BrowserPath 预期 %s，实际 %s", expectedPath, actualPath)
	}

	// 测试 LastConfigID
	expectedConfigID := "config-test-123"
	r.SetLastConfigID(context.Background(), expectedConfigID)
	actualConfigID := r.GetLastConfigID(context.Background())
	if actualConfigID != expectedConfigID {
		t.Errorf("LastConfigID 预期 %s，实际 %s", expectedConfigID, actualConfigID)
	}

	// 测试默认值 - Theme
	r.DeleteByKey(context.Background(), model.SettingKeyTheme)
	resetTheme := r.GetTheme(context.Background())
	if resetTheme != "system" {
		t.Errorf("Theme 默认值应为 system，实际为 %s", resetTheme)
	}
}
