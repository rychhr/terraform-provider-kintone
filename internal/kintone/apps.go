// SPDX-License-Identifier: MPL-2.0

package kintone

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

// User identifies an app's creator or modifier.
type User struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// App is the published app metadata. Placement is nil outside a space.
type App struct {
	AppID       string  `json:"appId"`
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	SpaceID     *string `json:"spaceId"`
	ThreadID    *string `json:"threadId"`
	CreatedAt   string  `json:"createdAt"`
	Creator     User    `json:"creator"`
	ModifiedAt  string  `json:"modifiedAt"`
	Modifier    User    `json:"modifier"`
}

// ListAppsOptions filters every page of a ListApps request.
type ListAppsOptions struct {
	IDs      []string
	Codes    []string
	Name     string
	SpaceIDs []string
}

// GetApp retrieves metadata from the live environment.
func (c *Client) GetApp(ctx context.Context, appID string) (App, error) {
	if err := validateID(appID); err != nil {
		return App{}, fmt.Errorf("GetApp: %w", err)
	}
	var app App
	if err := c.request(ctx, http.MethodGet, "/k/v1/app.json", url.Values{"id": {appID}}, nil, &app, true); err != nil {
		return App{}, fmt.Errorf("GetApp: %w", err)
	}
	if err := validateID(app.AppID); err != nil {
		return App{}, errors.New("GetApp: invalid app ID in response")
	}
	if canonicalID(app.AppID) != canonicalID(appID) {
		return App{}, errors.New("GetApp: response contains a different app")
	}
	return app, nil
}

// ListApps retrieves all matching published apps. A failed page discards partial results.
func (c *Client) ListApps(ctx context.Context, options ListAppsOptions) ([]App, error) {
	if err := c.requirePassword("ListApps"); err != nil {
		return nil, err
	}
	if len(options.IDs) > 100 || len(options.SpaceIDs) > 100 || len(options.Codes) > 100 || utf8.RuneCountInString(options.Name) > 64 {
		return nil, errors.New("ListApps: filter exceeds API limits")
	}
	query := url.Values{"limit": {"100"}}
	for key, ids := range map[string][]string{"ids": options.IDs, "spaceIds": options.SpaceIDs} {
		for i, id := range ids {
			if err := validateID(id); err != nil {
				return nil, fmt.Errorf("ListApps: %w", err)
			}
			query.Set(fmt.Sprintf("%s[%d]", key, i), id)
		}
	}
	for i, code := range options.Codes {
		if code == "" || utf8.RuneCountInString(code) > 64 {
			return nil, errors.New("ListApps: app codes must contain 1 to 64 characters")
		}
		query.Set(fmt.Sprintf("codes[%d]", i), code)
	}
	if options.Name != "" {
		query.Set("name", options.Name)
	}
	apps := make([]App, 0)
	for offset := int64(0); offset <= 2147483647; offset += 100 {
		query.Set("offset", strconv.FormatInt(offset, 10))
		var page struct {
			Apps *[]App `json:"apps"`
		}
		if err := c.request(ctx, http.MethodGet, "/k/v1/apps.json", query, nil, &page, true); err != nil {
			return nil, fmt.Errorf("ListApps: %w", err)
		}
		if page.Apps == nil || len(*page.Apps) > 100 {
			return nil, errors.New("ListApps: invalid apps page")
		}
		for _, app := range *page.Apps {
			if err := validateID(app.AppID); err != nil {
				return nil, errors.New("ListApps: invalid app ID in response")
			}
		}
		apps = append(apps, (*page.Apps)...)
		if len(*page.Apps) < 100 {
			return apps, nil
		}
	}
	return nil, errors.New("ListApps: pagination exceeds API offset limit")
}

// CreateAppOptions defines the initial name, optional normal-space placement,
// and settings to apply before the app's single deployment. Settings.Name must
// be omitted; Name is the sole source of the app's initial name.
type CreateAppOptions struct {
	Name     string
	SpaceID  *string
	ThreadID *string
	Settings SettingsUpdate
}

// AppResult contains the created or updated app ID and verified live settings.
type AppResult struct {
	AppID    string
	Settings Settings
}

// CreateApp creates a preview app, applies settings, and deploys once. After an
// ID is assigned, failures are returned as *OperationError retaining that ID.
func (c *Client) CreateApp(ctx context.Context, options CreateAppOptions) (AppResult, error) {
	if err := c.requirePassword("CreateApp"); err != nil {
		return AppResult{}, err
	}
	if options.Name == "" || utf8.RuneCountInString(options.Name) > 64 {
		return AppResult{}, errors.New("CreateApp: name must contain 1 to 64 characters")
	}
	if options.Settings.Name != nil {
		return AppResult{}, errors.New("CreateApp: use Name instead of Settings.Name")
	}
	if err := validateSettingsUpdate(options.Settings); err != nil {
		return AppResult{}, fmt.Errorf("CreateApp: %w", err)
	}
	body := struct {
		Name   string       `json:"name"`
		Space  *json.Number `json:"space,omitempty"`
		Thread *json.Number `json:"thread,omitempty"`
	}{Name: options.Name}
	for _, id := range []*string{options.SpaceID, options.ThreadID} {
		if id != nil {
			if err := validateID(*id); err != nil {
				return AppResult{}, fmt.Errorf("CreateApp: %w", err)
			}
		}
	}
	if options.ThreadID != nil && options.SpaceID == nil {
		return AppResult{}, errors.New("CreateApp: thread requires a space")
	}
	// The create endpoint documents placement as JSON integers. Keep the public
	// IDs as strings and serialize exact decimal values without float conversion.
	if options.SpaceID != nil {
		n := json.Number(canonicalID(*options.SpaceID))
		body.Space = &n
	}
	if options.ThreadID != nil {
		n := json.Number(canonicalID(*options.ThreadID))
		body.Thread = &n
	}
	var created struct {
		App      string `json:"app"`
		Revision string `json:"revision"`
	}
	if err := c.request(ctx, http.MethodPost, "/k/v1/preview/app.json", nil, body, &created, false); err != nil {
		if validateID(created.App) == nil {
			return AppResult{AppID: created.App}, &OperationError{AppID: created.App, Stage: "create response", Err: err}
		}
		return AppResult{}, fmt.Errorf("CreateApp: %w", err)
	}
	if err := validateID(created.App); err != nil {
		return AppResult{}, errors.New("CreateApp: response did not contain a valid app ID; manual inspection may be required")
	}
	result := AppResult{AppID: created.App}
	fail := func(stage string, err error) (AppResult, error) {
		return result, &OperationError{AppID: created.App, Stage: stage, Err: err}
	}
	if err := validateRevision(created.Revision); err != nil {
		return fail("create response", err)
	}
	release, err := acquireApp(ctx, c.baseURL, created.App)
	if err != nil {
		return fail("lock", err)
	}
	defer release()
	revision := created.Revision
	if !options.Settings.empty() {
		revision, err = c.writeSettings(ctx, created.App, options.Settings)
		if err != nil {
			return fail("settings", err)
		}
	}
	result.Settings, err = c.deployAndRead(ctx, created.App, revision)
	if err != nil {
		return result, err
	}
	return result, nil
}

// UpdateApp serializes preview read/merge/write, deploy, and live read-back.
// An empty update reads live settings without deploying existing preview drafts.
func (c *Client) UpdateApp(ctx context.Context, appID string, update SettingsUpdate) (AppResult, error) {
	if err := validateID(appID); err != nil {
		return AppResult{}, fmt.Errorf("UpdateApp: %w", err)
	}
	if err := validateSettingsUpdate(update); err != nil {
		return AppResult{}, fmt.Errorf("UpdateApp: %w", err)
	}
	result := AppResult{AppID: appID}
	release, err := acquireApp(ctx, c.baseURL, appID)
	if err != nil {
		return result, &OperationError{AppID: appID, Stage: "lock", Err: err}
	}
	defer release()
	if update.empty() {
		result.Settings, err = c.GetLiveSettings(ctx, appID)
		return result, err
	}
	revision, err := c.writeSettings(ctx, appID, update)
	if err != nil {
		return result, &OperationError{AppID: appID, Stage: "settings", Err: err}
	}
	result.Settings, err = c.deployAndRead(ctx, appID, revision)
	return result, err
}

func validateID(id string) error {
	if !decimal(id) {
		return errors.New("ID must be a positive decimal integer")
	}
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 {
		return errors.New("ID must be between 1 and 9223372036854775807")
	}
	return nil
}

func validateRevision(revision string) error {
	if !decimal(revision) {
		return errors.New("invalid revision in API response")
	}
	return nil
}

func decimal(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func canonicalID(id string) string { return strings.TrimLeft(id, "0") }
