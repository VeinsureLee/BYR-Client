package byr

import (
	"strings"
	"testing"

	"BYR-Client/internal/domain"
)

func TestThreadSeparatesFeaturedRepliesFromEmptyOriginalPost(t *testing.T) {
	page := `<div class="b-head"><span class="n-left">文章主题: 高考考上北邮和北邮博士毕业哪个难？</span></div>
<div class="a-wrap"><div class="a-pos">楼主</div><div class="a-u-name">IWhisper#152</div>
<div class="a-content-wrap"><div class="a-content">发信人: IWhisper#152 (我爱北邮人!), 信区: IWhisper
标  题: 高考考上北邮和北邮博士毕业哪个难？
发信站: 北邮人论坛 (Tue Oct 20 13:04:58 2026), 站内
--
※ 来源:·北邮人论坛手机客户端 bbs.byr.cn·[FROM: 匿名天使的家]</div>
<div class="a-content-reply">精彩回复收起 ▲
<div class="a-wrap"><div class="a-pos">第2楼</div><div class="a-u-name">bob</div><div class="a-content">分人，北邮博士有很一般学校上来的。<img src="/featured.png"></div>赞(3) 踩(0) 回复</div>
<div>第5楼：精选副本，赞(3) 踩(0) 回复</div></div></div></div>
<div class="a-wrap"><div class="a-pos">第2楼</div><div class="a-u-name">bob</div><div class="a-content-wrap"><div class="a-content">分人，北邮博士有很一般学校上来的。</div></div></div>
<div class="a-wrap"><div class="a-pos">第5楼</div><div class="a-u-name">alice</div><div class="a-content">得了吧，都是看导师罢了。</div></div>`
	doc, err := parseHTML([]byte(page))
	if err != nil {
		t.Fatal(err)
	}
	key := domain.ArticleKey{SiteID: SiteID, BoardID: "IWhisper", ArticleID: "152"}
	article, replies, err := parseThread(doc, key, "https://bbs.byr.cn")
	if err != nil {
		t.Fatal(err)
	}
	if article.Content != "" || len(article.Images) != 0 {
		t.Fatalf("empty original post contains featured content: %q, images=%+v", article.Content, article.Images)
	}
	if article.Author != "IWhisper#152" || article.CreatedAt.IsZero() {
		t.Fatalf("original metadata lost: %+v", article)
	}
	if len(replies) != 2 || replies[0].Floor != 2 || replies[1].Floor != 5 {
		t.Fatalf("featured replies duplicated or renumbered: %+v", replies)
	}
	if replies[0].Content != "分人，北邮博士有很一般学校上来的。" || len(replies[0].Images) != 0 {
		t.Fatalf("reply uses featured copy: %+v", replies[0])
	}
}

func TestThreadContentBoundaries(t *testing.T) {
	tests := []struct {
		name, html, want string
		imageCount       int
	}{
		{"wrapper-only empty body", `<div class="a-content-wrap">发信人: alice, 信区: Go
发信站: 北邮人论坛 (Tue Oct 20 13:04:58 2026), 站内

--
※ 来源: 北邮人论坛
<div class="a-content-reply">精彩回复 收起 ▲ 不应出现的回复</div></div>`, "", 0},
		{"image-only original", `<div class="a-content-wrap"><div class="a-content">发信人: alice, 信区: Go
发信站: 北邮人论坛 (Tue Oct 20 13:04:58 2026), 站内

<img src="/original.png">
--
※ 来源: 北邮人论坛</div><div class="a-content-reply"><img src="/featured.png">赞(3)</div></div>`, "", 1},
		{"real text and phrase remain", `<div class="a-content-wrap"><div class="a-content">发信人: alice, 信区: Go
发信站: 北邮人论坛 (Tue Oct 20 13:04:58 2026), 站内

正文中的精彩回复只是正常文字。
第二段。
--
※ 来源: 北邮人论坛</div><div class="hot-replies">精选副本</div></div>`, "正文中的精彩回复只是正常文字。\n第二段。", 0},
		{"controls inside content", `<div class="a-content">保留正文<div class="a-content-reply">精彩回复 收起 ▲<img src="/featured.png"></div><button>回复</button><script>bad()</script></div>`, "保留正文", 0},
		{"headers without body or footer", `<div class="a-content">发信人: alice, 信区: Go
标 题: 图片帖
发信站: 北邮人论坛 (Tue Oct 20 13:04:58 2026), 站内</div>`, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := parseHTML([]byte(`<div class="a-wrap"><div class="a-pos">楼主</div><div class="a-u-name">alice</div>` + tt.html + `</div>`))
			if err != nil {
				t.Fatal(err)
			}
			article, replies, err := parseThread(doc, domain.ArticleKey{ArticleID: "1001"}, "https://bbs.byr.cn")
			if err != nil {
				t.Fatal(err)
			}
			if article.Content != tt.want || len(article.Images) != tt.imageCount || len(replies) != 0 {
				t.Fatalf("content=%q images=%+v replies=%+v", article.Content, article.Images, replies)
			}
			for _, img := range article.Images {
				if strings.Contains(img.URL, "featured") {
					t.Fatalf("featured image leaked: %s", img.URL)
				}
			}
		})
	}
}
