//go:build !windows

package platform

func retryableFileError(error) bool { return false }
