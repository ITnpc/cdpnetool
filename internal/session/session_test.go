package session_test

import (
	"testing"

	"cdpnetool/internal/session"
	"cdpnetool/pkg/domain"
	"cdpnetool/pkg/rulespec"
)

func TestNew(t *testing.T) {
	sess := session.New("session1")
	if sess == nil {
		t.Error("New() returned nil")
	}
	if sess.ID != "session1" {
		t.Errorf("got ID %v, want session1", sess.ID)
	}
}

func TestAddTarget(t *testing.T) {
	sess := session.New("session1")
	sess.AddTarget("target1")

	targets := sess.GetTargets()
	if len(targets) != 1 {
		t.Errorf("got %d targets, want 1", len(targets))
	}
	if targets[0] != "target1" {
		t.Errorf("got target %v, want target1", targets[0])
	}
}

func TestAddTarget_Multiple(t *testing.T) {
	sess := session.New("session1")
	sess.AddTarget("target1")
	sess.AddTarget("target2")
	sess.AddTarget("target3")

	targets := sess.GetTargets()
	if len(targets) != 3 {
		t.Errorf("got %d targets, want 3", len(targets))
	}

	// 验证所有目标都存在
	targetMap := make(map[domain.TargetID]bool)
	for _, id := range targets {
		targetMap[id] = true
	}
	if !targetMap["target1"] || !targetMap["target2"] || !targetMap["target3"] {
		t.Error("missing expected targets")
	}
}

func TestAddTarget_Duplicate(t *testing.T) {
	sess := session.New("session1")
	sess.AddTarget("target1")
	sess.AddTarget("target1")

	targets := sess.GetTargets()
	if len(targets) != 1 {
		t.Errorf("got %d targets, want 1 (duplicate should be ignored)", len(targets))
	}
}

func TestRemoveTarget(t *testing.T) {
	sess := session.New("session1")
	sess.AddTarget("target1")
	sess.AddTarget("target2")

	sess.RemoveTarget("target1")

	targets := sess.GetTargets()
	if len(targets) != 1 {
		t.Errorf("got %d targets, want 1", len(targets))
	}
	if targets[0] != "target2" {
		t.Errorf("got target %v, want target2", targets[0])
	}
}

func TestRemoveTarget_NotExists(t *testing.T) {
	sess := session.New("session1")
	sess.AddTarget("target1")

	// 移除不存在的目标不应该报错
	sess.RemoveTarget("target2")

	targets := sess.GetTargets()
	if len(targets) != 1 {
		t.Errorf("got %d targets, want 1", len(targets))
	}
}

func TestRemoveTarget_All(t *testing.T) {
	sess := session.New("session1")
	sess.AddTarget("target1")
	sess.AddTarget("target2")

	sess.RemoveTarget("target1")
	sess.RemoveTarget("target2")

	targets := sess.GetTargets()
	if len(targets) != 0 {
		t.Errorf("got %d targets, want 0", len(targets))
	}
}

func TestGetTargets_Empty(t *testing.T) {
	sess := session.New("session1")
	targets := sess.GetTargets()
	if len(targets) != 0 {
		t.Errorf("got %d targets, want 0", len(targets))
	}
}

func TestUpdateConfig(t *testing.T) {
	sess := session.New("session1")
	cfg := rulespec.NewConfig("test")
	cfg.Rules = []rulespec.Rule{
		{
			ID:      "rule1",
			Name:    "test rule",
			Enabled: true,
		},
	}

	sess.UpdateConfig(cfg)

	if sess.Config == nil {
		t.Error("Config is nil")
	}
	if sess.Config.Name != "test" {
		t.Errorf("got config name %v, want test", sess.Config.Name)
	}
	if len(sess.Config.Rules) != 1 {
		t.Errorf("got %d rules, want 1", len(sess.Config.Rules))
	}
}

func TestUpdateConfig_Nil(t *testing.T) {
	sess := session.New("session1")
	cfg := rulespec.NewConfig("initial")
	sess.UpdateConfig(cfg)

	// 更新为 nil
	sess.UpdateConfig(nil)

	if sess.Config != nil {
		t.Error("Config should be nil")
	}
}

func TestUpdateConfig_Multiple(t *testing.T) {
	sess := session.New("session1")

	cfg1 := rulespec.NewConfig("config1")
	sess.UpdateConfig(cfg1)
	if sess.Config.Name != "config1" {
		t.Errorf("got config name %v, want config1", sess.Config.Name)
	}

	cfg2 := rulespec.NewConfig("config2")
	sess.UpdateConfig(cfg2)
	if sess.Config.Name != "config2" {
		t.Errorf("got config name %v, want config2", sess.Config.Name)
	}
}

func TestConcurrency_AddTarget(t *testing.T) {
	sess := session.New("session1")
	done := make(chan bool)

	// 并发添加目标
	for i := 0; i < 10; i++ {
		go func(id int) {
			sess.AddTarget(domain.TargetID("target" + string(rune('0'+id))))
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	targets := sess.GetTargets()
	if len(targets) != 10 {
		t.Errorf("got %d targets, want 10", len(targets))
	}
}

func TestConcurrency_RemoveTarget(t *testing.T) {
	sess := session.New("session1")

	// 先添加目标
	for i := 0; i < 10; i++ {
		sess.AddTarget(domain.TargetID("target" + string(rune('0'+i))))
	}

	done := make(chan bool)

	// 并发移除目标
	for i := 0; i < 10; i++ {
		go func(id int) {
			sess.RemoveTarget(domain.TargetID("target" + string(rune('0'+id))))
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	targets := sess.GetTargets()
	if len(targets) != 0 {
		t.Errorf("got %d targets, want 0", len(targets))
	}
}

func TestConcurrency_GetTargets(t *testing.T) {
	sess := session.New("session1")
	sess.AddTarget("target1")
	sess.AddTarget("target2")

	done := make(chan bool)

	// 并发读取目标
	for i := 0; i < 10; i++ {
		go func() {
			targets := sess.GetTargets()
			if len(targets) != 2 {
				t.Errorf("got %d targets, want 2", len(targets))
			}
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestConcurrency_UpdateConfig(t *testing.T) {
	sess := session.New("session1")
	done := make(chan bool)

	// 并发更新配置
	for i := 0; i < 10; i++ {
		go func(id int) {
			cfg := rulespec.NewConfig("config" + string(rune('0'+id)))
			sess.UpdateConfig(cfg)
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证最终有一个配置被设置
	if sess.Config == nil {
		t.Error("Config should not be nil")
	}
}
