//go:build !windows

package agent

import (
	"errors"
	"syscall"
)

func isPTYEOF(err error) bool {
	return errors.Is(err, syscall.EIO)
}
