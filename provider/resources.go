// Copyright 2023 Tetrate
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package check

import (
	"unicode"

	framework "github.com/hashicorp/terraform-plugin-framework/provider"
	tfpfbridge "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	provider "github.com/tetratelabs/terraform-provider-tsb/pkg"
)

const tsbPkg = "tsb"
const tsbMod = "index"

func getProvider() framework.Provider {
	return provider.NewProvider()
}

func checkMember(mod string, mem string) tokens.ModuleMember {
	return tokens.ModuleMember(tsbPkg + ":" + mod + ":" + mem)
}

func checkType(mod string, typ string) tokens.Type {
	return tokens.Type(checkMember(mod, typ))
}

func checkResourceTok(mod string, res string) tokens.Type {
	fn := string(unicode.ToLower(rune(res[0]))) + res[1:]
	return checkType(mod+"/"+fn, res)
}

func Provider() tfpfbridge.ProviderInfo {
	info := tfbridge.ProviderInfo{
		Name:              "tsb",
		GitHubOrg:         "tetratelabs",
		TFProviderVersion: "0.0.2",
		Version:           "0.0.1",
		DataSources: map[string]*tfbridge.DataSourceInfo{
			"tsb_organization": {Tok: checkMember(tsbMod, "Organization")},
		},
		Resources: map[string]*tfbridge.ResourceInfo{
			"tsb_tenant": {Tok: checkResourceTok(tsbMod, "Tenant")},
		},
		JavaScript: &tfbridge.JavaScriptInfo{
			Dependencies: map[string]string{
				"@pulumi/pulumi": "^3.0.0",
			},
			DevDependencies: map[string]string{
				"@types/node": "^10.0.0", // so we can access strongly typed node definitions.
			},
		},
	}
	return tfpfbridge.ProviderInfo{
		ProviderInfo: info,
		NewProvider:  getProvider,
	}
}
