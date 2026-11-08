// Command callseam is a product-shaped binary that calls each gate seam. The
// seams panic outside a test binary (FM10); gatefixture_test.go runs this and
// expects the panic.
package main

import (
	"os"

	"github.com/yusefmosiah/go-choir/internal/disclosure"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

func main() {
	switch os.Args[1] {
	case "objectgraph":
		objectgraph.SetModeForTest(objectgraph.WriterGateObserve)
	case "disclosure":
		disclosure.SetModeForTest(disclosure.ModeObserve)
	}
	os.Stdout.WriteString("seam returned\n")
}
