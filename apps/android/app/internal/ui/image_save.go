package ui

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

type imageSaveWindow interface {
	SaveImage(context.Context, string, []byte) (string, error)
}

func imageSaveFilename(rawURL string, data []byte, now time.Time) string {
	ext := imageDataExtension(data)
	if ext == "" {
		if parsed, err := url.Parse(rawURL); err == nil {
			ext = strings.ToLower(filepath.Ext(parsed.Path))
		}
	}
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
	default:
		ext = ".img"
	}
	return "BYR-" + now.Format("20060102-150405.000") + ext
}

func imageDataExtension(data []byte) string {
	switch {
	case len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n":
		return ".png"
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		return ".gif"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return ".webp"
	case len(data) >= 2 && data[0] == 0xff && data[1] == 0xd8:
		return ".jpg"
	case len(data) >= 2 && string(data[:2]) == "BM":
		return ".bmp"
	default:
		return ""
	}
}

func (a *App) saveImageViewer() {
	if a.busy {
		return
	}
	entry := a.images[a.imageViewer]
	if a.imageViewer == "" || entry == nil {
		a.fail(errors.New("没有可保存的图片"))
		return
	}
	if entry.loading {
		a.notice("图片仍在加载")
		return
	}
	if entry.err != nil || len(entry.raw) == 0 {
		a.fail(errors.New("无法保存原图"))
		return
	}
	saver, ok := a.window.(imageSaveWindow)
	if !ok {
		a.fail(errors.New("当前设备不支持保存图片"))
		return
	}
	name := imageSaveFilename(a.imageViewer, entry.raw, time.Now())
	data := append([]byte(nil), entry.raw...)
	a.run("保存图片", func(ctx context.Context) (func(), error) {
		path, err := saver.SaveImage(ctx, name, data)
		if err != nil {
			return nil, err
		}
		return func() { a.notice("图片已保存到：" + path) }, nil
	})
}
