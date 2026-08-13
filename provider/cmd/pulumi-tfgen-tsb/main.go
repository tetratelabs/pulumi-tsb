// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package main

import (
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfgen"

	tsb "github.com/tetratelabs/pulumi-tsb/provider"
)

func main() {
	tfgen.Main("tsb", tsb.Provider())
}
