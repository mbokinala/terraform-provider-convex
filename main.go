package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/mbokinala/terraform-provider-convex/internal/provider"
)

const version = "dev"

func main() {
	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/mbokinala/convex",
	})
	if err != nil {
		log.Fatal(err)
	}
}
