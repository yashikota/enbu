package main

import (
	"log"

	"github.com/yashikota/enbu/tui"
)

func main() {
	if err := tui.RunDemo(); err != nil {
		log.Fatal(err)
	}
}
