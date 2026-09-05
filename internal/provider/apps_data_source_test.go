// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/rychhr/terraform-provider-kintone/internal/kintone"
)

func readDataSource(t *testing.T, ds datasource.DataSource, handler http.HandlerFunc, config map[string]attr.Value) datasource.ReadResponse {
	t.Helper()
	ctx := context.Background()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	retries := 0
	client, err := kintone.NewClient(kintone.Config{BaseURL: server.URL, Username: "test", Password: "test", MaxRetries: &retries})
	if err != nil {
		t.Fatal(err)
	}
	cr := datasource.ConfigureResponse{}
	ds.(datasource.DataSourceWithConfigure).Configure(ctx, datasource.ConfigureRequest{ProviderData: client}, &cr)
	if cr.Diagnostics.HasError() {
		t.Fatal(cr.Diagnostics)
	}
	sr := datasource.SchemaResponse{}
	ds.Schema(ctx, datasource.SchemaRequest{}, &sr)
	attrs := map[string]attr.Type{}
	values := map[string]attr.Value{}
	for name, a := range sr.Schema.Attributes {
		attrs[name] = a.GetType()
		v, err := a.GetType().ValueFromTerraform(ctx, nullTFValue(t, a.GetType()))
		if err != nil {
			t.Fatal(err)
		}
		values[name] = v
	}
	for name, v := range config {
		values[name] = v
	}
	obj := types.ObjectValueMust(attrs, values)
	raw, err := obj.ToTerraformValue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: sr.Schema}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Raw: raw, Schema: sr.Schema}}, &resp)
	return resp
}

func TestAppsDataSourceRead(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{true: "empty", false: "numeric order"}[empty], func(t *testing.T) {
			resp := readDataSource(t, NewAppsDataSource(), func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/k/v1/apps.json" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				if r.URL.Query().Get("name") != "demo" {
					t.Error("name filter missing")
				}
				apps := []kintone.App{}
				if !empty {
					apps = []kintone.App{{AppID: "10", Name: "ten"}, {AppID: "2", Name: "two"}}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"apps": apps})
			}, map[string]attr.Value{"name": types.StringValue("demo")})
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			var apps types.List
			d := resp.State.GetAttribute(context.Background(), path.Root("apps"), &apps)
			if d.HasError() {
				t.Fatal(d)
			}
			if apps.IsNull() {
				t.Fatal("apps must be a concrete list")
			}
			if empty {
				if len(apps.Elements()) != 0 {
					t.Fatal(apps)
				}
				return
			}
			if apps.Elements()[0].(types.Object).Attributes()["id"] != types.StringValue("2") {
				t.Fatal(apps)
			}
		})
	}
}

func TestAppDataSourceMissing(t *testing.T) {
	resp := readDataSource(t, NewAppDataSource(), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"GAIA_AP01","message":"missing"}`))
	}, map[string]attr.Value{"id": types.StringValue("12")})
	if !resp.Diagnostics.HasError() {
		t.Fatal("missing app must be an error")
	}
}

func nullTFValue(t *testing.T, typ attr.Type) tftypes.Value {
	t.Helper()
	return tftypes.NewValue(typ.TerraformType(context.Background()), nil)
}

func TestAppsDataSourcePagination(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "all pages", true: "failed page"}[fail], func(t *testing.T) {
			calls := 0
			resp := readDataSource(t, NewAppsDataSource(), func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls == 2 && fail {
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"code":"CB_NO02"}`))
					return
				}
				apps := []kintone.App{}
				if calls == 1 {
					for i := 100; i >= 1; i-- {
						apps = append(apps, kintone.App{AppID: strconv.Itoa(i)})
					}
				} else {
					if r.URL.Query().Get("offset") != "100" {
						t.Error("second page offset missing")
					}
					apps = append(apps, kintone.App{AppID: "101"})
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"apps": apps})
			}, nil)
			if calls != 2 {
				t.Fatalf("got %d requests", calls)
			}
			if fail {
				if !resp.Diagnostics.HasError() {
					t.Fatal("failed page must not return partial success")
				}
				return
			}
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			var apps types.List
			d := resp.State.GetAttribute(context.Background(), path.Root("apps"), &apps)
			if d.HasError() || len(apps.Elements()) != 101 {
				t.Fatalf("unexpected apps: %v %v", apps, d)
			}
		})
	}
}

func TestAppDataSourceLiveSettings(t *testing.T) {
	paths := []string{}
	resp := readDataSource(t, NewAppDataSource(), func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/k/v1/app.json":
			_ = json.NewEncoder(w).Encode(kintone.App{AppID: "12", Name: "metadata", Creator: kintone.User{Code: "user", Name: "User"}})
		case "/k/v1/app/settings.json":
			_ = json.NewEncoder(w).Encode(kintone.Settings{Name: "live", Description: "", Theme: "WHITE", Revision: "3", FirstMonthOfFiscalYear: "4", TitleField: kintone.TitleField{SelectionMode: "AUTO", Code: "record_number"}, NumberPrecision: kintone.NumberPrecision{Digits: "16", DecimalPlaces: "0", RoundingMode: "HALF_EVEN"}, EnableComments: false})
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
		}
	}, map[string]attr.Value{"id": types.StringValue("12")})
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(paths) != 2 {
		t.Fatal(paths)
	}
	for _, check := range []struct {
		p    path.Path
		want attr.Value
	}{
		{path.Root("name"), types.StringValue("live")},
		{path.Root("space_id"), types.StringNull()},
		{path.Root("title_field").AtName("field_code"), types.StringValue("record_number")},
		{path.Root("number_precision").AtName("decimal_places"), types.Int64Value(0)},
		{path.Root("first_month_of_fiscal_year"), types.Int64Value(4)},
		{path.Root("comments_enabled"), types.BoolValue(false)},
	} {
		var got attr.Value
		diags := resp.State.GetAttribute(context.Background(), check.p, &got)
		if diags.HasError() || !check.want.Equal(got) {
			t.Errorf("%s: got %v, want %v; %v", check.p, got, check.want, diags)
		}
	}
}
