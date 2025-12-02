package main

import (
	"context"
	"log"

	"terraform-provider-tsuga/internal/provider"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var (
	// These will be set by the `goreleaser` configuration
	// to appropriate values for the compiled binary.
	version string = "dev"
	commit  string = "none"
	date    string = "unknown"
)

func main() {
	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/tsuga-dev/tsuga",
	}

	err := providerserver.Serve(context.Background(), provider.New(version, commit, date), opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}
