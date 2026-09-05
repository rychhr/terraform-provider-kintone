// SPDX-License-Identifier: MPL-2.0
package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/rychhr/terraform-provider-kintone/internal/kintone"
)

func appTestSchema() schema.Schema {
	var response resource.SchemaResponse
	(&appResource{}).Schema(context.Background(), resource.SchemaRequest{}, &response)
	return response.Schema
}
func appTestState(t *testing.T, m appModel) tfsdk.State {
	t.Helper()
	if m.TitleField.IsNull() {
		m.TitleField = types.ObjectNull(titleTypes)
	}
	if m.NumberPrecision.IsNull() {
		m.NumberPrecision = types.ObjectNull(precisionTypes)
	}
	s := tfsdk.State{Schema: appTestSchema()}
	if d := s.Set(context.Background(), &m); d.HasError() {
		t.Fatal(d)
	}
	return s
}

func TestAppResourceCreateFailures(t *testing.T) {
	for _, stage := range []string{"create", "settings", "deploy", "poll", "timeout", "read-back", "metadata", "invalid-settings"} {
		t.Run(stage, func(t *testing.T) {
			posts := 0
			settings := newAppFixture().preview
			if stage == "invalid-settings" {
				settings.NumberPrecision.Digits = "invalid"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fail := (stage == "create" && r.URL.Path == "/k/v1/preview/app.json") || (stage == "settings" && r.Method == "PUT") || (stage == "deploy" && r.Method == "POST" && strings.HasSuffix(r.URL.Path, "deploy.json")) || (stage == "poll" && r.Method == "GET" && strings.HasSuffix(r.URL.Path, "deploy.json")) || (stage == "read-back" && r.URL.Path == "/k/v1/app/settings.json") || (stage == "metadata" && r.URL.Path == "/k/v1/app.json")
				if r.Method == "POST" && r.URL.Path == "/k/v1/preview/app.json" {
					posts++
				}
				if fail {
					w.WriteHeader(403)
					_, _ = w.Write([]byte(`{"code":"DENIED"}`))
					return
				}
				var body any = settings
				switch r.Method + " " + r.URL.Path {
				case "POST /k/v1/preview/app.json":
					body = map[string]string{"app": "100", "revision": "1"}
				case "PUT /k/v1/preview/app/settings.json":
					body = map[string]string{"revision": "1"}
				case "POST /k/v1/preview/app/deploy.json":
					body = map[string]any{}
				case "GET /k/v1/preview/app/deploy.json":
					status := "SUCCESS"
					if stage == "timeout" {
						status = "PROCESSING"
					}
					body = map[string]any{"apps": []map[string]string{{"app": "100", "status": status}}}
				}
				_ = json.NewEncoder(w).Encode(body)
			}))
			defer server.Close()
			client, err := kintone.NewClient(kintone.Config{BaseURL: server.URL, Username: "fixture", Password: "fixture", PollInterval: time.Millisecond, DeployTimeout: 30 * time.Millisecond})
			if err != nil {
				t.Fatal(err)
			}
			r := appResource{client: client}
			config := appModel{Name: types.StringValue("tfacc-local"), Description: types.StringValue("managed")}
			plan := config
			plan.ID = types.StringUnknown()
			plan.Revision = types.StringUnknown()
			plan.TitleField = types.ObjectUnknown(titleTypes)
			plan.NumberPrecision = types.ObjectUnknown(precisionTypes)
			cs, ps := appTestState(t, config), appTestState(t, plan)
			resp := resource.CreateResponse{State: tfsdk.State{Schema: ps.Schema, Raw: tftypes.NewValue(ps.Schema.Type().TerraformType(context.Background()), nil)}}
			r.Create(context.Background(), resource.CreateRequest{Config: tfsdk.Config(cs), Plan: tfsdk.Plan(ps)}, &resp)
			if !resp.Diagnostics.HasError() || posts != 1 {
				t.Fatalf("expected one attempt and diagnostic: posts=%d, %v", posts, resp.Diagnostics)
			}
			if stage == "invalid-settings" {
				found := false
				for _, d := range resp.Diagnostics {
					if strings.Contains(d.Detail(), "App 100: live read-back decoding failed") {
						found = true
					}
				}
				if !found {
					t.Fatal("missing app ID or failure stage diagnostic")
				}
			}
			if stage == "create" {
				if !resp.State.Raw.IsNull() {
					t.Fatal("invented state before app ID")
				}
				return
			}
			var saved appModel
			if d := resp.State.Get(context.Background(), &saved); d.HasError() {
				t.Fatal(d)
			}
			if saved.ID.ValueString() != "100" || !resp.State.Raw.IsFullyKnown() {
				t.Fatalf("lost app ID or persisted unknowns: %v", saved.ID)
			}
		})
	}
}
func TestAppResourceReadFailureRetainsState(t *testing.T) {
	for _, status := range []int{401, 403, 404, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"code":"FIXTURE"}`))
			}))
			defer server.Close()
			retries := 0
			client, err := kintone.NewClient(kintone.Config{BaseURL: server.URL, APITokens: []string{"fixture"}, MaxRetries: &retries})
			if err != nil {
				t.Fatal(err)
			}
			state := appTestState(t, appModel{ID: types.StringValue("100"), Name: types.StringValue("retained")})
			response := resource.ReadResponse{State: state}
			(&appResource{client: client}).Read(context.Background(), resource.ReadRequest{State: state}, &response)
			if !response.Diagnostics.HasError() || !response.State.Raw.Equal(state.Raw) {
				t.Fatal("failed read lost state")
			}
		})
	}
}
func TestAppResourceDeleteOnlyRemovesState(t *testing.T) {
	state := appTestState(t, appModel{ID: types.StringValue("100"), Name: types.StringValue("tfacc-local")})
	resp := resource.DeleteResponse{State: state}
	(&appResource{}).Delete(context.Background(), resource.DeleteRequest{State: state}, &resp)
	if !resp.State.Raw.IsNull() || resp.Diagnostics.WarningsCount() != 1 {
		t.Fatal("expected removal and manual cleanup warning")
	}
}
