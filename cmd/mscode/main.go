package main

import (
	"log"
	"os"

	app "github.com/vigo999/mindspore-code/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
