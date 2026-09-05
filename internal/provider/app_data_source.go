// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/rychhr/terraform-provider-kintone/internal/kintone"
)

var _ datasource.DataSourceWithConfigure = (*appDataSource)(nil)

type appDataSource struct{ client *kintone.Client }

// NewAppDataSource returns the live app metadata and settings data source.
func NewAppDataSource() datasource.DataSource { return &appDataSource{} }

func (d *appDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}

func (d *appDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*kintone.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider configuration", "Expected a kintone client.")
		return
	}
	d.client = client
}

func dataSourceMetadataSchema() map[string]schema.Attribute {
	attrs := map[string]schema.Attribute{}
	for _, name := range []string{"id", "code", "name", "description", "space_id", "thread_id", "created_at", "modified_at"} {
		attrs[name] = schema.StringAttribute{Computed: true, Description: "Published app " + name + "."}
	}
	for _, name := range []string{"creator", "modifier"} {
		attrs[name] = schema.SingleNestedAttribute{Computed: true, Description: "App " + name + ".", Attributes: map[string]schema.Attribute{"code": schema.StringAttribute{Computed: true, Description: "User code."}, "name": schema.StringAttribute{Computed: true, Description: "User display name."}}}
	}
	return attrs
}

func (d *appDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := dataSourceMetadataSchema()
	attrs["id"] = schema.StringAttribute{Required: true, Description: "App ID as a positive decimal string."}
	for _, name := range []string{"theme", "revision"} {
		attrs[name] = schema.StringAttribute{Computed: true, Description: "Live settings " + name + "."}
	}
	for _, name := range []string{"thumbnails_enabled", "bulk_deletion_enabled", "comments_enabled", "record_duplication_enabled", "inline_record_editing_enabled"} {
		attrs[name] = schema.BoolAttribute{Computed: true, Description: "Whether the app feature is enabled."}
	}
	attrs["first_month_of_fiscal_year"] = schema.Int64Attribute{Computed: true, Description: "First fiscal month, from 1 to 12."}
	attrs["title_field"] = schema.SingleNestedAttribute{Computed: true, Description: "Observed record title selection, including the automatically selected field code in AUTO mode.", Attributes: map[string]schema.Attribute{"selection_mode": schema.StringAttribute{Computed: true, Description: "AUTO or MANUAL."}, "field_code": schema.StringAttribute{Computed: true, Description: "Selected field code."}}}
	attrs["number_precision"] = schema.SingleNestedAttribute{Computed: true, Description: "Number calculation precision.", Attributes: map[string]schema.Attribute{"total_digits": schema.Int64Attribute{Computed: true, Description: "Total significant digits."}, "decimal_places": schema.Int64Attribute{Computed: true, Description: "Number of decimal places."}, "rounding_mode": schema.StringAttribute{Computed: true, Description: "Number rounding mode."}}}
	resp.Schema = schema.Schema{Description: "Reads live app metadata and general settings. A missing app returns an error.", Attributes: attrs}
}

func (d *appDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if id.IsNull() || id.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("id"), "Unknown app ID", "Supply a known app ID before reading the app.")
		return
	}
	if d.client == nil {
		resp.Diagnostics.AddError("Unconfigured client", "Configure the provider before reading apps.")
		return
	}
	app, err := d.client.GetApp(ctx, id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read app", err.Error())
		return
	}
	settings, err := d.client.GetLiveSettings(ctx, id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read app settings", err.Error())
		return
	}
	values := dataSourceMetadataValues(app)
	// Preserve the configured identity, including an accepted leading-zero form.
	values["id"] = id
	values["name"] = types.StringValue(settings.Name)
	values["description"] = types.StringValue(settings.Description)
	values["theme"] = types.StringValue(settings.Theme)
	values["revision"] = types.StringValue(settings.Revision)
	values["title_field"] = dataSourceObject(map[string]attr.Value{"selection_mode": types.StringValue(settings.TitleField.SelectionMode), "field_code": types.StringValue(settings.TitleField.Code)})
	numbers := map[string]string{"first_month_of_fiscal_year": settings.FirstMonthOfFiscalYear, "total_digits": settings.NumberPrecision.Digits, "decimal_places": settings.NumberPrecision.DecimalPlaces}
	parsed := map[string]attr.Value{}
	for name, s := range numbers {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			resp.Diagnostics.AddError("Invalid app settings response", fmt.Sprintf("The API returned a non-integer %s.", name))
			return
		}
		parsed[name] = types.Int64Value(n)
	}
	values["first_month_of_fiscal_year"] = parsed["first_month_of_fiscal_year"]
	values["number_precision"] = dataSourceObject(map[string]attr.Value{"total_digits": parsed["total_digits"], "decimal_places": parsed["decimal_places"], "rounding_mode": types.StringValue(settings.NumberPrecision.RoundingMode)})
	for name, value := range map[string]bool{"thumbnails_enabled": settings.EnableThumbnails, "bulk_deletion_enabled": settings.EnableBulkDeletion, "comments_enabled": settings.EnableComments, "record_duplication_enabled": settings.EnableDuplicateRecord, "inline_record_editing_enabled": settings.EnableInlineRecordEditing} {
		values[name] = types.BoolValue(value)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, dataSourceObject(values))...)
}

func dataSourceObject(values map[string]attr.Value) types.Object {
	attrs := make(map[string]attr.Type, len(values))
	for name, value := range values {
		attrs[name] = value.Type(context.Background())
	}
	return types.ObjectValueMust(attrs, values)
}

func dataSourceMetadataValues(app kintone.App) map[string]attr.Value {
	values := map[string]attr.Value{}
	for name, value := range map[string]string{"id": app.AppID, "code": app.Code, "name": app.Name, "description": app.Description, "created_at": app.CreatedAt, "modified_at": app.ModifiedAt} {
		values[name] = types.StringValue(value)
	}
	values["space_id"] = types.StringPointerValue(app.SpaceID)
	values["thread_id"] = types.StringPointerValue(app.ThreadID)
	for name, user := range map[string]kintone.User{"creator": app.Creator, "modifier": app.Modifier} {
		values[name] = dataSourceObject(map[string]attr.Value{"code": types.StringValue(user.Code), "name": types.StringValue(user.Name)})
	}
	return values
}
