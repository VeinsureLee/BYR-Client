//go:build windows

package desktop

import (
	"fmt"
	"github.com/egoist/mygo"
	"syscall"
	"unsafe"
)

// Send mouse messages only to this test's own native surface. Coordinates are
// DIPs verified against the default-window captures, then converted using the
// window's actual DPI. No foreground input or real account is used.
func nativeSidebarClick(w *mygo.Window, page string) error {
	index := map[string]int{"home": 0, "top": 1, "favorites": 2, "compose": 3, "search": 4, "contacts": 5, "self": 6}
	dll := syscall.NewLazyDLL("user32.dll")
	find := dll.NewProc("FindWindowExW")
	send := dll.NewProc("SendMessageW")
	dpi := dll.NewProc("GetDpiForWindow")
	rect := dll.NewProc("GetClientRect")
	class, _ := syscall.UTF16PtrFromString("MyGoSurface")
	hwnd := w.NativeHandle()
	child, _, _ := find.Call(hwnd, 0, uintptr(unsafe.Pointer(class)), 0)
	if child == 0 {
		return fmt.Errorf("native surface not found")
	}
	d, _, _ := dpi.Call(hwnd)
	scale := float64(d) / 96
	x, y := int(54*scale), int((77+42.5*float64(index[page]))*scale)
	if page == "settings" {
		var r struct{ Left, Top, Right, Bottom int32 }
		ok, _, _ := rect.Call(child, uintptr(unsafe.Pointer(&r)))
		if ok == 0 {
			return fmt.Errorf("client size unavailable")
		}
		y = int(r.Bottom) - int(24*scale)
	}
	point := uintptr(uint16(x)) | uintptr(uint16(y))<<16
	send.Call(child, 0x0200, 0, point)
	send.Call(child, 0x0201, 1, point)
	send.Call(child, 0x0202, 0, point)
	return nil
}
