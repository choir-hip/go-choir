package main

import (
	"os"

	"github.com/yusefmosiah/go-choir/internal/autoputer"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "zot-session" {
		os.Exit(autoputer.RunZotSession(os.Stdin, os.Stdout, os.Stderr))
	}
	autoputer.Run()
}
