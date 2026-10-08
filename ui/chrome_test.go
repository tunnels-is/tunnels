package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
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

func TestWrapToWidth_MonoASCIIDoesNotExceed(t *testing.T) {
	a := test.NewApp()
	t.Cleanup(a.Quit)
	applyZoomTokens(1)

	style := fyne.TextStyle{Monospace: true}
	msg := strings.Repeat("failed to resolve host example.internal ", 8)
	maxW := fyne.MeasureText("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", fsSmall, style).Width
	lines := wrapToWidth(msg, maxW, fsSmall, style)
	if len(lines) < 2 {
		t.Fatalf("expected wrap, got %d", len(lines))
	}
	for i, line := range lines {
		if fyne.MeasureText(line, fsSmall, style).Width > maxW+1 {
			t.Fatalf("line %d exceeds width: %q", i, line)
		}
	}
}

func TestWrapToWidth_TrailingSpaceDoesNotMakeBlankLine(t *testing.T) {
	a := test.NewApp()
	t.Cleanup(a.Quit)
	applyZoomTokens(1)

	style := fyne.TextStyle{Monospace: true}
	s := "hello world"
	fit := fyne.MeasureText(s, fsSmall, style).Width
	// A trailing space that does not fit must not become a second blank line.
	lines := wrapToWidth(s+" ", fit, fsSmall, style)
	if len(lines) != 1 || lines[0] != s {
		t.Fatalf("got %#v, want %q on one line", lines, s)
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

func TestHugCardStaysContentWidth(t *testing.T) {
	a := test.NewApp()
	t.Cleanup(a.Quit)
	applyZoomTokens(1)
	setLiveTheme(themeTunnelsDark)

	sys := hug(card("System", "Paths this instance is running with.",
		packedKVRows([][2]string{
			{"Base path", "/tmp/tunnels"},
			{"Log file", "tunnels.log"},
		})))
	flow := container.New(&cardFlowLayout{minCol: z(300), maxCol: 3, gap: sp4}, sys)

	width := z(900)
	flow.Resize(fyne.NewSize(width, flow.MinSize().Height))
	flow.Resize(fyne.NewSize(width, flow.MinSize().Height))

	if sys.Size().Width >= width-1 {
		t.Fatalf("system card stretched to %v, window is %v", sys.Size().Width, width)
	}
	natural := sys.MinSize().Width
	if sys.Size().Width > natural+1 {
		t.Fatalf("system card width %v exceeds content %v", sys.Size().Width, natural)
	}
}

func TestSectionHeadPaintsTitleAndRule(t *testing.T) {
	a := test.NewApp()
	t.Cleanup(a.Quit)
	applyZoomTokens(1)
	setLiveTheme(themeTunnelsDark)

	head := sectionHead("DNS").(fyne.Widget)
	var sawText, sawRule bool
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch co := o.(type) {
		case *canvas.Text:
			if co.Text == "DNS" {
				sawText = true
			}
		case *canvas.Rectangle:
			sawRule = true
		case *fyne.Container:
			for _, child := range co.Objects {
				walk(child)
			}
		case fyne.Widget:
			for _, child := range co.CreateRenderer().Objects() {
				walk(child)
			}
		}
	}
	walk(head)
	if !sawText || !sawRule {
		t.Fatalf("section head visible to the renderer: title=%v rule=%v", sawText, sawRule)
	}
}

func TestSectionHeadSpansAllColumns(t *testing.T) {
	a := test.NewApp()
	t.Cleanup(a.Quit)
	applyZoomTokens(1)
	setLiveTheme(themeTunnelsDark)

	head := sectionHead("DNS")
	aCard := card("A", "", vspace(z(40)))
	flow := container.New(&cardFlowLayout{minCol: z(300), maxCol: 3, gap: sp4}, aCard, head)

	width := z(800)
	flow.Resize(fyne.NewSize(width, flow.MinSize().Height))
	flow.Resize(fyne.NewSize(width, flow.MinSize().Height))

	if head.Size().Width < width-1 {
		t.Fatalf("section head width = %v, want full row %v", head.Size().Width, width)
	}
	if head.Position().Y < aCard.Size().Height {
		t.Fatalf("section head should sit below the card, y=%v card h=%v", head.Position().Y, aCard.Size().Height)
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
