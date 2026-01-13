package main

import (
	"fmt"

	fyne "fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// NewTargetsTab 创建 Targets 标签页
func NewTargetsTab(app *App) fyne.CanvasObject {
	targetList := widget.NewList(
		func() int {
			return len(app.GetTargets())
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			targets := app.GetTargets()
			if int(i) < 0 || int(i) >= len(targets) {
				return
			}
			t := targets[i]
			attached := " "
			if t.Attached {
				attached = "*"
			}
			label := o.(*widget.Label)
			label.SetText(fmt.Sprintf("%s %s | %s", attached, t.Title, t.URL))
		},
	)

	targetList.OnSelected = func(id widget.ListItemID) {
		app.SetCurrentTarget(int(id))
	}

	refreshBtn := widget.NewButton("刷新目标", func() {
		if err := app.RefreshTargets(); err != nil {
			// TODO: 显示错误对话框
			return
		}
		targetList.Refresh()
	})

	attachBtn := widget.NewButton("附加选中", func() {
		if err := app.AttachSelectedTarget(); err != nil {
			// TODO: 显示错误对话框
			return
		}
		if err := app.RefreshTargets(); err != nil {
			return
		}
		targetList.Refresh()
	})

	detachBtn := widget.NewButton("移除选中", func() {
		if err := app.DetachSelectedTarget(); err != nil {
			// TODO: 显示错误对话框
			return
		}
		if err := app.RefreshTargets(); err != nil {
			return
		}
		targetList.Refresh()
	})

	toolbar := container.NewHBox(refreshBtn, attachBtn, detachBtn)
	return container.NewBorder(toolbar, nil, nil, nil, targetList)
}
