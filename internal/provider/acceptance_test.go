// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Validate the entire live suite before any test can create an app.
func TestMain(m *testing.M) {
	if os.Getenv("TF_ACC") == "1" {
		if err := validateAcceptanceEnvironment(os.Getenv); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

func validateAcceptanceEnvironment(lookup func(string) string) error {
	if lookup("KINTONE_DEV_ALLOW_ACCEPTANCE_TESTS") != "1" {
		return fmt.Errorf("KINTONE_DEV_ALLOW_ACCEPTANCE_TESTS=1 is required")
	}
	for _, key := range []string{"KINTONE_DEV_BASE_URL", "KINTONE_DEV_USERNAME", "KINTONE_DEV_PASSWORD", "KINTONE_DEV_TOKEN_APP_ID", "KINTONE_DEV_API_TOKENS"} {
		if strings.TrimSpace(lookup(key)) == "" {
			return fmt.Errorf("%s is required; generic credentials are never used", key)
		}
	}
	id := lookup("KINTONE_DEV_TOKEN_APP_ID")
	_, err := strconv.ParseInt(id, 10, 64)
	if err != nil || !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(id) {
		return fmt.Errorf("KINTONE_DEV_TOKEN_APP_ID must be a positive decimal app ID")
	}
	tokens := strings.Split(lookup("KINTONE_DEV_API_TOKENS"), ",")
	if len(tokens) > 9 {
		return fmt.Errorf("KINTONE_DEV_API_TOKENS must contain at most nine tokens")
	}
	for _, token := range tokens {
		if strings.TrimSpace(token) == "" || strings.ContainsAny(token, "\r\n") {
			return fmt.Errorf("KINTONE_DEV_API_TOKENS must contain non-empty comma-separated tokens")
		}
	}
	return nil
}

func setAcceptanceEnvironment(t *testing.T, tokenOnly bool) {
	t.Helper()
	t.Setenv("KINTONE_BASE_URL", os.Getenv("KINTONE_DEV_BASE_URL"))
	t.Setenv("KINTONE_USERNAME", "")
	t.Setenv("KINTONE_PASSWORD", "")
	t.Setenv("KINTONE_API_TOKENS", "")
	if tokenOnly {
		t.Setenv("KINTONE_API_TOKENS", os.Getenv("KINTONE_DEV_API_TOKENS"))
	} else {
		t.Setenv("KINTONE_USERNAME", os.Getenv("KINTONE_DEV_USERNAME"))
		t.Setenv("KINTONE_PASSWORD", os.Getenv("KINTONE_DEV_PASSWORD"))
	}
}

func tokenAppConfig(id string) string {
	return fmt.Sprintf(`provider "kintone" {}
data "kintone_app" "token" { id = %q }
`, id)
}

func tokenAppChecks(id string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr("data.kintone_app.token", "id", id),
		resource.TestCheckResourceAttrSet("data.kintone_app.token", "name"),
		resource.TestCheckResourceAttrSet("data.kintone_app.token", "revision"),
		resource.TestCheckResourceAttrSet("data.kintone_app.token", "number_precision.total_digits"),
	)
}

// Read a dedicated, already deployed app; never create, update, or deploy it.
func TestAccAppAPIToken(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" {
		t.Skip("set TF_ACC=1 for live acceptance tests")
	}
	setAcceptanceEnvironment(t, true)
	id := os.Getenv("KINTONE_DEV_TOKEN_APP_ID")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: integrationFactories(),
		Steps:                    []resource.TestStep{{Config: tokenAppConfig(id), Check: tokenAppChecks(id)}},
	})
}

func TestAcceptanceEnvironmentValidation(t *testing.T) {
	valid := map[string]string{
		"KINTONE_DEV_ALLOW_ACCEPTANCE_TESTS": "1", "KINTONE_DEV_BASE_URL": "https://example.invalid",
		"KINTONE_DEV_USERNAME": "fixture-user", "KINTONE_DEV_PASSWORD": "fixture-password",
		"KINTONE_DEV_TOKEN_APP_ID": "100", "KINTONE_DEV_API_TOKENS": "fixture-token",
	}
	lookup := func(k string) string { return valid[k] }
	if err := validateAcceptanceEnvironment(lookup); err != nil {
		t.Fatal(err)
	}
	for key := range valid {
		t.Run(key, func(t *testing.T) {
			for _, missing := range []string{"", " "} {
				err := validateAcceptanceEnvironment(func(k string) string {
					if k == key {
						return missing
					}
					if strings.HasPrefix(k, "KINTONE_DEV_") {
						return valid[k]
					}
					return "generic-credential-must-not-be-used"
				})
				if err == nil || !strings.Contains(err.Error(), key) {
					t.Fatalf("expected rejection naming %s", key)
				}
			}
		})
	}
	for _, tc := range []struct{ key, value string }{
		{"KINTONE_DEV_ALLOW_ACCEPTANCE_TESTS", "true"},
		{"KINTONE_DEV_TOKEN_APP_ID", "0"}, {"KINTONE_DEV_TOKEN_APP_ID", "9223372036854775808"},
		{"KINTONE_DEV_API_TOKENS", strings.Repeat("fixture,", 9) + "fixture"},
		{"KINTONE_DEV_API_TOKENS", "fixture\ntoken"}, {"KINTONE_DEV_TOKEN_APP_ID", "100\""},
		{"KINTONE_DEV_API_TOKENS", " , "}, {"KINTONE_DEV_API_TOKENS", "fixture-token,"},
	} {
		err := validateAcceptanceEnvironment(func(k string) string {
			if k == tc.key {
				return tc.value
			}
			return valid[k]
		})
		if err == nil || !strings.Contains(err.Error(), tc.key) {
			t.Fatalf("expected rejection naming %s", tc.key)
		}
	}
}

func TestAcceptanceTokenIsolation(t *testing.T) {
	fixture := newAppFixture()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || (r.URL.Path != "/k/v1/app.json" && r.URL.Path != "/k/v1/app/settings.json") {
			t.Error("token acceptance test attempted an unexpected operation")
			http.Error(w, "unexpected operation", http.StatusForbidden)
			return
		}
		if r.Header.Get("X-Cybozu-Authorization") != "" || r.Header.Get("Authorization") != "" || r.Header.Get("X-Cybozu-API-Token") != "fixture-token" {
			t.Error("token acceptance authentication was not isolated")
			http.Error(w, "unexpected authentication", http.StatusForbidden)
			return
		}
		fixture.ServeHTTP(w, r)
	}))
	defer server.Close()
	t.Setenv("KINTONE_DEV_BASE_URL", server.URL)
	t.Setenv("KINTONE_DEV_API_TOKENS", "fixture-token")
	for _, key := range []string{"KINTONE_USERNAME", "KINTONE_PASSWORD", "KINTONE_API_TOKENS", "KINTONE_DEV_USERNAME", "KINTONE_DEV_PASSWORD"} {
		t.Setenv(key, "must-not-be-sent")
	}
	setAcceptanceEnvironment(t, true)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: integrationFactories(),
		Steps:                    []resource.TestStep{{Config: tokenAppConfig("100"), Check: tokenAppChecks("100")}},
	})
}
