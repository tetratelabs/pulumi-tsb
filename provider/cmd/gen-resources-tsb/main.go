// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	tsbprovider "github.com/tetratelabs/terraform-provider-tsb/pkg/provider"
)

// providerTypeName is the Terraform provider type prefix every tsb
// resource's Metadata() call prepends its suffix to (e.g. "tsb_workspace").
const providerTypeName = "tsb"

// discoverResourceTypes instantiates every resource the live
// terraform-provider-tsb registers and asks each for its real Terraform
// type name, so the generated map always reflects what the provider
// actually exposes rather than a side artifact like doc filenames.
func discoverResourceTypes(ctx context.Context) ([]string, error) {
	p := tsbprovider.New()()

	var types []string
	for _, newResource := range p.Resources(ctx) {
		r := newResource()

		var resp resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: providerTypeName}, &resp)
		if resp.TypeName == "" {
			return nil, fmt.Errorf("resource %T returned an empty TypeName", r)
		}
		types = append(types, resp.TypeName)
	}

	sort.Strings(types)
	return types, nil
}

func main() {
	out := flag.String("o", "provider/resources_gen.go", "output file path")
	flag.Parse()

	ctx := context.Background()

	types, err := discoverResourceTypes(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen-resources-tsb:", err)
		os.Exit(1)
	}

	groups := tsbprovider.Groups()

	// sites and enums are wired up in a later task, once resource schemas are
	// available here to walk.
	src, err := generateSource(types, groups, nil, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen-resources-tsb: formatting generated source:", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*out, src, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gen-resources-tsb:", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "gen-resources-tsb: wrote %d resources to %s\n", len(types), *out)
}
