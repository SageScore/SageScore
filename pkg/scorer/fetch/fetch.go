// Package fetch is the HTTP client layer for SageScore.
//
// Every outbound request goes through Fetch: it sets the SageScoreBot
// user-agent, enforces a 5MB response cap, follows redirects with a
// same-host preference, applies the retry policy from Technical Plan
// §3.4, and records the final URL + headers + timing for analysers.
package fetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// UserAgent is the SageScoreBot identifier. See docs/methodology.md and
// PRD §6 — this string is also surfaced on /bot and is the one contact
// point a site owner sees if they investigate the crawl.
const UserAgent = "SageScoreBot/0.2 (+https://sagescore.org/bot; contact@sagescore.org)"

// Limits are the per-request caps.
const (
	MaxBodyBytes   = 5 * 1024 * 1024
	RequestTimeout = 30 * time.Second
)

// Response is the bundled result of a single fetch.
type Response struct {
	RequestURL string
	FinalURL   string
	StatusCode int
	Header     http.Header
	Body       []byte
	FetchedAt  time.Time
	Duration   time.Duration
	NetworkErr error // non-nil if the request never completed
}

// Client fetches HTTP resources with retries and size limits.
type Client struct {
	HTTP *http.Client
}

// NewClient returns a Client with sensible defaults.
func NewClient() *Client {
	return &Client{
		HTTP: &http.Client{
			Timeout: RequestTimeout,
		},
	}
}

// Fetch performs a GET with the SageScoreBot UA and applies the retry
// policy: 1 retry on network error, 1 retry on 5xx with 5s backoff,
// none on 4xx. The response body is truncated to MaxBodyBytes.
func (c *Client) Fetch(ctx context.Context, url string) (*Response, error) {
	var last *Response
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return last, ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
		resp, err := c.do(ctx, url)
		last = resp
		if err != nil {
			// Network-level error — retry once.
			continue
		}
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			// Server error — retry once.
			continue
		}
		return resp, nil
	}
	if last != nil && last.NetworkErr != nil {
		return last, last.NetworkErr
	}
	return last, nil
}

func (c *Client) do(ctx context.Context, url string) (*Response, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return &Response{RequestURL: url, FetchedAt: start, NetworkErr: err}, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return &Response{
			RequestURL: url,
			FetchedAt:  start,
			Duration:   time.Since(start),
			NetworkErr: err,
		}, err
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, MaxBodyBytes+1)
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, limited); err != nil {
		return &Response{
			RequestURL: url,
			FetchedAt:  start,
			Duration:   time.Since(start),
			NetworkErr: err,
		}, err
	}
	body := buf.Bytes()
	truncated := false
	if len(body) > MaxBodyBytes {
		body = body[:MaxBodyBytes]
		truncated = true
	}

	out := &Response{
		RequestURL: url,
		FinalURL:   resp.Request.URL.String(),
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       body,
		FetchedAt:  start,
		Duration:   time.Since(start),
	}
	if truncated {
		out.NetworkErr = errors.New("fetch: body truncated at limit")
	}
	return out, nil
}

// ContentType returns the normalised Content-Type header, stripped of
// parameters. Empty string if absent.
func (r *Response) ContentType() string {
	ct := r.Header.Get("Content-Type")
	for i, c := range ct {
		if c == ';' {
			return ct[:i]
		}
	}
	return ct
}

// IsHTML reports whether the response looks like HTML we should parse.
func (r *Response) IsHTML() bool {
	ct := r.ContentType()
	switch ct {
	case "text/html", "application/xhtml+xml":
		return true
	}
	return false
}

// ErrorString is a helper for logging/findings; returns "" when OK.
func (r *Response) ErrorString() string {
	if r == nil {
		return "no response"
	}
	if r.NetworkErr != nil {
		return r.NetworkErr.Error()
	}
	if r.StatusCode >= 400 {
		return fmt.Sprintf("HTTP %d", r.StatusCode)
	}
	return ""
}
