package ui

import (
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/tunnels-is/tunnels/client"
)

// Saving the config is expensive: client.SetConfig re-parses every block and
// allow list, re-applies the kill switch (which spawns netsh on Windows) and
// writes to disk. Doing that on the UI goroutine froze the window on every
// toggle, so it runs as a task with a loader instead.

// taskMu serialises tasks. Config updates are read-modify-write over shared
// state, so overlapping them would silently lose one of the changes.
var taskMu sync.Mutex

// ---------------------------------------------------------------- loader

// busyCard is the loader: a label over an indeterminate bar, centred on a
// dimmed page.
//
// The scrim is a canvas rectangle, not a widget, so it darkens the page without
// capturing input — the window stays responsive while the task runs, which
// matters because tasks are serialised rather than blocking.
//
// A progress bar rather than widget.Activity: Activity draws nothing until its
// animation ticks, so if the animation never runs the loader would show no
// indicator at all. The bar is visible at rest and animates when it can.
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

// showBusy displays the loader. Calls nest: the label of the most recent task
// wins and the loader stays up until the last one finishes.
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

// ---------------------------------------------------------------- tasks

// runTask runs work off the UI goroutine with the loader showing, then calls
// after on the UI goroutine. Tasks run one at a time.
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

// updateConfig mutates the config and persists it in the background.
//
// The clone happens inside the task, under the same lock as the write, so two
// quick toggles cannot each clone the original and clobber one another.
//
// after runs only on success. Most callers pass nil: a switch already shows its
// new state, and rebuilding the page underneath the user's cursor was half of
// what made toggling feel like an interruption. A failure always rebuilds, so
// the control snaps back to what was actually saved.
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
