package ui

import "testing"

func TestBufferSizeOKAt125Percent(t *testing.T) {
	const num = 150 // 125%
	// logical % 4 == 0 or 1: GLFW trunc matches compositor round
	for _, w := range []int{800, 801, 804, 805, 1200} {
		if !bufferSizeOK(w, num) {
			t.Fatalf("%d should be sharp at 125%% (glfw=%d compositor=%d)",
				w, glfwFramebuffer(w, num), compositorFramebuffer(w, num))
		}
	}
	// logical % 4 == 2 or 3: 0.5+ fractional pixel, compositor stretches
	for _, w := range []int{802, 803, 806, 807} {
		if bufferSizeOK(w, num) {
			t.Fatalf("%d should be blurry at 125%% (glfw=%d compositor=%d)",
				w, glfwFramebuffer(w, num), compositorFramebuffer(w, num))
		}
	}
}

func TestAlignLogicalGrowsToNextSharpSize(t *testing.T) {
	const num = 150
	if got := alignLogical(802, num, true); got != 804 {
		t.Fatalf("grow 802: got %d, want 804", got)
	}
	if got := alignLogical(803, num, true); got != 804 {
		t.Fatalf("grow 803: got %d, want 804", got)
	}
	if got := alignLogical(801, num, true); got != 801 {
		t.Fatalf("already sharp 801: got %d", got)
	}
}

func TestAlignLogicalShrinksToPrevSharpSize(t *testing.T) {
	const num = 150
	if got := alignLogical(802, num, false); got != 801 {
		t.Fatalf("shrink 802: got %d, want 801", got)
	}
	if got := alignLogical(803, num, false); got != 801 {
		t.Fatalf("shrink 803: got %d, want 801", got)
	}
}

func TestAlignLogicalNoOpAtIntegerScale(t *testing.T) {
	for _, w := range []int{800, 801, 802, 803} {
		if got := alignLogical(w, 120, true); got != w {
			t.Fatalf("100%% %d: got %d", w, got)
		}
		if got := alignLogical(w, 240, true); got != w {
			t.Fatalf("200%% %d: got %d", w, got)
		}
	}
}

func TestScaleToNumerator(t *testing.T) {
	if n := scaleToNumerator(1.25); n != 150 {
		t.Fatalf("1.25 → %d, want 150", n)
	}
	if n := scaleToNumerator(1); n != 120 {
		t.Fatalf("1 → %d, want 120", n)
	}
	if n := scaleToNumerator(1.5); n != 180 {
		t.Fatalf("1.5 → %d, want 180", n)
	}
}

func TestAlignLogical150PercentOdds(t *testing.T) {
	const num = 180 // 150%
	if bufferSizeOK(800, num) != true {
		t.Fatal("800 even should be sharp at 150%")
	}
	if bufferSizeOK(801, num) {
		t.Fatal("801 odd should be blurry at 150%")
	}
	if got := alignLogical(801, num, true); got != 802 {
		t.Fatalf("grow 801: got %d, want 802", got)
	}
}
