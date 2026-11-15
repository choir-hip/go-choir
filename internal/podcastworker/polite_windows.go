//go:build windows

package podcastworker

const MaxThreads = 4

// BeAPoliteNeighbour does nothing on this platform.
func BeAPoliteNeighbour() []error { return nil }
