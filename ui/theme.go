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

type palette struct {
	Primary        color.NRGBA
	PrimaryContent color.NRGBA
	Base100        color.NRGBA
	Base200        color.NRGBA
	Base300        color.NRGBA
	Content        color.NRGBA
	Muted          color.NRGBA
	Faint          color.NRGBA
	Success        color.NRGBA
	SuccessSoft    color.NRGBA
	Error          color.NRGBA
	Warning        color.NRGBA
	Info           color.NRGBA
	Hover          color.NRGBA
	Shadow         color.NRGBA
	Dark           bool
}

func hex(r, g, b uint8) color.NRGBA { return color.NRGBA{R: r, G: g, B: b, A: 255} }

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

// Palettes match frontend/src/app.css daisyUI themes.
var palettes = map[string]palette{
	"suzko": {
		Primary: hex(0xd8, 0x2e, 0x2e), PrimaryContent: hex(0xff, 0xff, 0xff),
		Base100: hex(0x18, 0x18, 0x1b), Base200: hex(0x1f, 0x1f, 0x23), Base300: hex(0x27, 0x27, 0x2a),
		Content: hex(0xf4, 0xf4, 0xf5), Muted: hex(0xa1, 0xa1, 0xaa), Faint: hex(0x71, 0x71, 0x7a),
		Success: hex(0x22, 0xc5, 0x5e), SuccessSoft: color.NRGBA{R: 0x22, G: 0xc5, B: 0x5e, A: 28},
		Error: hex(0xef, 0x44, 0x44), Warning: hex(0xf5, 0x9e, 0x0b), Info: hex(0x3b, 0x82, 0xf6),
		Hover: hex(0x27, 0x27, 0x2a), Shadow: color.NRGBA{A: 60}, Dark: true,
	},
	"tunnels": {
		Primary: hex(0x1d, 0x4e, 0xd8), PrimaryContent: hex(0xff, 0xff, 0xff),
		Base100: hex(0xff, 0xff, 0xff), Base200: hex(0xfd, 0xfc, 0xf8), Base300: hex(0xe7, 0xe3, 0xd7),
		Content: hex(0x0a, 0x0a, 0x0a), Muted: hex(0x52, 0x52, 0x52), Faint: hex(0x73, 0x73, 0x73),
		Success: hex(0x15, 0x80, 0x3d), SuccessSoft: color.NRGBA{R: 0x15, G: 0x80, B: 0x3d, A: 22},
		Error: hex(0xdc, 0x26, 0x26), Warning: hex(0xb4, 0x53, 0x09), Info: hex(0x1d, 0x4e, 0xd8),
		Hover: hex(0xf4, 0xf1, 0xe8), Shadow: color.NRGBA{A: 30}, Dark: false,
	},
	"tunnels-dark": {
		Primary: hex(0x3b, 0x82, 0xf6), PrimaryContent: hex(0xff, 0xff, 0xff),
		Base100: hex(0x15, 0x1a, 0x21), Base200: hex(0x0e, 0x11, 0x16), Base300: hex(0x24, 0x2b, 0x35),
		Content: hex(0xe5, 0xe7, 0xeb), Muted: hex(0x9c, 0xa3, 0xaf), Faint: hex(0x6b, 0x72, 0x80),
		Success: hex(0x22, 0xc5, 0x5e), SuccessSoft: color.NRGBA{R: 0x22, G: 0xc5, B: 0x5e, A: 28},
		Error: hex(0xef, 0x44, 0x44), Warning: hex(0xf5, 0x9e, 0x0b), Info: hex(0x3b, 0x82, 0xf6),
		Hover: hex(0x1c, 0x23, 0x2c), Shadow: color.NRGBA{A: 70}, Dark: true,
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
		return p.Base300
	case theme.ColorNameDisabled:
		return p.Muted
	case theme.ColorNamePlaceHolder:
		return p.Faint
	case theme.ColorNamePrimary:
		return p.Primary
	case theme.ColorNameForegroundOnPrimary:
		return p.PrimaryContent
	case theme.ColorNameHyperlink:
		return p.Primary
	case theme.ColorNameInputBackground:
		return p.Base100
	case theme.ColorNameInputBorder:
		return p.Base300
	case theme.ColorNameOverlayBackground:
		return withAlpha(p.Base200, 235)
	case theme.ColorNameMenuBackground, theme.ColorNameHeaderBackground:
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
	case theme.ColorNameHover, theme.ColorNamePressed:
		return p.Hover
	case theme.ColorNameFocus, theme.ColorNameSelection:
		return withAlpha(p.Primary, 48)
	case theme.ColorNameScrollBar:
		return p.Base300
	}
	if p.Dark {
		return theme.DefaultTheme().Color(n, theme.VariantDark)
	}
	return theme.DefaultTheme().Color(n, theme.VariantLight)
}

func (t *appTheme) Font(s fyne.TextStyle) fyne.Resource {
	if s.Monospace {
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
		return 8
	case theme.SizeNameInnerPadding:
		return 6
	case theme.SizeNameText:
		return 13
	case theme.SizeNameHeadingText:
		return 15
	case theme.SizeNameSubHeadingText:
		return 13
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameScrollBar:
		return 8
	case theme.SizeNameInlineIcon:
		return 16
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
