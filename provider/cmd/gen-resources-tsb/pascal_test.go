// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package main

import "testing"

func TestPascalCase(t *testing.T) {
	cases := map[string]string{
		"workspace":                        "Workspace",
		"gateway_group":                    "GatewayGroup",
		"istio_gateway_virtual_service":    "IstioGatewayVirtualService",
		"api":                              "Api",
		"oidc":                             "Oidc",
		"istiointernal_group":              "IstiointernalGroup",
		"rbac_role":                        "RbacRole",
		"access_binding":                   "AccessBinding",
	}
	for in, want := range cases {
		if got := pascalCase(in); got != want {
			t.Errorf("pascalCase(%q) = %q, want %q", in, got, want)
		}
	}
}
