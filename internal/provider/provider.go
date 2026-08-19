// SPDX-License-Identifier: MPL-2.0

// Package provider holds the terraform-plugin-framework glue: the provider
// definition together with its resources and data sources.
//
// Logic that talks to the kintone REST API — request construction, call
// ordering, retries — belongs in internal/kintone, not here.
package provider

import (
	"context"

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

// Schema declares the provider configuration block. It stays empty until
// provider authentication lands.
func (p *kintoneProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages kintone apps and their settings.",
	}
}

// Configure builds the API client shared by resources and data sources. It is
// a no-op until provider authentication lands.
func (p *kintoneProvider) Configure(_ context.Context, _ provider.ConfigureRequest, _ *provider.ConfigureResponse) {
}

// Resources returns the provider's resources. None are implemented yet.
func (p *kintoneProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}

// DataSources returns the provider's data sources. None are implemented yet.
func (p *kintoneProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}
