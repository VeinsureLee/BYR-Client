package main

import (
	"log"

	"BYR-Client/internal/desktop"
	"BYR-Client/resources"
)

func main() {
	if err := desktop.Run(resources.Icon); err != nil {
		log.Fatal(err)
	}
}
