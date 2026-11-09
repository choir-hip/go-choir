//go:build !linux

package main

// The guest is Linux; elsewhere the helper measures and drops nothing.

func fileResident(string, int64) int64 { return 0 }
func dropFile(string)                  {}
func syncAll()                         {}
