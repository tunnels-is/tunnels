package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
)

func TestWrapToWidth_ShortStaysOneLine(t *testing.T) {
	a := test.NewApp()
	t.Cleanup(a.Quit)
	applyZoomTokens(1)

	style := fyne.TextStyle{Monospace: true}
	lines := wrapToWidth("C:\\tunnels", z(400), fsSmall, style)
	if len(lines) != 1 || lines[0] != "C:\\tunnels" {
		t.Fatalf("short path: %#v", lines)
	}
}

func TestWrapToWidth_LongPathBreaks(t *testing.T) {
	a := test.NewApp()
	t.Cleanup(a.Quit)
	applyZoomTokens(1)

	path := `C:\Users\Administrator\AppData\Roaming\tunnels\accounts\` + strings.Repeat("ab", 32) + `\user`
	style := fyne.TextStyle{Monospace: true}
	wide := fyne.MeasureText(path, fsSmall, style).Width
	lines := wrapToWidth(path, wide/3, fsSmall, style)
	if len(lines) < 2 {
		t.Fatalf("expected wrap, got %d lines for width %v", len(lines), wide/3)
	}
	if strings.Join(lines, "") != path {
		t.Fatal("wrap must preserve the path")
	}
	for i, line := range lines {
		if fyne.MeasureText(line, fsSmall, style).Width > wide/3+1 {
			t.Fatalf("line %d exceeds width: %q", i, line)
		}
	}
}

func TestKvRowGrowsForLongPath(t *testing.T) {
	a := test.NewApp()
	t.Cleanup(a.Quit)
	applyZoomTokens(1)

	path := `C:\Users\Administrator\AppData\Roaming\tunnels\accounts\` + strings.Repeat("cd", 40) + `\user`
	row := kvRow("Base path", path, true)
	natural := row.MinSize()
	row.Resize(fyne.NewSize(z(360), natural.Height))
	got := row.MinSize()
	if got.Height <= natural.Height {
		t.Fatalf("narrow kvRow should grow; natural=%v after=%v", natural, got)
	}
}

func TestFullRowSpansAllColumns(t *testing.T) {
	a := test.NewApp()
	t.Cleanup(a.Quit)
	applyZoomTokens(1)

	aCard := card("A", "", vspace(z(40)))
	bCard := card("B", "", vspace(z(40)))
	sys := fullRow(card("System", "", kvRow("Base path", `C:\very\long\path\that\should\not\clip`, true)))
	flow := container.New(&cardFlowLayout{minCol: z(300), maxCol: 3, gap: sp4}, aCard, bCard, sys)

	width := z(800)
	flow.Resize(fyne.NewSize(width, flow.MinSize().Height))
	flow.Resize(fyne.NewSize(width, flow.MinSize().Height))

	if sys.Size().Width < width-1 {
		t.Fatalf("System card width = %v, want full row %v", sys.Size().Width, width)
	}
	if aCard.Size().Width >= width-1 {
		t.Fatalf("regular card should stay in a column, width=%v", aCard.Size().Width)
	}
}
