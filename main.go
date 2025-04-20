package main

import (
	"log"
	"os"

	"github.com/thushan/inspectre/cmd/inspectre"
	"github.com/thushan/inspectre/internal/version"
)

func main() {
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
	log.SetOutput(os.Stdout)
	vlog := log.New(log.Writer(), "", 0)
	version.PrintVersionInfo(false, vlog)

	if err := inspectre.Execute(); err != nil {
		os.Exit(1)
	}
}
