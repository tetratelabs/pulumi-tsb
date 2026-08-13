// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package main

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	// Linked for its side effect: registering the TSB API descriptors this
	// test looks up. main.go pulls the same packages in transitively.
	_ "github.com/tetratelabs/terraform-provider-tsb/pkg/provider"
)

// The two TLSMode enums are the case that motivates identity-based naming:
// same leaf name, different packages, different values.
func TestResolveEnumsReadsDescriptors(t *testing.T) {
	got, err := resolveEnums(map[protoreflect.FullName]bool{
		"tetrateio.api.tsb.auth.v2.TLSMode":    true,
		"tetrateio.api.tsb.profile.v2.TLSMode": true,
	})
	if err != nil {
		t.Fatalf("resolveEnums returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("resolveEnums returned %d enums, want 2", len(got))
	}

	// Sorted by FullName, so auth precedes profile.
	if want := protoreflect.FullName("tetrateio.api.tsb.auth.v2"); got[0].Package != want {
		t.Errorf("got[0].Package = %q, want %q", got[0].Package, want)
	}
	wantAuth := []string{"DISABLED", "SIMPLE", "MUTUAL", "ISTIO_MUTUAL"}
	if !reflect.DeepEqual(got[0].Values, wantAuth) {
		t.Errorf("auth TLSMode values = %v, want %v", got[0].Values, wantAuth)
	}
	wantProfile := []string{"DISABLED", "SIMPLE", "MUTUAL"}
	if !reflect.DeepEqual(got[1].Values, wantProfile) {
		t.Errorf("profile TLSMode values = %v, want %v", got[1].Values, wantProfile)
	}
}

func TestResolveEnumsRejectsUnknownNames(t *testing.T) {
	_, err := resolveEnums(map[protoreflect.FullName]bool{"nope.NotAnEnum": true})
	if err == nil {
		t.Error("resolveEnums(unknown name) = nil error, want an error")
	}
}
