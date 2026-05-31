// Package theme exposes the UI palette and spacing scale. Supports light
// and dark variants; SetMode swaps the active token values.
package theme

import (
	"image/color"

	"gioui.org/widget/material"
)

// Tokens is the full palette for one variant.
//
// Gray hierarchy: TextPrimary > TextSecondary > HelpText > TextMuted.
// User content stays at TextPrimary; field labels at TextSecondary;
// long help paragraphs at HelpText; placeholders at TextMuted.
type Tokens struct {
	Background      color.NRGBA
	Surface         color.NRGBA
	Chrome          color.NRGBA
	TextPrimary     color.NRGBA
	TextSecondary   color.NRGBA
	HelpText        color.NRGBA
	TextMuted       color.NRGBA
	Accent          color.NRGBA
	Divider         color.NRGBA
	InputLine       color.NRGBA
	StatusConnected color.NRGBA
	StatusError     color.NRGBA
}

// Light palette — warm aged-paper tones.
var Light = Tokens{
	Background:      color.NRGBA{R: 0xF5, G: 0xF1, B: 0xE8, A: 0xFF},
	Surface:         color.NRGBA{R: 0xEF, G: 0xE9, B: 0xDC, A: 0xFF},
	Chrome:          color.NRGBA{R: 0xEF, G: 0xE9, B: 0xDC, A: 0xFF},
	TextPrimary:     color.NRGBA{R: 0x2A, G: 0x26, B: 0x22, A: 0xFF},
	TextSecondary:   color.NRGBA{R: 0x6B, G: 0x62, B: 0x59, A: 0xFF},
	HelpText:        color.NRGBA{R: 0x83, G: 0x7A, B: 0x6F, A: 0xFF},
	TextMuted:       color.NRGBA{R: 0xB0, G: 0xA4, B: 0x98, A: 0xFF},
	Accent:          color.NRGBA{R: 0x7D, G: 0x6B, B: 0x4F, A: 0xFF},
	Divider:         color.NRGBA{R: 0xD4, G: 0xCD, B: 0xB8, A: 0xFF},
	InputLine:       color.NRGBA{R: 0xC9, G: 0xC0, B: 0xA8, A: 0xFF},
	StatusConnected: color.NRGBA{R: 0x7A, G: 0x8B, B: 0x6F, A: 0xFF},
	StatusError:     color.NRGBA{R: 0xA8, G: 0x6B, B: 0x5C, A: 0xFF},
}

// Dark palette — ink on dark paper, same warm hue as Light.
var Dark = Tokens{
	Background:      color.NRGBA{R: 0x1A, G: 0x17, B: 0x14, A: 0xFF},
	Surface:         color.NRGBA{R: 0x21, G: 0x1D, B: 0x19, A: 0xFF},
	Chrome:          color.NRGBA{R: 0x21, G: 0x1D, B: 0x19, A: 0xFF},
	TextPrimary:     color.NRGBA{R: 0xE8, G: 0xE2, B: 0xD4, A: 0xFF},
	TextSecondary:   color.NRGBA{R: 0xA8, G: 0x9F, B: 0x92, A: 0xFF},
	HelpText:        color.NRGBA{R: 0x86, G: 0x7C, B: 0x70, A: 0xFF},
	TextMuted:       color.NRGBA{R: 0x5A, G: 0x53, B: 0x4B, A: 0xFF},
	Accent:          color.NRGBA{R: 0xB8, G: 0xA5, B: 0x84, A: 0xFF},
	Divider:         color.NRGBA{R: 0x2E, G: 0x29, B: 0x24, A: 0xFF},
	InputLine:       color.NRGBA{R: 0x3A, G: 0x35, B: 0x30, A: 0xFF},
	StatusConnected: color.NRGBA{R: 0x9A, G: 0xAB, B: 0x8F, A: 0xFF},
	StatusError:     color.NRGBA{R: 0xC8, G: 0x8B, B: 0x7C, A: 0xFF},
}

// Active theme variables — package-level so screens can read them directly.
var (
	Background      color.NRGBA
	Surface         color.NRGBA
	Chrome          color.NRGBA
	TextPrimary     color.NRGBA
	TextSecondary   color.NRGBA
	HelpText        color.NRGBA
	TextMuted       color.NRGBA
	Accent          color.NRGBA
	Divider         color.NRGBA
	InputLine       color.NRGBA
	StatusConnected color.NRGBA
	StatusError     color.NRGBA

	current = "light"
)

func init() {
	SetMode("light")
}

// SetMode activates the given variant ("light" or "dark"). Anything else
// is treated as "light".
func SetMode(mode string) {
	var t Tokens
	if mode == "dark" {
		t = Dark
		current = "dark"
	} else {
		t = Light
		current = "light"
	}
	Background = t.Background
	Surface = t.Surface
	Chrome = t.Chrome
	TextPrimary = t.TextPrimary
	TextSecondary = t.TextSecondary
	HelpText = t.HelpText
	TextMuted = t.TextMuted
	Accent = t.Accent
	Divider = t.Divider
	InputLine = t.InputLine
	StatusConnected = t.StatusConnected
	StatusError = t.StatusError
}

// Mode returns the active variant: "light" or "dark".
func Mode() string {
	return current
}

// Apply pushes the active palette into the material.Theme. Call after
// SetMode so material widgets pick up the new colors.
func Apply(th *material.Theme) {
	th.Palette = material.Palette{
		Bg:         Background,
		Fg:         TextPrimary,
		ContrastBg: Accent,
		ContrastFg: Background,
	}
}
