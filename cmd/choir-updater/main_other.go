//go:build !linux

package main

import (
	"fmt"
	"os"
)

func main() {
	_, _ = fmt.Fprintln(os.Stderr, "choir-updater: Linux guest kernel is required")
	os.Exit(1)
}
