//go:build !linux

package shotsandbox

func platformProbe(kind, arg string) (Result, bool) { return Result{}, false }
