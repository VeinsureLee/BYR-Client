package demo

import (
	"bytes"
	"context"
	"image/png"
	"testing"
	"time"

	"BYR-Client/internal/domain"
)

func TestDemoImageDecodesAsPNG(t *testing.T) {
	source := New(time.Now())
	board, err := source.ReadBoardPage(context.Background(), domain.Scope{}, "b-go", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(board.Articles) == 0 {
		t.Fatal("empty demo board")
	}
	var key domain.ArticleKey
	for _, article := range board.Articles {
		if len(article.Images) > 0 {
			key = article.Key
			break
		}
	}
	if key.ArticleID == "" {
		t.Fatal("demo board has no image article")
	}
	thread, err := source.ReadThreadPage(context.Background(), domain.Scope{}, key, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(thread.Article.Images) == 0 {
		t.Fatal("thread lost its images")
	}
	data, err := source.ReadImage(context.Background(), thread.Article.Images[0].URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("demo image is not a PNG: %v", err)
	}
	foundEmoji := false
	for _, image := range thread.Article.Images {
		if image.Kind == domain.ImageEmoji {
			foundEmoji = true
		}
	}
	if !foundEmoji {
		t.Fatal("demo article has no emoji image")
	}
}
