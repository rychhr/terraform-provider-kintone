// SPDX-License-Identifier: MPL-2.0

package kintone

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestListAppsPagination(t *testing.T) {
	for _, count := range []int{0, 99, 100, 101, 200} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				q := r.URL.Query()
				if r.Method != "GET" || r.URL.Path != "/k/v1/apps.json" || q.Get("limit") != "100" || q.Get("offset") != strconv.Itoa(calls*100) || q.Get("ids[0]") != "9007199254740993" || q.Get("codes[0]") != "CODE" || q.Get("spaceIds[0]") != "20" || q.Get("name") != "日本語 & filter" {
					t.Errorf("unexpected query %s", r.URL)
				}
				remaining := min(100, count-calls*100)
				calls++
				apps := make([]App, remaining)
				for i := range apps {
					apps[i] = App{AppID: strconv.Itoa(i + 1)}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"apps": apps})
			}))
			defer srv.Close()
			c, err := NewClient(Config{BaseURL: srv.URL, Username: "user", Password: "password"})
			if err != nil {
				t.Fatal(err)
			}
			got, err := c.ListApps(t.Context(), ListAppsOptions{IDs: []string{"9007199254740993"}, Codes: []string{"CODE"}, SpaceIDs: []string{"20"}, Name: "日本語 & filter"})
			if err != nil || len(got) != count || calls != count/100+1 {
				t.Fatalf("count %d calls %d error %v", len(got), calls, err)
			}
		})
	}
}

func TestListAppsFailedPage(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 2 {
			w.WriteHeader(403)
			return
		}
		apps := make([]App, 100)
		for i := range apps {
			apps[i].AppID = strconv.Itoa(i + 1)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"apps": apps})
	}))
	defer srv.Close()
	c, err := NewClient(Config{BaseURL: srv.URL, Username: "user", Password: "password"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.ListApps(t.Context(), ListAppsOptions{})
	var api *APIError
	if got != nil || !errors.As(err, &api) || api.StatusCode != 403 {
		t.Fatalf("partial page returned: %v %v", got, err)
	}
}

func TestGetAppPlacementAndStringIDs(t *testing.T) {
	for _, placed := range []bool{false, true} {
		t.Run(strconv.FormatBool(placed), func(t *testing.T) {
			want := App{AppID: "9007199254740993", Name: "app", Code: "CODE", Creator: User{Code: "creator"}}
			if placed {
				space, thread := "20", "30"
				want.SpaceID = &space
				want.ThreadID = &thread
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/k/v1/app.json" || r.URL.Query().Get("id") != want.AppID {
					t.Error("incorrect live metadata URL")
				}
				_ = json.NewEncoder(w).Encode(want)
			}))
			defer srv.Close()
			c, err := NewClient(Config{BaseURL: srv.URL, APITokens: []string{"token"}})
			if err != nil {
				t.Fatal(err)
			}
			got, err := c.GetApp(t.Context(), want.AppID)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("got %#v error %v", got, err)
			}
		})
	}
}

func TestAppValidationBeforeCommunication(t *testing.T) {
	c, err := NewClient(Config{BaseURL: "https://example.com", APITokens: []string{"token"}, HTTPClient: &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		t.Error("unexpected communication")
		return nil, fmt.Errorf("unexpected communication")
	})}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.CreateApp(t.Context(), CreateAppOptions{Name: "app"}); err == nil || !strings.Contains(err.Error(), "CreateApp requires password") {
		t.Fatal(err)
	}
	if _, err = c.ListApps(t.Context(), ListAppsOptions{}); err == nil || !strings.Contains(err.Error(), "ListApps requires password") {
		t.Fatal(err)
	}
	for _, id := range []string{"", "0", "-1", "1&app=2", "9223372036854775808"} {
		if _, err = c.GetApp(t.Context(), id); err == nil {
			t.Fatalf("GetApp accepted %q", id)
		}
		if _, err = c.UpdateApp(t.Context(), id, SettingsUpdate{}); err == nil {
			t.Fatalf("UpdateApp accepted %q", id)
		}
	}
	c.passwordAuth = true
	if _, err = c.CreateApp(t.Context(), CreateAppOptions{}); err == nil {
		t.Error("accepted empty name")
	}
	thread := "1"
	if _, err = c.CreateApp(t.Context(), CreateAppOptions{Name: "app", ThreadID: &thread}); err == nil {
		t.Error("accepted thread without space")
	}
	if _, err = c.ListApps(t.Context(), ListAppsOptions{IDs: make([]string, 101)}); err == nil {
		t.Error("accepted too many ids")
	}
}

func TestAppLockCanonicalIDAndCancellation(t *testing.T) {
	release, err := acquireApp(t.Context(), "https://example.com", "001")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err = acquireApp(ctx, "https://example.com", "1"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	release()
	release, err = acquireApp(t.Context(), "https://example.com", "1")
	if err != nil {
		t.Fatal(err)
	}
	release()
	appLocks.Lock()
	defer appLocks.Unlock()
	if len(appLocks.entries) != 0 {
		t.Error("idle locks retained")
	}
}

func TestCreatePlacementUsesExactJSONNumbers(t *testing.T) {
	space, thread := "9007199254740993", "42"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/k/v1/preview/app.json" {
			t.Error("unexpected request")
		}
		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		var body map[string]any
		if err := decoder.Decode(&body); err != nil {
			t.Error(err)
		}
		want := map[string]any{"name": "example", "space": json.Number(space), "thread": json.Number(thread)}
		if !reflect.DeepEqual(body, want) {
			t.Errorf("body %#v", body)
		}
		// Stop after the request under test without deploying a fixture app.
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	c, err := NewClient(Config{BaseURL: srv.URL, Username: "user", Password: "password"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.CreateApp(t.Context(), CreateAppOptions{Name: "example", SpaceID: &space, ThreadID: &thread}); err == nil {
		t.Error("expected API error")
	}
}
