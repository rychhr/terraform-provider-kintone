// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestProviderConfiguration(t *testing.T) {
	env := map[string]string{"KINTONE_BASE_URL": "https://example.invalid", "KINTONE_USERNAME": "service", "KINTONE_PASSWORD": "fixture", "KINTONE_API_TOKENS": "one, two"}
	lookup := func(k string) string { return env[k] }
	cfg, diags := clientConfiguration(providerConfig{}, lookup)
	if diags.HasError() || cfg.Username != "service" || len(cfg.APITokens) != 2 || cfg.APITokens[1] != "two" {
		t.Fatalf("unexpected configuration: %v", diags)
	}
	cfg, diags = clientConfiguration(providerConfig{Username: types.StringValue("override"), APITokens: types.ListValueMust(types.StringType, []attr.Value{})}, lookup)
	if diags.HasError() || cfg.Username != "override" || len(cfg.APITokens) != 0 {
		t.Fatalf("HCL did not override environment: %v", diags)
	}
	for _, data := range []providerConfig{
		{BaseURL: types.StringUnknown()}, {Password: types.StringValue("")},
		{APITokens: types.ListUnknown(types.StringType)},
		{APITokens: types.ListValueMust(types.StringType, []attr.Value{types.StringUnknown()})},
	} {
		_, diags = clientConfiguration(data, lookup)
		if !diags.HasError() {
			t.Fatal("expected invalid or unknown configuration diagnostic")
		}
		for _, d := range diags {
			if strings.Contains(d.Detail(), "fixture") {
				t.Fatal("credential leaked")
			}
		}
	}
}
