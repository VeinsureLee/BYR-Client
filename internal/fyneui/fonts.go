package fyneui

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

// The unmodified SC subset contains the complete upstream Simplified Chinese
// repertoire. Embedding it makes desktop and Android rendering independent of
// fonts installed on the device. See fonts/README.md and fonts/LICENSE.
//
//go:embed fonts/NotoSansSC-Regular.otf
var chineseFontBytes []byte

//go:embed fonts/LICENSE
var fontLicense string

var chineseFont = fyne.NewStaticResource("NotoSansSC-Regular.otf", chineseFontBytes)

type chineseTheme struct{ fyne.Theme }

func fontTheme(base fyne.Theme) fyne.Theme { return &chineseTheme{Theme: base} }

func (t *chineseTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Symbol {
		return t.Theme.Font(style)
	}
	return chineseFont
}
