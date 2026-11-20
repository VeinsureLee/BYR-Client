package desktop

import "testing"

func TestImageViewerURLPrefersOriginal(t *testing.T) {
	const thumbnail = "https://cdn.example.test/thumb.jpg"
	const original = "https://bbs.example.test/original.jpg"
	if got := imageViewerURL(thumbnail, original); got != original {
		t.Fatalf("viewer URL = %q, want original %q", got, original)
	}
	if got := imageViewerURL(thumbnail, ""); got != thumbnail {
		t.Fatalf("viewer URL without original = %q, want thumbnail %q", got, thumbnail)
	}
	if got := imageViewerURL(thumbnail); got != thumbnail {
		t.Fatalf("viewer URL without full URL = %q, want thumbnail %q", got, thumbnail)
	}
}
