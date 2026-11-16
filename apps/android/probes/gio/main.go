package main

import (
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/op"

	"github.com/VeinsureLee/BYR-Client/apps/android/probes/gio/internal/probe"
)

func main() {
	go func() {
		window := new(app.Window)
		window.Option(app.Title("BYR Gio 技术验证"))
		if err := run(window); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(window *app.Window) error {
	view, err := probe.New()
	if err != nil {
		return err
	}
	var operations op.Ops
	for {
		switch event := window.Event().(type) {
		case app.DestroyEvent:
			return event.Err
		case app.FrameEvent:
			context := app.NewContext(&operations, event)
			view.Layout(context)
			event.Frame(context.Ops)
		}
	}
}
