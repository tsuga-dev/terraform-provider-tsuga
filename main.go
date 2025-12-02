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
		// NOTE: This is not a typical Terraform Registry provider address,
		// such as registry.terraform.io/hashicorp/hashicups. This specific
		// provider address is used in these tutorials in conjunction with a
		// specific Terraform CLI configuration for manual development testing
		// of this provider.
		Address: "hashicorp.com/edu/tsuga",
	}

	err := providerserver.Serve(context.Background(), provider.New(version, commit, date), opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}
