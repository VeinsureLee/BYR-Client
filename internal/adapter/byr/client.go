// Package byr adapts the BYR forum HTTP endpoints to the application ports.
// Login, session cookies and forum structure were checked against the
// thread-scope reference project; selectors still need real-site verification.
package byr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"

	"BYR-Client/internal/domain"
	"BYR-Client/internal/ports"
)

const SiteID = "byr"

const (
	maxBody      = 8 << 20
	maxImageBody = 20 << 20
)

type Config struct {
	BaseURL     string
	UserAgent   string
	HTTPClient  *http.Client
	Timeout     time.Duration
	SessionFile string
}

func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://bbs.byr.cn",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) BYRClient/0.1",
		Timeout:   20 * time.Second,
	}
}

type Client struct {
	cfg Config

	mu      sync.RWMutex
	cookies map[string]string
	session ports.Session

	loginMu sync.Mutex
}

func New(cfg Config) *Client {
	defaults := DefaultConfig()
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = defaults.BaseURL
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if strings.TrimSpace(cfg.UserAgent) == "" {
		cfg.UserAgent = defaults.UserAgent
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaults.Timeout
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: cfg.Timeout}
	}
	client := &Client{cfg: cfg, cookies: make(map[string]string)}
	client.loadSession()
	return client
}

type savedSession struct {
	Cookies map[string]string `json:"cookies"`
	Session ports.Session     `json:"session"`
}

func (c *Client) loadSession() {
	if c.cfg.SessionFile == "" {
		return
	}
	data, err := os.ReadFile(c.cfg.SessionFile)
	if err != nil {
		return
	}
	var saved savedSession
	if err := json.Unmarshal(data, &saved); err != nil {
		return
	}
	if !saved.Session.LoggedIn || len(saved.Cookies) == 0 {
		return
	}
	c.cookies = saved.Cookies
	c.session = saved.Session
}

func (c *Client) saveSession() error {
	if c.cfg.SessionFile == "" {
		return nil
	}
	c.mu.RLock()
	saved := savedSession{Cookies: make(map[string]string, len(c.cookies)), Session: c.session}
	for key, value := range c.cookies {
		saved.Cookies[key] = value
	}
	c.mu.RUnlock()
	if !saved.Session.LoggedIn || len(saved.Cookies) == 0 {
		return nil
	}
	data, err := json.Marshal(saved)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.cfg.SessionFile), 0o700); err != nil {
		return err
	}
	return os.WriteFile(c.cfg.SessionFile, data, 0o600)
}

func (c *Client) Session() ports.Session {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.session
}

func (c *Client) LoggedIn() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.session.LoggedIn
}

type loginResponse struct {
	AjaxST   int    `json:"ajax_st"`
	AjaxText string `json:"ajax_text"`
	AjaxMsg  string `json:"ajax_msg"`
	AjaxCode string `json:"ajax_code"`
	UserName string `json:"user_name"`
	IsLogin  bool   `json:"is_login"`
}

func (c *Client) Login(ctx context.Context, username, password string) (ports.Session, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return ports.Session{}, errors.New("账号和密码不能为空")
	}
	c.loginMu.Lock()
	defer c.loginMu.Unlock()

	c.mu.Lock()
	c.cookies = make(map[string]string)
	c.session = ports.Session{SiteID: SiteID, AccountID: username}
	epoch := c.session.Epoch + 1
	c.mu.Unlock()

	// 1. GET the index page for the guest cookies.
	body, _, err := c.do(ctx, http.MethodGet, "/index", nil, nil, false, false)
	if err != nil {
		return ports.Session{}, fmt.Errorf("获取登录页失败: %w", err)
	}
	_ = body

	// 2. POST the AJAX login form used by the reference client.
	form := url.Values{
		"id":         {username},
		"passwd":     {password},
		"mode":       {"0"},
		"CookieDate": {"2"},
	}
	headers := map[string]string{
		"Content-Type":     "application/x-www-form-urlencoded",
		"X-Requested-With": "XMLHttpRequest",
		"Accept":           "application/json",
		"Referer":          c.cfg.BaseURL + "/default",
	}
	body, _, err = c.do(ctx, http.MethodPost, "/user/ajax_login.json", []byte(form.Encode()), headers, true, false)
	if err != nil {
		c.clear()
		return ports.Session{}, fmt.Errorf("登录请求失败: %w", err)
	}
	var result loginResponse
	if err := json.Unmarshal(body, &result); err != nil {
		c.clear()
		return ports.Session{}, fmt.Errorf("登录响应无法解析: %w", err)
	}
	if result.AjaxST != 1 && !result.IsLogin {
		c.clear()
		message := strings.TrimSpace(result.AjaxText)
		if message == "" {
			message = strings.TrimSpace(result.AjaxMsg)
		}
		if message == "" {
			message = "账号或密码错误"
		}
		return ports.Session{}, errors.New(message)
	}
	display := strings.TrimSpace(result.UserName)
	if display == "" {
		display = username
	}
	session := ports.Session{
		SiteID:      SiteID,
		AccountID:   username,
		DisplayName: display,
		Epoch:       epoch,
		LoggedIn:    true,
	}
	c.mu.Lock()
	c.session = session
	c.mu.Unlock()
	_ = c.saveSession()
	return session, nil
}

func (c *Client) Restore(ctx context.Context) (ports.Session, bool, error) {
	c.mu.RLock()
	session := c.session
	c.mu.RUnlock()
	if !session.LoggedIn {
		return ports.Session{}, false, nil
	}
	// /default requires a session; an expired cookie is redirected to the login
	// page, which do detects as ErrAuthRequired.
	_, _, err := c.do(ctx, http.MethodGet, "/default", nil, map[string]string{
		"Accept": "text/html,application/xhtml+xml",
	}, false, true)
	if errors.Is(err, ports.ErrAuthRequired) {
		c.clear()
		return ports.Session{}, false, nil
	}
	if err != nil {
		return session, true, err
	}
	return session, true, nil
}

func (c *Client) Logout(context.Context) error {
	c.loginMu.Lock()
	defer c.loginMu.Unlock()
	c.clear()
	return nil
}

func protectedCookie(name string) bool {
	switch name {
	case "nforum[UTMPUSERID]", "nforum[UTMPKEY]", "nforum[UTMPNUM]", "nforum[PASSWORD]":
		return true
	}
	return false
}

func (c *Client) clear() {
	c.mu.Lock()
	epoch := c.session.Epoch + 1
	c.cookies = make(map[string]string)
	c.session = ports.Session{SiteID: SiteID, Epoch: epoch}
	c.mu.Unlock()
	if c.cfg.SessionFile != "" {
		_ = os.Remove(c.cfg.SessionFile)
	}
}

func (c *Client) requireLogin() error {
	if !c.LoggedIn() {
		return ports.ErrAuthRequired
	}
	return nil
}

func (c *Client) ReadImage(ctx context.Context, rawURL string) ([]byte, error) {
	if err := c.requireLogin(); err != nil {
		return nil, err
	}
	resolved := strings.TrimSpace(rawURL)
	if resolved == "" {
		return nil, errors.New("图片地址为空")
	}
	if strings.HasPrefix(resolved, "//") {
		resolved = "https:" + resolved
	} else if parsed, err := url.Parse(resolved); err == nil && !parsed.IsAbs() {
		base, _ := url.Parse(c.cfg.BaseURL)
		if base != nil {
			resolved = base.ResolveReference(parsed).String()
		}
	}
	if !strings.HasPrefix(resolved, "http://") && !strings.HasPrefix(resolved, "https://") {
		return nil, fmt.Errorf("不支持的图片地址: %s", rawURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolved, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "image/webp,image/png,image/jpeg,image/gif,image/bmp,*/*;q=0.8")
	if c.sameSite(resolved) {
		req.Header.Set("Referer", c.cfg.BaseURL+"/default")
		if cookie := c.cookieHeader(); cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
	}
	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, ports.ErrAuthRequired
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("图片 HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBody))
	if err != nil {
		return nil, err
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") && looksLikeLoginPage(body) {
		return nil, ports.ErrAuthRequired
	}
	return body, nil
}

func (c *Client) sameSite(rawURL string) bool {
	base, err := url.Parse(c.cfg.BaseURL)
	if err != nil {
		return false
	}
	target, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(base.Hostname(), target.Hostname())
}

func (c *Client) ArticleURL(key domain.ArticleKey) string {
	return c.cfg.BaseURL + "/article/" + escapePath(string(key.BoardID)) + "/" + escapePath(string(key.ArticleID))
}

func (c *Client) Cookies() []ports.Cookie {
	base, _ := url.Parse(c.cfg.BaseURL)
	domain := ""
	secure := true
	if base != nil {
		domain = base.Hostname()
		secure = base.Scheme == "https"
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]ports.Cookie, 0, len(c.cookies))
	for name, value := range c.cookies {
		result = append(result, ports.Cookie{
			Name: name, Value: value, Domain: domain, Path: "/", Secure: secure, HTTPOnly: true,
		})
	}
	return result
}

func (c *Client) cookieHeader() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.cookies) == 0 {
		return ""
	}
	keys := make([]string, 0, len(c.cookies))
	for key := range c.cookies {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+c.cookies[key])
	}
	return strings.Join(parts, "; ")
}

func (c *Client) mergeCookies(resp *http.Response, loginResponse bool) {
	if resp == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	lines := resp.Header.Values("Set-Cookie")
	responseHasLogin := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "nforum[PASSWORD]=") {
			responseHasLogin = true
			break
		}
	}
	loggedIn := c.session.LoggedIn
	for _, line := range lines {
		// net/http's cookie parser rejects names such as nforum[PASSWORD], which
		// BYR uses for its login session. Parse the first name=value pair here.
		kv := strings.TrimSpace(strings.SplitN(line, ";", 2)[0])
		index := strings.Index(kv, "=")
		if index <= 0 {
			continue
		}
		name := strings.TrimSpace(kv[:index])
		value := strings.TrimSpace(kv[index+1:])
		if loggedIn && !loginResponse && !responseHasLogin && protectedCookie(name) {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "max-age=0") || strings.Contains(lower, "expires=thu, 01 jan 1970") {
			delete(c.cookies, name)
			continue
		}
		c.cookies[name] = value
	}
}

func (c *Client) endpoint(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return c.cfg.BaseURL + path
}

// do performs one request, decodes the body and merges cookies. The login POST
// must keep the guest cookies it just obtained, hence mergeOnlyLogin=false.
func (c *Client) do(ctx context.Context, method, path string, body []byte, headers map[string]string, mergeOnlyLogin, checkAuth bool) ([]byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint(path), bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	if cookie := c.cookieHeader(); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, resp.Header, err
	}
	if mergeOnlyLogin {
		c.mergeCookies(resp, true)
	} else {
		c.mergeCookies(resp, false)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.Header, fmt.Errorf("BYR HTTP %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	decoded, err := decodeBody(raw, resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, resp.Header, err
	}
	if checkAuth && looksLikeLoginPage(decoded) {
		return nil, resp.Header, ports.ErrAuthRequired
	}
	return decoded, resp.Header, nil
}

func decodeBody(raw []byte, contentType string) ([]byte, error) {
	charset := ""
	if _, params, err := mime.ParseMediaType(contentType); err == nil {
		charset = strings.ToLower(params["charset"])
	}
	switch {
	case strings.Contains(charset, "gbk"), strings.Contains(charset, "gb2312"), strings.Contains(charset, "gb18030"):
		return simplifiedchinese.GBK.NewDecoder().Bytes(raw)
	case strings.Contains(charset, "utf-8"), strings.Contains(charset, "utf8"):
		return raw, nil
	}
	if utf8.Valid(raw) {
		return raw, nil
	}
	if decoded, err := simplifiedchinese.GB18030.NewDecoder().Bytes(raw); err == nil {
		return decoded, nil
	}
	return raw, nil
}

func looksLikeLoginPage(body []byte) bool {
	text := string(body)
	for _, signal := range []string{"您未登录", `id="login_box"`, `name="passwd"`} {
		if strings.Contains(text, signal) {
			return true
		}
	}
	return false
}

func escapePath(value string) string {
	return url.PathEscape(value)
}

func encodeQueryPart(value string) string {
	// BYR expects %20 for spaces rather than the + that url.Values uses.
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

func withUID(path, uid string) string {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + "_uid=" + url.QueryEscape(uid)
}
