package main

import (
	"os"

	"github.com/yusefmosiah/go-choir/internal/zot"
)

func main() {
	os.Exit(zot.RunSession(zot.SessionConfig{
		SessionID: os.Getenv("ZOT_SESSION_ID"),
		RootDir:   os.Getenv("ZOT_ROOT_DIR"),
		UserID:    os.Getenv("ZOT_USER_ID"),
	}, os.Stdin, os.Stdout, os.Stderr))
}
