// SPDX-License-Identifier: MPL-2.0

package kintone

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func settingsPointer[T any](value T) *T { return &value }

func TestMergeSettingsPreservesNestedSiblings(t *testing.T) {
	current := Settings{
		Name: "current", Theme: "BLUE", Revision: "7",
		TitleField:      TitleField{SelectionMode: "MANUAL", Code: "old"},
		NumberPrecision: NumberPrecision{Digits: "20", DecimalPlaces: "5", RoundingMode: "HALF_EVEN"},
	}
	update := SettingsUpdate{
		Description:      settingsPointer(""),
		TitleField:       &TitleFieldUpdate{Code: settingsPointer("new")},
		NumberPrecision:  &NumberPrecisionUpdate{DecimalPlaces: settingsPointer("0")},
		EnableThumbnails: settingsPointer(false), EnableBulkDeletion: settingsPointer(false),
		EnableComments: settingsPointer(false), EnableDuplicateRecord: settingsPointer(false), EnableInlineRecordEditing: settingsPointer(false),
	}
	merged, err := mergeSettings(current, update)
	if err != nil {
		t.Fatal(err)
	}
	if *merged.TitleField.SelectionMode != "MANUAL" || *merged.TitleField.Code != "new" {
		t.Fatalf("unexpected title: %+v", merged.TitleField)
	}
	if *merged.NumberPrecision.Digits != "20" || *merged.NumberPrecision.DecimalPlaces != "0" || *merged.NumberPrecision.RoundingMode != "HALF_EVEN" {
		t.Fatalf("unexpected precision: %+v", merged.NumberPrecision)
	}
	if update.TitleField.SelectionMode != nil || update.NumberPrecision.Digits != nil {
		t.Fatal("merge mutated caller input")
	}
	if current.TitleField.Code != "old" || current.NumberPrecision.DecimalPlaces != "5" {
		t.Fatal("merge mutated preview")
	}
	data, err := json.Marshal(merged)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"name", "theme", "firstMonthOfFiscalYear", "revision"} {
		if _, found := fields[key]; found {
			t.Errorf("untouched %s was included", key)
		}
	}
	if string(fields["description"]) != `""` {
		t.Fatal("empty description omitted")
	}
	for _, key := range []string{"enableThumbnails", "enableBulkDeletion", "enableComments", "enableDuplicateRecord", "enableInlineRecordEditing"} {
		if string(fields[key]) != "false" {
			t.Errorf("false %s omitted", key)
		}
	}
}

func TestSettingsValidation(t *testing.T) {
	tests := []struct {
		name   string
		update SettingsUpdate
	}{
		{"empty name", SettingsUpdate{Name: settingsPointer("")}},
		{"long name", SettingsUpdate{Name: settingsPointer(strings.Repeat("あ", 65))}},
		{"long description", SettingsUpdate{Description: settingsPointer(strings.Repeat("a", 10001))}},
		{"theme", SettingsUpdate{Theme: settingsPointer("SECRET")}},
		{"month zero", SettingsUpdate{FirstMonthOfFiscalYear: settingsPointer("0")}},
		{"month overflow", SettingsUpdate{FirstMonthOfFiscalYear: settingsPointer("13")}},
		{"month syntax", SettingsUpdate{FirstMonthOfFiscalYear: settingsPointer("+1")}},
		{"digits zero", SettingsUpdate{NumberPrecision: &NumberPrecisionUpdate{Digits: settingsPointer("0")}}},
		{"digits overflow", SettingsUpdate{NumberPrecision: &NumberPrecisionUpdate{Digits: settingsPointer("31")}}},
		{"decimal negative", SettingsUpdate{NumberPrecision: &NumberPrecisionUpdate{DecimalPlaces: settingsPointer("-1")}}},
		{"decimal overflow", SettingsUpdate{NumberPrecision: &NumberPrecisionUpdate{DecimalPlaces: settingsPointer("11")}}},
		{"rounding", SettingsUpdate{NumberPrecision: &NumberPrecisionUpdate{RoundingMode: settingsPointer("SECRET")}}},
		{"selection", SettingsUpdate{TitleField: &TitleFieldUpdate{SelectionMode: settingsPointer("SECRET")}}},
		{"manual empty code", SettingsUpdate{TitleField: &TitleFieldUpdate{SelectionMode: settingsPointer("MANUAL"), Code: settingsPointer("")}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateSettingsUpdate(test.update)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if strings.Contains(err.Error(), "SECRET") {
				t.Fatal("validation exposed input")
			}
		})
	}
	if err := validateSettingsUpdate(SettingsUpdate{Name: settingsPointer(strings.Repeat("あ", 64)), Description: settingsPointer(""), NumberPrecision: &NumberPrecisionUpdate{Digits: settingsPointer("30"), DecimalPlaces: settingsPointer("0"), RoundingMode: settingsPointer("HALF_EVEN")}}); err != nil {
		t.Fatal(err)
	}
}

func TestMergeSettingsValidatesCompletedTitle(t *testing.T) {
	_, err := mergeSettings(Settings{TitleField: TitleField{SelectionMode: "AUTO", Code: ""}}, SettingsUpdate{TitleField: &TitleFieldUpdate{SelectionMode: settingsPointer("MANUAL")}})
	if err == nil {
		t.Fatal("MANUAL with empty inherited code accepted")
	}
	merged, err := mergeSettings(Settings{TitleField: TitleField{SelectionMode: "MANUAL", Code: "existing"}}, SettingsUpdate{TitleField: &TitleFieldUpdate{SelectionMode: settingsPointer("AUTO")}})
	if err != nil {
		t.Fatal(err)
	}
	if *merged.TitleField.Code != "existing" {
		t.Fatal("inherited code lost")
	}
}

func TestSettingsUpdateEmpty(t *testing.T) {
	if !(SettingsUpdate{}).empty() {
		t.Fatal("zero update is not empty")
	}
	if (SettingsUpdate{EnableComments: settingsPointer(false)}).empty() {
		t.Fatal("explicit false is empty")
	}
	if (SettingsUpdate{TitleField: &TitleFieldUpdate{}}).empty() {
		t.Fatal("explicit nested object is empty")
	}
}

func TestSettingsReadsSeparateEnvironments(t *testing.T) {
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Method != http.MethodGet || r.URL.Query().Get("app") != "9007199254740993" {
			t.Errorf("unexpected settings request: %s %s", r.Method, r.URL.Path)
		}
		fmt.Fprint(w, `{"name":"read","revision":"9007199254740994","numberPrecision":{"digits":"20","decimalPlaces":"0","roundingMode":"HALF_EVEN"},"firstMonthOfFiscalYear":"1"}`)
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, APITokens: []string{"test-token"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, read := range []func(context.Context, string) (Settings, error){client.GetLiveSettings, client.GetPreviewSettings} {
		settings, err := read(context.Background(), "9007199254740993")
		if err != nil {
			t.Fatal(err)
		}
		if settings.Revision != "9007199254740994" || settings.NumberPrecision.DecimalPlaces != "0" || settings.FirstMonthOfFiscalYear != "1" {
			t.Fatalf("string values lost: %+v", settings)
		}
	}
	if len(paths) != 2 || paths[0] != "/k/v1/app/settings.json" || paths[1] != "/k/v1/preview/app/settings.json" {
		t.Fatalf("wrong environments: %v", paths)
	}
}

func TestSettingsReadRejectsInvalidRevision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{"revision":"SECRET"}`) }))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, APITokens: []string{"test-token"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.GetLiveSettings(context.Background(), "1")
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("expected sanitized revision error, got %v", err)
	}
}
