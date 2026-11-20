package main

import (
	"context"
	"image/color"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/VeinsureLee/BYR-Client/apps/android/app/internal/platform"
	"github.com/VeinsureLee/BYR-Client/apps/android/app/internal/ui"
)

var version = "0.6.3"

type appWindow struct{ *app.Window }

func (w appWindow) SetSystemBarColor(c color.NRGBA) {
	w.Option(app.StatusColor(c), app.NavigationColor(c))
}

func (appWindow) SaveImage(ctx context.Context, name string, data []byte) (string, error) {
	return platform.SaveImage(ctx, name, data)
}

func main() {
	go func() {
		window := new(app.Window)
		window.Option(app.Title("BYR Client"))
		// Keep system bars on the same surfaces as the Gio shell. Insets still
		// come exclusively from app.NewContext; these options do not add space.
		window.Option(app.StatusColor(color.NRGBA{R: 0xF4, G: 0xF7, B: 0xFC, A: 0xFF}))
		window.Option(app.NavigationColor(color.NRGBA{R: 0xF4, G: 0xF7, B: 0xFC, A: 0xFF}))
		// The client is designed for phone screens; keep the activity portrait
		// so it never renders as a stretched desktop-style landscape page.
		window.Option(app.PortraitOrientation.Option())
		// Ignored on mobile; gives a phone-sized window for desktop previews.
		window.Option(app.Size(unit.Dp(420), unit.Dp(820)))
		view, err := ui.New(appWindow{window})
		if err != nil {
			log.Print(err)
			os.Exit(1)
		}
		view.SetVersion(version)
		var operations op.Ops
		for {
			switch event := window.Event().(type) {
			case app.DestroyEvent:
				view.Close()
				if event.Err != nil {
					log.Print(event.Err)
				}
				os.Exit(0)
			case app.ConfigEvent:
				// Android reports resume as a focus change. Force a fresh frame
				// so a resumed activity does not keep an empty or stale surface.
				if event.Config.Focused {
					window.Invalidate()
				}
			case app.FrameEvent:
				context := app.NewContext(&operations, event)
				view.Layout(context)
				event.Frame(context.Ops)
			}
		}
	}()
	app.Main()
}
