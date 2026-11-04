package k

import (
	"context"

	"github.com/yusefmosiah/go-choir/internal/store"
)

var _ = store.Open

func Run(ctx context.Context) error { return ctx.Err() }
