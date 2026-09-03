package ui

import (
	"testing"

	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
)

func TestApplyZoomTokensSnapsTypeAndIcons(t *testing.T) {
	t.Cleanup(func() { applyZoomTokens(1) })

	applyZoomTokens(1)
	if fsCaption != 11 || fsSmall != 12 || fsBody != 13 || fsLarge != 15 || fsTitle != 19 {
		t.Fatalf("100%% type: caption=%v small=%v body=%v large=%v title=%v",
			fsCaption, fsSmall, fsBody, fsLarge, fsTitle)
	}
	if iconSize != 16 {
		t.Fatalf("100%% iconSize=%v, want 16", iconSize)
	}

	applyZoomTokens(1.1)
	// 11*1.1=12.1→12, 12*1.1=13.2→13, 13*1.1=14.3→14, 15*1.1=16.5→17, 19*1.1=20.9→21, 16*1.1=17.6→18
	if fsCaption != 12 || fsSmall != 13 || fsBody != 14 || fsLarge != 17 || fsTitle != 21 {
		t.Fatalf("110%% type: caption=%v small=%v body=%v large=%v title=%v",
			fsCaption, fsSmall, fsBody, fsLarge, fsTitle)
	}
	if iconSize != 18 {
		t.Fatalf("110%% iconSize=%v, want 18", iconSize)
	}
}

func TestIconImageUsesPixelScale(t *testing.T) {
	img := iconImage(theme.ConfirmIcon())
	if img.FillMode != canvas.ImageFillContain {
		t.Fatalf("FillMode=%v, want Contain", img.FillMode)
	}
	if img.ScaleMode != canvas.ImageScalePixels {
		t.Fatalf("ScaleMode=%v, want Pixels", img.ScaleMode)
	}
}
