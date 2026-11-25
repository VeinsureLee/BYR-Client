package appearance

import (
	"image/color"
	"math"
	"testing"
)

func TestResolvedSchemesMatchDesignTokensAndContrast(t *testing.T) {
	want := map[Scheme]struct {
		lightAccent, lightText, lightSelection color.NRGBA
		darkAccent, darkText, darkSelection    color.NRGBA
	}{
		BYRBlue: {
			lightAccent:    color.NRGBA{R: 0x25, G: 0x5D, B: 0xA8, A: 0xFF},
			lightText:      color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
			lightSelection: color.NRGBA{R: 0xE5, G: 0xEF, B: 0xFD, A: 0xFF},
			darkAccent:     color.NRGBA{R: 0xA8, G: 0xC8, B: 0xFF, A: 0xFF},
			darkText:       color.NRGBA{R: 0x14, G: 0x2B, B: 0x4B, A: 0xFF},
			darkSelection:  color.NRGBA{R: 0x25, G: 0x39, B: 0x54, A: 0xFF},
		},
		Pine: {
			lightAccent:    color.NRGBA{R: 0x26, G: 0x65, B: 0x4C, A: 0xFF},
			lightText:      color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
			lightSelection: color.NRGBA{R: 0xE3, G: 0xF1, B: 0xE9, A: 0xFF},
			darkAccent:     color.NRGBA{R: 0x94, G: 0xD9, B: 0xB5, A: 0xFF},
			darkText:       color.NRGBA{R: 0x14, G: 0x2B, B: 0x20, A: 0xFF},
			darkSelection:  color.NRGBA{R: 0x24, G: 0x3E, B: 0x33, A: 0xFF},
		},
		Plum: {
			lightAccent:    color.NRGBA{R: 0x68, G: 0x46, B: 0x8B, A: 0xFF},
			lightText:      color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
			lightSelection: color.NRGBA{R: 0xF0, G: 0xE8, B: 0xF8, A: 0xFF},
			darkAccent:     color.NRGBA{R: 0xD4, G: 0xB5, B: 0xF4, A: 0xFF},
			darkText:       color.NRGBA{R: 0x2A, G: 0x1A, B: 0x3D, A: 0xFF},
			darkSelection:  color.NRGBA{R: 0x3C, G: 0x2B, B: 0x49, A: 0xFF},
		},
	}
	if got := Schemes(); len(got) != 3 || got[0] != BYRBlue || got[1] != Pine || got[2] != Plum {
		t.Fatalf("schemes = %v", got)
	}
	for scheme, tokens := range want {
		light, dark := Resolve(scheme, false), Resolve(scheme, true)
		if light.Background != (color.NRGBA{R: 0xF4, G: 0xF6, B: 0xF8, A: 0xFF}) ||
			light.SurfaceHover != (color.NRGBA{R: 0xED, G: 0xF1, B: 0xF5, A: 0xFF}) ||
			light.Text != (color.NRGBA{R: 0x18, G: 0x21, B: 0x2D, A: 0xFF}) ||
			light.InputBorder != (color.NRGBA{R: 0x75, G: 0x85, B: 0x9A, A: 0xFF}) {
			t.Errorf("%s light neutral tokens differ from the design table", scheme)
		}
		if dark.Background != (color.NRGBA{R: 0x12, G: 0x16, B: 0x1C, A: 0xFF}) ||
			dark.Surface != (color.NRGBA{R: 0x1A, G: 0x20, B: 0x28, A: 0xFF}) ||
			dark.Text != (color.NRGBA{R: 0xED, G: 0xF1, B: 0xF6, A: 0xFF}) ||
			dark.InputBorder != (color.NRGBA{R: 0x77, G: 0x87, B: 0x9A, A: 0xFF}) {
			t.Errorf("%s dark neutral tokens differ from the design table", scheme)
		}
		if light.Accent != tokens.lightAccent || light.AccentText != tokens.lightText || light.Selection != tokens.lightSelection {
			t.Errorf("%s light tokens = accent:%v text:%v selection:%v", scheme, light.Accent, light.AccentText, light.Selection)
		}
		if dark.Accent != tokens.darkAccent || dark.AccentText != tokens.darkText || dark.Selection != tokens.darkSelection {
			t.Errorf("%s dark tokens = accent:%v text:%v selection:%v", scheme, dark.Accent, dark.AccentText, dark.Selection)
		}
		for _, background := range []color.NRGBA{light.Accent, light.AccentHover, light.AccentPressed} {
			if contrast := ratio(light.AccentText, background); contrast < 6.53 {
				t.Errorf("%s light button text contrast = %.2f", scheme, contrast)
			}
		}
		for _, background := range []color.NRGBA{dark.Accent, dark.AccentHover, dark.AccentPressed} {
			if contrast := ratio(dark.AccentText, background); contrast < 6.53 {
				t.Errorf("%s dark button text contrast = %.2f", scheme, contrast)
			}
		}
		if ratio(light.Accent, light.Selection) < 5.63 || ratio(dark.Accent, dark.Selection) < 5.63 {
			t.Errorf("%s selected-state text contrast below 5.63", scheme)
		}
		if ratio(light.Text, light.Surface) < 4.5 || ratio(light.TextMuted, light.Surface) < 4.5 || ratio(light.TextFaint, light.Surface) < 4.5 {
			t.Errorf("%s light text contrast below 4.5", scheme)
		}
		if ratio(dark.Text, dark.Surface) < 14.45 || ratio(dark.TextMuted, dark.Surface) < 4.5 || ratio(dark.TextFaint, dark.Background) < 4.5 {
			t.Errorf("%s dark text contrast below design minimum", scheme)
		}
		if ratio(light.InputBorder, light.Surface) < 3 || ratio(dark.InputBorder, dark.Surface) < 3 {
			t.Errorf("%s input focus border contrast below 3", scheme)
		}
	}
}

func TestSchemeNormalizationFallsBackSafely(t *testing.T) {
	for input, want := range map[Scheme]Scheme{
		"": BYRBlue, "unknown": BYRBlue, " Pine ": Pine, "PLUM": Plum,
	} {
		if got := Normalize(input); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
	if Name(Pine) != "松针绿" || Name(Plum) != "暮紫" || Name(BYRBlue) != "北邮蓝" {
		t.Fatalf("scheme labels = %q/%q/%q", Name(BYRBlue), Name(Pine), Name(Plum))
	}
	if Hex(color.NRGBA{R: 0x25, G: 0x5D, B: 0xA8, A: 0xFF}) != "#255DA8" {
		t.Fatal("Hex did not return the palette's RGB value")
	}
}

func ratio(a, b color.NRGBA) float64 {
	l1, l2 := luminance(a), luminance(b)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + .05) / (l2 + .05)
}

func luminance(c color.NRGBA) float64 {
	linear := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= .04045 {
			return x / 12.92
		}
		return pow((x+.055)/1.055, 2.4)
	}
	return .2126*linear(c.R) + .7152*linear(c.G) + .0722*linear(c.B)
}

func pow(x, n float64) float64 {
	// The exponent is fixed at 2.4; math.Pow keeps the contrast helper explicit.
	return math.Pow(x, n)
}
