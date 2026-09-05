// SPDX-License-Identifier: MPL-2.0

package kintone

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config configures an independent kintone client. Zero durations select defaults.
// MaxRetries is the number of retries after the first attempt; nil selects four.
// Wait can replace the context-aware delay for deterministic tests.
type Config struct {
	BaseURL        string
	Username       string
	Password       string
	APITokens      []string
	HTTPClient     *http.Client
	MaxRetries     *int
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
	PollInterval   time.Duration
	DeployTimeout  time.Duration
	Wait           func(context.Context, time.Duration) error
}

// Client performs kintone REST API operations without Terraform dependencies.
type Client struct {
	baseURL                                                    string
	httpClient                                                 *http.Client
	passwordAuth                                               bool
	username, password                                         string
	tokens                                                     []string
	maxRetries                                                 int
	retryBaseDelay, retryMaxDelay, pollInterval, deployTimeout time.Duration
	wait                                                       func(context.Context, time.Duration) error
}

// NewClient validates configuration and snapshots caller-owned slices and HTTP settings.
func NewClient(cfg Config) (*Client, error) {
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.RawPath != "" || u.Opaque != "" {
		return nil, errors.New("invalid kintone base URL: expected an HTTP or HTTPS origin")
	}
	if (cfg.Username == "") != (cfg.Password == "") {
		return nil, errors.New("kintone username and password must be supplied together")
	}
	if len(cfg.APITokens) > 9 {
		return nil, errors.New("kintone accepts at most nine API tokens")
	}
	for _, token := range cfg.APITokens {
		if strings.TrimSpace(token) == "" || strings.ContainsAny(token, "\r\n,") {
			return nil, errors.New("invalid kintone API token")
		}
	}
	if cfg.Username == "" && len(cfg.APITokens) == 0 {
		return nil, errors.New("kintone authentication is required")
	}
	if strings.Contains(cfg.Username, ":") {
		return nil, errors.New("kintone username must not contain a colon")
	}
	retries := 4
	if cfg.MaxRetries != nil {
		retries = *cfg.MaxRetries
	}
	if retries < 0 || cfg.RetryBaseDelay < 0 || cfg.RetryMaxDelay < 0 || cfg.PollInterval < 0 || cfg.DeployTimeout < 0 {
		return nil, errors.New("kintone retry and polling settings must not be negative")
	}
	base, maxDelay, poll, timeout := cfg.RetryBaseDelay, cfg.RetryMaxDelay, cfg.PollInterval, cfg.DeployTimeout
	if base == 0 {
		base = 500 * time.Millisecond
	}
	if maxDelay == 0 {
		maxDelay = 8 * time.Second
	}
	if poll == 0 {
		poll = time.Second
	}
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	if base > maxDelay {
		return nil, errors.New("kintone retry base delay exceeds maximum delay")
	}
	hc := http.Client{Timeout: 30 * time.Second}
	if cfg.HTTPClient != nil {
		hc = *cfg.HTTPClient
		if hc.Timeout == 0 {
			hc.Timeout = 30 * time.Second
		}
	}
	hc.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Path = ""
	if (u.Scheme == "https" && u.Port() == "443") || (u.Scheme == "http" && u.Port() == "80") {
		u.Host = u.Hostname()
		if strings.Contains(u.Host, ":") {
			u.Host = "[" + u.Host + "]"
		}
	}
	wait := cfg.Wait
	if wait == nil {
		wait = waitContext
	}
	return &Client{baseURL: u.String(), httpClient: &hc, passwordAuth: cfg.Username != "", username: cfg.Username, password: cfg.Password, tokens: append([]string(nil), cfg.APITokens...), maxRetries: retries, retryBaseDelay: base, retryMaxDelay: maxDelay, pollInterval: poll, deployTimeout: timeout, wait: wait}, nil
}

func waitContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) requirePassword(operation string) error {
	if !c.passwordAuth {
		return fmt.Errorf("%s requires password authentication", operation)
	}
	return nil
}

func (c *Client) request(ctx context.Context, method, path string, query url.Values, body, output any, replaySafe bool) error {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return errors.New("cannot encode kintone request")
		}
	}
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	safe := replaySafe && (method == http.MethodGet || method == http.MethodPut || method == http.MethodDelete || method == http.MethodHead || method == http.MethodOptions)
	delay := c.retryBaseDelay
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(payload))
		if err != nil {
			return errors.New("cannot construct kintone request")
		}
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if c.passwordAuth {
			req.Header.Set("X-Cybozu-Authorization", base64.StdEncoding.EncodeToString([]byte(c.username+":"+c.password)))
		} else {
			req.Header.Set("X-Cybozu-API-Token", strings.Join(c.tokens, ","))
		}
		// Prevent net/http from replaying a request with an uncertain outcome.
		req.GetBody = nil
		resp, transportErr := c.httpClient.Do(req)
		var resultErr error
		retry := false
		if transportErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			resultErr = errors.New("kintone HTTP request failed")
			retry = safe
		} else {
			data, readErr := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024+1))
			_ = resp.Body.Close()
			if ctx.Err() != nil {
				// Preserve recovery information from a complete successful response
				// (especially a created app ID), while still returning cancellation.
				if readErr == nil && len(data) <= 16*1024*1024 && resp.StatusCode >= 200 && resp.StatusCode < 300 && output != nil {
					_ = json.Unmarshal(data, output)
				}
				return ctx.Err()
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				apiErr := &APIError{StatusCode: resp.StatusCode}
				var detail struct {
					Code string `json:"code"`
					ID   string `json:"id"`
				}
				if readErr == nil {
					_ = json.Unmarshal(data, &detail)
					apiErr.Code = detail.Code
					apiErr.ID = detail.ID
				}
				resultErr = apiErr
				retry = resp.StatusCode == 429 || (safe && resp.StatusCode >= 500 && resp.StatusCode <= 599)
			} else if readErr != nil || len(data) > 16*1024*1024 {
				resultErr = errors.New("cannot read kintone response")
				retry = safe
			} else if output != nil {
				if err = json.Unmarshal(data, output); err != nil {
					return errors.New("cannot decode kintone response")
				}
				return nil
			} else {
				return nil
			}
		}
		if !retry || attempt >= c.maxRetries {
			return resultErr
		}
		// Equal jitter keeps delays bounded while avoiding synchronized callers.
		jittered := delay/2 + time.Duration(rand.Int64N(int64(delay-delay/2)+1))
		if err = c.wait(ctx, jittered); err != nil {
			return err
		}
		if delay >= c.retryMaxDelay/2 {
			delay = c.retryMaxDelay
		} else {
			delay *= 2
		}
	}
}
