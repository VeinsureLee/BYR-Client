package forum

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// AssetURL authenticates only our configured origin or the two official BBS
// hosts. A suffix match such as evilbyr.cn must never receive a credential.
func (c *Client) AssetURL(ctx context.Context, raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("图片或附件地址无效")
	}
	u = c.base.ResolveReference(u)
	if c.ipv6 != nil && c.ipv6() && u.Host == "bbs.byr.cn" && u.Scheme == "https" {
		u.Host = "bbs6.byr.cn"
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return "", errors.New("图片或附件地址无效")
	}
	trusted := strings.EqualFold(u.Host, c.base.Host) && u.Scheme == c.base.Scheme || u.Scheme == "https" && (u.Host == "bbs.byr.cn" || u.Host == "bbs6.byr.cn")
	q := u.Query()
	q.Del("oauth_token")
	if trusted {
		if strings.HasPrefix(u.Path, "/api/") {
			u.Path = "/open/" + strings.TrimPrefix(u.Path, "/api/")
			u.RawPath = ""
		}
		if c.token != nil {
			token, err := c.token(ctx)
			if err != nil {
				return "", err
			}
			q.Set("oauth_token", token)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
func (c *Client) FetchAsset(ctx context.Context, raw string) ([]byte, error) {
	endpoint, err := c.AssetURL(ctx, raw)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.New("could not create asset request")
	}
	req.Header.Set("Referer", c.base.String()+"/")
	resp, err := c.http.Do(req)
	if err != nil {
		var wrapped *url.Error
		if errors.As(err, &wrapped) {
			err = wrapped.Err
		}
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{StatusCode: resp.StatusCode}
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, MaxUploadSize+1))
	if err != nil {
		return nil, err
	}
	if len(b) > MaxUploadSize {
		return nil, ErrResponseTooLarge
	}
	return b, nil
}
func (c *Client) ThreadURL(board, id string) string {
	u := *c.base
	u.Path = strings.TrimRight(u.Path, "/") + "/article/" + board + "/" + id
	u.RawQuery = ""
	return u.String()
}
