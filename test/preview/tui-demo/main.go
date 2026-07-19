package main

import (
	"log"

	"github.com/enbu-net/enbu/tui"
)

func main() {
	if err := tui.RunDemo(); err != nil {
		log.Fatal(err)
	}
}
