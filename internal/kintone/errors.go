// SPDX-License-Identifier: MPL-2.0

package kintone

import (
	"fmt"
	"strings"
)

// APIError retains machine-readable API failure details without exposing response bodies.
type APIError struct {
	StatusCode int
	Code       string
	ID         string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("kintone API request failed (HTTP %d)", e.StatusCode)
}

// IsConflict reports revision conflicts, including API code variants.
func (e *APIError) IsConflict() bool {
	return e.StatusCode == 409 || strings.HasPrefix(e.Code, "GAIA_CO")
}
