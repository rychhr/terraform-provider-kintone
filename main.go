// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/rychhr/terraform-provider-kintone/internal/provider"
)

// version is overridden at build time with -ldflags by the release pipeline.
// Local builds keep "dev".
var version = "dev"

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		// The Registry address selects this provider binary. The kintone_
		// prefix on resource type names comes from the provider's Metadata
		// method instead.
		Address: "registry.terraform.io/rychhr/kintone",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatal(err.Error())
	}
}
