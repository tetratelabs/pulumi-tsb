// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package main

import (
	"context"
	_ "embed"

	"github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"

	tsb "github.com/tetratelabs/pulumi-tsb/provider"
)

//go:embed schema.json
var pulumiSchema []byte

//go:embed bridge-metadata.json
var bridgeMetadata []byte

func main() {
	meta := tfbridge.ProviderMetadata{
		PackageSchema:  pulumiSchema,
		BridgeMetadata: bridgeMetadata,
	}
	tfbridge.Main(context.Background(), "tsb", tsb.Provider(), meta)
}
