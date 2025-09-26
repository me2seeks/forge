package execute

import (
	"context"
	"fmt"
	"runtime/debug"
)

func RunWithContextDone(ctx context.Context, fn func() error) error {
	errChan := make(chan error, 1)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				errChan <- fmt.Errorf("exec func panic, %v \n %s", err, debug.Stack())
			}
			close(errChan)
		}()
		err := fn()
		errChan <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}
