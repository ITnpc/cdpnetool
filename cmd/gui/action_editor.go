package main

import (
	"fmt"
	"strconv"

	fyne "fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"cdpnetool/pkg/rulespec"
)

// ActionEditor Action 动作可视化编辑器
type ActionEditor struct {
	window fyne.Window
	action *rulespec.Action

	actionType string

	// Rewrite 字段
	urlEntry    *widget.Entry
	methodEntry *widget.Entry
	headersList *widget.List
	headerItems []KeyValuePair

	// Respond 字段
	statusEntry        *widget.Entry
	respondHeadersList *widget.List
	respondHeaderItems []KeyValuePair
	bodyEntry          *widget.Entry
	base64Check        *widget.Check

	// Fail 字段
	reasonEntry *widget.Entry

	// Pause 字段
	stageSelect         *widget.Select
	timeoutEntry        *widget.Entry
	defaultActionSelect *widget.Select
	defaultStatusEntry  *widget.Entry
	defaultReasonEntry  *widget.Entry

	dynamicForm *fyne.Container
}

// KeyValuePair 键值对
type KeyValuePair struct {
	Key   string
	Value string
}

// NewActionEditor 创建 Action 编辑器
func NewActionEditor(w fyne.Window, action *rulespec.Action, actionType string) *ActionEditor {
	editor := &ActionEditor{
		window:     w,
		action:     action,
		actionType: actionType,
	}

	if action == nil {
		editor.action = &rulespec.Action{}
	}

	return editor
}

// Build 构建 UI
func (a *ActionEditor) Build() fyne.CanvasObject {
	a.dynamicForm = container.NewVBox()
	a.rebuildForm()
	return a.dynamicForm
}

// rebuildForm 根据动作类型重建表单
func (a *ActionEditor) rebuildForm() {
	if a.dynamicForm == nil {
		return
	}

	a.dynamicForm.Objects = nil

	switch a.actionType {
	case "rewrite":
		a.buildRewriteForm()
	case "respond":
		a.buildRespondForm()
	case "fail":
		a.buildFailForm()
	case "pause":
		a.buildPauseForm()
	case "continue":
		a.dynamicForm.Add(widget.NewLabel("继续执行，无需额外配置"))
	default:
		a.dynamicForm.Add(widget.NewLabel("未知动作类型"))
	}

	a.dynamicForm.Refresh()
}

// buildRewriteForm 构建 Rewrite 表单
func (a *ActionEditor) buildRewriteForm() {
	a.dynamicForm.Add(widget.NewLabel("URL 重写"))
	a.urlEntry = widget.NewEntry()
	if a.action.Rewrite != nil && a.action.Rewrite.URL != nil {
		a.urlEntry.SetText(*a.action.Rewrite.URL)
	}
	a.urlEntry.SetPlaceHolder("留空表示不修改")
	a.dynamicForm.Add(a.urlEntry)

	a.dynamicForm.Add(widget.NewLabel("Method 重写"))
	a.methodEntry = widget.NewEntry()
	if a.action.Rewrite != nil && a.action.Rewrite.Method != nil {
		a.methodEntry.SetText(*a.action.Rewrite.Method)
	}
	a.methodEntry.SetPlaceHolder("如: GET, POST")
	a.dynamicForm.Add(a.methodEntry)

	a.dynamicForm.Add(widget.NewSeparator())
	a.dynamicForm.Add(widget.NewLabel("Headers 修改"))

	// 加载现有 Headers
	a.headerItems = make([]KeyValuePair, 0)
	if a.action.Rewrite != nil && a.action.Rewrite.Headers != nil {
		for k, v := range a.action.Rewrite.Headers {
			val := ""
			if v != nil {
				val = *v
			}
			a.headerItems = append(a.headerItems, KeyValuePair{Key: k, Value: val})
		}
	}

	a.headersList = widget.NewList(
		func() int { return len(a.headerItems) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			if int(i) >= len(a.headerItems) {
				return
			}
			item := a.headerItems[i]
			label := o.(*widget.Label)
			if item.Value == "" {
				label.SetText(fmt.Sprintf("%s: (删除)", item.Key))
			} else {
				label.SetText(fmt.Sprintf("%s: %s", item.Key, item.Value))
			}
		},
	)
	a.headersList.Resize(fyne.NewSize(0, 200))

	addHeaderBtn := widget.NewButton("添加/修改 Header", func() {
		a.showKeyValueEditor("Header", func(key, value string) {
			// 更新或添加
			found := false
			for i := range a.headerItems {
				if a.headerItems[i].Key == key {
					a.headerItems[i].Value = value
					found = true
					break
				}
			}
			if !found {
				a.headerItems = append(a.headerItems, KeyValuePair{Key: key, Value: value})
			}
			a.headersList.Refresh()
		})
	})

	deleteHeaderBtn := widget.NewButton("删除选中", func() {
		// TODO: 实现删除逻辑
	})

	a.dynamicForm.Add(container.NewHBox(addHeaderBtn, deleteHeaderBtn))
	a.dynamicForm.Add(a.headersList)

	a.dynamicForm.Add(widget.NewLabel("提示: Value 留空表示删除该 Header"))
}

// buildRespondForm 构建 Respond 表单
func (a *ActionEditor) buildRespondForm() {
	a.dynamicForm.Add(widget.NewLabel("响应状态码"))
	a.statusEntry = widget.NewEntry()
	if a.action.Respond != nil {
		a.statusEntry.SetText(strconv.Itoa(a.action.Respond.Status))
	}
	a.statusEntry.SetPlaceHolder("如: 200, 404, 500")
	a.dynamicForm.Add(a.statusEntry)

	a.dynamicForm.Add(widget.NewSeparator())
	a.dynamicForm.Add(widget.NewLabel("响应 Headers"))

	// 加载现有 Headers
	a.respondHeaderItems = make([]KeyValuePair, 0)
	if a.action.Respond != nil && a.action.Respond.Headers != nil {
		for k, v := range a.action.Respond.Headers {
			a.respondHeaderItems = append(a.respondHeaderItems, KeyValuePair{Key: k, Value: v})
		}
	}

	a.respondHeadersList = widget.NewList(
		func() int { return len(a.respondHeaderItems) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			if int(i) >= len(a.respondHeaderItems) {
				return
			}
			item := a.respondHeaderItems[i]
			label := o.(*widget.Label)
			label.SetText(fmt.Sprintf("%s: %s", item.Key, item.Value))
		},
	)
	a.respondHeadersList.Resize(fyne.NewSize(0, 200))

	addRespondHeaderBtn := widget.NewButton("添加 Header", func() {
		a.showKeyValueEditor("Header", func(key, value string) {
			found := false
			for i := range a.respondHeaderItems {
				if a.respondHeaderItems[i].Key == key {
					a.respondHeaderItems[i].Value = value
					found = true
					break
				}
			}
			if !found {
				a.respondHeaderItems = append(a.respondHeaderItems, KeyValuePair{Key: key, Value: value})
			}
			a.respondHeadersList.Refresh()
		})
	})

	a.dynamicForm.Add(addRespondHeaderBtn)
	a.dynamicForm.Add(a.respondHeadersList)

	a.dynamicForm.Add(widget.NewSeparator())
	a.dynamicForm.Add(widget.NewLabel("响应 Body"))
	a.bodyEntry = widget.NewMultiLineEntry()
	if a.action.Respond != nil && a.action.Respond.Body != nil {
		a.bodyEntry.SetText(string(a.action.Respond.Body))
	}
	a.bodyEntry.SetPlaceHolder("响应内容")
	a.bodyEntry.SetMinRowsVisible(5)
	a.dynamicForm.Add(a.bodyEntry)

	a.base64Check = widget.NewCheck("Body 是 Base64 编码", nil)
	if a.action.Respond != nil {
		a.base64Check.SetChecked(a.action.Respond.Base64)
	}
	a.dynamicForm.Add(a.base64Check)
}

// buildFailForm 构建 Fail 表单
func (a *ActionEditor) buildFailForm() {
	a.dynamicForm.Add(widget.NewLabel("失败原因"))
	a.reasonEntry = widget.NewEntry()
	if a.action.Fail != nil {
		a.reasonEntry.SetText(a.action.Fail.Reason)
	}
	a.reasonEntry.SetPlaceHolder("如: Blocked, Failed")
	a.dynamicForm.Add(a.reasonEntry)
}

// buildPauseForm 构建 Pause 表单
func (a *ActionEditor) buildPauseForm() {
	a.dynamicForm.Add(widget.NewLabel("暂停阶段"))
	a.stageSelect = widget.NewSelect(getStageOptions(), nil)
	if a.action.Pause != nil {
		a.stageSelect.SetSelected(findLabeledOption(string(a.action.Pause.Stage), stageLabels))
	} else {
		a.stageSelect.SetSelected(findLabeledOption("request", stageLabels))
	}
	a.dynamicForm.Add(a.stageSelect)

	a.dynamicForm.Add(widget.NewLabel("超时时间 (毫秒)"))
	a.timeoutEntry = widget.NewEntry()
	if a.action.Pause != nil {
		a.timeoutEntry.SetText(strconv.Itoa(a.action.Pause.TimeoutMS))
	}
	a.timeoutEntry.SetPlaceHolder("如: 30000 (30秒)")
	a.dynamicForm.Add(a.timeoutEntry)

	a.dynamicForm.Add(widget.NewSeparator())
	a.dynamicForm.Add(widget.NewLabel("超时默认动作"))

	pauseDefaultActionLabels := map[string]string{
		"continue_original": "继续原始请求 (continue_original)",
		"continue_mutated":  "继续修改后请求 (continue_mutated)",
		"fulfill":           "完成 (fulfill)",
		"fail":              "失败 (fail)",
	}

	defaultActionOptions := []string{
		pauseDefaultActionLabels["continue_original"],
		pauseDefaultActionLabels["continue_mutated"],
		pauseDefaultActionLabels["fulfill"],
		pauseDefaultActionLabels["fail"],
	}

	a.defaultActionSelect = widget.NewSelect(defaultActionOptions, nil)
	if a.action.Pause != nil {
		a.defaultActionSelect.SetSelected(findLabeledOption(string(a.action.Pause.DefaultAction.Type), pauseDefaultActionLabels))
	} else {
		a.defaultActionSelect.SetSelected(pauseDefaultActionLabels["continue_original"])
	}
	a.dynamicForm.Add(a.defaultActionSelect)

	a.dynamicForm.Add(widget.NewLabel("默认动作状态码 (可选)"))
	a.defaultStatusEntry = widget.NewEntry()
	if a.action.Pause != nil && a.action.Pause.DefaultAction.Status != 0 {
		a.defaultStatusEntry.SetText(strconv.Itoa(a.action.Pause.DefaultAction.Status))
	}
	a.defaultStatusEntry.SetPlaceHolder("仅 fulfill 时需要")
	a.dynamicForm.Add(a.defaultStatusEntry)

	a.dynamicForm.Add(widget.NewLabel("默认动作原因 (可选)"))
	a.defaultReasonEntry = widget.NewEntry()
	if a.action.Pause != nil {
		a.defaultReasonEntry.SetText(a.action.Pause.DefaultAction.Reason)
	}
	a.defaultReasonEntry.SetPlaceHolder("仅 fail 时需要")
	a.dynamicForm.Add(a.defaultReasonEntry)
}

// showKeyValueEditor 显示键值对编辑器
func (a *ActionEditor) showKeyValueEditor(title string, onSave func(key, value string)) {
	keyEntry := widget.NewEntry()
	keyEntry.SetPlaceHolder("键名")

	valueEntry := widget.NewEntry()
	valueEntry.SetPlaceHolder("值 (留空表示删除)")

	content := container.NewVBox(
		widget.NewLabel("键名"),
		keyEntry,
		widget.NewLabel("值"),
		valueEntry,
	)

	var dlg *widget.PopUp
	saveBtn := widget.NewButton("保存", func() {
		if onSave != nil {
			onSave(keyEntry.Text, valueEntry.Text)
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
			widget.NewLabel(fmt.Sprintf("编辑 %s", title)),
			container.NewHBox(saveBtn, cancelBtn),
			nil, nil,
			content,
		),
		a.window.Canvas(),
	)

	dlg.Resize(fyne.NewSize(400, 200))
	dlg.Show()
}

// GetAction 获取构建的 Action 对象
func (a *ActionEditor) GetAction() *rulespec.Action {
	action := &rulespec.Action{}

	switch a.actionType {
	case "rewrite":
		rewrite := &rulespec.Rewrite{
			Headers: make(map[string]*string),
		}

		if a.urlEntry != nil && a.urlEntry.Text != "" {
			url := a.urlEntry.Text
			rewrite.URL = &url
		}

		if a.methodEntry != nil && a.methodEntry.Text != "" {
			method := a.methodEntry.Text
			rewrite.Method = &method
		}

		for _, item := range a.headerItems {
			if item.Value == "" {
				rewrite.Headers[item.Key] = nil
			} else {
				val := item.Value
				rewrite.Headers[item.Key] = &val
			}
		}

		action.Rewrite = rewrite

	case "respond":
		respond := &rulespec.Respond{
			Headers: make(map[string]string),
		}

		if a.statusEntry != nil {
			status, _ := strconv.Atoi(a.statusEntry.Text)
			respond.Status = status
		}

		for _, item := range a.respondHeaderItems {
			respond.Headers[item.Key] = item.Value
		}

		if a.bodyEntry != nil {
			respond.Body = []byte(a.bodyEntry.Text)
		}

		if a.base64Check != nil {
			respond.Base64 = a.base64Check.Checked
		}

		action.Respond = respond

	case "fail":
		fail := &rulespec.Fail{}
		if a.reasonEntry != nil {
			fail.Reason = a.reasonEntry.Text
		}
		action.Fail = fail

	case "pause":
		pause := &rulespec.Pause{}

		if a.stageSelect != nil {
			pause.Stage = rulespec.PauseStage(extractValue(a.stageSelect.Selected))
		}

		if a.timeoutEntry != nil {
			timeout, _ := strconv.Atoi(a.timeoutEntry.Text)
			pause.TimeoutMS = timeout
		}

		if a.defaultActionSelect != nil {
			pause.DefaultAction.Type = rulespec.PauseDefaultActionType(extractValue(a.defaultActionSelect.Selected))
		}

		if a.defaultStatusEntry != nil && a.defaultStatusEntry.Text != "" {
			status, _ := strconv.Atoi(a.defaultStatusEntry.Text)
			pause.DefaultAction.Status = status
		}

		if a.defaultReasonEntry != nil {
			pause.DefaultAction.Reason = a.defaultReasonEntry.Text
		}

		action.Pause = pause
	}

	return action
}
