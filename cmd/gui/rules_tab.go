package main

import (
	"encoding/json"
	"fmt"

	fyne "fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"cdpnetool/pkg/rulespec"
)

// NewRulesTab 创建 Rules 标签页
func NewRulesTab(app *App, w fyne.Window) fyne.CanvasObject {
	ruleList := widget.NewList(
		func() int {
			return len(app.GetRules())
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			rules := app.GetRules()
			if int(i) < 0 || int(i) >= len(rules) {
				return
			}
			r := rules[i]
			label := o.(*widget.Label)
			label.SetText(fmt.Sprintf("[%d] %s (%s)", r.Priority, r.Name, r.Mode))
		},
	)

	loadRulesBtn := widget.NewButton("加载规则文件", func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			var rs rulespec.RuleSet
			if err := json.NewDecoder(reader).Decode(&rs); err != nil {
				dialog.ShowError(err, w)
				return
			}
			if err := app.LoadRules(rs); err != nil {
				dialog.ShowError(err, w)
				return
			}
			ruleList.Refresh()
			dialog.ShowInformation("成功", fmt.Sprintf("已加载 %d 条规则", len(rs.Rules)), w)
		}, w)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
		fd.Show()
	})

	newRuleBtn := widget.NewButton("新建规则", func() {
		NewRuleEditor(w, nil, func(rule *rulespec.Rule) {
			// TODO: 将规则添加到当前 RuleSet
			ruleList.Refresh()
		})
	})

	toolbar := container.NewHBox(loadRulesBtn, newRuleBtn)
	return container.NewBorder(toolbar, nil, nil, nil, ruleList)
}
