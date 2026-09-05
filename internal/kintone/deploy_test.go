// SPDX-License-Identifier: MPL-2.0

package kintone

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type cancelOnEOFBody struct {
	io.Reader
	cancel context.CancelFunc
}

func (b *cancelOnEOFBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	if errors.Is(err, io.EOF) {
		b.cancel()
	}
	return n, err
}

func (*cancelOnEOFBody) Close() error { return nil }

func TestLifecycleCreateCancellationRetainsID(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	calls := 0
	c, err := NewClient(Config{
		BaseURL: "https://example.com", Username: "test", Password: "test",
		HTTPClient: &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.Method != http.MethodPost || r.URL.Path != "/k/v1/preview/app.json" {
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       &cancelOnEOFBody{Reader: strings.NewReader(`{"app":"42","revision":"1"}`), cancel: cancel},
			}, nil
		})},
	})
	if err != nil {
		t.Fatal(err)
	}
	description := "initial settings"
	result, err := c.CreateApp(ctx, CreateAppOptions{Name: "example", Settings: SettingsUpdate{Description: &description}})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected cancellation, got %v", err)
	}
	var op *OperationError
	if !errors.As(err, &op) || op.AppID != "42" || op.Stage != "create response" || result.AppID != "42" {
		t.Errorf("created app ID must survive cancellation: result %#v, error %#v", result, err)
	}
	if calls != 1 {
		t.Errorf("expected only the create request, got %d requests", calls)
	}
}

func lifecycleClient(t *testing.T, origin string) *Client {
	t.Helper()
	zero := 0
	c, err := NewClient(Config{BaseURL: origin, Username: "test", Password: "test", MaxRetries: &zero, Wait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestLifecycleCallOrder(t *testing.T) {
	for _, mode := range []string{"update", "create", "create without settings"} {
		t.Run(mode, func(t *testing.T) {
			var calls []string
			var waits int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.Method+" "+r.URL.Path)
				switch r.Method + " " + r.URL.Path {
				case "POST /k/v1/preview/app.json":
					fmt.Fprint(w, `{"app":"42","revision":"3"}`)
				case "GET /k/v1/preview/app/settings.json":
					fmt.Fprint(w, `{"revision":"3"}`)
				case "PUT /k/v1/preview/app/settings.json":
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body["revision"] != "3" || body["app"] != "42" || body["description"] != "" {
						t.Errorf("unexpected conditional update: %#v", body)
					}
					fmt.Fprint(w, `{"revision":"4"}`)
				case "POST /k/v1/preview/app/deploy.json":
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if !reflect.DeepEqual(body, map[string]any{"apps": []any{map[string]any{"app": "42"}}}) {
						t.Errorf("deploy must omit revision: %#v", body)
					}
					fmt.Fprint(w, `{}`)
				case "GET /k/v1/preview/app/deploy.json":
					if r.URL.Query().Get("apps[0]") != "42" {
						t.Error("missing app status filter")
					}
					if waits == 0 {
						t.Error("initial poll did not wait")
					}
					fmt.Fprint(w, `{"apps":[{"app":"42","status":"SUCCESS"}]}`)
				case "GET /k/v1/app/settings.json":
					revision := "4"
					if mode == "create without settings" {
						revision = "3"
					}
					fmt.Fprintf(w, `{"revision":%q}`, revision)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(400)
				}
			}))
			defer server.Close()
			c := lifecycleClient(t, server.URL)
			c.wait = func(ctx context.Context, delay time.Duration) error {
				waits++
				if delay != time.Second {
					t.Errorf("poll interval %v", delay)
				}
				return ctx.Err()
			}
			description := ""
			update := SettingsUpdate{Description: &description}
			var result AppResult
			var err error
			if mode == "update" {
				result, err = c.UpdateApp(context.Background(), "42", update)
			} else {
				if mode == "create without settings" {
					update = SettingsUpdate{}
				}
				result, err = c.CreateApp(context.Background(), CreateAppOptions{Name: "example", Settings: update})
			}
			if err != nil || result.AppID != "42" {
				t.Fatalf("result %#v, error %v", result, err)
			}
			expected := []string{"GET /k/v1/preview/app/settings.json", "PUT /k/v1/preview/app/settings.json", "POST /k/v1/preview/app/deploy.json", "GET /k/v1/preview/app/deploy.json", "GET /k/v1/app/settings.json"}
			if mode == "create without settings" {
				expected = expected[2:]
			}
			if mode != "update" {
				expected = append([]string{"POST /k/v1/preview/app.json"}, expected...)
			}
			if !reflect.DeepEqual(calls, expected) {
				t.Errorf("calls %v, want %v", calls, expected)
			}
		})
	}
}

func TestLifecycleDeployStatesAndRevisions(t *testing.T) {
	for _, tc := range []struct {
		name      string
		statuses  []string
		revisions []string
		want      string
		conflict  bool
	}{
		{name: "processing then success", statuses: []string{"PROCESSING", "SUCCESS"}, revisions: []string{"4"}},
		{name: "stale success then current", statuses: []string{"SUCCESS", "SUCCESS"}, revisions: []string{"3", "4"}},
		{name: "newer revision", statuses: []string{"SUCCESS"}, revisions: []string{"5"}, want: "newer", conflict: true},
		{name: "fail", statuses: []string{"FAIL"}, want: "FAIL"},
		{name: "cancel", statuses: []string{"CANCEL"}, want: "CANCEL"},
		{name: "unknown", statuses: []string{"OTHER"}, want: "unknown"},
		{name: "missing", statuses: []string{""}, want: "missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			polls, reads := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					fmt.Fprint(w, `{}`)
					return
				}
				if r.URL.Path == "/k/v1/preview/app/deploy.json" {
					if polls >= len(tc.statuses) {
						t.Error("too many polls")
						w.WriteHeader(400)
						return
					}
					status := tc.statuses[polls]
					polls++
					if status == "" {
						fmt.Fprint(w, `{"apps":[{"app":"99","status":"SUCCESS"}]}`)
					} else {
						fmt.Fprintf(w, `{"apps":[{"app":"42","status":%q}]}`, status)
					}
					return
				}
				if reads >= len(tc.revisions) {
					t.Error("too many read-backs")
					w.WriteHeader(400)
					return
				}
				fmt.Fprintf(w, `{"revision":%q}`, tc.revisions[reads])
				reads++
			}))
			defer server.Close()
			_, err := lifecycleClient(t, server.URL).deployAndRead(context.Background(), "42", "4")
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v, want %q", err, tc.want)
			}
			if tc.conflict {
				var conflict *RevisionConflictError
				if !errors.As(err, &conflict) || conflict.Expected != "4" || conflict.Actual != "5" {
					t.Errorf("conflict detail: %v", err)
				}
			}
			if polls != len(tc.statuses) || reads != len(tc.revisions) {
				t.Errorf("polls=%d reads=%d", polls, reads)
			}
		})
	}
}

func TestLifecycleDeployDeadline(t *testing.T) {
	for _, stale := range []bool{false, true} {
		t.Run(fmt.Sprint(stale), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					fmt.Fprint(w, `{}`)
				} else if r.URL.Path == "/k/v1/app/settings.json" {
					fmt.Fprint(w, `{"revision":"3"}`)
				} else {
					status := "PROCESSING"
					if stale {
						status = "SUCCESS"
					}
					fmt.Fprintf(w, `{"apps":[{"app":"42","status":%q}]}`, status)
				}
			}))
			defer server.Close()
			c := lifecycleClient(t, server.URL)
			c.wait = waitContext
			c.pollInterval = time.Millisecond
			c.deployTimeout = 15 * time.Millisecond
			_, err := c.deployAndRead(context.Background(), "42", "4")
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("want deadline, got %v", err)
			}
		})
	}
}

func TestLifecycleCreateFailureRetainsID(t *testing.T) {
	for _, stage := range []string{"create response", "settings", "deploy", "poll", "read-back"} {
		t.Run(stage, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				key := r.Method + " " + r.URL.Path
				failure := map[string]string{"settings": "GET /k/v1/preview/app/settings.json", "deploy": "POST /k/v1/preview/app/deploy.json", "poll": "GET /k/v1/preview/app/deploy.json", "read-back": "GET /k/v1/app/settings.json"}[stage]
				if key == failure {
					w.WriteHeader(400)
					fmt.Fprint(w, `{"code":"BAD"}`)
					return
				}
				switch key {
				case "POST /k/v1/preview/app.json":
					if stage == "create response" {
						fmt.Fprint(w, `{"app":"42"}`)
					} else {
						fmt.Fprint(w, `{"app":"42","revision":"3"}`)
					}
				case "GET /k/v1/preview/app/settings.json":
					fmt.Fprint(w, `{"revision":"3"}`)
				case "PUT /k/v1/preview/app/settings.json":
					fmt.Fprint(w, `{"revision":"4"}`)
				case "POST /k/v1/preview/app/deploy.json":
					fmt.Fprint(w, `{}`)
				case "GET /k/v1/preview/app/deploy.json":
					fmt.Fprint(w, `{"apps":[{"app":"42","status":"SUCCESS"}]}`)
				default:
					t.Errorf("unexpected %s", key)
					w.WriteHeader(400)
				}
			}))
			defer server.Close()
			description := ""
			result, err := lifecycleClient(t, server.URL).CreateApp(context.Background(), CreateAppOptions{Name: "example", Settings: SettingsUpdate{Description: &description}})
			var op *OperationError
			if !errors.As(err, &op) || op.Stage != stage || op.AppID != "42" || result.AppID != "42" {
				t.Errorf("result %#v, error %#v", result, err)
			}
		})
	}
}

func TestLifecycleCrossClientLockThroughReadBack(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var previews atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /k/v1/preview/app/settings.json":
			previews.Add(1)
			fmt.Fprint(w, `{"revision":"3"}`)
		case "PUT /k/v1/preview/app/settings.json":
			fmt.Fprint(w, `{"revision":"4"}`)
		case "POST /k/v1/preview/app/deploy.json":
			fmt.Fprint(w, `{}`)
		case "GET /k/v1/preview/app/deploy.json":
			fmt.Fprintf(w, `{"apps":[{"app":%q,"status":"SUCCESS"}]}`, r.URL.Query().Get("apps[0]"))
		case "GET /k/v1/app/settings.json":
			if r.URL.Query().Get("app") == "42" {
				close(entered)
				<-release
			}
			fmt.Fprint(w, `{"revision":"4"}`)
		default:
			w.WriteHeader(400)
		}
	}))
	defer server.Close()
	c1, c2 := lifecycleClient(t, server.URL), lifecycleClient(t, server.URL+"/")
	description := ""
	update := SettingsUpdate{Description: &description}
	first := make(chan error, 1)
	go func() { _, err := c1.UpdateApp(context.Background(), "42", update); first <- err }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("first update did not reach read-back")
	}
	// Another app is allowed to finish while app 42's read-back remains blocked.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := c2.UpdateApp(ctx, "43", update); err != nil {
		close(release)
		t.Fatal(err)
	}
	waiting, cancelWaiting := context.WithTimeout(context.Background(), 20*time.Millisecond)
	_, err := c2.UpdateApp(waiting, "042", update)
	cancelWaiting()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("waiting update: %v", err)
	}
	if previews.Load() != 2 {
		t.Errorf("same app entered preview during read-back: %d", previews.Load())
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	// The canceled waiter and completed holder must both release references.
	unlock, err := acquireApp(ctx, c2.baseURL, "42")
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}

func TestLifecycleLockReleasedAfterFailure(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.WriteHeader(400)
			return
		}
		fmt.Fprint(w, `{"revision":"1"}`)
	}))
	defer server.Close()
	c := lifecycleClient(t, server.URL)
	description := ""
	if _, err := c.UpdateApp(context.Background(), "42", SettingsUpdate{Description: &description}); err == nil {
		t.Fatal("expected failure")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := c.UpdateApp(ctx, "42", SettingsUpdate{}); err != nil {
		t.Fatalf("lock leaked: %v", err)
	}
}

func TestLifecycleDeadlineCoversDeployAndReadBack(t *testing.T) {
	for _, stage := range []string{"deploy", "read-back"} {
		t.Run(stage, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				blocked := (stage == "deploy" && r.Method == http.MethodPost) || (stage == "read-back" && r.URL.Path == "/k/v1/app/settings.json")
				if blocked {
					select {
					case <-r.Context().Done():
					case <-time.After(time.Second):
					}
					return
				}
				if r.Method == http.MethodPost {
					fmt.Fprint(w, `{}`)
				} else {
					fmt.Fprint(w, `{"apps":[{"app":"42","status":"SUCCESS"}]}`)
				}
			}))
			defer server.Close()
			c := lifecycleClient(t, server.URL)
			c.deployTimeout = 20 * time.Millisecond
			_, err := c.deployAndRead(context.Background(), "42", "4")
			var op *OperationError
			if !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &op) || op.Stage != stage {
				t.Errorf("want %s deadline, got %v", stage, err)
			}
		})
	}
}

func TestLifecycleCreateMalformedRevisionRetainsID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"app":"42","revision":3}`)
	}))
	defer server.Close()
	result, err := lifecycleClient(t, server.URL).CreateApp(context.Background(), CreateAppOptions{Name: "example"})
	var op *OperationError
	if !errors.As(err, &op) || op.AppID != "42" || op.Stage != "create response" || result.AppID != "42" {
		t.Errorf("created app ID must survive malformed revision: result %#v, error %#v", result, err)
	}
}
