// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAppTerraformUnknownTitleCodeBecomesNull(t *testing.T) {
	fixture := newAppFixture()
	server := httptest.NewServer(fixture)
	defer server.Close()
	config := fmt.Sprintf(`
provider "kintone" {
 base_url = %q
 username = "fixture-user"
 password = "fixture-password"
}
resource "terraform_data" "optional" { input = null }
resource "kintone_app" "test" {
 name = "tfacc-local"
 title_field = {
  selection_mode = "AUTO"
  field_code = terraform_data.optional.id == "" ? "unused" : null
 }
}
`, server.URL)
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: integrationFactories(), Steps: []resource.TestStep{{Config: config, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("kintone_app.test", "title_field.field_code", "auto_title"), fixture.checkDeployments(1))}}})
}

// A value that resolves to a real code must still be rejected before app creation.
func TestAppTerraformUnknownTitleCodeBecomesNonNull(t *testing.T) {
	fixture := newAppFixture()
	server := httptest.NewServer(fixture)
	defer server.Close()
	config := fmt.Sprintf(`
provider "kintone" {
 base_url = %q
 username = "fixture-user"
 password = "fixture-password"
}
resource "terraform_data" "optional" { input = null }
resource "kintone_app" "test" {
 name = "tfacc-local"
 title_field = {
  selection_mode = "AUTO"
  field_code = terraform_data.optional.id == "" ? null : "manual_title"
 }
}
`, server.URL)
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: integrationFactories(), Steps: []resource.TestStep{{Config: config, ExpectError: regexp.MustCompile("Omit field_code in AUTO mode")}}})
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.creates != 0 {
		t.Fatalf("invalid title code created %d apps", fixture.creates)
	}
}
