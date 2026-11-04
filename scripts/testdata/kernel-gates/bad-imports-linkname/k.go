package k

import _ "unsafe"

//go:linkname hidden github.com/yusefmosiah/go-choir/internal/store.hidden
func hidden()

func Run() { hidden() }
