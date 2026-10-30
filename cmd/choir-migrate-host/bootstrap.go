package main

import (
	"context"

	"github.com/yusefmosiah/go-choir/internal/platform"
)

// bootstrapSchemas creates the control and graph schemas by opening the
// platform store, which bootstraps both databases (WP-D). It is the only
// place this command depends on the platform package; --bootstrap=false skips
// it and requires the schemas to exist.
func bootstrapSchemas(ctx context.Context, controlDSN, graphDSN string) error {
	s, err := platform.OpenStore(ctx, controlDSN, graphDSN)
	if err != nil {
		return err
	}
	return s.Close()
}
