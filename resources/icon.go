// Package resources shares the embedded artwork used by the desktop shell and UI.
package resources

import _ "embed"

// Icon is the application logo used by the window, taskbar and interface.
//
//go:embed icon.png
var Icon []byte
