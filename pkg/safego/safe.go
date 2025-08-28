package safego

import (
	"context"

	"github.com/me2seeks/forge/pkg/goutil"
)

func Go(ctx context.Context, fn func()) {
	go func() {
		defer goutil.Recovery(ctx)

		fn()
	}()
}
