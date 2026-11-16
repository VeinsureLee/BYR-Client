//go:build windows

package platform

import (
	"errors"
	"syscall"
)

const (
	errorSharingViolation = syscall.Errno(32)
	errorLockViolation    = syscall.Errno(33)
)

func retryableFileError(err error) bool {
	return errors.Is(err, syscall.ERROR_ACCESS_DENIED) ||
		errors.Is(err, errorSharingViolation) ||
		errors.Is(err, errorLockViolation)
}
