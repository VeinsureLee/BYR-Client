package main

import (
	_ "embed"
	"log"

	"BYR-Client/internal/desktop"
)

//go:embed resources/icon.png
var appIcon []byte

func main() {
	if err := desktop.Run(appIcon); err != nil {
		log.Fatal(err)
	}
}
