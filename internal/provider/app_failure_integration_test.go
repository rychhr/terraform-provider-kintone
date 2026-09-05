// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"

	"github.com/rychhr/terraform-provider-kintone/internal/kintone"
)

// Failed deployment leaves an unpublished app: normal refresh must report the
// read error and retain its tainted ID, rather than silently creating another app.
func TestAppTerraformCreateFailureRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, stage, status, diagnostic string
	}{
		{"before-id", "create", "", "CreateApp"},
		{"deploy-rejected", "deploy", "", "app 100: deploy"},
		{"deploy-failed", "poll", "FAIL", "deployment ended with FAIL"},
		{"deploy-canceled", "poll", "CANCEL", "deployment ended with CANCEL"},
		{"deploy-timeout", "poll", "PROCESSING", "context deadline exceeded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newAppFixture()
			published := false // Protected by fixture.mu, including test-step changes.
			createRequests, pollRequests := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fixture.mu.Lock()
				defer fixture.mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				switch r.Method + " " + r.URL.Path {
				case "POST /k/v1/preview/app.json":
					createRequests++
					if tc.stage == "create" {
						w.WriteHeader(http.StatusForbidden)
						_, _ = w.Write([]byte(`{"code":"GAIA_NO01","message":"Fixture create denied"}`))
						return
					}
					fixture.creates++
					_, _ = w.Write([]byte(`{"app":"100","revision":"1"}`))
				case "POST /k/v1/preview/app/deploy.json":
					fixture.deploys++
					if tc.stage == "deploy" {
						w.WriteHeader(http.StatusForbidden)
						_, _ = w.Write([]byte(`{"code":"GAIA_NO01","message":"Fixture deploy denied"}`))
						return
					}
					_, _ = w.Write([]byte(`{}`))
				case "GET /k/v1/preview/app/deploy.json":
					pollRequests++
					_, _ = fmt.Fprintf(w, `{"apps":[{"app":"100","status":%q}]}`, tc.status)
				case "GET /k/v1/app.json", "GET /k/v1/app/settings.json":
					if !published {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"code":"GAIA_AP01","message":"Fixture app is unpublished"}`))
						return
					}
					if r.URL.Path == "/k/v1/app.json" {
						_ = json.NewEncoder(w).Encode(kintone.App{AppID: "100", Name: fixture.live.Name})
					} else {
						_ = json.NewEncoder(w).Encode(fixture.live)
					}
				default:
					if r.Method == http.MethodDelete {
						fixture.deletes++
					}
					http.Error(w, "Unexpected fixture endpoint", http.StatusNotFound)
				}
			}))
			defer server.Close()
			config := fmt.Sprintf(`provider "kintone" {
 base_url = %q
 username = "fixture-user"
 password = "fixture-password"
}
resource "kintone_app" "test" { name = "tfacc-local" }
`, server.URL)
			workDir := t.TempDir()
			hasID := tc.stage != "create"
			checkState := func() { checkFailedAppRawState(t, workDir, hasID) }
			steps := []resource.TestStep{{Config: config, ExpectError: regexp.MustCompile(tc.diagnostic)}}
			if hasID {
				steps = append(steps, resource.TestStep{
					PreConfig: checkState, Config: config, PlanOnly: true, ExpectNonEmptyPlan: true,
					ExpectError: regexp.MustCompile(`(?s)Unable to read app.*State is retained`),
				})
			}
			steps = append(steps, resource.TestStep{
				PreConfig: func() {
					checkState()
					// Simulate an operator publishing the existing app, allowing ordinary
					// refresh and the harness's state-only destroy to complete locally.
					fixture.mu.Lock()
					published = true
					fixture.mu.Unlock()
				},
				Config: config, PlanOnly: true, ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPreRefresh: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("kintone_app.test", func() plancheck.ResourceActionType {
						if hasID {
							return plancheck.ResourceActionDestroyBeforeCreate
						}
						return plancheck.ResourceActionCreate
					}()),
				}},
			})
			resource.UnitTest(t, resource.TestCase{
				WorkingDir: workDir,
				ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
					"kintone": providerserver.NewProtocol6WithError(&shortDeployTestProvider{Provider: New("test")(), baseURL: server.URL}),
				},
				Steps: steps,
			})
			fixture.mu.Lock()
			defer fixture.mu.Unlock()
			wantCreates := 0
			if hasID {
				wantCreates = 1
			}
			if createRequests != 1 || fixture.creates != wantCreates || fixture.deletes != 0 || fixture.deploys != wantCreates {
				t.Fatalf("unexpected mutations: create requests=%d, apps=%d, deploys=%d, deletes=%d", createRequests, fixture.creates, fixture.deploys, fixture.deletes)
			}
			if (tc.stage == "poll") != (pollRequests > 0) {
				t.Fatalf("unexpected poll count: %d", pollRequests)
			}
		})
	}
}

// ExpectError skips normal Check hooks. Inspect Terraform's persisted state at
// the next step to verify both the failed apply and the failed refresh result.
func checkFailedAppRawState(t *testing.T, workDir string, hasID bool) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(workDir, "work*", "terraform.tfstate"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 && !hasID {
		return
	}
	if len(paths) != 1 {
		t.Fatalf("expected one state file, got %d", len(paths))
	}
	contents, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		Resources []struct {
			Type      string `json:"type"`
			Name      string `json:"name"`
			Instances []struct {
				Status     string `json:"status"`
				Attributes struct {
					ID string `json:"id"`
				} `json:"attributes"`
			} `json:"instances"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(contents, &state); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range state.Resources {
		if entry.Type != "kintone_app" || entry.Name != "test" {
			continue
		}
		for _, instance := range entry.Instances {
			count++
			if instance.Attributes.ID != "100" || instance.Status != "tainted" {
				t.Fatalf("failed app state: ID=%q, status=%q", instance.Attributes.ID, instance.Status)
			}
		}
	}
	want := 0
	if hasID {
		want = 1
	}
	if count != want {
		t.Fatalf("failed app instances: got %d, want %d", count, want)
	}
}

// Exercise the production provider with a short client deadline without adding
// a test-only setting to its public schema or waiting five minutes per timeout.
type shortDeployTestProvider struct {
	frameworkprovider.Provider
	baseURL string
}

func (p *shortDeployTestProvider) Configure(ctx context.Context, req frameworkprovider.ConfigureRequest, resp *frameworkprovider.ConfigureResponse) {
	p.Provider.Configure(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	client, err := kintone.NewClient(kintone.Config{
		BaseURL: p.baseURL, Username: "fixture-user", Password: "fixture-password",
		PollInterval: 10 * time.Millisecond, DeployTimeout: 250 * time.Millisecond,
	})
	if err != nil {
		resp.Diagnostics.AddError("Invalid fixture client", err.Error())
		return
	}
	resp.ResourceData = client
	resp.DataSourceData = client
}
