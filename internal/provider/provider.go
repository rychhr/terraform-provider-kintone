// SPDX-License-Identifier: MPL-2.0

// Package provider holds the terraform-plugin-framework glue: the provider
// definition together with its resources and data sources.
//
// Logic that talks to the kintone REST API — request construction, call
// ordering, retries — belongs in internal/kintone, not here.
package provider

import (
	"context"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/rychhr/terraform-provider-kintone/internal/kintone"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ provider.Provider = (*kintoneProvider)(nil)

// kintoneProvider implements the kintone Terraform provider.
type kintoneProvider struct {
	// version is injected at build time and reported back to Terraform.
	version string
}

// New returns a factory for the kintone provider, as expected by
// providerserver.Serve and by the acceptance-test harness.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &kintoneProvider{version: version}
	}
}

// Metadata sets the provider type name. This — not the Registry address — is
// what gives resource and data source type names their kintone_ prefix.
func (p *kintoneProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "kintone"
	resp.Version = p.version
}

// Schema declares provider credentials and the service origin.
func (p *kintoneProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages kintone apps and their settings. Password authentication takes precedence when both authentication modes are configured.",
		Attributes: map[string]schema.Attribute{
			"base_url":   schema.StringAttribute{Optional: true, Description: "kintone HTTP(S) origin. Falls back to KINTONE_BASE_URL when omitted."},
			"username":   schema.StringAttribute{Optional: true, Description: "Service-account login name. Falls back to KINTONE_USERNAME when omitted."},
			"password":   schema.StringAttribute{Optional: true, Sensitive: true, Description: "Service-account password. Falls back to KINTONE_PASSWORD when omitted."},
			"api_tokens": schema.ListAttribute{Optional: true, Sensitive: true, ElementType: types.StringType, Description: "Up to nine API tokens. Falls back to comma-separated KINTONE_API_TOKENS when omitted. Tokens cannot create or list apps."},
		},
	}
}

// Configure builds the API client shared by resources and data sources.
func (p *kintoneProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data providerConfig
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cfg, diags := clientConfiguration(data, os.Getenv)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	client, err := kintone.NewClient(cfg)
	if err != nil {
		resp.Diagnostics.AddError("Invalid provider configuration", err.Error())
		return
	}
	resp.ResourceData = client
	resp.DataSourceData = client
}

type providerConfig struct {
	BaseURL   types.String `tfsdk:"base_url"`
	Username  types.String `tfsdk:"username"`
	Password  types.String `tfsdk:"password"`
	APITokens types.List   `tfsdk:"api_tokens"`
}

func clientConfiguration(data providerConfig, getenv func(string) string) (kintone.Config, diag.Diagnostics) {
	var cfg kintone.Config
	var diags diag.Diagnostics
	resolve := func(name, env string, value types.String) string {
		if value.IsUnknown() {
			diags.AddAttributeError(path.Root(name), "Unknown provider configuration", "This value must be known before configuring the provider.")
			return ""
		}
		if value.IsNull() {
			return getenv(env)
		}
		if value.ValueString() == "" {
			diags.AddAttributeError(path.Root(name), "Empty provider configuration", "An explicitly configured value must not be empty.")
		}
		return value.ValueString()
	}
	cfg.BaseURL = resolve("base_url", "KINTONE_BASE_URL", data.BaseURL)
	cfg.Username = resolve("username", "KINTONE_USERNAME", data.Username)
	cfg.Password = resolve("password", "KINTONE_PASSWORD", data.Password)
	switch {
	case data.APITokens.IsUnknown():
		diags.AddAttributeError(path.Root("api_tokens"), "Unknown API tokens", "API tokens must be known before configuring the provider.")
	case data.APITokens.IsNull():
		if value := getenv("KINTONE_API_TOKENS"); value != "" {
			cfg.APITokens = strings.Split(value, ",")
			for i := range cfg.APITokens {
				cfg.APITokens[i] = strings.TrimSpace(cfg.APITokens[i])
			}
		}
	default:
		for _, value := range data.APITokens.Elements() {
			token, ok := value.(types.String)
			if !ok || token.IsNull() || token.IsUnknown() {
				diags.AddAttributeError(path.Root("api_tokens"), "Invalid API token", "Every token must be a known, non-null string.")
				continue
			}
			cfg.APITokens = append(cfg.APITokens, token.ValueString())
		}
	}
	return cfg, diags
}

// Resources returns the managed app resource.
func (p *kintoneProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{NewAppResource}
}

// DataSources returns live app lookups.
func (p *kintoneProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{NewAppDataSource, NewAppsDataSource}
}
