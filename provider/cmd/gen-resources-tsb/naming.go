// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package main

import (
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// tsbPkg is the Pulumi package name; it matches tsbPkg in provider/resources.go.
const tsbPkg = "tsb"

// protoPkgPrefix is the namespace every TSB API proto package lives under.
const protoPkgPrefix = "tetrateio.api."

// versionSegment matches a proto package's trailing version segment (v2,
// v1alpha1, v1beta2), which carries no meaning in a Pulumi module name.
var versionSegment = regexp.MustCompile(`^v[0-9]+[a-z0-9]*$`)

// reservedModules are names the generated nodejs SDK already uses for its own
// directories, or that are not usable as TypeScript identifiers.
var reservedModules = map[string]bool{
	"config":    true,
	"private":   true, // TypeScript keyword
	"provider":  true,
	"scripts":   true,
	"types":     true,
	"utilities": true,
}

// moduleRemap resolves derived names that land in reservedModules. A reserved
// name absent from this table is a generation error rather than a silent
// rename, so a future upstream package cannot quietly produce a broken SDK.
var moduleRemap = map[string]string{
	"types":   "coreTypes",
	"private": "privateApi",
}

// enumModule derives the Pulumi module name for a proto package.
//
// The core tsb package maps to "index", matching how resources with an empty
// API group are tokenised by tsbResourceTok.
func enumModule(pkg protoreflect.FullName) (string, error) {
	rest, ok := strings.CutPrefix(string(pkg), protoPkgPrefix)
	if !ok {
		return "", fmt.Errorf(
			"proto package %q is outside %s so no Pulumi module can be derived; "+
				"add an explicit entry to moduleRemap if it is genuinely needed", pkg, protoPkgPrefix)
	}

	segs := strings.Split(rest, ".")
	if n := len(segs); n > 1 && versionSegment.MatchString(segs[n-1]) {
		segs = segs[:n-1]
	}
	if len(segs) == 1 && segs[0] == "tsb" {
		return "index", nil
	}
	if segs[0] == "tsb" {
		segs = segs[1:]
	}

	mod := camelJoin(segs)
	if reservedModules[mod] {
		remapped, ok := moduleRemap[mod]
		if !ok {
			return "", fmt.Errorf(
				"proto package %q derives reserved module name %q; add an entry to moduleRemap", pkg, mod)
		}
		return remapped, nil
	}
	return mod, nil
}

// camelJoin joins lowercase proto package segments into a camelCase identifier.
func camelJoin(segs []string) string {
	var b strings.Builder
	for i, s := range segs {
		if s == "" {
			continue
		}
		if i == 0 {
			b.WriteString(s)
			continue
		}
		b.WriteString(strings.ToUpper(s[:1]) + s[1:])
	}
	return b.String()
}

// enumTypeName derives the Pulumi type name from an enum's position inside its
// proto package. Nested enums keep their message path: the leaf name alone is
// not unique even within a single package.
func enumTypeName(pkg, full protoreflect.FullName) string {
	rel := strings.TrimPrefix(string(full), string(pkg)+".")
	return strings.ReplaceAll(rel, ".", "")
}

// enumToken renders the Pulumi type token for an enum, e.g.
// "tsb:auth/TLSMode:TLSMode".
func enumToken(e enumInfo) (string, error) {
	mod, err := enumModule(e.Package)
	if err != nil {
		return "", err
	}
	name := enumTypeName(e.Package, e.FullName)
	return fmt.Sprintf("%s:%s/%s:%s", tsbPkg, mod, name, name), nil
}

// enumMemberName converts a SCREAMING_SNAKE proto value name into a PascalCase
// Pulumi member name (ISTIO_MUTUAL -> IstioMutual).
//
// This is deliberately not pascalCase, which preserves each segment's tail so
// that "api" becomes "Api". Proto value names are fully upper-cased, so
// preserving the tail would yield "ISTIOMUTUAL".
func enumMemberName(v string) (string, error) {
	var b strings.Builder
	for _, part := range strings.Split(v, "_") {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]) + strings.ToLower(part[1:]))
	}
	name := b.String()
	if name == "" {
		return "", fmt.Errorf("enum value %q yields an empty member name", v)
	}
	if name[0] >= '0' && name[0] <= '9' {
		return "", fmt.Errorf(
			"enum value %q yields member name %q, which is not a valid identifier", v, name)
	}
	return name, nil
}
