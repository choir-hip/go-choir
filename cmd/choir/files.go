// Owner files surface (P5-roster overlay install path).
//
// `choir files mkdir|put|get` speak the autoputer files API
// (/api/files/{path}) with the same owner-scoped Bearer auth as every other
// verb; the commands are declared in the registry (commands.go).

package main

import "io"

// runFiles runs `choir files <args>` through the registry.
func runFiles(args []string, stdout, stderr io.Writer) int {
	return run(append([]string{"files"}, args...), stdout, stderr)
}
