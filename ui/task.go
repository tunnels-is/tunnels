package ui

import (
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/tunnels-is/tunnels/client"
)

var taskMu sync.Mutex

func busyCard(label string) fyne.CanvasObject {
	p := pal()

	bar := container.New(strictLayout{w: z(220), h: z(6)}, widget.NewProgressBarInfinite())
	inner := vstack(sp4,
		container.NewCenter(text(label, fsBody, p.Content, false)),
		container.NewCenter(bar),
	)
	card := container.NewStack(
		surface(radLg, p.Base100, p.Base300),
		insetXY(sp6, sp5, container.New(strictLayout{
			w: z(240), h: inner.MinSize().Height,
		}, inner)),
	)

	scrim := canvas.NewRectangle(withAlpha(p.Base200, 190))
	return container.NewStack(scrim, container.NewCenter(dropShadow(radLg, card)))
}

func (a *App) showBusy(label string) {
	a.busyN++
	if a.busyBox == nil {
		return
	}
	a.busyBox.Objects = []fyne.CanvasObject{busyCard(label)}
	a.busyBox.Refresh()
}

func (a *App) hideBusy() {
	if a.busyN > 0 {
		a.busyN--
	}
	if a.busyN > 0 || a.busyBox == nil {
		return
	}
	a.busyBox.Objects = nil
	a.busyBox.Refresh()
}

func (a *App) runTask(label string, work func() error, after func(error)) {
	a.showBusy(label)
	go func() {
		taskMu.Lock()
		err := work()
		taskMu.Unlock()

		a.uiDo(func() {
			a.hideBusy()
			if after != nil {
				after(err)
			}
		})
	}()
}

func (a *App) updateConfig(label string, mutate func(*client.Config), after func()) {
	a.runTask(label, func() error {
		next := client.CloneConfig()
		if next == nil {
			return nil
		}
		mutate(next)
		return client.SetConfig(next)
	}, func(err error) {
		if err != nil {
			a.fail(err.Error())
			a.refreshState()
			a.reloadCurrent()
			return
		}
		a.refreshState()
		if after != nil {
			after()
		}
	})
}
