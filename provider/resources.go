// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package tsb

import (
	"unicode"

	framework "github.com/hashicorp/terraform-plugin-framework/provider"
	tfpfbridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	tsbprovider "github.com/tetratelabs/terraform-provider-tsb/pkg/provider"
)

const tsbPkg = "tsb"
const tsbMod = "index"

// getProvider unwraps terraform-provider-tsb's constructor — New() returns
// a func() provider.Provider (for providerserver.Serve), not a value.
func getProvider() framework.Provider {
	return tsbprovider.New()()
}

func tsbMember(mod string, mem string) tokens.ModuleMember {
	return tokens.ModuleMember(tsbPkg + ":" + mod + ":" + mem)
}

func tsbType(mod string, typ string) tokens.Type {
	return tokens.Type(tsbMember(mod, typ))
}

func tsbResourceTok(mod string, res string) tokens.Type {
	fn := string(unicode.ToLower(rune(res[0]))) + res[1:]
	return tsbType(mod+"/"+fn, res)
}

func Provider() tfbridge.ProviderInfo {
	return tfbridge.ProviderInfo{
		P:                 tfpfbridge.ShimProvider(getProvider()),
		Name:              "tsb",
		GitHubOrg:         "tetratelabs",
		TFProviderVersion: "0.1.4",
		Version:           "0.1.4",
		Resources:         generatedResources,
		ExtraTypes:        generatedEnums,
		JavaScript: &tfbridge.JavaScriptInfo{
			PackageName:   "@tetratelabs/pulumi-tsb",
			ContainsEnums: true,
			Dependencies: map[string]string{
				"@pulumi/pulumi": "^3.0.0",
			},
			DevDependencies: map[string]string{
				"@types/node": "^10.0.0",
			},
		},
		MetadataInfo: tfbridge.NewProviderMetadata([]byte{}),
	}
}
