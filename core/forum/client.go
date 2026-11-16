// Package forum provides the UI-independent BYR protocol foundation. The
// desktop and eventual Android application import this exact Go module.
package forum

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://bbs.byr.cn"
const responseLimit = 8 << 20

// TokenSource is an injected function (Strategy). Platform credential storage
// may supply it without coupling the shared core to its persistence mechanism.
type TokenSource func(context.Context) (string, error)

type Options struct {
	BaseURL    string
	HTTPClient *http.Client
	Token      TokenSource
	IPv6       func() bool
}

// Client is the common Facade over protocol requests, authentication injection,
// JSON decoding and API errors. It owns no UI, navigation or global session.
type Client struct {
	base  *url.URL
	http  *http.Client
	token TokenSource
	ipv6  func() bool
}

// New validates dependencies but does not make a network request.
func New(options Options) (*Client, error) {
	baseURL := options.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, errors.New("invalid forum base URL")
	}
	if (base.Scheme != "https" && base.Scheme != "http") || base.Host == "" ||
		base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("forum base URL must be an HTTP(S) origin or path prefix")
	}
	transport := &http.Client{Timeout: 30 * time.Second}
	if options.HTTPClient != nil {
		copy := *options.HTTPClient
		transport = &copy
	}
	previousRedirect := transport.CheckRedirect
	transport.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) == 0 || !strings.EqualFold(request.URL.Host, via[0].URL.Host) || request.URL.Scheme != via[0].URL.Scheme {
			return ErrRedirectOrigin
		}
		if previousRedirect != nil {
			return previousRedirect(request, via)
		}
		if len(via) >= 10 {
			return errors.New("too many forum redirects")
		}
		return nil
	}
	return &Client{base: base, http: transport, token: options.Token, ipv6: options.IPv6}, nil
}

func (c *Client) Sections(ctx context.Context) (SectionList, error) {
	var result SectionList
	err := c.read(ctx, []string{"section.json"}, nil, &result)
	return result, err
}

func (c *Client) Section(ctx context.Context, name string) (Section, error) {
	var result Section
	if err := segment(name); err != nil {
		return result, err
	}
	err := c.read(ctx, []string{"section", name + ".json"}, nil, &result)
	return result, err
}

func (c *Client) Board(ctx context.Context, name string, page Page) (Board, error) {
	var result Board
	if err := segment(name); err != nil {
		return result, err
	}
	query, err := page.query()
	if err != nil {
		return result, err
	}
	err = c.read(ctx, []string{"board", name + ".json"}, query, &result)
	return result, err
}

func (c *Client) Thread(ctx context.Context, board, id string, page Page) (Thread, error) {
	var result Thread
	if err := segment(board); err != nil {
		return result, err
	}
	if err := segment(id); err != nil {
		return result, err
	}
	query, err := page.query()
	if err != nil {
		return result, err
	}
	err = c.read(ctx, []string{"threads", board, id + ".json"}, query, &result)
	return result, err
}

func segment(value string) error {
	if strings.TrimSpace(value) == "" || value == "." || value == ".." || strings.ContainsAny(value, "/\\?#%") {
		return errors.New("invalid forum resource identifier")
	}
	return nil
}

func (p Page) query() (url.Values, error) {
	if p.Number == 0 {
		p.Number = 1
	}
	if p.Size == 0 {
		p.Size = 50
	}
	if p.Number < 1 || p.Size < 1 || p.Size > 100 {
		return nil, errors.New("invalid forum pagination")
	}
	return url.Values{"page": {strconv.Itoa(p.Number)}, "count": {strconv.Itoa(p.Size)}}, nil
}

func (c *Client) read(ctx context.Context, parts []string, query url.Values, destination any) error {
	return c.request(ctx, http.MethodGet, "open", parts, query, nil, "", true, destination)
}

func (c *Client) request(ctx context.Context, method, prefix string, parts []string, query url.Values, input io.Reader, contentType string, auth bool, destination any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if query == nil {
		query = make(url.Values)
	}
	if auth {
		if c.token == nil {
			return ErrUnauthenticated
		}
		token, err := c.token(ctx)
		if err != nil {
			return fmt.Errorf("read forum credential: %w", err)
		}
		if strings.TrimSpace(token) == "" {
			return ErrUnauthenticated
		}
		query.Set("oauth_token", token)
	}
	path := append([]string{prefix}, parts...)
	for index := range path {
		path[index] = url.PathEscape(path[index])
	}
	endpoint := *c.base
	// Keep the empty attachment ID: /attachment/<board>/add/.json is a
	// different endpoint from /attachment/<board>/add.json.
	endpoint.RawPath = strings.TrimRight(c.base.EscapedPath(), "/") + "/" + strings.Join(path, "/")
	endpoint.Path, _ = url.PathUnescape(endpoint.RawPath)
	if c.ipv6 != nil && c.ipv6() && endpoint.Host == "bbs.byr.cn" && prefix != "oauth2" && prefix != "user" {
		endpoint.Host = "bbs6.byr.cn"
	}
	if (prefix == "oauth2" || prefix == "user") && endpoint.Host == "bbs6.byr.cn" {
		endpoint.Host = "bbs.byr.cn"
	}
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), input)
	if err != nil {
		return errors.New("could not create forum request")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "BYR-Client-Go")
	request.Header.Set("Referer", c.base.String()+"/")
	if prefix == "user" {
		request.Header.Set("X-Requested-With", "XMLHttpRequest")
		request.Header.Set("Origin", c.base.Scheme+"://"+c.base.Host)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := c.http.Do(request)
	if err != nil {
		// net/http's url.Error includes oauth_token in its URL. Preserve the
		// underlying error for errors.Is without exporting the credential.
		var wrapped *url.Error
		if errors.As(err, &wrapped) {
			err = wrapped.Err
		}
		return fmt.Errorf("forum request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		// Error statuses usually carry the same JSON envelope as success
		// responses. Surface its code and message; a bare HTTP status hides
		// the real reason ("已经赞过", "不能评价自己的帖子" and so on).
		if body, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10)); readErr == nil {
			var envelope struct {
				Code    Scalar `json:"code"`
				Message string `json:"msg"`
				Error   string `json:"error"`
			}
			if json.Unmarshal(bytes.TrimSpace(body), &envelope) == nil && envelope.Code != "" {
				message := envelope.Message
				if message == "" {
					message = envelope.Error
				}
				return &APIError{Code: envelope.Code.String(), Message: message}
			}
		}
		return &HTTPError{StatusCode: response.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, responseLimit+1))
	if err != nil {
		return fmt.Errorf("read forum response: %w", err)
	}
	if len(body) > responseLimit {
		return ErrResponseTooLarge
	}
	body = bytes.TrimSpace(body)
	if len(body) == 0 || bytes.Equal(body, []byte("null")) {
		return ErrEmptyResponse
	}
	var envelope struct {
		Code    Scalar `json:"code"`
		Message string `json:"msg"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return &DecodeError{Part: "envelope", Cause: err}
	}
	if envelope.Code != "" && envelope.Code != "0" && envelope.Code != "0000" {
		message := envelope.Message
		if message == "" {
			message = envelope.Error
		}
		return &APIError{Code: envelope.Code.String(), Message: message}
	}
	if err := json.Unmarshal(body, destination); err != nil {
		return &DecodeError{Part: "data", Cause: err}
	}
	return nil
}
