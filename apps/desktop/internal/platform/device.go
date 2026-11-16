package platform

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// Device methods run on the task goroutine; dialogs must not block UI frames.
type Device struct{ Window *mygo.Window }

func (d Device) Copy(text string) error { mygo.Clipboard.WriteText(text); return nil }
func (d Device) Open(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("仅支持 HTTP/HTTPS 链接")
	}
	return mygo.Shell.OpenExternal(raw)
}
func (d Device) Pick(ctx context.Context) (string, []byte, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: d.Window, Title: "选择附件"})
	if err != nil || len(paths) == 0 {
		return "", nil, err
	}
	if err = ctx.Err(); err != nil {
		return "", nil, err
	}
	info, err := os.Stat(paths[0])
	if err != nil {
		return "", nil, err
	}
	if info.Size() > 32<<20 {
		return "", nil, errors.New("附件超过 32 MiB")
	}
	b, err := os.ReadFile(paths[0])
	return filepath.Base(paths[0]), b, err
}
func (d Device) Save(name string, data []byte) error {
	p, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: d.Window, Title: "保存附件", DefaultPath: filepath.Base(name)})
	if err != nil || p == "" {
		return err
	}
	return os.WriteFile(p, data, 0600)
}
