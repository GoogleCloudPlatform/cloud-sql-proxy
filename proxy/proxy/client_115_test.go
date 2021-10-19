//go:build go1.15
// +build go1.15

package proxy

import (
	"context"
	"errors"
	"os"
)

func errorIsDeadlineOrTimeout(err error) bool {
	if errors.Is(err, context.Canceled) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// os.ErrDeadlineExceeded was added in Go 1.15, hence the build constraints.
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	return false
}
