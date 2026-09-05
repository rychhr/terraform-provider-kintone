// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/rychhr/terraform-provider-kintone/internal/kintone"
)

func integrationFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){"kintone": providerserver.NewProtocol6WithError(New("test")())}
}

// This test uses real Terraform but all HTTP requests stay on the local fixture.
func TestAppTerraformLifecycle(t *testing.T) {
	fixture := newAppFixture()
	server := httptest.NewServer(fixture)
	defer server.Close()
	config := func(settings string) string {
		return fmt.Sprintf(`provider "kintone" {
 base_url = %q
 username = "fixture-user"
 password = "fixture-password"
}
resource "kintone_app" "test" {
 name = "tfacc-local"
 %s
}`, server.URL, settings)
	}
	initial := config(`description = "managed"
comments_enabled = true
number_precision = { total_digits = 12, decimal_places = 3, rounding_mode = "HALF_EVEN" }
title_field = { selection_mode = "MANUAL", field_code = "manual_title" }`)
	updated := config(`description = ""
comments_enabled = false
number_precision = { decimal_places = 4 }
title_field = { selection_mode = "AUTO" }`)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: integrationFactories(),
		Steps: []resource.TestStep{
			{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("kintone_app.test", "id", "100"), fixture.checkDeployments(1),
			)},
			{ResourceName: "kintone_app.test", ImportState: true, ImportStateVerify: true},
			{Config: initial, PlanOnly: true},
			{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("kintone_app.test", "description", ""),
				resource.TestCheckResourceAttr("kintone_app.test", "comments_enabled", "false"),
				resource.TestCheckResourceAttr("kintone_app.test", "number_precision.total_digits", "12"),
				resource.TestCheckResourceAttr("kintone_app.test", "number_precision.decimal_places", "4"),
				resource.TestCheckResourceAttr("kintone_app.test", "number_precision.rounding_mode", "HALF_EVEN"),
				resource.TestCheckResourceAttr("kintone_app.test", "title_field.field_code", "auto_title"),
				fixture.checkDeployments(2),
			)},
			{Config: config(""), Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("kintone_app.test", "comments_enabled", "false"),
				resource.TestCheckResourceAttr("kintone_app.test", "number_precision.decimal_places", "4"),
				fixture.checkDeployments(2),
			)},
		},
		CheckDestroy: func(_ *terraform.State) error {
			fixture.mu.Lock()
			defer fixture.mu.Unlock()
			if fixture.creates != 1 || fixture.deletes != 0 {
				return fmt.Errorf("expected one creation and no deletion, got %d and %d", fixture.creates, fixture.deletes)
			}
			return nil
		},
	})
}

func TestAppTerraformDataSources(t *testing.T) {
	fixture := newAppFixture()
	server := httptest.NewServer(fixture)
	defer server.Close()
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: integrationFactories(),
		Steps: []resource.TestStep{{
			Config: fmt.Sprintf(`provider "kintone" {
 base_url = %q
 username = "fixture-user"
 password = "fixture-password"
}
data "kintone_app" "test" { id = "100" }
data "kintone_apps" "test" { ids = ["100"] }
data "kintone_apps" "empty" { name = "no-match" }
`, server.URL),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.kintone_app.test", "name", "tfacc-local"),
				resource.TestCheckResourceAttr("data.kintone_app.test", "title_field.field_code", "auto_title"),
				resource.TestCheckResourceAttr("data.kintone_apps.test", "apps.#", "1"),
				resource.TestCheckResourceAttr("data.kintone_apps.test", "apps.0.id", "100"),
				resource.TestCheckResourceAttr("data.kintone_apps.empty", "apps.#", "0"),
			),
		}},
	})
}

func TestAppTerraformTokenOnly(t *testing.T) {
	// Prevent generic password credentials from changing the tested authentication.
	t.Setenv("KINTONE_USERNAME", "")
	t.Setenv("KINTONE_PASSWORD", "")
	fixture := newAppFixture()
	server := httptest.NewServer(fixture)
	defer server.Close()
	config := fmt.Sprintf(`provider "kintone" {
 base_url = %q
 api_tokens = ["fixture-token"]
}
`, server.URL)
	for _, tc := range []struct{ name, body, errorPattern string }{
		{"single-read", `data "kintone_app" "test" { id = "100" }`, ""},
		{"create", `resource "kintone_app" "test" { name = "tfacc-local" }`, "CreateApp.*password"},
		{"list", `data "kintone_apps" "test" {}`, "ListApps.*password"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			step := resource.TestStep{Config: config + tc.body}
			if tc.errorPattern != "" {
				step.ExpectError = regexp.MustCompile(tc.errorPattern)
			}
			resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: integrationFactories(), Steps: []resource.TestStep{step}})
		})
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.creates != 0 {
		t.Fatalf("token-only configuration created %d apps", fixture.creates)
	}
}

func TestAppTerraformRejectedUpdates(t *testing.T) {
	fixture := newAppFixture()
	server := httptest.NewServer(fixture)
	defer server.Close()
	config := func(settings string) string {
		return fmt.Sprintf(`provider "kintone" {
 base_url = %q
 username = "fixture-user"
 password = "fixture-password"
}
resource "kintone_app" "test" {
 name = "tfacc-local"
 %s
}`, server.URL, settings)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: integrationFactories(),
		Steps: []resource.TestStep{
			{Config: config("")},
			{Config: config(`space_id = "10"`), PlanOnly: true, ExpectError: regexp.MustCompile("Placement cannot be changed")},
			{Config: config(`title_field = { selection_mode = "AUTO", field_code = "manual_title" }`), PlanOnly: true, ExpectError: regexp.MustCompile("Omit field_code in AUTO mode")},
			{Config: config(""), Check: fixture.checkDeployments(1)},
		},
	})
}

// A known ID survives a failed Create, but Terraform still taints it. A retry
// therefore proposes replacement and requires manual operator recovery first.
func TestAppTerraformPartialCreateTainted(t *testing.T) {
	fixture := newAppFixture()
	fixture.failReadBack = true
	server := httptest.NewServer(fixture)
	defer server.Close()
	config := fmt.Sprintf(`provider "kintone" {
 base_url = %q
 username = "fixture-user"
 password = "fixture-password"
}
resource "kintone_app" "test" { name = "tfacc-local" }
`, server.URL)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: integrationFactories(),
		Steps: []resource.TestStep{
			{Config: config, ExpectError: regexp.MustCompile(`(?s)100.*read-back|read-back.*100`)},
			{Config: config, PlanOnly: true, ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPreRefresh: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("kintone_app.test", plancheck.ResourceActionDestroyBeforeCreate),
				}},
			},
		},
	})
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.creates != 1 || fixture.deletes != 0 {
		t.Fatalf("unexpected mutation after failure: creates=%d deletes=%d", fixture.creates, fixture.deletes)
	}
}

// TestAccApp creates one real app. Running it requires separate maintainer approval;
// the environment guard alone is not authorization. Delete only removes state.
func TestAccApp(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" {
		t.Skip("set TF_ACC for live acceptance tests")
	}
	if os.Getenv("KINTONE_DEV_ALLOW_ACCEPTANCE_TESTS") != "1" {
		t.Fatal("KINTONE_DEV_ALLOW_ACCEPTANCE_TESTS=1 is required")
	}
	for _, pair := range [][2]string{{"KINTONE_DEV_BASE_URL", "KINTONE_BASE_URL"}, {"KINTONE_DEV_USERNAME", "KINTONE_USERNAME"}, {"KINTONE_DEV_PASSWORD", "KINTONE_PASSWORD"}} {
		value := os.Getenv(pair[0])
		if value == "" {
			t.Fatalf("%s is required; generic credentials are never used", pair[0])
		}
		t.Setenv(pair[1], value)
	}
	t.Setenv("KINTONE_API_TOKENS", "")
	name := fmt.Sprintf("tfacc-app-%d", time.Now().UnixNano())
	t.Logf("Manual cleanup required for app %s, including after test failure", name)
	config := func(description string) string {
		return fmt.Sprintf(`provider "kintone" {}
resource "kintone_app" "test" {
 name = %q
 description = %q
}`, name, description)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: integrationFactories(),
		Steps: []resource.TestStep{
			{Config: config("Acceptance test"), Check: func(state *terraform.State) error {
				app, ok := state.RootModule().Resources["kintone_app.test"]
				if !ok || app.Primary.ID == "" {
					return fmt.Errorf("created app ID is missing")
				}
				t.Logf("Manual cleanup: app %s (ID %s)", name, app.Primary.ID)
				return nil
			}},
			{ResourceName: "kintone_app.test", ImportState: true, ImportStateVerify: true},
			{Config: config("Updated acceptance test")},
			{Config: config("Updated acceptance test"), PlanOnly: true},
		},
	})
}

type appFixture struct {
	mu                        sync.Mutex
	preview                   kintone.Settings
	live                      kintone.Settings
	creates, deploys, deletes int
	failReadBack              bool
}

func newAppFixture() *appFixture {
	settings := kintone.Settings{
		Name: "tfacc-local", Theme: "WHITE", Revision: "1",
		TitleField:             kintone.TitleField{SelectionMode: "AUTO", Code: "auto_title"},
		NumberPrecision:        kintone.NumberPrecision{Digits: "16", DecimalPlaces: "4", RoundingMode: "HALF_EVEN"},
		FirstMonthOfFiscalYear: "1", EnableComments: true, EnableThumbnails: true,
		EnableDuplicateRecord: true, EnableInlineRecordEditing: true,
	}
	return &appFixture{preview: settings, live: settings}
}

func (f *appFixture) checkDeployments(want int) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.deploys != want {
			return fmt.Errorf("deploy count: got %d, want %d", f.deploys, want)
		}
		return nil
	}
}

func (f *appFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	var result any
	switch r.Method + " " + r.URL.Path {
	case "POST /k/v1/preview/app.json":
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", 400)
			return
		}
		f.creates++
		f.preview.Name = body.Name
		result = map[string]string{"app": "100", "revision": f.preview.Revision}
	case "GET /k/v1/preview/app/settings.json":
		result = f.preview
	case "GET /k/v1/app/settings.json":
		if f.failReadBack {
			f.failReadBack = false
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"code":"GAIA_NO01","message":"Fixture read-back denied"}`))
			return
		}
		result = f.live
	case "PUT /k/v1/preview/app/settings.json":
		revision, _ := strconv.Atoi(f.preview.Revision)
		if err := json.NewDecoder(r.Body).Decode(&f.preview); err != nil {
			http.Error(w, "invalid JSON", 400)
			return
		}
		if f.preview.TitleField.SelectionMode == "AUTO" {
			f.preview.TitleField.Code = "auto_title"
		}
		f.preview.Revision = strconv.Itoa(revision + 1)
		result = map[string]string{"revision": f.preview.Revision}
	case "POST /k/v1/preview/app/deploy.json":
		f.deploys++
		f.live = f.preview
		result = map[string]any{}
	case "GET /k/v1/preview/app/deploy.json":
		result = map[string]any{"apps": []map[string]string{{"app": "100", "status": "SUCCESS"}}}
	case "GET /k/v1/apps.json":
		apps := []kintone.App{}
		if r.URL.Query().Get("name") != "no-match" {
			apps = append(apps, kintone.App{AppID: "100", Name: f.live.Name, Description: f.live.Description})
		}
		result = map[string]any{"apps": apps}
	case "GET /k/v1/app.json":
		result = kintone.App{AppID: "100", Name: f.live.Name, Description: f.live.Description,
			CreatedAt: "2026-01-01T00:00:00Z", ModifiedAt: "2026-01-01T00:00:00Z",
			Creator: kintone.User{Code: "fixture", Name: "Fixture"}, Modifier: kintone.User{Code: "fixture", Name: "Fixture"}}
	default:
		if r.Method == http.MethodDelete {
			f.deletes++
		}
		http.Error(w, `{"code":"NOT_FOUND","message":"Unexpected endpoint"}`, http.StatusNotFound)
		return
	}
	if err := json.NewEncoder(w).Encode(result); err != nil {
		return
	}
}
