// SPDX-License-Identifier: MPL-2.0

package kintone

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"sync"
)

// OperationError retains the app ID and failure stage after a mutation starts.
// A failed operation may leave preview settings or an undeployed app behind;
// the client never reverts other actors' drafts or attempts physical deletion.
type OperationError struct {
	AppID string
	Stage string
	Err   error
}

func (e *OperationError) Error() string {
	return fmt.Sprintf("app %s: %s: %v", e.AppID, e.Stage, e.Err)
}
func (e *OperationError) Unwrap() error { return e.Err }

// RevisionConflictError means another writer deployed beyond our revision.
type RevisionConflictError struct {
	AppID    string
	Expected string
	Actual   string
}

func (e *RevisionConflictError) Error() string {
	return fmt.Sprintf("app %s: live revision is newer than the requested revision", e.AppID)
}
func (e *RevisionConflictError) IsConflict() bool { return true }

// writeSettings is the shared read-modify-write foundation. The caller holds
// the app lock through this operation and the subsequent deploy/read-back.
func (c *Client) writeSettings(ctx context.Context, appID string, update SettingsUpdate) (string, error) {
	current, err := c.GetPreviewSettings(ctx, appID)
	if err != nil {
		return "", err
	}
	merged, err := mergeSettings(current, update)
	if err != nil {
		return "", err
	}
	body := struct {
		App      string `json:"app"`
		Revision string `json:"revision"`
		SettingsUpdate
	}{App: appID, Revision: current.Revision, SettingsUpdate: merged}
	var written struct {
		Revision string `json:"revision"`
	}
	// Even though PUT is idempotent, an uncertain response cannot be replayed
	// with a revision condition: the first request may have consumed that revision.
	if err := c.request(ctx, http.MethodPut, "/k/v1/preview/app/settings.json", nil, body, &written, false); err != nil {
		return "", err
	}
	if err := validateRevision(written.Revision); err != nil {
		return "", err
	}
	return written.Revision, nil
}

func (c *Client) deployAndRead(ctx context.Context, appID, revision string) (Settings, error) {
	ctx, cancel := context.WithTimeout(ctx, c.deployTimeout)
	defer cancel()
	fail := func(stage string, err error) (Settings, error) {
		return Settings{}, &OperationError{AppID: appID, Stage: stage, Err: err}
	}
	// Deploy revision semantics remain unmeasured; omit it as required by the
	// repository API constraints. Live revision verification detects overtakes.
	body := struct {
		Apps []map[string]string `json:"apps"`
	}{Apps: []map[string]string{{"app": appID}}}
	if err := c.request(ctx, http.MethodPost, "/k/v1/preview/app/deploy.json", nil, body, nil, false); err != nil {
		return fail("deploy", err)
	}
	for {
		if err := c.wait(ctx, c.pollInterval); err != nil {
			return fail("poll", err)
		}
		if err := ctx.Err(); err != nil {
			return fail("poll", err)
		}
		var statuses struct {
			Apps []struct {
				App    string `json:"app"`
				Status string `json:"status"`
			} `json:"apps"`
		}
		if err := c.request(ctx, http.MethodGet, "/k/v1/preview/app/deploy.json", url.Values{"apps[0]": {appID}}, nil, &statuses, true); err != nil {
			return fail("poll", err)
		}
		status := ""
		for _, entry := range statuses.Apps {
			if canonicalID(entry.App) == canonicalID(appID) {
				if status != "" {
					return fail("poll", errors.New("duplicate app in deploy status response"))
				}
				status = entry.Status
			}
		}
		switch status {
		case "PROCESSING":
			continue
		case "FAIL", "CANCEL":
			return fail("poll", fmt.Errorf("deployment ended with %s", status))
		case "SUCCESS":
			live, err := c.GetLiveSettings(ctx, appID)
			if err != nil {
				return fail("read-back", err)
			}
			if err := ctx.Err(); err != nil {
				return fail("read-back", err)
			}
			actual, _ := new(big.Int).SetString(live.Revision, 10)
			expected, _ := new(big.Int).SetString(revision, 10)
			switch actual.Cmp(expected) {
			case 0:
				return live, nil
			case 1:
				return fail("read-back", &RevisionConflictError{AppID: appID, Expected: revision, Actual: live.Revision})
			}
			// An earlier deployment may still report SUCCESS. Wait until the live
			// revision matches this write, under the same deadline and app lock.
		case "":
			return fail("poll", errors.New("app missing from deploy status response"))
		default:
			return fail("poll", errors.New("unknown deploy status"))
		}
	}
}

type appLock struct {
	token chan struct{}
	refs  int
}

var appLocks = struct {
	sync.Mutex
	entries map[string]*appLock
}{entries: make(map[string]*appLock)}

// Locks span all clients for a normalized origin and numeric app ID. Reference
// counting removes idle keys without allowing a waiter to acquire a stale lock.
func acquireApp(ctx context.Context, origin, appID string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key := origin + "/" + canonicalID(appID)
	appLocks.Lock()
	lock := appLocks.entries[key]
	if lock == nil {
		lock = &appLock{token: make(chan struct{}, 1)}
		lock.token <- struct{}{}
		appLocks.entries[key] = lock
	}
	lock.refs++
	appLocks.Unlock()
	drop := func() {
		appLocks.Lock()
		defer appLocks.Unlock()
		lock.refs--
		if lock.refs == 0 {
			delete(appLocks.entries, key)
		}
	}
	select {
	case <-ctx.Done():
		drop()
		return nil, ctx.Err()
	case <-lock.token:
		release := func() { lock.token <- struct{}{}; drop() }
		if err := ctx.Err(); err != nil {
			release()
			return nil, err
		}
		return release, nil
	}
}
