package fyneui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
)

var (
	ink       = color.NRGBA{R: 31, G: 43, B: 62, A: 255}
	muted     = color.NRGBA{R: 108, G: 121, B: 139, A: 255}
	accent    = color.NRGBA{R: 43, G: 100, B: 218, A: 255}
	backdrop  = color.NRGBA{R: 243, G: 246, B: 251, A: 255}
	borderCol = color.NRGBA{R: 223, G: 230, B: 240, A: 255}
)

type prototypeTheme struct{}

func (prototypeTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return backdrop
	case theme.ColorNameForeground:
		return ink
	case theme.ColorNamePrimary, theme.ColorNameHyperlink, theme.ColorNameFocus:
		return accent
	case theme.ColorNameInputBackground, theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return color.White
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return borderCol
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return muted
	case theme.ColorNameSelection:
		return color.NRGBA{R: 218, G: 231, B: 255, A: 255}
	case theme.ColorNameButton:
		return color.NRGBA{R: 232, G: 238, B: 248, A: 255}
	}
	return theme.DefaultTheme().Color(name, theme.VariantLight)
}

func (prototypeTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (prototypeTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (prototypeTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 15
	case theme.SizeNamePadding:
		return 7
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 8
	}
	return theme.DefaultTheme().Size(name)
}

func padded(obj fyne.CanvasObject, top, bottom, left, right float32) *fyne.Container {
	return container.New(layout.NewCustomPaddedLayout(top, bottom, left, right), obj)
}

func panel(obj fyne.CanvasObject) *fyne.Container {
	bg := canvas.NewRectangle(color.White)
	bg.CornerRadius = 12
	bg.StrokeColor = borderCol
	bg.StrokeWidth = 1
	return container.NewStack(bg, padded(obj, 12, 12, 14, 14))
}

// fixedLayout deliberately does not propagate a child's unwrapped text width
// into the window minimum, so the same content can fit a phone viewport.
type fixedLayout struct{ minimum fyne.Size }

func (l fixedLayout) MinSize([]fyne.CanvasObject) fyne.Size { return l.minimum }
func (l fixedLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		object.Move(fyne.NewPos(0, 0))
		object.Resize(size)
	}
}

func constrained(obj fyne.CanvasObject, minimum fyne.Size) *fyne.Container {
	return container.New(fixedLayout{minimum: minimum}, obj)
}
