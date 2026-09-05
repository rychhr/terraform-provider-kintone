// SPDX-License-Identifier: MPL-2.0

package kintone

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"unicode/utf8"
)

// Settings is the general settings of an app in the requested environment.
// API numeric values remain strings to preserve their wire representation.
type Settings struct {
	Name                      string          `json:"name"`
	Description               string          `json:"description"`
	Theme                     string          `json:"theme"`
	TitleField                TitleField      `json:"titleField"`
	NumberPrecision           NumberPrecision `json:"numberPrecision"`
	FirstMonthOfFiscalYear    string          `json:"firstMonthOfFiscalYear"`
	EnableThumbnails          bool            `json:"enableThumbnails"`
	EnableBulkDeletion        bool            `json:"enableBulkDeletion"`
	EnableComments            bool            `json:"enableComments"`
	EnableDuplicateRecord     bool            `json:"enableDuplicateRecord"`
	EnableInlineRecordEditing bool            `json:"enableInlineRecordEditing"`
	Revision                  string          `json:"revision"`
}

// TitleField selects the field used for record titles.
type TitleField struct {
	Code          string `json:"code"`
	SelectionMode string `json:"selectionMode"`
}

// NumberPrecision controls number calculation and display precision.
type NumberPrecision struct {
	Digits        string `json:"digits"`
	DecimalPlaces string `json:"decimalPlaces"`
	RoundingMode  string `json:"roundingMode"`
}

// SettingsUpdate distinguishes omitted properties from explicit empty or false values.
// Modified nested objects are completed from preview settings before they are sent.
type SettingsUpdate struct {
	Name                      *string                `json:"name,omitempty"`
	Description               *string                `json:"description,omitempty"`
	Theme                     *string                `json:"theme,omitempty"`
	TitleField                *TitleFieldUpdate      `json:"titleField,omitempty"`
	NumberPrecision           *NumberPrecisionUpdate `json:"numberPrecision,omitempty"`
	FirstMonthOfFiscalYear    *string                `json:"firstMonthOfFiscalYear,omitempty"`
	EnableThumbnails          *bool                  `json:"enableThumbnails,omitempty"`
	EnableBulkDeletion        *bool                  `json:"enableBulkDeletion,omitempty"`
	EnableComments            *bool                  `json:"enableComments,omitempty"`
	EnableDuplicateRecord     *bool                  `json:"enableDuplicateRecord,omitempty"`
	EnableInlineRecordEditing *bool                  `json:"enableInlineRecordEditing,omitempty"`
}

// TitleFieldUpdate contains the title properties to change.
type TitleFieldUpdate struct {
	Code          *string `json:"code,omitempty"`
	SelectionMode *string `json:"selectionMode,omitempty"`
}

// NumberPrecisionUpdate contains the precision properties to change.
type NumberPrecisionUpdate struct {
	Digits        *string `json:"digits,omitempty"`
	DecimalPlaces *string `json:"decimalPlaces,omitempty"`
	RoundingMode  *string `json:"roundingMode,omitempty"`
}

func (u SettingsUpdate) empty() bool { return u == (SettingsUpdate{}) }

func mergeSettings(current Settings, update SettingsUpdate) (SettingsUpdate, error) {
	if update.TitleField != nil {
		value := *update.TitleField
		if value.Code == nil {
			value.Code = &current.TitleField.Code
		}
		if value.SelectionMode == nil {
			value.SelectionMode = &current.TitleField.SelectionMode
		}
		update.TitleField = &value
	}
	if update.NumberPrecision != nil {
		value := *update.NumberPrecision
		if value.Digits == nil {
			value.Digits = &current.NumberPrecision.Digits
		}
		if value.DecimalPlaces == nil {
			value.DecimalPlaces = &current.NumberPrecision.DecimalPlaces
		}
		if value.RoundingMode == nil {
			value.RoundingMode = &current.NumberPrecision.RoundingMode
		}
		update.NumberPrecision = &value
	}
	if err := validateSettingsUpdate(update); err != nil {
		return SettingsUpdate{}, err
	}
	return update, nil
}

// Validation errors deliberately name properties without echoing supplied values.
func validateSettingsUpdate(update SettingsUpdate) error {
	if update.Name != nil && (utf8.RuneCountInString(*update.Name) < 1 || utf8.RuneCountInString(*update.Name) > 64) {
		return errors.New("name must contain between 1 and 64 characters")
	}
	if update.Description != nil && utf8.RuneCountInString(*update.Description) > 10000 {
		return errors.New("description must contain at most 10000 characters")
	}
	if err := validateChoice("theme", update.Theme, "WHITE", "RED", "BLUE", "GREEN", "YELLOW", "BLACK"); err != nil {
		return err
	}
	if err := validateNumber("firstMonthOfFiscalYear", update.FirstMonthOfFiscalYear, 1, 12); err != nil {
		return err
	}
	if title := update.TitleField; title != nil {
		if err := validateChoice("titleField.selectionMode", title.SelectionMode, "AUTO", "MANUAL"); err != nil {
			return err
		}
		// A missing code may be populated from preview; an explicit empty code cannot.
		if title.SelectionMode != nil && *title.SelectionMode == "MANUAL" && title.Code != nil && *title.Code == "" {
			return errors.New("titleField.code must be nonempty for MANUAL selection")
		}
	}
	if precision := update.NumberPrecision; precision != nil {
		if err := validateNumber("numberPrecision.digits", precision.Digits, 1, 30); err != nil {
			return err
		}
		if err := validateNumber("numberPrecision.decimalPlaces", precision.DecimalPlaces, 0, 10); err != nil {
			return err
		}
		if err := validateChoice("numberPrecision.roundingMode", precision.RoundingMode, "HALF_EVEN", "UP", "DOWN"); err != nil {
			return err
		}
	}
	return nil
}

func validateChoice(property string, value *string, choices ...string) error {
	if value == nil {
		return nil
	}
	for _, choice := range choices {
		if *value == choice {
			return nil
		}
	}
	return fmt.Errorf("%s has an unsupported value", property)
}

func validateNumber(property string, value *string, minimum, maximum int) error {
	if value == nil {
		return nil
	}
	valid := *value != ""
	for _, digit := range *value {
		if digit < '0' || digit > '9' {
			valid = false
		}
	}
	number, err := strconv.Atoi(*value)
	if !valid || err != nil || number < minimum || number > maximum {
		return fmt.Errorf("%s must be a decimal integer between %d and %d", property, minimum, maximum)
	}
	return nil
}

// GetLiveSettings returns settings currently in effect, suitable for drift detection.
func (c *Client) GetLiveSettings(ctx context.Context, appID string) (Settings, error) {
	return c.getSettings(ctx, appID, "/k/v1/app/settings.json")
}

// GetPreviewSettings returns draft settings, suitable for merging a settings update.
func (c *Client) GetPreviewSettings(ctx context.Context, appID string) (Settings, error) {
	return c.getSettings(ctx, appID, "/k/v1/preview/app/settings.json")
}

func (c *Client) getSettings(ctx context.Context, appID, path string) (Settings, error) {
	if err := validateID(appID); err != nil {
		return Settings{}, err
	}
	var result Settings
	if err := c.request(ctx, http.MethodGet, path, url.Values{"app": {appID}}, nil, &result, true); err != nil {
		return Settings{}, err
	}
	if err := validateRevision(result.Revision); err != nil {
		return Settings{}, errors.New("settings response contains an invalid revision")
	}
	return result, nil
}
