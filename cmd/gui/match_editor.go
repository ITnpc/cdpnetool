package main

import (
	"fmt"

	fyne "fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"cdpnetool/pkg/rulespec"
)

// MatchEditor Match 条件可视化编辑器
type MatchEditor struct {
	allOfList  *widget.List
	anyOfList  *widget.List
	noneOfList *widget.List

	allOfConditions  []rulespec.Condition
	anyOfConditions  []rulespec.Condition
	noneOfConditions []rulespec.Condition

	currentGroup string
}

// NewMatchEditor 创建 Match 编辑器
func NewMatchEditor(match *rulespec.Match) *MatchEditor {
	editor := &MatchEditor{
		allOfConditions:  make([]rulespec.Condition, 0),
		anyOfConditions:  make([]rulespec.Condition, 0),
		noneOfConditions: make([]rulespec.Condition, 0),
	}

	if match != nil {
		editor.allOfConditions = match.AllOf
		editor.anyOfConditions = match.AnyOf
		editor.noneOfConditions = match.NoneOf
	}

	return editor
}

// Build 构建 UI
func (m *MatchEditor) Build(w fyne.Window) fyne.CanvasObject {
	// AllOf 列表
	m.allOfList = widget.NewList(
		func() int { return len(m.allOfConditions) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			if int(i) >= len(m.allOfConditions) {
				return
			}
			c := m.allOfConditions[i]
			label := o.(*widget.Label)
			label.SetText(m.formatCondition(c))
		},
	)

	// AnyOf 列表
	m.anyOfList = widget.NewList(
		func() int { return len(m.anyOfConditions) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			if int(i) >= len(m.anyOfConditions) {
				return
			}
			c := m.anyOfConditions[i]
			label := o.(*widget.Label)
			label.SetText(m.formatCondition(c))
		},
	)

	// NoneOf 列表
	m.noneOfList = widget.NewList(
		func() int { return len(m.noneOfConditions) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			if int(i) >= len(m.noneOfConditions) {
				return
			}
			c := m.noneOfConditions[i]
			label := o.(*widget.Label)
			label.SetText(m.formatCondition(c))
		},
	)

	// 分组选择
	groupSelect := widget.NewSelect(getMatchGroupOptions(), func(s string) {
		m.currentGroup = extractValue(s)
	})
	groupSelect.SetSelected(findLabeledOption("allOf", matchGroupLabels))
	m.currentGroup = "allOf"

	// 添加条件按钮
	addBtn := widget.NewButton("添加条件", func() {
		m.showConditionEditor(w, nil, m.currentGroup)
	})

	// 删除条件按钮
	deleteBtn := widget.NewButton("删除选中", func() {
		m.deleteSelectedCondition()
	})

	toolbar := container.NewHBox(
		widget.NewLabel("条件组:"),
		groupSelect,
		addBtn,
		deleteBtn,
	)

	// 使用 Tabs 展示三个条件组
	tabs := container.NewAppTabs(
		container.NewTabItem("全部满足 (AllOf)", m.allOfList),
		container.NewTabItem("任一满足 (AnyOf)", m.anyOfList),
		container.NewTabItem("全部不满足 (NoneOf)", m.noneOfList),
	)

	return container.NewBorder(toolbar, nil, nil, nil, tabs)
}

// formatCondition 格式化条件显示
func (m *MatchEditor) formatCondition(c rulespec.Condition) string {
	typeLabel := conditionTypeLabels[string(c.Type)]
	if typeLabel == "" {
		typeLabel = string(c.Type)
	}

	switch c.Type {
	case "url", "text", "mime":
		return fmt.Sprintf("%s: %s", typeLabel, c.Pattern)
	case "method":
		return fmt.Sprintf("%s: %v", typeLabel, c.Values)
	case "header", "query", "cookie":
		opLabel := conditionOpLabels[string(c.Op)]
		return fmt.Sprintf("%s[%s] %s %s", typeLabel, c.Key, opLabel, c.Value)
	case "size":
		opLabel := conditionOpLabels[string(c.Op)]
		return fmt.Sprintf("%s %s %s", typeLabel, opLabel, c.Value)
	case "stage":
		return fmt.Sprintf("%s: %s", typeLabel, c.Value)
	default:
		return fmt.Sprintf("%s", typeLabel)
	}
}

// showConditionEditor 显示条件编辑对话框
func (m *MatchEditor) showConditionEditor(w fyne.Window, condition *rulespec.Condition, group string) {
	editor := NewConditionEditor(w, condition, func(c *rulespec.Condition) {
		// 保存条件
		switch group {
		case "allOf":
			m.allOfConditions = append(m.allOfConditions, *c)
			m.allOfList.Refresh()
		case "anyOf":
			m.anyOfConditions = append(m.anyOfConditions, *c)
			m.anyOfList.Refresh()
		case "noneOf":
			m.noneOfConditions = append(m.noneOfConditions, *c)
			m.noneOfList.Refresh()
		}
	})
	editor.Show()
}

// deleteSelectedCondition 删除选中条件
func (m *MatchEditor) deleteSelectedCondition() {
	// TODO: 实现删除逻辑
}

// GetMatch 获取构建的 Match 对象
func (m *MatchEditor) GetMatch() rulespec.Match {
	return rulespec.Match{
		AllOf:  m.allOfConditions,
		AnyOf:  m.anyOfConditions,
		NoneOf: m.noneOfConditions,
	}
}

// ConditionEditor 单个条件编辑器
type ConditionEditor struct {
	window    fyne.Window
	condition *rulespec.Condition
	onSave    func(*rulespec.Condition)

	typeSelect   *widget.Select
	keyEntry     *widget.Entry
	opSelect     *widget.Select
	valueEntry   *widget.Entry
	patternEntry *widget.Entry
	valuesEntry  *widget.Entry

	dynamicForm *fyne.Container
}

// NewConditionEditor 创建条件编辑器
func NewConditionEditor(w fyne.Window, condition *rulespec.Condition, onSave func(*rulespec.Condition)) *ConditionEditor {
	editor := &ConditionEditor{
		window: w,
		onSave: onSave,
	}

	if condition == nil {
		editor.condition = &rulespec.Condition{
			Type: "url",
		}
	} else {
		editor.condition = condition
	}

	return editor
}

// Show 显示编辑对话框
func (e *ConditionEditor) Show() {
	// 先创建 dynamicForm，避免 SetSelected 触发回调时空指针
	e.dynamicForm = container.NewVBox()

	e.typeSelect = widget.NewSelect(getConditionTypeOptions(), func(s string) {
		condType := rulespec.ConditionType(extractValue(s))
		e.condition.Type = condType
		e.rebuildForm()
	})
	e.typeSelect.SetSelected(findLabeledOption(string(e.condition.Type), conditionTypeLabels))

	e.rebuildForm()

	content := container.NewVBox(
		widget.NewLabel("条件类型"),
		e.typeSelect,
		widget.NewSeparator(),
		e.dynamicForm,
	)

	scrollContent := container.NewVScroll(content)
	scrollContent.SetMinSize(fyne.NewSize(500, 400))

	var dlg *widget.PopUp
	saveBtn := widget.NewButton("保存", func() {
		e.collectData()
		if e.onSave != nil {
			e.onSave(e.condition)
		}
		if dlg != nil {
			dlg.Hide()
		}
	})

	cancelBtn := widget.NewButton("取消", func() {
		if dlg != nil {
			dlg.Hide()
		}
	})

	dlg = widget.NewModalPopUp(
		container.NewBorder(
			widget.NewLabel("编辑条件"),
			container.NewHBox(saveBtn, cancelBtn),
			nil, nil,
			scrollContent,
		),
		e.window.Canvas(),
	)

	dlg.Resize(fyne.NewSize(550, 450))
	dlg.Show()
}

// rebuildForm 根据条件类型重建表单
func (e *ConditionEditor) rebuildForm() {
	if e.dynamicForm == nil {
		return
	}

	e.dynamicForm.Objects = nil

	switch e.condition.Type {
	case "url", "text", "mime":
		e.patternEntry = widget.NewEntry()
		e.patternEntry.SetText(e.condition.Pattern)
		e.patternEntry.SetPlaceHolder("输入匹配模式")
		e.dynamicForm.Add(widget.NewLabel("匹配模式"))
		e.dynamicForm.Add(e.patternEntry)

	case "method":
		e.valuesEntry = widget.NewEntry()
		e.valuesEntry.SetPlaceHolder("输入方法列表，用逗号分隔，如: GET,POST")
		e.dynamicForm.Add(widget.NewLabel("请求方法"))
		e.dynamicForm.Add(e.valuesEntry)

	case "header", "query", "cookie":
		e.keyEntry = widget.NewEntry()
		e.keyEntry.SetText(e.condition.Key)
		e.keyEntry.SetPlaceHolder("输入键名")

		e.opSelect = widget.NewSelect(getConditionOpOptions(), nil)
		e.opSelect.SetSelected(findLabeledOption(string(e.condition.Op), conditionOpLabels))

		e.valueEntry = widget.NewEntry()
		e.valueEntry.SetText(e.condition.Value)
		e.valueEntry.SetPlaceHolder("输入值")

		e.dynamicForm.Add(widget.NewLabel("键名"))
		e.dynamicForm.Add(e.keyEntry)
		e.dynamicForm.Add(widget.NewLabel("操作符"))
		e.dynamicForm.Add(e.opSelect)
		e.dynamicForm.Add(widget.NewLabel("值"))
		e.dynamicForm.Add(e.valueEntry)

	case "size":
		e.opSelect = widget.NewSelect(getConditionOpOptions(), nil)
		e.opSelect.SetSelected(findLabeledOption(string(e.condition.Op), conditionOpLabels))

		e.valueEntry = widget.NewEntry()
		e.valueEntry.SetText(e.condition.Value)
		e.valueEntry.SetPlaceHolder("输入大小值，如: 1024")

		e.dynamicForm.Add(widget.NewLabel("操作符"))
		e.dynamicForm.Add(e.opSelect)
		e.dynamicForm.Add(widget.NewLabel("大小"))
		e.dynamicForm.Add(e.valueEntry)

	case "stage":
		stageSelect := widget.NewSelect(getStageOptions(), nil)
		stageSelect.SetSelected(findLabeledOption(e.condition.Value, stageLabels))
		e.valueEntry = widget.NewEntry()
		e.valueEntry.SetText(e.condition.Value)

		e.dynamicForm.Add(widget.NewLabel("阶段"))
		e.dynamicForm.Add(stageSelect)
	}

	e.dynamicForm.Refresh()
}

// collectData 从 UI 收集数据
func (e *ConditionEditor) collectData() {
	switch e.condition.Type {
	case "url", "text", "mime":
		if e.patternEntry != nil {
			e.condition.Pattern = e.patternEntry.Text
		}
	case "method":
		if e.valuesEntry != nil {
			// TODO: 解析逗号分隔的值
			e.condition.Values = []string{e.valuesEntry.Text}
		}
	case "header", "query", "cookie":
		if e.keyEntry != nil {
			e.condition.Key = e.keyEntry.Text
		}
		if e.opSelect != nil {
			e.condition.Op = rulespec.ConditionOp(extractValue(e.opSelect.Selected))
		}
		if e.valueEntry != nil {
			e.condition.Value = e.valueEntry.Text
		}
	case "size":
		if e.opSelect != nil {
			e.condition.Op = rulespec.ConditionOp(extractValue(e.opSelect.Selected))
		}
		if e.valueEntry != nil {
			e.condition.Value = e.valueEntry.Text
		}
	case "stage":
		if e.valueEntry != nil {
			e.condition.Value = e.valueEntry.Text
		}
	}
}
