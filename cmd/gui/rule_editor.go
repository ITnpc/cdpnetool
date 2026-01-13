package main

import (
	"fmt"

	fyne "fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"cdpnetool/pkg/model"
	"cdpnetool/pkg/rulespec"
)

// RuleEditor 规则编辑器对话框
type RuleEditor struct {
	window fyne.Window
	rule   *rulespec.Rule

	idEntry       *widget.Entry
	nameEntry     *widget.Entry
	priorityEntry *widget.Entry
	modeSelect    *widget.Select
	stageSelect   *widget.Select
	actionSelect  *widget.Select

	matchEditor     *MatchEditor
	actionEditor    *ActionEditor
	actionContainer *fyne.Container
}

// NewRuleEditor 创建规则编辑器
func NewRuleEditor(w fyne.Window, rule *rulespec.Rule, onSave func(*rulespec.Rule)) {
	editor := &RuleEditor{
		window: w,
		rule:   rule,
	}

	if rule == nil {
		editor.rule = &rulespec.Rule{
			ID:       "new_rule",
			Name:     "新规则",
			Priority: 100,
			Mode:     "short_circuit",
		}
	}

	editor.buildUI(onSave)
}

func (e *RuleEditor) buildUI(onSave func(*rulespec.Rule)) {
	e.idEntry = widget.NewEntry()
	e.idEntry.SetText(string(e.rule.ID))

	e.nameEntry = widget.NewEntry()
	e.nameEntry.SetText(e.rule.Name)

	e.priorityEntry = widget.NewEntry()
	e.priorityEntry.SetText(fmt.Sprintf("%d", e.rule.Priority))

	e.modeSelect = widget.NewSelect(getModeOptions(), nil)
	e.modeSelect.SetSelected(findLabeledOption(string(e.rule.Mode), modeLabels))

	// 先初始化 actionContainer
	e.actionContainer = container.NewVBox()

	// 确定当前 Action 类型
	currentActionType := "continue"
	if e.rule.Action.Pause != nil {
		currentActionType = "pause"
	} else if e.rule.Action.Rewrite != nil {
		currentActionType = "rewrite"
	} else if e.rule.Action.Respond != nil {
		currentActionType = "respond"
	} else if e.rule.Action.Fail != nil {
		currentActionType = "fail"
	}

	e.actionSelect = widget.NewSelect(getActionOptions(), func(selected string) {
		actionType := extractValue(selected)
		e.rebuildActionEditor(actionType)
	})
	e.actionSelect.SetSelected(findLabeledOption(currentActionType, actionLabels))

	// 初始化 Action 编辑器
	e.actionEditor = NewActionEditor(e.window, &e.rule.Action, currentActionType)
	e.actionContainer.Objects = []fyne.CanvasObject{e.actionEditor.Build()}

	e.matchEditor = NewMatchEditor(&e.rule.Match)

	// 使用 Tab 布局，让每个区域有更大空间
	basicInfoForm := container.NewVBox(
		widget.NewLabel("基础信息"),
		container.NewGridWithColumns(2,
			widget.NewLabel("规则 ID:"), e.idEntry,
			widget.NewLabel("规则名称:"), e.nameEntry,
			widget.NewLabel("优先级:"), e.priorityEntry,
			widget.NewLabel("模式:"), e.modeSelect,
		),
		widget.NewSeparator(),
		widget.NewLabel("动作类型"),
		container.NewGridWithColumns(2,
			widget.NewLabel("动作:"), e.actionSelect,
		),
	)

	// Match 条件 Tab
	matchContent := container.NewBorder(
		widget.NewLabel("匹配条件配置"),
		nil, nil, nil,
		e.matchEditor.Build(e.window),
	)
	matchTab := container.NewVScroll(matchContent)

	// Action 配置 Tab
	actionContent := container.NewBorder(
		widget.NewLabel("动作参数配置"),
		nil, nil, nil,
		e.actionContainer,
	)
	actionTab := container.NewVScroll(actionContent)

	tabs := container.NewAppTabs(
		container.NewTabItem("基础信息", basicInfoForm),
		container.NewTabItem("匹配条件 (Match)", matchTab),
		container.NewTabItem("动作配置 (Action)", actionTab),
	)

	content := container.NewMax(tabs)

	d := dialog.NewCustomConfirm("规则编辑器", "保存", "取消", content, func(save bool) {
		if save && onSave != nil {
			e.collectData()
			onSave(e.rule)
		}
	}, e.window)

	d.Resize(fyne.NewSize(900, 750))
	d.Show()
}

// rebuildActionEditor 根据动作类型重建 Action 编辑器
func (e *RuleEditor) rebuildActionEditor(actionType string) {
	if e.actionContainer == nil {
		return
	}

	e.actionEditor = NewActionEditor(e.window, &e.rule.Action, actionType)
	e.actionContainer.Objects = []fyne.CanvasObject{e.actionEditor.Build()}
	e.actionContainer.Refresh()
}

// collectData 从 UI 收集数据
func (e *RuleEditor) collectData() {
	e.rule.ID = model.RuleID(e.idEntry.Text)
	e.rule.Name = e.nameEntry.Text

	if priority, err := fmt.Sscanf(e.priorityEntry.Text, "%d", &e.rule.Priority); err != nil || priority == 0 {
		e.rule.Priority = 100
	}

	e.rule.Mode = rulespec.RuleMode(extractValue(e.modeSelect.Selected))

	// 从 Match 编辑器收集数据
	if e.matchEditor != nil {
		e.rule.Match = e.matchEditor.GetMatch()
	}

	// 从 Action 编辑器收集数据
	if e.actionEditor != nil {
		e.rule.Action = *e.actionEditor.GetAction()
	}
}
