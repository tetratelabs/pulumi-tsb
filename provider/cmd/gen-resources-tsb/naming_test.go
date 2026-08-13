// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package main

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestEnumModule(t *testing.T) {
	cases := map[protoreflect.FullName]string{
		"tetrateio.api.tsb.v2":                         "index",
		"tetrateio.api.tsb.auth.v2":                    "auth",
		"tetrateio.api.tsb.gateway.v2":                 "gateway",
		"tetrateio.api.tsb.observability.telemetry.v2": "observabilityTelemetry",
		"tetrateio.api.install.controlplane.v1alpha1":  "installControlplane",
		"tetrateio.api.install.dataplane.v1alpha1":     "installDataplane",
		// Reserved names must remap, never silently pass through.
		"tetrateio.api.tsb.types.v2":   "coreTypes",
		"tetrateio.api.tsb.private.v2": "privateApi",
	}
	for pkg, want := range cases {
		got, err := enumModule(pkg)
		if err != nil {
			t.Errorf("enumModule(%q) returned error: %v", pkg, err)
			continue
		}
		if got != want {
			t.Errorf("enumModule(%q) = %q, want %q", pkg, got, want)
		}
	}
}

func TestEnumModuleRejectsForeignPackages(t *testing.T) {
	// An enum outside tetrateio.api has no derivable module. Guessing one is
	// worse than reporting it.
	if _, err := enumModule("google.protobuf"); err == nil {
		t.Error("enumModule(\"google.protobuf\") = nil error, want an error")
	}
}

func TestEnumTypeName(t *testing.T) {
	cases := []struct {
		pkg, full protoreflect.FullName
		want      string
	}{
		{"tetrateio.api.tsb.auth.v2", "tetrateio.api.tsb.auth.v2.TLSMode", "TLSMode"},
		{
			"tetrateio.api.tsb.gateway.v2",
			"tetrateio.api.tsb.gateway.v2.ServerTLSSettings.TLSMode",
			"ServerTLSSettingsTLSMode",
		},
		{
			"tetrateio.api.tsb.traffic.v2",
			"tetrateio.api.tsb.traffic.v2.RateLimitSettings.RateLimitValue.Unit",
			"RateLimitSettingsRateLimitValueUnit",
		},
	}
	for _, c := range cases {
		if got := enumTypeName(c.pkg, c.full); got != c.want {
			t.Errorf("enumTypeName(%q, %q) = %q, want %q", c.pkg, c.full, got, c.want)
		}
	}
}

func TestEnumToken(t *testing.T) {
	e := enumInfo{
		FullName: "tetrateio.api.tsb.auth.v2.TLSMode",
		Package:  "tetrateio.api.tsb.auth.v2",
	}
	got, err := enumToken(e)
	if err != nil {
		t.Fatalf("enumToken returned error: %v", err)
	}
	if want := "tsb:auth/TLSMode:TLSMode"; got != want {
		t.Errorf("enumToken = %q, want %q", got, want)
	}
}

func TestEnumMemberName(t *testing.T) {
	cases := map[string]string{
		"DISABLED":          "Disabled",
		"ISTIO_MUTUAL":      "IstioMutual",
		"CURRENT_UNDEFINED": "CurrentUndefined",
		"TLSV1_0":           "Tlsv10",
		"https":             "Https",
		"UNSET":             "Unset",
	}
	for in, want := range cases {
		got, err := enumMemberName(in)
		if err != nil {
			t.Errorf("enumMemberName(%q) returned error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("enumMemberName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEnumMemberNameRejectsInvalidIdentifiers(t *testing.T) {
	if _, err := enumMemberName("1_FIRST"); err == nil {
		t.Error("enumMemberName(\"1_FIRST\") = nil error, want an error")
	}
}
