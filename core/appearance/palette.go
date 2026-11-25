package appearance

import (
	"image/color"
	"strings"
)

type Scheme string

const (
	BYRBlue Scheme = "byr-blue"
	Pine    Scheme = "pine"
	Plum    Scheme = "plum"
)

// Palette carries UI-independent semantic colors so each native renderer can
// map the same appearance values into its own theme and controls.
type Palette struct {
	Background, Surface, SurfaceHover, SurfacePressed color.NRGBA
	Text, TextMuted, TextFaint                        color.NRGBA
	Border, InputBorder                               color.NRGBA
	Accent, AccentHover, AccentPressed                color.NRGBA
	AccentText, Selection                             color.NRGBA
	Danger, DangerSoft                                color.NRGBA
	Success, SuccessSoft                              color.NRGBA
	Warning, WarningSoft                              color.NRGBA
}

func Schemes() []Scheme { return []Scheme{BYRBlue, Pine, Plum} }

func Normalize(s Scheme) Scheme {
	switch Scheme(strings.ToLower(strings.TrimSpace(string(s)))) {
	case BYRBlue:
		return BYRBlue
	case Pine:
		return Pine
	case Plum:
		return Plum
	default:
		return BYRBlue
	}
}

func Name(s Scheme) string {
	switch Normalize(s) {
	case Pine:
		return "松针绿"
	case Plum:
		return "暮紫"
	default:
		return "北邮蓝"
	}
}

// Resolve returns the full semantic palette for one scheme and display mode.
// Neutral and status colors are shared; only the accent family varies by scheme.
func Resolve(scheme Scheme, dark bool) Palette {
	scheme = Normalize(scheme)
	var p Palette
	if dark {
		p = Palette{
			Background:     color.NRGBA{R: 0x12, G: 0x16, B: 0x1C, A: 0xFF},
			Surface:        color.NRGBA{R: 0x1A, G: 0x20, B: 0x28, A: 0xFF},
			SurfaceHover:   color.NRGBA{R: 0x24, G: 0x2D, B: 0x38, A: 0xFF},
			SurfacePressed: color.NRGBA{R: 0x30, G: 0x3B, B: 0x49, A: 0xFF},
			Text:           color.NRGBA{R: 0xED, G: 0xF1, B: 0xF6, A: 0xFF},
			TextMuted:      color.NRGBA{R: 0xBE, G: 0xCA, B: 0xDB, A: 0xFF},
			TextFaint:      color.NRGBA{R: 0xA2, G: 0xAF, B: 0xC0, A: 0xFF},
			Border:         color.NRGBA{R: 0x30, G: 0x39, B: 0x4A, A: 0xFF},
			InputBorder:    color.NRGBA{R: 0x77, G: 0x87, B: 0x9A, A: 0xFF},
			Danger:         color.NRGBA{R: 0xFF, G: 0xB4, B: 0xAA, A: 0xFF},
			DangerSoft:     color.NRGBA{R: 0x38, G: 0x26, B: 0x28, A: 0xFF},
			Success:        color.NRGBA{R: 0x8B, G: 0xD6, B: 0xB3, A: 0xFF},
			SuccessSoft:    color.NRGBA{R: 0x20, G: 0x35, B: 0x2D, A: 0xFF},
			Warning:        color.NRGBA{R: 0xF3, G: 0xCB, B: 0x80, A: 0xFF},
			WarningSoft:    color.NRGBA{R: 0x37, G: 0x2E, B: 0x21, A: 0xFF},
		}
		switch scheme {
		case Pine:
			p.Accent = color.NRGBA{R: 0x94, G: 0xD9, B: 0xB5, A: 0xFF}
			p.AccentText = color.NRGBA{R: 0x14, G: 0x2B, B: 0x20, A: 0xFF}
			p.Selection = color.NRGBA{R: 0x24, G: 0x3E, B: 0x33, A: 0xFF}
		case Plum:
			p.Accent = color.NRGBA{R: 0xD4, G: 0xB5, B: 0xF4, A: 0xFF}
			p.AccentText = color.NRGBA{R: 0x2A, G: 0x1A, B: 0x3D, A: 0xFF}
			p.Selection = color.NRGBA{R: 0x3C, G: 0x2B, B: 0x49, A: 0xFF}
		default:
			p.Accent = color.NRGBA{R: 0xA8, G: 0xC8, B: 0xFF, A: 0xFF}
			p.AccentText = color.NRGBA{R: 0x14, G: 0x2B, B: 0x4B, A: 0xFF}
			p.Selection = color.NRGBA{R: 0x25, G: 0x39, B: 0x54, A: 0xFF}
		}
		p.AccentHover = mix(p.Accent, color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, .08)
		p.AccentPressed = mix(p.Accent, color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, .16)
		return p
	}

	p = Palette{
		Background:     color.NRGBA{R: 0xF4, G: 0xF6, B: 0xF8, A: 0xFF},
		Surface:        color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
		SurfaceHover:   color.NRGBA{R: 0xED, G: 0xF1, B: 0xF5, A: 0xFF},
		SurfacePressed: color.NRGBA{R: 0xE1, G: 0xE7, B: 0xEF, A: 0xFF},
		Text:           color.NRGBA{R: 0x18, G: 0x21, B: 0x2D, A: 0xFF},
		TextMuted:      color.NRGBA{R: 0x46, G: 0x56, B: 0x6A, A: 0xFF},
		TextFaint:      color.NRGBA{R: 0x56, G: 0x66, B: 0x7A, A: 0xFF},
		Border:         color.NRGBA{R: 0xDC, G: 0xE3, B: 0xEB, A: 0xFF},
		InputBorder:    color.NRGBA{R: 0x75, G: 0x85, B: 0x9A, A: 0xFF},
		Danger:         color.NRGBA{R: 0xA3, G: 0x2D, B: 0x29, A: 0xFF},
		DangerSoft:     color.NRGBA{R: 0xFC, G: 0xEC, B: 0xEA, A: 0xFF},
		Success:        color.NRGBA{R: 0x22, G: 0x64, B: 0x56, A: 0xFF},
		SuccessSoft:    color.NRGBA{R: 0xE8, G: 0xF5, B: 0xEE, A: 0xFF},
		Warning:        color.NRGBA{R: 0x85, G: 0x53, B: 0x00, A: 0xFF},
		WarningSoft:    color.NRGBA{R: 0xFF, G: 0xF3, B: 0xD8, A: 0xFF},
	}
	switch scheme {
	case Pine:
		p.Accent = color.NRGBA{R: 0x26, G: 0x65, B: 0x4C, A: 0xFF}
		p.AccentText = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
		p.Selection = color.NRGBA{R: 0xE3, G: 0xF1, B: 0xE9, A: 0xFF}
	case Plum:
		p.Accent = color.NRGBA{R: 0x68, G: 0x46, B: 0x8B, A: 0xFF}
		p.AccentText = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
		p.Selection = color.NRGBA{R: 0xF0, G: 0xE8, B: 0xF8, A: 0xFF}
	default:
		p.Accent = color.NRGBA{R: 0x25, G: 0x5D, B: 0xA8, A: 0xFF}
		p.AccentText = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
		p.Selection = color.NRGBA{R: 0xE5, G: 0xEF, B: 0xFD, A: 0xFF}
	}
	p.AccentHover = mix(p.Accent, color.NRGBA{A: 0xFF}, .10)
	p.AccentPressed = mix(p.Accent, color.NRGBA{A: 0xFF}, .20)
	return p
}

func mix(a, b color.NRGBA, amount float64) color.NRGBA {
	blend := func(x, y uint8) uint8 { return uint8(float64(x)*(1-amount) + float64(y)*amount + .5) }
	return color.NRGBA{R: blend(a.R, b.R), G: blend(a.G, b.G), B: blend(a.B, b.B), A: 0xFF}
}

// Hex returns a CSS-style RGB string for native toolkit constructors.
func Hex(c color.NRGBA) string {
	const digits = "0123456789ABCDEF"
	b := []byte{'#', '0', '0', '0', '0', '0', '0'}
	for i, value := range []uint8{c.R, c.G, c.B} {
		b[1+i*2], b[2+i*2] = digits[value>>4], digits[value&0x0F]
	}
	return string(b)
}
