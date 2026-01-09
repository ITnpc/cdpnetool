package main

import (
	"fmt"
	"log"
	"sync"

	fyne "fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	api "cdpnetool/pkg/api"
	"cdpnetool/pkg/model"
)

// main 是 GUI 应用入口
func main() {
	a := app.New()
	w := a.NewWindow("cdpnetool GUI")

	state := &guiState{
		svc:             api.NewService(),
		currentSession: -1,
		currentTarget:  -1,
	}

	devToolsEntry := widget.NewEntry()
	devToolsEntry.SetPlaceHolder("http://127.0.0.1:9222")

	sessionList := widget.NewList(
		func() int {
			state.mu.Lock()
			defer state.mu.Unlock()
			return len(state.sessions)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			state.mu.Lock()
			defer state.mu.Unlock()
			if i < 0 || i >= len(state.sessions) {
				return
			}
			s := state.sessions[i]
			status := "禁用"
			if s.Enabled {
				status = "启用"
			}
			label := o.(*widget.Label)
			label.SetText(fmt.Sprintf("[%s] %s", status, s.DevToolsURL))
		},
	)

	var targetList *widget.List

	newSessionBtn := widget.NewButton("新建会话", func() {
		state.startSession(devToolsEntry.Text)
		sessionList.Refresh()
		state.refreshTargets()
		if targetList != nil {
			targetList.Refresh()
		}
	})

	sessionList.OnSelected = func(id widget.ListItemID) {
		state.setCurrentSession(int(id))
		state.refreshTargets()
		if targetList != nil {
			targetList.Refresh()
		}
	}

	refreshTargetsBtn := widget.NewButton("刷新目标", func() {
		state.refreshTargets()
		if targetList != nil {
			targetList.Refresh()
		}
	})

	attachTargetBtn := widget.NewButton("附加选中", func() {
		if err := state.attachSelectedTarget(); err != nil {
			log.Println("attach target:", err)
		}
		state.refreshTargets()
		if targetList != nil {
			targetList.Refresh()
		}
	})

	detachTargetBtn := widget.NewButton("移除选中", func() {
		if err := state.detachSelectedTarget(); err != nil {
			log.Println("detach target:", err)
		}
		state.refreshTargets()
		if targetList != nil {
			targetList.Refresh()
		}
	})

	targetList = widget.NewList(
		func() int {
			state.mu.Lock()
			defer state.mu.Unlock()
			return len(state.targets)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			state.mu.Lock()
			defer state.mu.Unlock()
			if i < 0 || i >= len(state.targets) {
				return
			}
			t := state.targets[i]
			attached := " "
			if t.Attached {
				attached = "*"
			}
			label := o.(*widget.Label)
			label.SetText(fmt.Sprintf("%s %s | %s", attached, t.Title, t.URL))
		},
	)

	targetList.OnSelected = func(id widget.ListItemID) {
		state.setCurrentTarget(int(id))
	}

	sessionPanel := container.NewBorder(
		container.NewVBox(devToolsEntry, newSessionBtn),
		nil,
		nil,
		nil,
		sessionList,
	)

	targetToolbar := container.NewHBox(refreshTargetsBtn, attachTargetBtn, detachTargetBtn)
	targetsTab := container.NewBorder(targetToolbar, nil, nil, nil, targetList)

	tabs := container.NewAppTabs(
		container.NewTabItem("Targets", targetsTab),
		container.NewTabItem("Rules", widget.NewLabel("规则将在后续阶段实现")),
		container.NewTabItem("Events", widget.NewLabel("事件视图将在后续阶段实现")),
		container.NewTabItem("Pending", widget.NewLabel("Pause 审批将在后续阶段实现")),
	)

	root := container.NewBorder(nil, nil, sessionPanel, nil, tabs)
	w.SetContent(root)
	w.Resize(fyne.NewSize(1024, 768))
	w.ShowAndRun()
}

type guiState struct {
	mu sync.Mutex

	svc api.Service

	sessions       []sessionItem
	currentSession int

	targets       []targetItem
	currentTarget int
}

type sessionItem struct {
	ID          string
	DevToolsURL string
	Enabled     bool
}

type targetItem struct {
	ID       string
	Title    string
	URL      string
	Type     string
	Attached bool
	IsUser   bool
}

func (g *guiState) startSession(devToolsURL string) {
	if devToolsURL == "" {
		devToolsURL = "http://127.0.0.1:9222"
	}
	cfg := model.SessionConfig{DevToolsURL: devToolsURL}
	id, err := g.svc.StartSession(cfg)
	if err != nil {
		log.Println("start session:", err)
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.sessions = append(g.sessions, sessionItem{ID: string(id), DevToolsURL: devToolsURL})
	if g.currentSession == -1 {
		g.currentSession = 0
	}
}

func (g *guiState) setCurrentSession(idx int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if idx < 0 || idx >= len(g.sessions) {
		g.currentSession = -1
		return
	}
	g.currentSession = idx
}

func (g *guiState) currentSessionID() (model.SessionID, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.currentSession < 0 || g.currentSession >= len(g.sessions) {
		return "", false
	}
	return model.SessionID(g.sessions[g.currentSession].ID), true
}

func (g *guiState) refreshTargets() {
	id, ok := g.currentSessionID()
	if !ok {
		g.mu.Lock()
		defer g.mu.Unlock()
		g.targets = nil
		g.currentTarget = -1
		return
	}
	targets, err := g.svc.ListTargets(id)
	if err != nil {
		log.Println("list targets:", err)
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.targets = g.targets[:0]
	for _, t := range targets {
		g.targets = append(g.targets, targetItem{
			ID:       string(t.ID),
			Title:    t.Title,
			URL:      t.URL,
			Type:     t.Type,
			Attached: t.IsCurrent,
			IsUser:   t.IsUser,
		})
	}
	g.currentTarget = -1
}

func (g *guiState) setCurrentTarget(idx int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if idx < 0 || idx >= len(g.targets) {
		g.currentTarget = -1
		return
	}
	g.currentTarget = idx
}

func (g *guiState) attachSelectedTarget() error {
	id, ok := g.currentSessionID()
	if !ok {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.currentTarget < 0 || g.currentTarget >= len(g.targets) {
		return nil
	}
	t := g.targets[g.currentTarget]
	return g.svc.AttachTarget(id, model.TargetID(t.ID))
}

func (g *guiState) detachSelectedTarget() error {
	id, ok := g.currentSessionID()
	if !ok {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.currentTarget < 0 || g.currentTarget >= len(g.targets) {
		return nil
	}
	t := g.targets[g.currentTarget]
	return g.svc.DetachTarget(id, model.TargetID(t.ID))
}
