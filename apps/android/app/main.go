package main

import (
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/VeinsureLee/BYR-Client/apps/android/app/internal/ui"
)

var version = "0.4.0"

func main() {
	go func() {
		window := new(app.Window)
		window.Option(app.Title("BYR Client"))
		// The client is designed for phone screens; keep the activity portrait
		// so it never renders as a stretched desktop-style landscape page.
		window.Option(app.PortraitOrientation.Option())
		// Ignored on mobile; gives a phone-sized window for desktop previews.
		window.Option(app.Size(unit.Dp(420), unit.Dp(820)))
		view, err := ui.New(window)
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
			case app.FrameEvent:
				context := app.NewContext(&operations, event)
				view.Layout(context)
				event.Frame(context.Ops)
			}
		}
	}()
	app.Main()
}
