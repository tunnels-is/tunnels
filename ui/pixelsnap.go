package ui

import (
	"math"

	"fyne.io/fyne/v2"
)

// Wayland wp-fractional-scale-v1 reports scale in 120ths. GLFW sizes the
// EGL buffer with integer truncation:
//
//	fb = (logical * numerator) / 120
//
// The compositor sizes the viewport with round-half-away-from-zero:
//
//	fb = (logical * numerator + 60) / 120
//
// When those differ by a pixel the compositor bilinear-scales the whole
// window — fonts look sharp, then muddy, then sharp again as the edge is
// dragged. alignLogical picks the nearest logical size where they agree,
// walking in the direction the user is already resizing.
const waylandScaleBase = 120

func scaleToNumerator(scale float32) int {
	n := int(math.Round(float64(scale) * waylandScaleBase))
	if n <= 0 {
		return waylandScaleBase
	}
	return n
}

func glfwFramebuffer(logical, numerator int) int {
	if logical <= 0 {
		return 0
	}
	return logical * numerator / waylandScaleBase
}

func compositorFramebuffer(logical, numerator int) int {
	if logical <= 0 {
		return 0
	}
	return (logical*numerator + waylandScaleBase/2) / waylandScaleBase
}

func bufferSizeOK(logical, numerator int) bool {
	return glfwFramebuffer(logical, numerator) == compositorFramebuffer(logical, numerator)
}

func alignLogical(logical, numerator int, growing bool) int {
	if logical <= 0 || numerator%waylandScaleBase == 0 || bufferSizeOK(logical, numerator) {
		return logical
	}
	if growing {
		for d := 1; d <= waylandScaleBase; d++ {
			if bufferSizeOK(logical+d, numerator) {
				return logical + d
			}
		}
		return logical
	}
	for d := 1; d < logical && d <= waylandScaleBase; d++ {
		if bufferSizeOK(logical-d, numerator) {
			return logical - d
		}
	}
	return logical
}

func canvasTexScale(c fyne.Canvas) float32 {
	if c == nil {
		return 1
	}
	s := c.Scale()
	if s <= 0 {
		s = 1
	}
	const probe float32 = waylandScaleBase
	px, _ := c.PixelCoordinateForPosition(fyne.NewPos(probe, 0))
	tex := float32(px) / (probe * s)
	if tex < 0.5 {
		return 1
	}
	return tex
}

func alignCanvasSize(c fyne.Canvas, size, prev fyne.Size) fyne.Size {
	if c == nil {
		return size
	}
	s := c.Scale()
	if s <= 0 {
		s = 1
	}
	num := scaleToNumerator(canvasTexScale(c))
	sw := int(math.Ceil(float64(size.Width * s)))
	sh := int(math.Ceil(float64(size.Height * s)))
	aw := alignLogical(sw, num, size.Width >= prev.Width)
	ah := alignLogical(sh, num, size.Height >= prev.Height)
	if aw == sw && ah == sh {
		return size
	}
	return fyne.NewSize(float32(aw)/s, float32(ah)/s)
}
