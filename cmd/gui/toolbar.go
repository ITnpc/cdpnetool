package main

import (
	fyne "fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// NewToolbar 创建顶部工具条
func NewToolbar(app *App, onRefresh func()) fyne.CanvasObject {
	enableBtn := widget.NewButton("启用拦截", func() {
		if err := app.EnableInterception(); err != nil {
			// TODO: 显示错误对话框
			return
		}
		if onRefresh != nil {
			onRefresh()
		}
	})

	disableBtn := widget.NewButton("停用拦截", func() {
		if err := app.DisableInterception(); err != nil {
			// TODO: 显示错误对话框
			return
		}
		if onRefresh != nil {
			onRefresh()
		}
	})

	attachDefaultBtn := widget.NewButton("附加默认页面", func() {
		if err := app.AttachDefaultTarget(); err != nil {
			// TODO: 显示错误对话框
			return
		}
		if onRefresh != nil {
			onRefresh()
		}
	})

	return container.NewHBox(enableBtn, disableBtn, attachDefaultBtn)
}
