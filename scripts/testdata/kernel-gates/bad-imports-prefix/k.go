package k

import (
	"context"

	"github.com/yusefmosiah/go-choir/internal/kernel2"
)

var _ = kernel2.X

func Run(ctx context.Context) error { return ctx.Err() }
