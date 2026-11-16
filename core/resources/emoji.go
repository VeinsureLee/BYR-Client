package resources

import (
	"embed"
	"io/fs"
)

// Emoji are the original 201 GIFs, shared as resources rather than widgets.
//
//go:embed em/*.gif
var Emoji embed.FS

func EmojiNames() []string {
	entries, _ := fs.ReadDir(Emoji, "em")
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}
