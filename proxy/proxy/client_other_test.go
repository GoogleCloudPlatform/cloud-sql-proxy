//go:build !go1.15
// +build !go1.15

package proxy

import (
	"context"
	"errors"
	"strings"
)

func errorIsDeadlineOrTimeout(err error) bool {
	if errors.Is(err, context.Canceled) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if strings.Contains(err.Error(), "i/o timeout") {
		// We should use os.ErrDeadlineExceeded exceeded here,
		// but it is not present in Go versions below 1.15.
		return true
	}
	return false
}
