// SPDX-License-Identifier: MPL-2.0
package provider

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/rychhr/terraform-provider-kintone/internal/kintone"
)

var titleTypes = map[string]attr.Type{"selection_mode": types.StringType, "field_code": types.StringType}
var precisionTypes = map[string]attr.Type{"total_digits": types.Int64Type, "decimal_places": types.Int64Type, "rounding_mode": types.StringType}

type appModel struct {
	ID                       types.String `tfsdk:"id"`
	Name                     types.String `tfsdk:"name"`
	Description              types.String `tfsdk:"description"`
	SpaceID                  types.String `tfsdk:"space_id"`
	ThreadID                 types.String `tfsdk:"thread_id"`
	Revision                 types.String `tfsdk:"revision"`
	Theme                    types.String `tfsdk:"theme"`
	TitleField               types.Object `tfsdk:"title_field"`
	NumberPrecision          types.Object `tfsdk:"number_precision"`
	FirstMonth               types.Int64  `tfsdk:"first_month_of_fiscal_year"`
	ThumbnailsEnabled        types.Bool   `tfsdk:"thumbnails_enabled"`
	BulkDeletionEnabled      types.Bool   `tfsdk:"bulk_deletion_enabled"`
	CommentsEnabled          types.Bool   `tfsdk:"comments_enabled"`
	RecordDuplicationEnabled types.Bool   `tfsdk:"record_duplication_enabled"`
	InlineEditingEnabled     types.Bool   `tfsdk:"inline_record_editing_enabled"`
}

func objectString(object types.Object, key string) types.String {
	if object.IsUnknown() {
		return types.StringUnknown()
	}
	if object.IsNull() {
		return types.StringNull()
	}
	value, ok := object.Attributes()[key].(types.String)
	if !ok {
		return types.StringNull()
	}
	return value
}
func objectInt(object types.Object, key string) types.Int64 {
	if object.IsUnknown() {
		return types.Int64Unknown()
	}
	if object.IsNull() {
		return types.Int64Null()
	}
	value, ok := object.Attributes()[key].(types.Int64)
	if !ok {
		return types.Int64Null()
	}
	return value
}
func stringPatch(config, plan, prior types.String) *string {
	if config.IsNull() || plan.IsUnknown() || plan.IsNull() || plan.Equal(prior) {
		return nil
	}
	value := plan.ValueString()
	return &value
}
func boolPatch(config, plan, prior types.Bool) *bool {
	if config.IsNull() || plan.IsUnknown() || plan.IsNull() || plan.Equal(prior) {
		return nil
	}
	value := plan.ValueBool()
	return &value
}
func intPatch(config, plan, prior types.Int64) *string {
	if config.IsNull() || plan.IsUnknown() || plan.IsNull() || plan.Equal(prior) {
		return nil
	}
	value := strconv.FormatInt(plan.ValueInt64(), 10)
	return &value
}

// appSettingsPatch sends only managed, changed, known values. The client completes
// nested objects from preview, preserving omitted children through the deployment.
func appSettingsPatch(config, plan, prior appModel) kintone.SettingsUpdate {
	update := kintone.SettingsUpdate{
		Name:                      stringPatch(config.Name, plan.Name, prior.Name),
		Description:               stringPatch(config.Description, plan.Description, prior.Description),
		Theme:                     stringPatch(config.Theme, plan.Theme, prior.Theme),
		FirstMonthOfFiscalYear:    intPatch(config.FirstMonth, plan.FirstMonth, prior.FirstMonth),
		EnableThumbnails:          boolPatch(config.ThumbnailsEnabled, plan.ThumbnailsEnabled, prior.ThumbnailsEnabled),
		EnableBulkDeletion:        boolPatch(config.BulkDeletionEnabled, plan.BulkDeletionEnabled, prior.BulkDeletionEnabled),
		EnableComments:            boolPatch(config.CommentsEnabled, plan.CommentsEnabled, prior.CommentsEnabled),
		EnableDuplicateRecord:     boolPatch(config.RecordDuplicationEnabled, plan.RecordDuplicationEnabled, prior.RecordDuplicationEnabled),
		EnableInlineRecordEditing: boolPatch(config.InlineEditingEnabled, plan.InlineEditingEnabled, prior.InlineEditingEnabled),
	}
	title := kintone.TitleFieldUpdate{
		SelectionMode: stringPatch(objectString(config.TitleField, "selection_mode"), objectString(plan.TitleField, "selection_mode"), objectString(prior.TitleField, "selection_mode")),
		Code:          stringPatch(objectString(config.TitleField, "field_code"), objectString(plan.TitleField, "field_code"), objectString(prior.TitleField, "field_code")),
	}
	if title.SelectionMode != nil || title.Code != nil {
		update.TitleField = &title
	}
	precision := kintone.NumberPrecisionUpdate{
		Digits:        intPatch(objectInt(config.NumberPrecision, "total_digits"), objectInt(plan.NumberPrecision, "total_digits"), objectInt(prior.NumberPrecision, "total_digits")),
		DecimalPlaces: intPatch(objectInt(config.NumberPrecision, "decimal_places"), objectInt(plan.NumberPrecision, "decimal_places"), objectInt(prior.NumberPrecision, "decimal_places")),
		RoundingMode:  stringPatch(objectString(config.NumberPrecision, "rounding_mode"), objectString(plan.NumberPrecision, "rounding_mode"), objectString(prior.NumberPrecision, "rounding_mode")),
	}
	if precision.Digits != nil || precision.DecimalPlaces != nil || precision.RoundingMode != nil {
		update.NumberPrecision = &precision
	}
	return update
}

func (m *appModel) setSettings(s kintone.Settings) error {
	numbers := []struct {
		name, value string
		min, max    int64
	}{{"total_digits", s.NumberPrecision.Digits, 1, 30}, {"decimal_places", s.NumberPrecision.DecimalPlaces, 0, 10}, {"first_month_of_fiscal_year", s.FirstMonthOfFiscalYear, 1, 12}}
	parsed := make([]int64, len(numbers))
	for i, n := range numbers {
		v, err := strconv.ParseInt(n.value, 10, 64)
		if err != nil || v < n.min || v > n.max {
			return fmt.Errorf("invalid %s in API response", n.name)
		}
		parsed[i] = v
	}
	m.Name = types.StringValue(s.Name)
	m.Description = types.StringValue(s.Description)
	m.Theme = types.StringValue(s.Theme)
	m.Revision = types.StringValue(s.Revision)
	m.TitleField = types.ObjectValueMust(titleTypes, map[string]attr.Value{"selection_mode": types.StringValue(s.TitleField.SelectionMode), "field_code": types.StringValue(s.TitleField.Code)})
	m.NumberPrecision = types.ObjectValueMust(precisionTypes, map[string]attr.Value{"total_digits": types.Int64Value(parsed[0]), "decimal_places": types.Int64Value(parsed[1]), "rounding_mode": types.StringValue(s.NumberPrecision.RoundingMode)})
	m.FirstMonth = types.Int64Value(parsed[2])
	m.ThumbnailsEnabled = types.BoolValue(s.EnableThumbnails)
	m.BulkDeletionEnabled = types.BoolValue(s.EnableBulkDeletion)
	m.CommentsEnabled = types.BoolValue(s.EnableComments)
	m.RecordDuplicationEnabled = types.BoolValue(s.EnableDuplicateRecord)
	m.InlineEditingEnabled = types.BoolValue(s.EnableInlineRecordEditing)
	return nil
}
func (m *appModel) setPlacement(app kintone.App) {
	m.SpaceID = types.StringPointerValue(app.SpaceID)
	m.ThreadID = types.StringPointerValue(app.ThreadID)
}
