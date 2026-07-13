package main

import (
	"os"

	"github.com/yusefmosiah/go-choir/internal/sandbox"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "zot-session" {
		os.Exit(sandbox.RunZotSession(os.Stdin, os.Stdout, os.Stderr))
	}
	sandbox.Run()
}
