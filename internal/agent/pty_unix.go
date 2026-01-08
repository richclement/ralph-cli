//go:build !windows

package agent

import (
	"errors"
	"os"
	"syscall"
)

func isPTYEOF(err error) bool {
	return errors.Is(err, syscall.EIO) || errors.Is(err, os.ErrClosed)
}
