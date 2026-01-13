package main

import (
	"fmt"

	fyne "fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// NewSessionPanel 创建左侧会话面板
func NewSessionPanel(app *App, onSessionChanged func()) fyne.CanvasObject {
	devToolsEntry := widget.NewEntry()
	devToolsEntry.SetPlaceHolder("http://127.0.0.1:9222")

	sessionList := widget.NewList(
		func() int {
			return len(app.GetSessions())
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			sessions := app.GetSessions()
			if int(i) < 0 || int(i) >= len(sessions) {
				return
			}
			s := sessions[i]
			status := "禁用"
			if s.Enabled {
				status = "启用"
			}
			label := o.(*widget.Label)
			label.SetText(fmt.Sprintf("[%s] %s", status, s.DevToolsURL))
		},
	)

	newSessionBtn := widget.NewButton("新建会话", func() {
		if err := app.StartSession(devToolsEntry.Text); err != nil {
			// TODO: 显示错误对话框
			return
		}
		sessionList.Refresh()
		if onSessionChanged != nil {
			onSessionChanged()
		}
	})

	sessionList.OnSelected = func(id widget.ListItemID) {
		app.SetCurrentSession(int(id))
		if onSessionChanged != nil {
			onSessionChanged()
		}
	}

	panel := container.NewBorder(
		container.NewVBox(devToolsEntry, newSessionBtn),
		nil, nil, nil,
		sessionList,
	)

	// 设置固定宽度
	panel.Resize(fyne.NewSize(300, 0))
	return panel
}
