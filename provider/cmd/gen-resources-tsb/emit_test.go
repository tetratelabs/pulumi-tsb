// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package main

import (
	"strings"
	"testing"
	"unicode"
)

func TestGenerateSourceEmitsEnumsAndNestedFields(t *testing.T) {
	enums := []enumInfo{{
		FullName: "tetrateio.api.tsb.auth.v2.TLSMode",
		Package:  "tetrateio.api.tsb.auth.v2",
		Values:   []string{"DISABLED", "SIMPLE", "MUTUAL", "ISTIO_MUTUAL"},
	}}
	sites := []enumSite{
		{Resource: "tsb_cluster", Path: []string{"spec", "tls", "mode"},
			Enum: "tetrateio.api.tsb.auth.v2.TLSMode"},
		{Resource: "tsb_cluster", Path: []string{"modes"}, Element: true,
			Enum: "tetrateio.api.tsb.auth.v2.TLSMode"},
	}

	got, err := generateSource([]string{"tsb_cluster"}, map[string]string{}, sites, enums)
	if err != nil {
		t.Fatalf("generateSource returned error: %v", err)
	}
	// gofmt aligns consecutive map entries, inserting padding after the colon,
	// so compare with all whitespace stripped rather than against exact spacing.
	src := stripSpace(string(got))

	for _, want := range []string{
		`"tsb:auth/TLSMode:TLSMode":{`,
		`{Name:"IstioMutual",Value:"ISTIO_MUTUAL"}`,
		`"mode":{Type:"tsb:auth/TLSMode:TLSMode"}`,
		`"modes":{Elem:&tfbridge.SchemaInfo{Type:"tsb:auth/TLSMode:TLSMode"}}`,
		`"spec":{Elem:&tfbridge.SchemaInfo{Fields:map[string]*tfbridge.SchemaInfo{`,
	} {
		if !strings.Contains(src, stripSpace(want)) {
			t.Errorf("generated source is missing %q\n---\n%s", want, got)
		}
	}
}

// stripSpace removes every whitespace character so assertions do not depend on
// gofmt's alignment decisions.
func stripSpace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

// Two sites under one parent must merge into a single Fields entry. Emitting
// the key twice would be a duplicate map key: go/format only parses, so it
// would NOT catch this — it surfaces later as a compile error in Task 6.
func TestGenerateSourceMergesSiblingPaths(t *testing.T) {
	enums := []enumInfo{{
		FullName: "tetrateio.api.tsb.auth.v2.TLSMode",
		Package:  "tetrateio.api.tsb.auth.v2",
		Values:   []string{"DISABLED"},
	}}
	sites := []enumSite{
		{Resource: "tsb_cluster", Path: []string{"spec", "a"}, Enum: "tetrateio.api.tsb.auth.v2.TLSMode"},
		{Resource: "tsb_cluster", Path: []string{"spec", "b"}, Enum: "tetrateio.api.tsb.auth.v2.TLSMode"},
	}

	got, err := generateSource([]string{"tsb_cluster"}, map[string]string{}, sites, enums)
	if err != nil {
		t.Fatalf("generateSource returned error: %v", err)
	}
	if n := strings.Count(stripSpace(string(got)), `"spec":`); n != 1 {
		t.Errorf("generated source has %d \"spec\" keys, want 1\n---\n%s", n, got)
	}
}

// Two proto values that mangle to the same member name would emit a duplicate
// TypeScript enum member. Detect it at generation time.
func TestGenerateSourceRejectsCollidingMemberNames(t *testing.T) {
	enums := []enumInfo{{
		FullName: "tetrateio.api.tsb.auth.v2.Clashing",
		Package:  "tetrateio.api.tsb.auth.v2",
		Values:   []string{"TLSV1_0", "TLSV10"}, // both mangle to "Tlsv10"
	}}
	_, err := generateSource([]string{"tsb_cluster"}, map[string]string{}, nil, enums)
	if err == nil {
		t.Error("generateSource(colliding member names) = nil error, want an error")
	}
}
