// SPDX-License-Identifier: MPL-2.0
package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestAppSettingsPatchPreservesOmittedChildren(t *testing.T) {
	prior := appModel{Description: types.StringValue("existing"), CommentsEnabled: types.BoolValue(true)}
	config := appModel{CommentsEnabled: types.BoolValue(false), NumberPrecision: types.ObjectValueMust(precisionTypes, map[string]attr.Value{"total_digits": types.Int64Null(), "decimal_places": types.Int64Value(2), "rounding_mode": types.StringNull()})}
	plan := config
	update := appSettingsPatch(config, plan, prior)
	if update.Description != nil || update.EnableComments == nil || *update.EnableComments {
		t.Fatal("omission or explicit false lost")
	}
	if update.NumberPrecision == nil || update.NumberPrecision.Digits != nil || update.NumberPrecision.RoundingMode != nil || *update.NumberPrecision.DecimalPlaces != "2" {
		t.Fatal("nested siblings were overwritten")
	}
	config.Description = types.StringValue("")
	plan.Description = config.Description
	if u := appSettingsPatch(config, plan, prior); u.Description == nil || *u.Description != "" {
		t.Fatal("explicit empty description lost")
	}
	plan.NumberPrecision = types.ObjectUnknown(precisionTypes)
	if u := appSettingsPatch(config, plan, prior); u.NumberPrecision != nil {
		t.Fatal("unknown plan became mutation")
	}
}

func TestAppSettingsPatchUnknownChild(t *testing.T) {
	config := appModel{NumberPrecision: types.ObjectValueMust(precisionTypes, map[string]attr.Value{"total_digits": types.Int64Null(), "decimal_places": types.Int64Value(2), "rounding_mode": types.StringNull()})}
	plan := appModel{NumberPrecision: types.ObjectValueMust(precisionTypes, map[string]attr.Value{"total_digits": types.Int64Unknown(), "decimal_places": types.Int64Value(2), "rounding_mode": types.StringUnknown()})}
	prior := appModel{NumberPrecision: types.ObjectValueMust(precisionTypes, map[string]attr.Value{"total_digits": types.Int64Value(16), "decimal_places": types.Int64Value(4), "rounding_mode": types.StringValue("HALF_EVEN")})}
	update := appSettingsPatch(config, plan, prior)
	if update.NumberPrecision == nil || update.NumberPrecision.Digits != nil || update.NumberPrecision.RoundingMode != nil || update.NumberPrecision.DecimalPlaces == nil || *update.NumberPrecision.DecimalPlaces != "2" {
		t.Fatal("unknown siblings became writes or masked the known update")
	}
}
