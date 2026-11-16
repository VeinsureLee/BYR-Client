//go:build !windows

package desktop

import (
	"errors"
	"github.com/egoist/mygo"
)

func nativeSidebarClick(*mygo.Window, string) error {
	return errors.New("native mouse regression is Windows-only")
}
