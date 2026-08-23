package ui

import (
	"fmt"
	"os"
	"runtime/debug"

	"fyne.io/fyne/v2"
)

func (a *App) uiDo(fn func()) {
	fyne.Do(func() {
		defer a.recoverUI("uiDo")
		fn()
	})
}

func (a *App) recoverUI(where string) {
	rec := recover()
	if rec == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "tunnels ui panic in %s: %v\n%s\n", where, rec, debug.Stack())
}

func (a *App) reloadCurrent() {
	defer a.recoverUI("reloadCurrent")
	if a.content == nil {
		return
	}
	if a.reloading {
		return
	}
	a.reloading = true
	defer func() { a.reloading = false }()
	a.dropLiveLists()
	page := a.buildPage(a.current)
	for _, o := range a.content.Objects {
		if o != nil {
			o.Hide()
		}
	}
	a.content.RemoveAll()
	a.content.Add(page)
}
