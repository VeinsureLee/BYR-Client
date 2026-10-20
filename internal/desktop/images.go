package desktop

import (
	"context"
	"time"

	"github.com/egoist/mygo/ui"

	"BYR-Client/internal/domain"
)

type imageState struct {
	bitmap  *ui.Bitmap
	err     string
	loading bool
}

func (u *UI) imageView(c *ui.Context, image domain.Image, maxWidth, maxHeight float32, clickable bool) {
	rawURL := image.URL
	if rawURL == "" {
		return
	}
	if clickable && image.Kind == domain.ImageEmoji {
		maxWidth, maxHeight = 20, 20
	}
	t := c.Theme()
	state := u.images[rawURL]
	if state == nil {
		state = &imageState{}
		u.images[rawURL] = state
	}
	if state.bitmap == nil && !state.loading && state.err == "" {
		u.startImageLoad(rawURL, state)
	}

	switch {
	case state.bitmap != nil:
		imgElement := ui.Image(c, state.bitmap).MaxWidth(maxWidth).MaxHeight(maxHeight).Radius(8).
			Transition(ui.ElementTransition{
				Colors: true, Duration: 160 * time.Millisecond,
				Enter: &ui.Motion{},
			})
		if clickable {
			imgElement.Cursor(ui.CursorPointer).OnClick(func() {
				u.previewImage = image
				u.previewZoom = 0
				u.previewOpen = true
			})
		}
	case state.loading:
		if image.Kind == domain.ImageEmoji {
			ui.Box(c).Size(maxWidth, maxHeight).Radius(6).Background(t.Surface).
				Border(1, t.Border).Center().Children(func() { ui.Spinner(c) })
			return
		}
		width, height := imagePlaceholderSize(image, maxWidth, maxHeight)
		ui.Column(c).Width(width).Height(height).Gap(8).Center().Radius(8).
			Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Spinner(c)
			ui.Text(c, "正在加载图片…").FontSize(u.fs(11)).TextColor(t.TextMuted)
		})
	default:
		if image.Kind == domain.ImageEmoji {
			ui.Box(c).Size(maxWidth, maxHeight).Radius(6).Background(t.Surface).
				Border(1, t.Border).Center().Cursor(ui.CursorPointer).
				Tooltip("表情加载失败：" + state.err).OnClick(func() {
				state.err = ""
				state.loading = false
			}).Children(func() {
				ui.Text(c, "!").FontSize(u.fs(12)).Bold().TextColor(t.Danger)
			})
			return
		}
		ui.Column(c).MaxWidth(maxWidth).Padding(12).Gap(4).Radius(8).
			Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Text(c, "图片加载失败").FontSize(u.fs(11)).Bold().TextColor(t.Danger)
			ui.Text(c, truncate(state.err, 120)).FontSize(u.fs(10)).TextColor(t.TextMuted)
			ui.Button(c, "重试").FontSize(u.fs(11)).OnClick(func() {
				state.err = ""
				state.loading = false
			})
		})
	}
}

func imagePlaceholderSize(image domain.Image, maxWidth, maxHeight float32) (float32, float32) {
	if image.Width > 0 && image.Height > 0 {
		scale := min(maxWidth/float32(image.Width), maxHeight/float32(image.Height), 1)
		return float32(image.Width) * scale, float32(image.Height) * scale
	}
	return maxWidth, min(132, maxHeight)
}

func (u *UI) startImageLoad(rawURL string, state *imageState) {
	application := u.app
	if application == nil {
		return
	}
	win := u.window()
	if win == nil {
		state.err = "窗口尚未就绪"
		return
	}
	state.loading = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		data, err := application.FetchImage(ctx, rawURL)
		win.Update(func() {
			if u.closed {
				return
			}
			state.loading = false
			if err != nil {
				state.err = err.Error()
				return
			}
			bitmap, decodeErr := ui.DecodeBitmap(data)
			if decodeErr != nil {
				state.err = "图片格式无法识别: " + decodeErr.Error()
				return
			}
			state.bitmap = bitmap
			state.err = ""
		})
	}()
}
