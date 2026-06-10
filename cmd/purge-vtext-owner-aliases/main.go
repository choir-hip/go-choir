package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/yusefmosiah/go-choir/internal/store"
)

func main() {
	ownerID := "universal-wire-platform"
	if len(os.Args) > 1 {
		ownerID = os.Args[1]
	}
	storePath := os.Getenv("RUNTIME_STORE_PATH")
	if storePath == "" {
		log.Fatal("RUNTIME_STORE_PATH is required")
	}

	s, err := store.Open(storePath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer func() { _ = s.Close() }()

	rows, err := s.DeleteVTextAliasesByOwner(context.Background(), ownerID)
	if err != nil {
		log.Fatalf("delete aliases: %v", err)
	}
	fmt.Printf("deleted %d aliases for owner %s\n", rows, ownerID)
}
