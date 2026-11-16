package main

import (
	_ "embed"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/VeinsureLee/BYR-Client/apps/desktop/internal/bootstrap"
	"github.com/VeinsureLee/BYR-Client/apps/desktop/internal/desktop"
	"github.com/VeinsureLee/BYR-Client/apps/desktop/internal/platform"
	"github.com/VeinsureLee/BYR-Client/core/resources/fonts"
)

// The mirrored BYR butterfly logo. The build embeds the same PNG into the
// executables' resources, so the window, taskbar and dock share one icon.
//
//go:embed resources/icon.png
var appIconPNG []byte

func main() {
	if err := ui.RegisterFont(fonts.Regular, fonts.Family); err != nil {
		log.Fatal(err)
	}
	dataDir, err := platform.DataDir()
	if err != nil {
		log.Fatal(err)
	}
	services, err := bootstrap.New(bootstrap.Config{BaseURL: os.Getenv("BYR_BASE_URL"), Storage: platform.FileState{Path: filepath.Join(dataDir, "state.json")}, Credentials: platform.Credentials{}})
	if err != nil {
		log.Fatal(err)
	}
	mygo.App.WhenReady(func() {
		var window *mygo.Window
		device := &platform.Device{}
		app := desktop.New(services.App, device, func(fn func()) { window.Update(fn) })
		app.SystemDark = mygo.Theme.IsDark
		if version := mygo.App.Version(); version != "" {
			app.Version = version
		}
		window = mygo.NewWindow(mygo.WindowOptions{
			Title:     "BYR Client · " + app.Version,
			Width:     1100,
			Height:    760,
			MinWidth:  960,
			MinHeight: 480,
			Content:   ui.View(app.View),
		})
		if err := window.SetIcon(appIconPNG); err != nil {
			log.Printf("set window icon: %v", err)
		}
		if runtime.GOOS == "darwin" {
			if err := mygo.App.Dock.SetIcon(appIconPNG); err != nil {
				log.Printf("set dock icon: %v", err)
			}
		}
		device.Window = window
		window.OnClosed(app.Close)
		app.Start()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
