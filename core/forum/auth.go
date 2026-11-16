package forum

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// LoginForm follows the recovered official protocol, including rounding the
// timestamp. MD5 here is a server-required signature, not password storage.
func LoginForm(username, password string, now time.Time) (url.Values, error) {
	encode := func(s string) (string, error) {
		var b []byte
		for _, r := range s {
			if r > 255 {
				return "", errors.New("登录协议仅支持单字节用户名和密码")
			}
			b = append(b, byte(r))
		}
		return base64.StdEncoding.EncodeToString(b), nil
	}
	u, err := encode(username)
	if err != nil {
		return nil, err
	}
	p, err := encode(password)
	if err != nil {
		return nil, err
	}
	ts := strconv.FormatInt((now.UnixMilli()+500)/1000, 10)
	sum := md5.Sum([]byte("39c43ccebd1dafca245bec76b3be8107" + "wtfaaa" + "com.byr.bbs" + ts))
	return url.Values{"appkey": {hex.EncodeToString(sum[:])}, "source": {ts + "-1"}, "username": {u}, "password": {p}}, nil
}
func (c *Client) Login(ctx context.Context, username, password string) (OAuthInfo, error) {
	var info OAuthInfo
	if strings.TrimSpace(username) == "" || password == "" {
		return info, errors.New("请输入用户名和密码")
	}
	f, err := LoginForm(strings.TrimSpace(username), password, time.Now())
	if err != nil {
		return info, err
	}
	err = c.request(ctx, http.MethodPost, "oauth2", []string{"official"}, nil, strings.NewReader(f.Encode()), "application/x-www-form-urlencoded", false, &info)
	if err == nil && info.AccessToken == "" {
		return info, errors.New("登录响应缺少 access_token")
	}
	return info, err
}

type WebLogin struct {
	State   Scalar `json:"ajax_st"`
	Code    Scalar `json:"ajax_code"`
	Message string `json:"ajax_msg"`
}

func (w WebLogin) OK() bool { return w.State == "1" }

// Web diagnostics never replaces or clears an OAuth session.
func (c *Client) CheckWebLogin(ctx context.Context, username, password string) (WebLogin, error) {
	var v WebLogin
	err := c.request(ctx, http.MethodPost, "user", []string{"ajax_login.json"}, nil, strings.NewReader(url.Values{"id": {username}, "passwd": {password}}.Encode()), "application/x-www-form-urlencoded", false, &v)
	return v, err
}
