//go:build windows

package platform

import (
	"syscall"
	"testing"
)

func TestRetryableWindowsFileErrors(t *testing.T) {
	for _, err := range []error{syscall.ERROR_ACCESS_DENIED, errorSharingViolation, errorLockViolation} {
		if !retryableFileError(err) {
			t.Fatalf("%v should be retried", err)
		}
	}
	if retryableFileError(syscall.ENOENT) {
		t.Fatal("missing files must not be retried")
	}
}
