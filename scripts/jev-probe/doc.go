//go:build !jevprobe

// Package main is the Jev latency and consistency probe. Its code is in
// main.go and builds only with the jevprobe tag, so no CI job runs it; this
// file gives the directory a package without the tag, so go build, go vet and
// go test see a valid (empty) package.
package main

func main() {}
