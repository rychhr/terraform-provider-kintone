// SPDX-License-Identifier: MPL-2.0

// Package kintone is a Terraform-independent HTTP client for the kintone REST
// API, hand-rolled on net/http.
//
// The package boundary is architectural and enforced by depguard: nothing here
// may import terraform-plugin-framework or any other Terraform package. All
// kintone API call-order logic — writing to the preview environment, deploying,
// and polling until the deployment settles — belongs in this package rather
// than in internal/provider.
package kintone
