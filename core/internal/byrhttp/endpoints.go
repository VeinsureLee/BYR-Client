package byrhttp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

// The endpoint families share transport, not application navigation. There is
// no untyped Execute(method, url) exposed to either UI.
func resource(parts ...string) ([]string, error) {
	for _, p := range parts {
		if err := segment(p); err != nil {
			return nil, err
		}
	}
	parts = append([]string(nil), parts...)
	parts[len(parts)-1] += ".json"
	return parts, nil
}
func get[T any](c *Client, ctx context.Context, query url.Values, parts ...string) (T, error) {
	var v T
	p, err := resource(parts...)
	if err == nil {
		err = c.read(ctx, p, query, &v)
	}
	return v, err
}
func paged[T any](c *Client, ctx context.Context, page Page, parts ...string) (T, error) {
	q, err := pageQuery(page)
	if err != nil {
		var v T
		return v, err
	}
	return get[T](c, ctx, q, parts...)
}
func post[T any](c *Client, ctx context.Context, form url.Values, parts ...string) (T, error) {
	var v T
	p, err := resource(parts...)
	if err == nil {
		err = c.request(ctx, http.MethodPost, "open", p, nil, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", true, &v)
	}
	return v, err
}
func bit(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func (c *Client) Article(ctx context.Context, board, id string) (Article, error) {
	return get[Article](c, ctx, nil, "article", board, id)
}

func (c *Client) PostArticle(ctx context.Context, p Post) (Article, error) {
	f := url.Values{"title": {p.Title}, "content": {p.Content}, "anonymous": {bit(p.Anonymous)}, "signature": {bit(p.Signature)}, "email": {bit(p.Email)}, "outgo": {bit(p.Outgo)}}
	if strings.TrimSpace(p.ReplyID) != "" {
		if err := segment(p.ReplyID); err != nil {
			return Article{}, err
		}
		f.Set("reid", p.ReplyID)
	}
	return post[Article](c, ctx, f, "article", p.Board, "post")
}
func (c *Client) UpdateArticle(ctx context.Context, board, id, title, content string) (Article, error) {
	return post[Article](c, ctx, url.Values{"title": {title}, "content": {content}}, "article", board, "update", id)
}
func (c *Client) DeleteArticle(ctx context.Context, board, id string) (Article, error) {
	return post[Article](c, ctx, nil, "article", board, "delete", id)
}
func (c *Client) Like(ctx context.Context, board, id string) (Reaction, error) {
	return get[Reaction](c, ctx, nil, "article", board, "like", id)
}
func (c *Client) VoteDown(ctx context.Context, board, id string) (Reaction, error) {
	return get[Reaction](c, ctx, nil, "article", board, "votedown", id)
}
func (c *Client) Attachments(ctx context.Context, board, id string) (Attachment, error) {
	return get[Attachment](c, ctx, nil, "attachment", board, id)
}
func (c *Client) DeleteAttachment(ctx context.Context, board, id string) (Attachment, error) {
	return post[Attachment](c, ctx, nil, "attachment", board, "delete", id)
}

const MaxUploadSize = 32 << 20

// Upload takes bytes/streams, not an OS file path: desktop and Android file
// pickers are separate adapters. No retry of a potentially successful write.
func (c *Client) Upload(ctx context.Context, board, id, name string, source io.Reader) (Attachment, error) {
	var v Attachment
	if err := segment(board); err != nil {
		return v, err
	}
	if id != "" {
		if err := segment(id); err != nil {
			return v, err
		}
	}
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "." || name == "" || strings.ContainsAny(name, "\r\n") {
		return v, errors.New("invalid upload filename")
	}
	if source == nil {
		return v, errors.New("upload source is required")
	}
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, err := w.CreateFormFile("file", name)
	if err != nil {
		return v, err
	}
	n, err := io.Copy(part, io.LimitReader(source, MaxUploadSize+1))
	if err != nil {
		return v, err
	}
	if n > MaxUploadSize {
		return v, errors.New("attachment exceeds 32 MiB")
	}
	if err = w.Close(); err != nil {
		return v, err
	}
	err = c.request(ctx, http.MethodPost, "open", []string{"attachment", board, "add", id + ".json"}, nil, &b, w.FormDataContentType(), true, &v)
	return v, err
}
func (c *Client) Favorites(ctx context.Context, dir string) (Favorite, error) {
	return get[Favorite](c, ctx, nil, "favorite", dir)
}
func (c *Client) AddFavorite(ctx context.Context, board, dir string) (Favorite, error) {
	return post[Favorite](c, ctx, url.Values{"name": {board}, "dir": {dir}}, "favorite", "add", dir)
}
func (c *Client) RemoveFavorite(ctx context.Context, board, dir string) (Favorite, error) {
	return post[Favorite](c, ctx, url.Values{"name": {board}, "dir": {dir}}, "favorite", "delete", dir)
}
func (c *Client) Collection(ctx context.Context, id string, page Page) (Collection, error) {
	q, err := pageQuery(page)
	if err != nil {
		return Collection{}, err
	}
	if id != "" {
		q.Set("id", id)
	}
	return get[Collection](c, ctx, q, "collection")
}
func (c *Client) AddCollection(ctx context.Context, board, id string) (Article, error) {
	return post[Article](c, ctx, url.Values{"board": {board}, "id": {id}}, "collection", "add")
}
func (c *Client) RemoveCollection(ctx context.Context, board, id string) (Article, error) {
	return post[Article](c, ctx, url.Values{"board": {board}, "id": {id}}, "collection", "delete")
}
func (c *Client) FavorPosts(ctx context.Context, page Page) (ArticleList, error) {
	return paged[ArticleList](c, ctx, page, "favorpost")
}
func (c *Client) TopTen(ctx context.Context) (Widget, error) {
	return get[Widget](c, ctx, nil, "widget", "topten")
}
func (c *Client) Recommendations(ctx context.Context) (Recommendations, error) {
	return get[Recommendations](c, ctx, nil, "widget", "recommend")
}

func (c *Client) SearchArticles(ctx context.Context, s Search) (SearchResults, error) {
	q, err := pageQuery(s.Page)
	if err != nil {
		return SearchResults{}, err
	}
	if s.Order == "" {
		s.Order = "1"
	}
	for k, v := range map[string]string{"board": s.Board, "title1": s.Title, "title2": s.Title2, "titlen": s.ExcludeTitle, "author": s.Author, "day": s.Day, "m": s.Marked, "a": s.Attachment, "o": s.Order} {
		q.Set(k, v)
	}
	return get[SearchResults](c, ctx, q, "search", "article")
}
func (c *Client) SearchBoards(ctx context.Context, keyword string) (BoardList, error) {
	return get[BoardList](c, ctx, url.Values{"board": {keyword}}, "search", "board")
}
func (c *Client) Self(ctx context.Context) (User, error) {
	return get[User](c, ctx, nil, "user", "getinfo")
}
func (c *Client) User(ctx context.Context, id string) (User, error) {
	return get[User](c, ctx, nil, "user", "query", id)
}
func (c *Client) Blacklist(ctx context.Context, page int) (Blacklist, error) {
	if page < 1 {
		page = 1
	}
	return get[Blacklist](c, ctx, url.Values{"page": {strconv.Itoa(page)}}, "blacklist", "list")
}
func (c *Client) AddBlacklist(ctx context.Context, id string) (Status, error) {
	return post[Status](c, ctx, url.Values{"id": {id}}, "blacklist", "add")
}
func (c *Client) RemoveBlacklist(ctx context.Context, id string) (Status, error) {
	return post[Status](c, ctx, url.Values{"id": {id}}, "blacklist", "delete")
}
func (c *Client) Mailbox(ctx context.Context, box string, page Page) (Mailbox, error) {
	return paged[Mailbox](c, ctx, page, "mail", box)
}
func (c *Client) Mail(ctx context.Context, box, id string) (Mail, error) {
	return get[Mail](c, ctx, nil, "mail", box, id)
}
func (c *Client) SendMail(ctx context.Context, id, title, content string) (Status, error) {
	return post[Status](c, ctx, url.Values{"id": {id}, "title": {title}, "content": {content}, "signature": {"0"}, "backup": {"1"}}, "mail", "send")
}
func (c *Client) ReplyMail(ctx context.Context, box, id, title, content string) (Mail, error) {
	return post[Mail](c, ctx, url.Values{"title": {title}, "content": {content}, "signature": {"0"}, "backup": {"1"}}, "mail", box, "reply", id)
}
func (c *Client) ForwardMail(ctx context.Context, box, id, target string) (Mail, error) {
	return post[Mail](c, ctx, url.Values{"target": {target}, "noansi": {"0"}, "big5": {"0"}}, "mail", box, "forward", id)
}
func (c *Client) DeleteMail(ctx context.Context, box, id string) (Mail, error) {
	return post[Mail](c, ctx, nil, "mail", box, "delete", id)
}
func (c *Client) Refers(ctx context.Context, kind string, page Page) (Refers, error) {
	return paged[Refers](c, ctx, page, "refer", kind)
}
func (c *Client) ReferInfo(ctx context.Context, kind string) (ReferInfo, error) {
	return get[ReferInfo](c, ctx, nil, "refer", kind, "info")
}
func (c *Client) ReadRefer(ctx context.Context, kind, index string) (Refers, error) {
	return post[Refers](c, ctx, nil, "refer", kind, "setRead", index)
}
func (c *Client) ReadAllRefers(ctx context.Context, kind string) (Status, error) {
	return post[Status](c, ctx, nil, "refer", kind, "setRead")
}
func (c *Client) Vote(ctx context.Context, aid string) (Vote, error) {
	return get[Vote](c, ctx, nil, "vote", aid)
}
func (c *Client) SubmitVote(ctx context.Context, aid, choice string) (Vote, error) {
	return post[Vote](c, ctx, url.Values{"vote": {choice}}, "vote", aid)
}

// Keep the recovered multi-option form explicit until its wire format has been
// verified with the service; do not silently rewrite it into a guessed schema.
func (c *Client) SubmitMultipleVote(ctx context.Context, aid string, form url.Values) (Vote, error) {
	return post[Vote](c, ctx, form, "vote", aid)
}
func (c *Client) Bet(ctx context.Context, id string) (Bet, error) {
	return get[Bet](c, ctx, nil, "bet", id)
}
func (c *Client) SubmitBet(ctx context.Context, id, item string, score int) (Bet, error) {
	if score <= 0 {
		return Bet{}, errors.New("积分必须大于 0")
	}
	return post[Bet](c, ctx, url.Values{"biid": {item}, "score": {strconv.Itoa(score)}}, "bet", id)
}
func (c *Client) Banners(ctx context.Context) (Banners, error) {
	return get[Banners](c, ctx, nil, "banner")
}
func (c *Client) Welcome(ctx context.Context) (Welcome, error) {
	var v Welcome
	err := c.request(ctx, http.MethodGet, "open", []string{"welcomeimg.json"}, nil, nil, "", false, &v)
	return v, err
}
func (c *Client) Version(ctx context.Context, beta bool) (Versions, error) {
	channel := "release"
	if beta {
		channel = "beta"
	}
	var v Versions
	err := c.request(ctx, http.MethodGet, "open", []string{"app", channel, "version.json"}, nil, nil, "", false, &v)
	return v, err
}
