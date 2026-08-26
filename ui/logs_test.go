package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestLogRowWrapsLongMessage(t *testing.T) {
	a := test.NewApp()
	t.Cleanup(a.Quit)
	applyZoomTokens(1)

	row := newLogRow()
	msg := strings.Repeat("failed to resolve host example.internal ", 12)
	row.set(0, nil, "01-02 15:04:05", "ERROR", "DNSResolver", msg)

	wide := fyne.NewSize(z(900), row.MinSize().Height)
	row.Resize(wide)
	tall := row.MinSize().Height

	narrow := fyne.NewSize(z(360), row.MinSize().Height)
	row.Resize(narrow)
	got := row.MinSize()
	if got.Height <= tall {
		t.Fatalf("narrow log row should wrap and grow; wide=%v narrow=%v", tall, got.Height)
	}
	one := logLineHeight() + 2
	if got.Height <= one+1 {
		t.Fatalf("expected more than one line, height=%v line=%v", got.Height, one)
	}
}

func TestLogRowShortMessageStaysOneLine(t *testing.T) {
	a := test.NewApp()
	t.Cleanup(a.Quit)
	applyZoomTokens(1)

	row := newLogRow()
	row.set(0, nil, "01-02 15:04:05", "INFO", "main", "ready")
	row.Resize(fyne.NewSize(z(400), 10))
	got := row.MinSize().Height
	want := logLineHeight() + 2
	if got > want+1 {
		t.Fatalf("short line height=%v want ~%v", got, want)
	}
}
