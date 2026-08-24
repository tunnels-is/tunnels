package ui

import (
	_ "embed"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

//go:embed fonts/Inter-Regular.ttf
var interRegular []byte

//go:embed fonts/Inter-SemiBold.ttf
var interSemibold []byte

var (
	resInterRegular  = fyne.NewStaticResource("Inter-Regular.ttf", interRegular)
	resInterSemibold = fyne.NewStaticResource("Inter-SemiBold.ttf", interSemibold)
)

// Design tokens at 100%. Everything the UI measures derives from these, so
// zoom is a single multiplier applied here rather than a canvas transform.
const (
	baseSp1 float32 = 4
	baseSp2 float32 = 8
	baseSp3 float32 = 12
	baseSp4 float32 = 16
	baseSp5 float32 = 20
	baseSp6 float32 = 24
	baseSp8 float32 = 32

	baseRadSm float32 = 6
	baseRadMd float32 = 8
	baseRadLg float32 = 10

	baseFsCaption float32 = 11
	baseFsSmall   float32 = 12
	baseFsBody    float32 = 13
	baseFsLarge   float32 = 15
	baseFsTitle   float32 = 19

	baseRail        float32 = 224
	baseGutter      float32 = 24
	baseRowHeight   float32 = 54
	baseNavRowH     float32 = 36
	baseCtrlHeight  float32 = 32
	baseFormWidth   float32 = 460
	baseSearchWidth float32 = 240
	baseIcon        float32 = 16

	baseSwWidth  float32 = 36
	baseSwHeight float32 = 20
	baseSwKnob   float32 = 14
	baseSegH     float32 = 30
	baseSegInset float32 = 3
	baseLogLevel float32 = 62
	baseLogFn    float32 = 150
)

// radFull is a radius cap, not a measurement, so it never scales.
const radFull float32 = 999

// Live tokens. Recomputed by applyZoomTokens; read everywhere else.
var (
	sp1, sp2, sp3, sp4, sp5, sp6, sp8            float32
	radSm, radMd, radLg                          float32
	fsCaption, fsSmall, fsBody, fsLarge, fsTitle float32
	railWidth, gutter, rowHeight, navRowH        float32
	ctrlHeight, formWidth, searchWidth, iconSize float32
	swWidth, swHeight, swKnob                    float32
	segHeight, segInset, logLevelCol             float32
	logFnCol                                     float32

	// rtPad cancels the padding widget.RichText bakes around its text; it has
	// to track SizeNameInnerPadding exactly, which is sp2.
	rtPad float32
)

// uiZoom is the live zoom multiplier. 1 is 100%.
var uiZoom float32 = 1

// z scales a one-off measurement that has no named token.
func z(n float32) float32 { return n * uiZoom }

// applyZoomTokens recomputes every token for a zoom factor. Callers must
// refresh the theme afterwards so Fyne drops its cached sizes.
func applyZoomTokens(f float32) {
	uiZoom = f

	sp1, sp2, sp3 = z(baseSp1), z(baseSp2), z(baseSp3)
	sp4, sp5, sp6, sp8 = z(baseSp4), z(baseSp5), z(baseSp6), z(baseSp8)

	radSm, radMd, radLg = z(baseRadSm), z(baseRadMd), z(baseRadLg)

	fsCaption, fsSmall = z(baseFsCaption), z(baseFsSmall)
	fsBody, fsLarge, fsTitle = z(baseFsBody), z(baseFsLarge), z(baseFsTitle)

	railWidth, gutter = z(baseRail), z(baseGutter)
	rowHeight, navRowH, ctrlHeight = z(baseRowHeight), z(baseNavRowH), z(baseCtrlHeight)
	formWidth, searchWidth = z(baseFormWidth), z(baseSearchWidth)
	iconSize = z(baseIcon)

	swWidth, swHeight, swKnob = z(baseSwWidth), z(baseSwHeight), z(baseSwKnob)
	segHeight, segInset = z(baseSegH), z(baseSegInset)
	logLevelCol, logFnCol = z(baseLogLevel), z(baseLogFn)

	rtPad = sp2
}

func init() { applyZoomTokens(1) }

// Custom theme names so RichText segments can pick up palette tones and the
// 12px step, which Fyne has no built-in name for.
const (
	colMuted     fyne.ThemeColorName = "tunnelsMuted"
	colFaint     fyne.ThemeColorName = "tunnelsFaint"
	colContent   fyne.ThemeColorName = "tunnelsContent"
	colOnPrimary fyne.ThemeColorName = "tunnelsOnPrimary"
	colOnSolid   fyne.ThemeColorName = "tunnelsOnSolid"
	sizeSmall    fyne.ThemeSizeName  = "tunnelsSmall"
)

type palette struct {
	// Surfaces, dark to light in dark mode (and the reverse in light mode).
	Base100 color.NRGBA // cards, sidebar, inputs
	Base200 color.NRGBA // window background
	Base300 color.NRGBA // borders
	Elevate color.NRGBA // hovered/raised surface
	Divider color.NRGBA // hairlines inside cards
	Input   color.NRGBA // entry/select fill, inset against Base100

	// Text.
	Content color.NRGBA
	Muted   color.NRGBA
	Faint   color.NRGBA

	// Accents. Each has a solid tone plus a low-alpha wash for badges.
	Primary        color.NRGBA
	PrimaryHover   color.NRGBA
	PrimaryContent color.NRGBA
	PrimarySoft    color.NRGBA
	Success        color.NRGBA
	SuccessHover   color.NRGBA
	SuccessSoft    color.NRGBA
	Error          color.NRGBA
	ErrorSoft      color.NRGBA
	Warning        color.NRGBA
	WarningSoft    color.NRGBA
	Info           color.NRGBA
	InfoSoft       color.NRGBA

	Hover  color.NRGBA
	Ring   color.NRGBA
	Shadow color.NRGBA
	Dark   bool
}

func hex(r, g, b uint8) color.NRGBA { return color.NRGBA{R: r, G: g, B: b, A: 255} }

func rgba(r, g, b, a uint8) color.NRGBA { return color.NRGBA{R: r, G: g, B: b, A: a} }

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

// Palettes track frontend/src/app.css daisyUI themes.
var palettes = map[string]palette{
	"tunnels-dark": {
		Base100: hex(0x12, 0x17, 0x1e), Base200: hex(0x0b, 0x0e, 0x13), Base300: hex(0x23, 0x2b, 0x36),
		Elevate: hex(0x18, 0x1e, 0x27), Divider: hex(0x1c, 0x23, 0x2d),
		Input:   hex(0x0a, 0x0d, 0x12),
		Content: hex(0xe8, 0xeb, 0xf0), Muted: hex(0x98, 0xa2, 0xb3), Faint: hex(0x64, 0x70, 0x7f),
		Primary: hex(0x4f, 0x8c, 0xff), PrimaryHover: hex(0x6a, 0x9d, 0xff),
		PrimaryContent: hex(0xff, 0xff, 0xff), PrimarySoft: rgba(0x4f, 0x8c, 0xff, 38),
		Success: hex(0x1c, 0xa9, 0x51), SuccessHover: hex(0x22, 0xbb, 0x5c), SuccessSoft: rgba(0x2f, 0xc4, 0x6d, 38),
		Error: hex(0xe5, 0x48, 0x4d), ErrorSoft: rgba(0xe5, 0x48, 0x4d, 38),
		Warning: hex(0xd9, 0x8b, 0x0c), WarningSoft: rgba(0xf5, 0x9e, 0x0b, 38),
		Info: hex(0x38, 0xbd, 0xf8), InfoSoft: rgba(0x38, 0xbd, 0xf8, 38),
		Hover: hex(0x1b, 0x22, 0x2c), Ring: rgba(0x4f, 0x8c, 0xff, 90), Shadow: rgba(0, 0, 0, 90), Dark: true,
	},
	"tunnels": {
		Base100: hex(0xff, 0xff, 0xff), Base200: hex(0xf7, 0xf6, 0xf1), Base300: hex(0xe1, 0xdd, 0xd0),
		Elevate: hex(0xfa, 0xf8, 0xf3), Divider: hex(0xee, 0xeb, 0xe1),
		Input:   hex(0xf4, 0xf2, 0xea),
		Content: hex(0x16, 0x16, 0x14), Muted: hex(0x56, 0x55, 0x4e), Faint: hex(0x86, 0x84, 0x7b),
		Primary: hex(0x1d, 0x4e, 0xd8), PrimaryHover: hex(0x2b, 0x5d, 0xe8),
		PrimaryContent: hex(0xff, 0xff, 0xff), PrimarySoft: rgba(0x1d, 0x4e, 0xd8, 28),
		Success: hex(0x15, 0x80, 0x3d), SuccessHover: hex(0x18, 0x8f, 0x45), SuccessSoft: rgba(0x15, 0x80, 0x3d, 28),
		Error: hex(0xd0, 0x27, 0x27), ErrorSoft: rgba(0xd0, 0x27, 0x27, 26),
		Warning: hex(0xb4, 0x53, 0x09), WarningSoft: rgba(0xb4, 0x53, 0x09, 26),
		Info: hex(0x1d, 0x4e, 0xd8), InfoSoft: rgba(0x1d, 0x4e, 0xd8, 26),
		Hover: hex(0xf1, 0xee, 0xe4), Ring: rgba(0x1d, 0x4e, 0xd8, 80), Shadow: rgba(0, 0, 0, 34), Dark: false,
	},
	"suzko": {
		Base100: hex(0x1a, 0x1a, 0x1e), Base200: hex(0x12, 0x12, 0x15), Base300: hex(0x2e, 0x2e, 0x34),
		Elevate: hex(0x21, 0x21, 0x27), Divider: hex(0x25, 0x25, 0x2b),
		Input:   hex(0x11, 0x11, 0x14),
		Content: hex(0xf4, 0xf4, 0xf5), Muted: hex(0xa1, 0xa1, 0xaa), Faint: hex(0x6f, 0x6f, 0x78),
		Primary: hex(0xd8, 0x2e, 0x2e), PrimaryHover: hex(0xe6, 0x43, 0x43),
		PrimaryContent: hex(0xff, 0xff, 0xff), PrimarySoft: rgba(0xd8, 0x2e, 0x2e, 42),
		Success: hex(0x1c, 0xa9, 0x51), SuccessHover: hex(0x22, 0xbb, 0x5c), SuccessSoft: rgba(0x2f, 0xc4, 0x6d, 38),
		Error: hex(0xe5, 0x48, 0x4d), ErrorSoft: rgba(0xe5, 0x48, 0x4d, 38),
		Warning: hex(0xd9, 0x8b, 0x0c), WarningSoft: rgba(0xf5, 0x9e, 0x0b, 38),
		Info: hex(0x60, 0xa5, 0xfa), InfoSoft: rgba(0x60, 0xa5, 0xfa, 38),
		Hover: hex(0x24, 0x24, 0x2a), Ring: rgba(0xd8, 0x2e, 0x2e, 96), Shadow: rgba(0, 0, 0, 96), Dark: true,
	},
}

const (
	themeSuzko       = "suzko"
	themeTunnels     = "tunnels"
	themeTunnelsDark = "tunnels-dark"
	defaultThemeName = themeTunnelsDark
)

var live = &appTheme{name: defaultThemeName, pal: palettes[defaultThemeName]}

func pal() palette { return live.pal }

type appTheme struct {
	name string
	pal  palette
}

func (t *appTheme) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	p := t.pal
	switch n {
	case theme.ColorNameBackground:
		return p.Base200
	case theme.ColorNameForeground:
		return p.Content
	case theme.ColorNameButton:
		return p.Base100
	case theme.ColorNameDisabledButton:
		return p.Elevate
	case theme.ColorNameDisabled:
		return p.Faint
	case theme.ColorNamePlaceHolder:
		return p.Faint
	case theme.ColorNamePrimary:
		return p.Primary
	case theme.ColorNameForegroundOnPrimary:
		return p.PrimaryContent
	case theme.ColorNameForegroundOnError, theme.ColorNameForegroundOnSuccess, theme.ColorNameForegroundOnWarning:
		return hex(0xff, 0xff, 0xff)
	case theme.ColorNameHyperlink:
		return p.Primary
	case theme.ColorNameInputBackground:
		return p.Input
	case theme.ColorNameInputBorder:
		return p.Base300
	case theme.ColorNameOverlayBackground:
		return p.Base100
	case theme.ColorNameMenuBackground:
		return p.Base100
	case theme.ColorNameHeaderBackground:
		return p.Base100
	case theme.ColorNameSeparator:
		return p.Base300
	case theme.ColorNameShadow:
		return p.Shadow
	case theme.ColorNameSuccess:
		return p.Success
	case theme.ColorNameError:
		return p.Error
	case theme.ColorNameWarning:
		return p.Warning
	case theme.ColorNameHover:
		return p.Hover
	case theme.ColorNamePressed:
		return p.Elevate
	case theme.ColorNameFocus:
		return p.Ring
	case theme.ColorNameSelection:
		return withAlpha(p.Primary, 60)
	case theme.ColorNameScrollBar:
		return withAlpha(p.Faint, 120)
	case theme.ColorNameScrollBarBackground:
		return color.Transparent
	case colMuted:
		return p.Muted
	case colFaint:
		return p.Faint
	case colContent:
		return p.Content
	case colOnPrimary:
		return p.PrimaryContent
	case colOnSolid:
		return hex(0xff, 0xff, 0xff)
	}
	if p.Dark {
		return theme.DefaultTheme().Color(n, theme.VariantDark)
	}
	return theme.DefaultTheme().Color(n, theme.VariantLight)
}

func (t *appTheme) Font(s fyne.TextStyle) fyne.Resource {
	if s.Monospace || s.Symbol {
		return theme.DefaultTheme().Font(s)
	}
	if s.Bold {
		return resInterSemibold
	}
	return resInterRegular
}

func (t *appTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(n)
}

func (t *appTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNamePadding:
		return sp2
	case theme.SizeNameInnerPadding:
		return sp2
	case theme.SizeNameLineSpacing:
		return sp1
	case theme.SizeNameText:
		return fsBody
	case theme.SizeNameHeadingText:
		return fsTitle
	case theme.SizeNameSubHeadingText:
		return fsLarge
	case theme.SizeNameCaptionText:
		return fsCaption
	case sizeSmall:
		return fsSmall
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameInputRadius, theme.SizeNameButtonRadius, theme.SizeNameSelectionRadius:
		return radSm
	case theme.SizeNameCardRadius:
		return radLg
	case theme.SizeNameDialogRadius, theme.SizeNamePopupRadius, theme.SizeNameMenuRadius:
		return radLg
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameScrollBar:
		return 10
	case theme.SizeNameScrollBarSmall:
		return 4
	case theme.SizeNameScrollBarRadius:
		return radFull
	case theme.SizeNameInlineIcon:
		return iconSize
	}
	return theme.DefaultTheme().Size(n)
}

func setLiveTheme(name string) {
	if _, ok := palettes[name]; !ok {
		name = defaultThemeName
	}
	live = &appTheme{name: name, pal: palettes[name]}
	if fyne.CurrentApp() != nil {
		fyne.CurrentApp().Settings().SetTheme(live)
	}
}
