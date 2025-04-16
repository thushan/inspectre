package main

import (
	"github.com/thushan/inspectre/cmd/inspectre"
	"log"
	"os"
)

func main() {
	app := commands.NewApp()

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
