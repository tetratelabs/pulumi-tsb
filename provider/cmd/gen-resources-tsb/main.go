// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"google.golang.org/protobuf/reflect/protoreflect"

	tsbprovider "github.com/tetratelabs/terraform-provider-tsb/pkg/provider"
)

// providerTypeName is the Terraform provider type prefix every tsb
// resource's Metadata() call prepends its suffix to (e.g. "tsb_workspace").
const providerTypeName = "tsb"

// discoverResources instantiates every resource the live terraform-provider-tsb
// registers, asking each for its real Terraform type name and walking its
// schema for enum leaves, so the generated map always reflects what the
// provider actually exposes rather than a side artifact like doc filenames.
func discoverResources(ctx context.Context) ([]string, []enumSite, error) {
	p := tsbprovider.New()()

	var (
		types []string
		sites []enumSite
	)
	for _, newResource := range p.Resources(ctx) {
		r := newResource()

		var md resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: providerTypeName}, &md)
		if md.TypeName == "" {
			return nil, nil, fmt.Errorf("resource %T returned an empty TypeName", r)
		}
		types = append(types, md.TypeName)

		var sr resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &sr)
		if sr.Diagnostics.HasError() {
			return nil, nil, fmt.Errorf("resource %s: schema diagnostics: %v", md.TypeName, sr.Diagnostics)
		}
		found, err := walkSchema(md.TypeName, sr.Schema)
		if err != nil {
			return nil, nil, err
		}
		sites = append(sites, found...)
	}

	sort.Strings(types)
	return types, sites, nil
}

func main() {
	out := flag.String("o", "provider/resources_gen.go", "output file path")
	flag.Parse()

	ctx := context.Background()

	types, sites, err := discoverResources(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen-resources-tsb:", err)
		os.Exit(1)
	}

	names := make(map[protoreflect.FullName]bool, len(sites))
	for _, s := range sites {
		names[s.Enum] = true
	}
	enums, err := resolveEnums(names)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen-resources-tsb:", err)
		os.Exit(1)
	}

	src, err := generateSource(types, tsbprovider.Groups(), sites, enums)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen-resources-tsb: generating source:", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*out, src, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gen-resources-tsb:", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "gen-resources-tsb: wrote %d resources, %d enums, %d enum sites to %s\n",
		len(types), len(enums), len(sites), *out)
}
