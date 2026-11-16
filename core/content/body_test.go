package content

import (
	"github.com/VeinsureLee/BYR-Client/core/forum"
	"testing"
)

func TestOriginalBodyMarkersAndUnusedAttachments(t *testing.T) {
	attachments := []forum.Attachment{{Files: []forum.File{{Name: "photo.png", URL: "/api/photo.png"}, {Name: "notes.zip", URL: "/api/notes.zip"}, {Name: "extra.jpg", URL: "/api/extra.jpg"}}}}
	parts := Parse("[b]你好[/b] [upload=1][/upload]\n[emc50r] [url=https://example.org]链接[/url]<img src=\"https://example.org/x.jpg\">", attachments)
	counts := map[Kind]int{}
	for _, p := range parts {
		counts[p.Kind]++
		if p.Text == "你好" && !p.Style.Bold {
			t.Error("bold style lost")
		}
	}
	if counts[Image] != 3 || counts[File] != 1 || counts[Emoji] != 1 || counts[Link] != 1 {
		t.Fatalf("parts %+v", parts)
	}
	if got := Plain("a<br>[i]b[/i]&amp;c"); got != "a\nb&c" {
		t.Fatalf("plain %q", got)
	}
}
func TestMissingUploadAndNestedStyles(t *testing.T) {
	parts := Parse("[b][i]a[/i]b[/b][upload=8][/upload]", nil)
	if len(parts) != 3 || !parts[0].Style.Italic || !parts[1].Style.Bold || parts[1].Style.Italic || parts[2].Text != "[图片缺失]" {
		t.Fatalf("%+v", parts)
	}
}
