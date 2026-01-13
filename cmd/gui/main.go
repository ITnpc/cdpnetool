package main

import (
	fyne "fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
)

func main() {
	a := app.New()
	w := a.NewWindow("cdpnetool GUI")

	guiApp := NewApp()

	var sessionPanel fyne.CanvasObject
	var toolbar fyne.CanvasObject
	var targetsTab fyne.CanvasObject
	var rulesTab fyne.CanvasObject

	refreshAll := func() {
		if sessionPanel != nil {
			sessionPanel.Refresh()
		}
		if toolbar != nil {
			toolbar.Refresh()
		}
		if targetsTab != nil {
			targetsTab.Refresh()
		}
		if rulesTab != nil {
			rulesTab.Refresh()
		}
	}

	sessionPanel = NewSessionPanel(guiApp, func() {
		_ = guiApp.RefreshTargets()
		refreshAll()
	})

	toolbar = NewToolbar(guiApp, refreshAll)

	targetsTab = NewTargetsTab(guiApp)
	rulesTab = NewRulesTab(guiApp, w)

	tabs := container.NewAppTabs(
		container.NewTabItem("Targets", targetsTab),
		container.NewTabItem("Rules", rulesTab),
		container.NewTabItem("Events", container.NewCenter(
			container.NewVBox(
				container.NewPadded(container.NewVBox()),
			),
		)),
		container.NewTabItem("Pending", container.NewCenter(
			container.NewVBox(
				container.NewPadded(container.NewVBox()),
			),
		)),
	)

	root := container.NewBorder(toolbar, nil, container.NewMax(sessionPanel), nil, tabs)
	w.SetContent(root)
	w.Resize(fyne.NewSize(1200, 768))
	w.ShowAndRun()
}
