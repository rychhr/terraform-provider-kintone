// SPDX-License-Identifier: MPL-2.0

package kintone

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func noWait(context.Context, time.Duration) error                         { return nil }

func TestClientAuthentication(t *testing.T) {
	for _, password := range []bool{false, true} {
		t.Run(map[bool]string{false: "token", true: "password"}[password], func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if password {
					if got := r.Header.Get("X-Cybozu-Authorization"); got != base64.StdEncoding.EncodeToString([]byte("user:secret")) {
						t.Errorf("wrong password header")
					}
					if r.Header.Get("X-Cybozu-API-Token") != "" {
						t.Error("both credentials sent")
					}
				} else if r.Header.Get("X-Cybozu-API-Token") != "one,two" {
					t.Error("wrong tokens")
				}
				_, _ = w.Write([]byte(`{}`))
			}))
			defer srv.Close()
			cfg := Config{BaseURL: srv.URL, APITokens: []string{"one", "two"}}
			if password {
				cfg.Username = "user"
				cfg.Password = "secret"
			}
			c, err := NewClient(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err = c.request(t.Context(), "GET", "/", nil, nil, &struct{}{}, true); err != nil {
				t.Fatal(err)
			}
			if !password && c.requirePassword("CreateApp") == nil {
				t.Fatal("token operation accepted")
			}
		})
	}
}

func TestClientConfiguration(t *testing.T) {
	invalid := []Config{{BaseURL: "https://example.com"}, {BaseURL: "https://example.com", Username: "user"}, {BaseURL: "https://user:secret@example.com", APITokens: []string{"token"}}, {BaseURL: "https://example.com/path", APITokens: []string{"token"}}, {BaseURL: "https://example.com?q=secret", APITokens: []string{"token"}}, {BaseURL: "https://example.com", APITokens: []string{"a\nb"}}, {BaseURL: "https://example.com", APITokens: []string{""}}, {BaseURL: "https://example.com", APITokens: []string{"token"}, PollInterval: -time.Second}}
	for i, cfg := range invalid {
		if _, err := NewClient(cfg); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
	hc := &http.Client{}
	tokens := []string{"token"}
	c, err := NewClient(Config{BaseURL: "https://EXAMPLE.com:443/", APITokens: tokens, HTTPClient: hc})
	if err != nil {
		t.Fatal(err)
	}
	tokens[0] = "changed"
	if c.tokens[0] != "token" || hc.CheckRedirect != nil || hc.Timeout != 0 || c.baseURL != "https://example.com" || c.httpClient.Timeout != 30*time.Second {
		t.Fatal("configuration was not isolated or normalized")
	}
}

func TestClientRetryPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, method string
		status       int
		safe         bool
		want         int
	}{{"get 500", "GET", 500, true, 3}, {"post 500", "POST", 500, true, 1}, {"conditional put 500", "PUT", 500, false, 1}, {"put 500", "PUT", 500, true, 3}, {"post 429", "POST", 429, false, 3}, {"put 429", "PUT", 429, false, 3}, {"conflict", "PUT", 409, false, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			attempts := 0
			retryLimit := 2
			hc := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				attempts++
				body, _ := io.ReadAll(r.Body)
				if string(body) != `{"value":"secret"}` {
					t.Errorf("body not replayed: %s", body)
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(`{"code":"GAIA_CO03","id":"id","message":"secret"}`)), Header: make(http.Header)}, nil
			})}
			c, err := NewClient(Config{BaseURL: "https://example.com", APITokens: []string{"secret"}, HTTPClient: hc, MaxRetries: &retryLimit, Wait: noWait})
			if err != nil {
				t.Fatal(err)
			}
			err = c.request(t.Context(), tc.method, "/", nil, map[string]string{"value": "secret"}, nil, tc.safe)
			var apiErr *APIError
			if !errors.As(err, &apiErr) || attempts != tc.want {
				t.Fatalf("attempts=%d err=%v", attempts, err)
			}
			if strings.Contains(err.Error(), "secret") || !apiErr.IsConflict() || apiErr.ID != "id" {
				t.Fatal("unsafe error or lost details")
			}
		})
	}
}

type failedReader struct{}

func (failedReader) Read([]byte) (int, error) { return 0, errors.New("secret read error") }
func (failedReader) Close() error             { return nil }

func TestClientUncertainFailures(t *testing.T) {
	for _, readFailure := range []bool{false, true} {
		for _, safe := range []bool{false, true} {
			attempts := 0
			limit := 1
			hc := &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
				attempts++
				if !readFailure {
					return nil, errors.New("secret transport error")
				}
				return &http.Response{StatusCode: 200, Body: failedReader{}, Header: make(http.Header)}, nil
			})}
			c, err := NewClient(Config{BaseURL: "https://example.com", APITokens: []string{"secret"}, HTTPClient: hc, MaxRetries: &limit, Wait: noWait})
			if err != nil {
				t.Fatal(err)
			}
			err = c.request(t.Context(), "PUT", "/", nil, struct{}{}, nil, safe)
			want := 1
			if safe {
				want = 2
			}
			if err == nil || strings.Contains(err.Error(), "secret") || attempts != want {
				t.Fatalf("safe=%t read=%t attempts=%d err=%v", safe, readFailure, attempts, err)
			}
		}
	}
}

func TestClientRedirectAndCancellation(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Redirect(w, r, "/secret", http.StatusFound)
	}))
	defer srv.Close()
	c, err := NewClient(Config{BaseURL: srv.URL, APITokens: []string{"secret"}})
	if err != nil {
		t.Fatal(err)
	}
	if err = c.request(t.Context(), "GET", "/", nil, nil, nil, true); err == nil || calls != 1 {
		t.Fatalf("redirect followed: calls=%d err=%v", calls, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err = c.request(ctx, "GET", "/", nil, nil, nil, true); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err = waitContext(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestClientRetryDelayAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	calls := 0
	delays := []time.Duration{}
	c, err := NewClient(Config{BaseURL: "https://example.com", APITokens: []string{"token"}, RetryBaseDelay: 10 * time.Millisecond, RetryMaxDelay: 20 * time.Millisecond, HTTPClient: &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}, Wait: func(ctx context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		if len(delays) == 3 {
			cancel()
			return ctx.Err()
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	err = c.request(ctx, "POST", "/", nil, map[string]string{"name": "test"}, nil, false)
	if !errors.Is(err, context.Canceled) || calls != 3 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	for i, delay := range delays {
		maxDelay := 20 * time.Millisecond
		if i == 0 {
			maxDelay = 10 * time.Millisecond
		}
		if delay < maxDelay/2 || delay > maxDelay {
			t.Errorf("delay %d out of bounds: %s", i, delay)
		}
	}
}

func TestClientRetryRecoversAndHonorsDefaultLimit(t *testing.T) {
	for _, recoverAfter := range []int{1, 4, 5} {
		t.Run(fmt.Sprint(recoverAfter), func(t *testing.T) {
			attempts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts++
				if attempts <= recoverAfter {
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				_, _ = w.Write([]byte(`{"appId":"1"}`))
			}))
			defer server.Close()
			c, err := NewClient(Config{BaseURL: server.URL, APITokens: []string{"token"}, Wait: noWait})
			if err != nil {
				t.Fatal(err)
			}
			app, err := c.GetApp(t.Context(), "1")
			if recoverAfter < 5 {
				if err != nil || app.AppID != "1" || attempts != recoverAfter+1 {
					t.Fatalf("attempts %d app %#v err %v", attempts, app, err)
				}
			} else if err == nil || attempts != 5 {
				t.Fatalf("default retry limit not honored: %d %v", attempts, err)
			}
		})
	}
}
