//go:build !unix

package bundle

import "os"

func mapPack(*os.File, int64) ([]byte, func() error) { return nil, nil }
