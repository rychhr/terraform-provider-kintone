// SPDX-License-Identifier: MPL-2.0
package provider

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"unicode/utf8"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/rychhr/terraform-provider-kintone/internal/kintone"
)

var _ resource.ResourceWithConfigure = (*appResource)(nil)
var _ resource.ResourceWithImportState = (*appResource)(nil)
var _ resource.ResourceWithModifyPlan = (*appResource)(nil)
var _ resource.ResourceWithValidateConfig = (*appResource)(nil)

type appResource struct{ client *kintone.Client }

func NewAppResource() resource.Resource { return &appResource{} }
func (r *appResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}
func (r *appResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	optionalString := func(description string) schema.StringAttribute {
		return schema.StringAttribute{Optional: true, Computed: true, Description: description}
	}
	placement := func(description string) schema.StringAttribute {
		v := optionalString(description)
		v.PlanModifiers = []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
		return v
	}
	boolean := func(description string) schema.BoolAttribute {
		return schema.BoolAttribute{Optional: true, Computed: true, Description: description}
	}
	resp.Schema = schema.Schema{
		Description: "Manages an app and its general settings. Omitted settings retain their remote values. Do not leave unrelated preview changes on this app: deployment includes those changes. Destroy only removes Terraform state; delete the app manually. Creation failures require manual inspection before retrying.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, Description: "App ID. Also used for import.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":        schema.StringAttribute{Required: true, Description: "App name (1 to 64 characters)."},
			"description": optionalString("App description, up to 10000 characters. An explicit empty string clears it."),
			"space_id":    placement("Normal-space ID, configured only at creation. Omission preserves placement, including after import. Changes are rejected."),
			"thread_id":   placement("Thread ID within the space, configured only at creation. Requires space_id. Changes are rejected."),
			"revision":    schema.StringAttribute{Computed: true, Description: "Live settings revision as a string."},
			"theme":       optionalString("App theme: WHITE, RED, BLUE, GREEN, YELLOW, or BLACK."),
			"title_field": schema.SingleNestedAttribute{Optional: true, Computed: true, Description: "Record title settings. In AUTO mode, omit field_code and observe the selected code in state.", Attributes: map[string]schema.Attribute{
				"selection_mode": optionalString("AUTO or MANUAL. Omission retains the current selection mode."),
				"field_code":     optionalString("Nonempty field code for MANUAL mode. Must be omitted in AUTO mode; state contains the automatically selected code."),
			}},
			"number_precision": schema.SingleNestedAttribute{Optional: true, Computed: true, Description: "Number calculation and display precision. Omitted children are preserved.", Attributes: map[string]schema.Attribute{
				"total_digits":   schema.Int64Attribute{Optional: true, Computed: true, Description: "Total digits, from 1 to 30."},
				"decimal_places": schema.Int64Attribute{Optional: true, Computed: true, Description: "Decimal places, from 0 to 10."},
				"rounding_mode":  optionalString("HALF_EVEN, UP, or DOWN."),
			}},
			"first_month_of_fiscal_year":    schema.Int64Attribute{Optional: true, Computed: true, Description: "First fiscal month, from 1 to 12."},
			"thumbnails_enabled":            boolean("Whether record attachment thumbnails are shown."),
			"bulk_deletion_enabled":         boolean("Whether bulk record deletion is enabled."),
			"comments_enabled":              boolean("Whether record comments are enabled."),
			"record_duplication_enabled":    boolean("Whether record duplication is enabled."),
			"inline_record_editing_enabled": boolean("Whether inline record editing is enabled."),
		},
	}
}
func (r *appResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*kintone.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider client", "Expected a kintone API client.")
		return
	}
	r.client = client
}

var appIDPattern = regexp.MustCompile(`^[1-9][0-9]*$`)

func validateAppModel(m appModel) diag.Diagnostics {
	var diags diag.Diagnostics
	check := func(p path.Path, valid bool, detail string) {
		if !valid {
			diags.AddAttributeError(p, "Invalid app setting", detail)
		}
	}
	checkString := func(name string, v types.String, min, max int) {
		if !v.IsNull() && !v.IsUnknown() {
			n := utf8.RuneCountInString(v.ValueString())
			check(path.Root(name), n >= min && n <= max, fmt.Sprintf("Must contain %d to %d characters.", min, max))
		}
	}
	checkString("name", m.Name, 1, 64)
	checkString("description", m.Description, 0, 10000)
	for name, v := range map[string]types.String{"space_id": m.SpaceID, "thread_id": m.ThreadID} {
		if !v.IsNull() && !v.IsUnknown() {
			n, err := strconv.ParseInt(v.ValueString(), 10, 64)
			check(path.Root(name), err == nil && n > 0 && appIDPattern.MatchString(v.ValueString()), "Must be a positive decimal ID without leading zeros.")
		}
	}
	choice := func(p path.Path, v types.String, allowed ...string) {
		if v.IsNull() || v.IsUnknown() {
			return
		}
		for _, s := range allowed {
			if s == v.ValueString() {
				return
			}
		}
		check(p, false, "Unsupported value.")
	}
	choice(path.Root("theme"), m.Theme, "WHITE", "RED", "BLUE", "GREEN", "YELLOW", "BLACK")
	choice(path.Root("title_field").AtName("selection_mode"), objectString(m.TitleField, "selection_mode"), "AUTO", "MANUAL")
	choice(path.Root("number_precision").AtName("rounding_mode"), objectString(m.NumberPrecision, "rounding_mode"), "HALF_EVEN", "UP", "DOWN")
	number := func(p path.Path, v types.Int64, min, max int64) {
		if !v.IsNull() && !v.IsUnknown() {
			check(p, v.ValueInt64() >= min && v.ValueInt64() <= max, fmt.Sprintf("Must be an integer from %d to %d.", min, max))
		}
	}
	number(path.Root("first_month_of_fiscal_year"), m.FirstMonth, 1, 12)
	number(path.Root("number_precision").AtName("total_digits"), objectInt(m.NumberPrecision, "total_digits"), 1, 30)
	number(path.Root("number_precision").AtName("decimal_places"), objectInt(m.NumberPrecision, "decimal_places"), 0, 10)
	code := objectString(m.TitleField, "field_code")
	mode := objectString(m.TitleField, "selection_mode")
	if !code.IsNull() && !code.IsUnknown() {
		check(path.Root("title_field").AtName("field_code"), code.ValueString() != "", "Field code must not be empty.")
		if mode.ValueString() == "AUTO" {
			check(path.Root("title_field").AtName("field_code"), false, "Omit field_code in AUTO mode; the API selects it.")
		}
	}
	return diags
}
func (r *appResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data appModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validateAppModel(data)...)
}
func (r *appResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var config, plan, prior appModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if !req.State.Raw.IsNull() {
		resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validateAppModel(config)...)
	if !req.State.Raw.IsNull() {
		for name, pair := range map[string][2]types.String{"space_id": {config.SpaceID, prior.SpaceID}, "thread_id": {config.ThreadID, prior.ThreadID}} {
			if !pair[0].IsNull() && !pair[0].IsUnknown() && !pair[0].Equal(pair[1]) {
				resp.Diagnostics.AddAttributeError(path.Root(name), "Placement cannot be changed", "Placement is create-only. This provider does not force replacement because apps cannot be physically deleted.")
			}
		}
	} else if !config.ThreadID.IsNull() && config.SpaceID.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("thread_id"), "Missing space_id", "thread_id requires space_id at creation.")
	}
	mode := objectString(config.TitleField, "selection_mode")
	if mode.IsNull() {
		mode = objectString(prior.TitleField, "selection_mode")
		if mode.IsNull() {
			mode = types.StringValue("AUTO")
		}
	}
	code := objectString(config.TitleField, "field_code")
	// An unknown expression can resolve to null. Defer this constraint until
	// Terraform supplies a known code instead of rejecting a valid omission.
	if mode.ValueString() == "AUTO" && !code.IsNull() && !code.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("title_field").AtName("field_code"), "Automatic title selection", "Omit field_code in AUTO mode; observe the selected value in state.")
	}
	if mode.ValueString() == "MANUAL" && code.IsNull() {
		oldCode := objectString(prior.TitleField, "field_code")
		if oldCode.IsNull() || oldCode.IsUnknown() || oldCode.ValueString() == "" {
			resp.Diagnostics.AddAttributeError(path.Root("title_field").AtName("field_code"), "Missing manual title field", "MANUAL mode requires a nonempty field_code or an existing observed title field.")
		}
	}
	// A mode change can change AUTO's returned code, even if the API accepts the
	// previous manual code in the request. Never freeze that computed child.
	if code.IsNull() && !plan.TitleField.IsNull() && !plan.TitleField.IsUnknown() && !objectString(prior.TitleField, "selection_mode").Equal(mode) {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("title_field").AtName("field_code"), types.StringUnknown())...)
	}
}

func (r *appResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var config, plan appModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Missing provider client", "Configure the provider before creating an app.")
		return
	}
	update := appSettingsPatch(config, plan, appModel{})
	update.Name = nil
	result, err := r.client.CreateApp(ctx, kintone.CreateAppOptions{Name: plan.Name.ValueString(), SpaceID: config.SpaceID.ValueStringPointer(), ThreadID: config.ThreadID.ValueStringPointer(), Settings: update})
	if result.AppID != "" {
		plan.ID = types.StringValue(result.AppID)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		// Failed creates still need a valid, known partial state for manual recovery.
		raw, transformErr := tftypes.Transform(resp.State.Raw, func(_ *tftypes.AttributePath, v tftypes.Value) (tftypes.Value, error) {
			if !v.IsKnown() {
				return tftypes.NewValue(v.Type(), nil), nil
			}
			return v, nil
		})
		if transformErr != nil {
			resp.Diagnostics.AddError("Unable to preserve app state", "Failed to normalize partial state.")
		} else {
			resp.State.Raw = raw
		}
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to create app", fmt.Sprintf("%v. If an app ID was assigned, inspect that app before retrying. Terraform may mark this resource tainted and plan a new app; creation does not automatically resume. Manual cleanup may be required.", err))
		return
	}
	if err = plan.setSettings(result.Settings); err != nil {
		resp.Diagnostics.AddError("Invalid live settings", fmt.Sprintf("App %s: live read-back decoding failed: %v. Inspect the app before retrying; Terraform may plan recreation.", result.AppID, err))
		return
	}
	app, err := r.client.GetApp(ctx, result.AppID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read created app", fmt.Sprintf("App %s exists. %v. Inspect it before retrying; Terraform may plan recreation.", result.AppID, err))
		return
	}
	plan.setPlacement(app)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *appResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data appModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Missing provider client", "Configure the provider before reading an app.")
		return
	}
	app, err := r.client.GetApp(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read app", fmt.Sprintf("%v. State is retained. Verify access and whether the app is deployed or was manually deleted before changing state.", err))
		return
	}
	settings, err := r.client.GetLiveSettings(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read app settings", err.Error())
		return
	}
	if err = data.setSettings(settings); err != nil {
		resp.Diagnostics.AddError("Invalid live settings", err.Error())
		return
	}
	data.setPlacement(app)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *appResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var config, plan, prior appModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Missing provider client", "Configure the provider before updating an app.")
		return
	}
	result, err := r.client.UpdateApp(ctx, prior.ID.ValueString(), appSettingsPatch(config, plan, prior))
	if err != nil {
		resp.Diagnostics.AddError("Unable to update app", fmt.Sprintf("%v. State is retained. Inspect preview and live settings before retrying; the provider does not revert changes.", err))
		return
	}
	if err = plan.setSettings(result.Settings); err != nil {
		resp.Diagnostics.AddError("Invalid live settings", err.Error())
		return
	}
	plan.ID = prior.ID
	plan.SpaceID = prior.SpaceID
	plan.ThreadID = prior.ThreadID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *appResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data appModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.AddWarning("Manual app cleanup required", fmt.Sprintf("App %s (%s) still exists in kintone. Terraform removed only its state. Delete the app manually if it is no longer needed.", data.ID.ValueString(), data.Name.ValueString()))
	resp.State.RemoveResource(ctx)
}
func (r *appResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil || id <= 0 || !appIDPattern.MatchString(req.ID) {
		resp.Diagnostics.AddError("Invalid app import ID", "Use a positive decimal app ID without leading zeros.")
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
