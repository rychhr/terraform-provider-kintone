// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"sort"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/rychhr/terraform-provider-kintone/internal/kintone"
)

var _ datasource.DataSourceWithConfigure = (*appsDataSource)(nil)

type appsDataSource struct{ appDataSource }

// NewAppsDataSource returns the paginated app metadata data source.
func NewAppsDataSource() datasource.DataSource { return &appsDataSource{} }

func (d *appsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_apps"
}

func (d *appsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{"name": schema.StringAttribute{Optional: true, Description: "App name search filter, at most 64 characters."}, "apps": schema.ListNestedAttribute{Computed: true, Description: "All matching app metadata, ordered by numeric app ID. An empty result is an empty list.", NestedObject: schema.NestedAttributeObject{Attributes: dataSourceMetadataSchema()}}}
	for _, name := range []string{"ids", "codes", "space_ids"} {
		attrs[name] = schema.SetAttribute{Optional: true, ElementType: types.StringType, Description: "App listing filter, up to 100 values. IDs are positive decimal strings."}
	}
	resp.Schema = schema.Schema{Description: "Lists all matching published apps. Requires password authentication; general settings are not fetched.", Attributes: attrs}
}

func (d *appsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	options := kintone.ListAppsOptions{}
	for name, target := range map[string]*[]string{"ids": &options.IDs, "codes": &options.Codes, "space_ids": &options.SpaceIDs} {
		var values types.Set
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(name), &values)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if values.IsUnknown() {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Unknown app filter", "Supply known filter values before listing apps.")
			return
		}
		if !values.IsNull() {
			resp.Diagnostics.Append(values.ElementsAs(ctx, target, false)...)
		}
	}
	var name types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("name"), &name)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if name.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("name"), "Unknown app filter", "Supply a known name before listing apps.")
		return
	}
	options.Name = name.ValueString()
	if d.client == nil {
		resp.Diagnostics.AddError("Unconfigured client", "Configure the provider before listing apps.")
		return
	}
	apps, err := d.client.ListApps(ctx, options)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list apps", err.Error())
		return
	}
	sort.SliceStable(apps, func(i, j int) bool {
		a, _ := strconv.ParseInt(apps[i].AppID, 10, 64)
		b, _ := strconv.ParseInt(apps[j].AppID, 10, 64)
		return a < b
	})
	values := make([]attr.Value, 0, len(apps))
	for _, app := range apps {
		values = append(values, dataSourceObject(dataSourceMetadataValues(app)))
	}
	elem := dataSourceObject(dataSourceMetadataValues(kintone.App{})).Type(ctx)
	result, diags := types.ListValue(elem, values)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.State.Raw = req.Config.Raw
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("apps"), result)...)
}
