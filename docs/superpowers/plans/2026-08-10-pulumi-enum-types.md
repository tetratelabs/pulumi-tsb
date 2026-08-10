# Pulumi Enum Types (Phase 2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Emit real Pulumi enum types for every TSB API enum, so the generated SDK offers completion and compile-time checking instead of bare `string`.

**Architecture:** `gen-resources-tsb` gains a walk over each resource's live `fwschema.Schema`, reading proto identity off the `EnumType` custom type that Phase 1 added. Identity resolves to member names via the global protobuf registry — no string parsing anywhere. Output is a `generatedEnums` map (fed to `ProviderInfo.ExtraTypes`) plus enum tokens threaded into the existing `generatedResources` map as nested `Fields`/`Elem` chains.

**Tech Stack:** Go, `terraform-plugin-framework` schema types, `google.golang.org/protobuf` (protoreflect/protoregistry), `pulumi-terraform-bridge/v3` (`tfbridge`, `pschema`), stdlib `testing`.

**Repo:** `tetratelabs/pulumi-tsb`, at `~/src/pulumi-tsb`. All paths below are relative to that repo root.

**Prerequisite:** Phase 1 (`docs/superpowers/plans/2026-08-10-enum-identity-upstream.md`) must be merged, and `terraform-provider-tsb` must be tagged with the bumped `github.com/tetrateio/tetrate`. Task 1 does that bump.

**Spec:** `docs/superpowers/specs/2026-08-10-pulumi-enum-types-design.md`

## Global Constraints

- Every new file starts with `// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.` — `make licenser` applies it, and `make check` enforces it.
- Commits are signed and carry a sign-off trailer: `git commit -S -s`. Never add `Co-Authored-By` or AI attribution.
- Work on a branch. Nothing is pushed without explicit confirmation. Single remote (`origin`), no fork ambiguity.
- **Fail loudly, never skip.** A silent drop is indistinguishable from success. Every unresolved enum, reserved module name, token collision, or unrecognised schema node is a generation error.
- Generated files (`provider/resources_gen.go`, `provider/cmd/pulumi-resource-tsb/schema.json`, `sdk/`) are committed. `make check` fails on a dirty tree after regeneration.
- Enum member names use `enumMemberName`, **not** the existing `pascalCase`. `pascalCase` preserves each segment's tail (`api` → `Api`), which on a fully upper-cased proto value yields `ISTIOMUTUAL`.
- Shared types used across tasks live in `provider/cmd/gen-resources-tsb/types.go`.

---

### Task 1: Bump to the enum-carrying provider

**Files:**
- Modify: `go.mod`
- Modify: `Makefile:4` (`PROVIDER_VERSION`)
- Modify: `provider/resources.go:43` (`TFProviderVersion`)

**Interfaces:**
- Consumes: the tagged `terraform-provider-tsb` release from Phase 1.
- Produces: a build in which `prototypes.EnumType` values carry a populated `FullName`. Every later task depends on this.

- [ ] **Step 1: Bump the dependency**

Replace `<NEW_VERSION>` with the tag `terraform-provider-tsb` published after Phase 1 (e.g. `0.1.4`):

```bash
cd ~/src/pulumi-tsb
go get github.com/tetratelabs/terraform-provider-tsb@v<NEW_VERSION>
go mod tidy
```

- [ ] **Step 2: Sync the pinned version strings**

`Makefile` and `provider/resources.go` both hardcode the version, and `make providerversion` cross-checks them. Set all three to the new value:

- `Makefile:4` — `PROVIDER_VERSION=<NEW_VERSION>`
- `provider/resources.go:43` — `TFProviderVersion: "<NEW_VERSION>",`
- `provider/resources.go:44` — `Version: "<NEW_VERSION>",` and `Makefile:7` — `VERSION=<NEW_VERSION>`

- [ ] **Step 3: Verify enum identity actually arrives**

This is the gate for the whole plan — confirm the data is present before building anything on it. Create a throwaway file `/tmp/enumcheck/main.go`:

```go
package main

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	prototypes "github.com/tetrateio/tetrate/api/protoc-plugins/protoc-gen-terraform/pkg/types/basetypes"
	tsbprovider "github.com/tetratelabs/terraform-provider-tsb/pkg/provider"
)

func main() {
	ctx := context.Background()
	for _, nr := range tsbprovider.New()().Resources(ctx) {
		r := nr()
		var md resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "tsb"}, &md)
		var sr resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &sr)
		for name, a := range sr.Schema.Attributes {
			if et, ok := a.GetType().(prototypes.EnumType); ok {
				fmt.Printf("%s.%s -> %q\n", md.TypeName, name, et.FullName)
			}
		}
	}
}
```

```bash
cd ~/src/pulumi-tsb
go run /tmp/enumcheck/main.go | head -10
```

Expected: lines with non-empty proto full names, e.g. `tsb_cluster.<attr> -> "tetrateio.api.tsb.v2.<Enum>"`.
An empty `FullName` means the Phase 1 release was not picked up — stop and fix that first.

- [ ] **Step 4: Verify the existing build is unaffected**

```bash
cd ~/src/pulumi-tsb
make generate && git diff --stat provider/resources_gen.go
```

Expected: no diff. Phase 1 changed attribute types, not the resource set.

- [ ] **Step 5: Commit**

```bash
cd ~/src/pulumi-tsb
rm -rf /tmp/enumcheck
git add go.mod go.sum Makefile provider/resources.go
git commit -S -s -m "chore: bump terraform-provider-tsb for enum identity

Picks up protoc-gen-terraform's EnumType.FullName, which the enum
generation in the following commits reads."
```

---

### Task 2: Enum naming helpers

**Files:**
- Create: `provider/cmd/gen-resources-tsb/types.go`
- Create: `provider/cmd/gen-resources-tsb/naming.go`
- Test: `provider/cmd/gen-resources-tsb/naming_test.go`

**Interfaces:**
- Consumes: nothing (pure functions).
- Produces:
  - `type enumInfo struct { FullName, Package protoreflect.FullName; Values []string }`
  - `type enumSite struct { Resource string; Path []string; Element bool; Enum protoreflect.FullName }`
  - `func enumModule(pkg protoreflect.FullName) (string, error)`
  - `func enumTypeName(pkg, full protoreflect.FullName) string`
  - `func enumToken(e enumInfo) (string, error)`
  - `func enumMemberName(v string) (string, error)`

- [ ] **Step 1: Write the failing test**

Create `provider/cmd/gen-resources-tsb/naming_test.go`:

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd ~/src/pulumi-tsb
go test ./provider/cmd/gen-resources-tsb/ -run 'TestEnum' -v
```

Expected: FAIL — `undefined: enumModule`, `undefined: enumInfo`, etc.

- [ ] **Step 3: Add the shared types**

Create `provider/cmd/gen-resources-tsb/types.go`:

```go
// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package main

import "google.golang.org/protobuf/reflect/protoreflect"

// enumInfo is one proto enum reachable from a resource schema.
type enumInfo struct {
	// FullName is the proto full name, e.g. "tetrateio.api.tsb.auth.v2.TLSMode".
	FullName protoreflect.FullName
	// Package is the enum's proto package, e.g. "tetrateio.api.tsb.auth.v2".
	Package protoreflect.FullName
	// Values are the declared member names in proto declaration order.
	Values []string
}

// enumSite is one attribute path in one resource whose leaf is an enum.
type enumSite struct {
	// Resource is the Terraform type name, e.g. "tsb_cluster".
	Resource string
	// Path is the attribute names from the resource root to the enum leaf.
	Path []string
	// Element reports that the leaf is a list or map ELEMENT rather than a
	// scalar attribute, which changes where the token attaches.
	Element bool
	// Enum is the proto full name of the enum at the leaf.
	Enum protoreflect.FullName
}
```

- [ ] **Step 4: Implement the naming rules**

Create `provider/cmd/gen-resources-tsb/naming.go`:

```go
// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package main

import (
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

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
```

`tsbPkg` is the existing `const tsbPkg = "tsb"` in `provider/resources.go` — but that is package `tsb`,
not package `main`. Add a local constant at the top of `naming.go` instead:

```go
// tsbPkg is the Pulumi package name; it matches tsbPkg in provider/resources.go.
const tsbPkg = "tsb"
```

- [ ] **Step 5: Run the tests to verify they pass**

```bash
cd ~/src/pulumi-tsb
go test ./provider/cmd/gen-resources-tsb/ -v
```

Expected: PASS, including the pre-existing `TestPascalCase`.

- [ ] **Step 6: Commit**

```bash
cd ~/src/pulumi-tsb
git add provider/cmd/gen-resources-tsb/types.go \
        provider/cmd/gen-resources-tsb/naming.go \
        provider/cmd/gen-resources-tsb/naming_test.go
git commit -S -s -m "feat(gen): enum module, type and member naming rules

Modules come from the proto package so the ten leaf-name collisions
across packages resolve by namespace rather than mangling. Reserved
names that collide with generated SDK directories must be remapped
explicitly; anything else is an error."
```

---

### Task 3: Resolve enum descriptors from the registry

**Files:**
- Create: `provider/cmd/gen-resources-tsb/enums.go`
- Test: `provider/cmd/gen-resources-tsb/enums_test.go`

**Interfaces:**
- Consumes: `enumInfo` (Task 2).
- Produces: `func resolveEnums(names map[protoreflect.FullName]bool) ([]enumInfo, error)`, returning entries sorted by `FullName`.

- [ ] **Step 1: Write the failing test**

Create `provider/cmd/gen-resources-tsb/enums_test.go`:

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd ~/src/pulumi-tsb
go test ./provider/cmd/gen-resources-tsb/ -run TestResolveEnums -v
```

Expected: FAIL — `undefined: resolveEnums`.

- [ ] **Step 3: Implement the resolver**

Create `provider/cmd/gen-resources-tsb/enums.go`:

```go
// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package main

import (
	"fmt"
	"sort"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// resolveEnums looks each proto enum up in the global protobuf registry and
// returns its package and declared member names, sorted by full name so the
// generated output is deterministic.
//
// Every TSB API package is linked into this binary through
// terraform-provider-tsb, so a registry miss means a schema names an enum this
// build does not know about. That is an error, never a skip: dropping it would
// silently emit a plain string where an enum belongs.
func resolveEnums(names map[protoreflect.FullName]bool) ([]enumInfo, error) {
	sorted := make([]protoreflect.FullName, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	out := make([]enumInfo, 0, len(sorted))
	for _, fn := range sorted {
		d, err := protoregistry.GlobalFiles.FindDescriptorByName(fn)
		if err != nil {
			return nil, fmt.Errorf("enum %q is not in the protobuf registry: %w", fn, err)
		}
		ed, ok := d.(protoreflect.EnumDescriptor)
		if !ok {
			return nil, fmt.Errorf("descriptor %q is a %T, want an enum descriptor", fn, d)
		}

		vals := ed.Values()
		values := make([]string, 0, vals.Len())
		for i := 0; i < vals.Len(); i++ {
			values = append(values, string(vals.Get(i).Name()))
		}

		out = append(out, enumInfo{
			FullName: fn,
			Package:  ed.ParentFile().Package(),
			Values:   values,
		})
	}
	return out, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
cd ~/src/pulumi-tsb
go test ./provider/cmd/gen-resources-tsb/ -v
```

Expected: PASS. If the value lists differ from the assertions, upstream changed those enums — verify against the descriptor before editing the test.

- [ ] **Step 5: Commit**

```bash
cd ~/src/pulumi-tsb
git add provider/cmd/gen-resources-tsb/enums.go provider/cmd/gen-resources-tsb/enums_test.go
git commit -S -s -m "feat(gen): resolve enum members from the protobuf registry

Member names come from the descriptor, so nothing parses validator
diagnostics or description text. A registry miss is an error."
```

---

### Task 4: Walk resource schemas for enum leaves

**Files:**
- Create: `provider/cmd/gen-resources-tsb/walk.go`
- Test: `provider/cmd/gen-resources-tsb/walk_test.go`

**Interfaces:**
- Consumes: `enumSite` (Task 2).
- Produces: `func walkSchema(resourceType string, s schema.Schema) ([]enumSite, error)`, returning sites sorted by dotted path.

- [ ] **Step 1: Write the failing test**

Create `provider/cmd/gen-resources-tsb/walk_test.go`:

```go
// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package main

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	prototypes "github.com/tetrateio/tetrate/api/protoc-plugins/protoc-gen-terraform/pkg/types/basetypes"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func enumAttr(name protoreflect.FullName) schema.StringAttribute {
	return schema.StringAttribute{CustomType: prototypes.EnumType{FullName: name}}
}

func TestWalkSchemaFindsEnumLeaves(t *testing.T) {
	s := schema.Schema{Attributes: map[string]schema.Attribute{
		"name":  schema.StringAttribute{},
		"count": schema.Int64Attribute{},
		"tags":  schema.ListAttribute{ElementType: types.StringType},
		"mode":  enumAttr("test.Mode"),
		"modes": schema.ListAttribute{
			ElementType: prototypes.EnumType{FullName: "test.Mode"},
		},
		"labels": schema.MapAttribute{
			ElementType: prototypes.EnumType{FullName: "test.Label"},
		},
		"spec": schema.SingleNestedAttribute{Attributes: map[string]schema.Attribute{
			"tls": schema.SingleNestedAttribute{Attributes: map[string]schema.Attribute{
				"mode": enumAttr("test.TLSMode"),
			}},
		}},
		"rules": schema.ListNestedAttribute{
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{"action": enumAttr("test.Action")},
			},
		},
	}}

	got, err := walkSchema("tsb_thing", s)
	if err != nil {
		t.Fatalf("walkSchema returned error: %v", err)
	}

	want := []enumSite{
		{Resource: "tsb_thing", Path: []string{"labels"}, Element: true, Enum: "test.Label"},
		{Resource: "tsb_thing", Path: []string{"mode"}, Enum: "test.Mode"},
		{Resource: "tsb_thing", Path: []string{"modes"}, Element: true, Enum: "test.Mode"},
		{Resource: "tsb_thing", Path: []string{"rules", "action"}, Enum: "test.Action"},
		{Resource: "tsb_thing", Path: []string{"spec", "tls", "mode"}, Enum: "test.TLSMode"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("walkSchema =\n%#v\nwant\n%#v", got, want)
	}
}

// An unrecognised attribute kind must stop the build. Skipping it would emit a
// plain string where an enum belongs, which looks exactly like success.
func TestWalkSchemaRejectsUnknownAttributeKinds(t *testing.T) {
	s := schema.Schema{Attributes: map[string]schema.Attribute{
		"opaque": schema.ObjectAttribute{AttributeTypes: map[string]attr.Type{}},
	}}
	if _, err := walkSchema("tsb_thing", s); err == nil {
		t.Error("walkSchema(ObjectAttribute) = nil error, want an error")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd ~/src/pulumi-tsb
go test ./provider/cmd/gen-resources-tsb/ -run TestWalkSchema -v
```

Expected: FAIL — `undefined: walkSchema`.

- [ ] **Step 3: Implement the walk**

Create `provider/cmd/gen-resources-tsb/walk.go`:

```go
// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	prototypes "github.com/tetrateio/tetrate/api/protoc-plugins/protoc-gen-terraform/pkg/types/basetypes"
)

// walkSchema returns every enum-typed leaf reachable from a resource schema,
// ordered by dotted attribute path so generated output is deterministic.
func walkSchema(resourceType string, s schema.Schema) ([]enumSite, error) {
	var sites []enumSite
	if err := walkAttributes(resourceType, nil, s.Attributes, &sites); err != nil {
		return nil, err
	}
	sort.Slice(sites, func(i, j int) bool {
		return pathKey(sites[i].Path) < pathKey(sites[j].Path)
	})
	return sites, nil
}

// pathKey renders an attribute path for sorting and diagnostics.
func pathKey(p []string) string { return strings.Join(p, ".") }

func walkAttributes(res string, prefix []string, attrs map[string]schema.Attribute, out *[]enumSite) error {
	names := make([]string, 0, len(attrs))
	for n := range attrs {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		// Copy rather than append in place: sibling branches must not share
		// backing storage with this path.
		path := append(append([]string{}, prefix...), name)
		if err := walkAttribute(res, path, attrs[name], out); err != nil {
			return err
		}
	}
	return nil
}

// walkAttribute descends one attribute. The kinds handled here are exactly the
// kinds protoc-gen-terraform emits; anything else is an error rather than a
// skip, so a future generator change surfaces as a build failure instead of
// silently missing enums.
func walkAttribute(res string, path []string, a schema.Attribute, out *[]enumSite) error {
	switch t := a.(type) {
	case schema.StringAttribute:
		if et, ok := t.GetType().(prototypes.EnumType); ok {
			*out = append(*out, enumSite{Resource: res, Path: path, Enum: et.FullName})
		}
		return nil

	case schema.BoolAttribute, schema.Int64Attribute, schema.Float64Attribute:
		return nil

	case schema.ListAttribute:
		appendElementSite(res, path, t.ElementType, out)
		return nil

	case schema.MapAttribute:
		appendElementSite(res, path, t.ElementType, out)
		return nil

	case schema.SingleNestedAttribute:
		return walkAttributes(res, path, t.Attributes, out)

	case schema.ListNestedAttribute:
		return walkAttributes(res, path, t.NestedObject.Attributes, out)

	case schema.MapNestedAttribute:
		return walkAttributes(res, path, t.NestedObject.Attributes, out)

	default:
		return fmt.Errorf(
			"%s: attribute %q has unhandled kind %T; teach walkAttribute this kind rather than skipping it",
			res, pathKey(path), a)
	}
}

// appendElementSite records a collection whose ELEMENT is an enum. The token
// attaches one level deeper than a scalar attribute's, hence Element.
func appendElementSite(res string, path []string, et attr.Type, out *[]enumSite) {
	if e, ok := et.(prototypes.EnumType); ok {
		*out = append(*out, enumSite{Resource: res, Path: path, Element: true, Enum: e.FullName})
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
cd ~/src/pulumi-tsb
go test ./provider/cmd/gen-resources-tsb/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd ~/src/pulumi-tsb
git add provider/cmd/gen-resources-tsb/walk.go provider/cmd/gen-resources-tsb/walk_test.go
git commit -S -s -m "feat(gen): walk resource schemas for enum leaves

Handles exactly the attribute kinds protoc-gen-terraform emits; any
other kind is a build error, because skipping one would emit a plain
string where an enum belongs."
```

---

### Task 5: Emit the enum map and nested `SchemaInfo` tree

**Files:**
- Modify: `provider/cmd/gen-resources-tsb/main.go:52-70` (move `generateSource` out)
- Create: `provider/cmd/gen-resources-tsb/emit.go`
- Test: `provider/cmd/gen-resources-tsb/emit_test.go`

**Interfaces:**
- Consumes: `enumSite`, `enumInfo` (Task 2), `enumToken`, `enumMemberName` (Task 2).
- Produces: `func generateSource(types []string, groups map[string]string, sites []enumSite, enums []enumInfo) ([]byte, error)`, replacing the two-argument version in `main.go`.

- [ ] **Step 1: Write the failing test**

Create `provider/cmd/gen-resources-tsb/emit_test.go`:

```go
// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package main

import (
	"strings"
	"testing"
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
```

Add `"unicode"` to the test file's imports alongside `"strings"` and `"testing"`.

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd ~/src/pulumi-tsb
go test ./provider/cmd/gen-resources-tsb/ -run TestGenerateSource -v
```

Expected: FAIL — `too many arguments in call to generateSource`.

- [ ] **Step 3: Move and extend the emitter**

Delete `generateSource` from `main.go` (lines 52-70, the function and its doc comment), leaving the
`discoverResourceTypes` function and `main` in place. Create
`provider/cmd/gen-resources-tsb/emit.go`:

```go
// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package main

import (
	"fmt"
	"go/format"
	"sort"
	"strconv"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// fieldNode merges many enum sites into one nested SchemaInfo literal per
// resource. Non-leaf nodes carry children; leaves carry a token.
type fieldNode struct {
	children map[string]*fieldNode
	token    string // non-empty at an enum leaf
	element  bool   // leaf is a collection element, so the token nests one deeper
}

func newFieldNode() *fieldNode {
	return &fieldNode{children: map[string]*fieldNode{}}
}

// buildFieldTrees folds the flat site list into one tree per resource.
func buildFieldTrees(sites []enumSite, tokens map[protoreflect.FullName]string) (map[string]*fieldNode, error) {
	roots := map[string]*fieldNode{}
	for _, s := range sites {
		tok, ok := tokens[s.Enum]
		if !ok {
			return nil, fmt.Errorf("no Pulumi token was computed for enum %q", s.Enum)
		}
		root, ok := roots[s.Resource]
		if !ok {
			root = newFieldNode()
			roots[s.Resource] = root
		}
		n := root
		for _, seg := range s.Path {
			child, ok := n.children[seg]
			if !ok {
				child = newFieldNode()
				n.children[seg] = child
			}
			n = child
		}
		n.token, n.element = tok, s.Element
	}
	return roots, nil
}

// renderFields writes a map[string]*tfbridge.SchemaInfo literal. Indentation is
// left to format.Source.
func renderFields(b *strings.Builder, children map[string]*fieldNode) {
	b.WriteString("map[string]*tfbridge.SchemaInfo{\n")
	names := make([]string, 0, len(children))
	for n := range children {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(b, "%q: ", name)
		renderNode(b, children[name])
		b.WriteString(",\n")
	}
	b.WriteString("}")
}

// renderNode writes a single *tfbridge.SchemaInfo literal. Every nesting level
// costs one .Elem, matching how tfgen descends (pkg/tfgen/generate.go:519,541).
func renderNode(b *strings.Builder, n *fieldNode) {
	switch {
	case n.token != "" && n.element:
		fmt.Fprintf(b, "{Elem: &tfbridge.SchemaInfo{Type: %q}}", n.token)
	case n.token != "":
		fmt.Fprintf(b, "{Type: %q}", n.token)
	default:
		b.WriteString("{Elem: &tfbridge.SchemaInfo{Fields: ")
		renderFields(b, n.children)
		b.WriteString("}}")
	}
}

// generateSource renders provider/resources_gen.go: the resource token map with
// enum tokens threaded in, plus the enum type definitions fed to
// ProviderInfo.ExtraTypes.
//
// groups maps each resource's type suffix (without the "tsb_" prefix) to its
// TSB API group, per terraform-provider-tsb's pkg/provider.Groups(); a missing
// entry and an explicit "" both fall back to the tsbMod identifier, so
// hand-written resources (e.g. "access_binding", absent from Groups()) and real
// core resources (group "") are handled identically with no special-casing.
func generateSource(
	types []string,
	groups map[string]string,
	sites []enumSite,
	enums []enumInfo,
) ([]byte, error) {
	tokens := make(map[protoreflect.FullName]string, len(enums))
	byToken := make(map[string]enumInfo, len(enums))
	for _, e := range enums {
		tok, err := enumToken(e)
		if err != nil {
			return nil, err
		}
		if prev, clash := byToken[tok]; clash {
			return nil, fmt.Errorf(
				"enums %q and %q both derive Pulumi token %q", prev.FullName, e.FullName, tok)
		}
		tokens[e.FullName] = tok
		byToken[tok] = e
	}

	trees, err := buildFieldTrees(sites, tokens)
	if err != nil {
		return nil, err
	}

	var b strings.Builder
	b.WriteString("// Code generated by gen-resources-tsb. DO NOT EDIT.\n\n")
	b.WriteString("package tsb\n\n")
	b.WriteString("import (\n")
	b.WriteString("\tpschema \"github.com/pulumi/pulumi/pkg/v3/codegen/schema\"\n")
	b.WriteString("\t\"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge\"\n")
	b.WriteString(")\n\n")

	b.WriteString("var generatedResources = map[string]*tfbridge.ResourceInfo{\n")
	for _, t := range types {
		suffix := strings.TrimPrefix(t, providerTypeName+"_")
		mod := "tsbMod"
		if g := groups[suffix]; g != "" {
			mod = strconv.Quote(g)
		}
		fmt.Fprintf(&b, "%q: {Tok: tsbResourceTok(%s, %q)", t, mod, pascalCase(suffix))
		if tree, ok := trees[t]; ok {
			b.WriteString(", Fields: ")
			renderFields(&b, tree.children)
		}
		b.WriteString("},\n")
	}
	b.WriteString("}\n\n")

	b.WriteString("var generatedEnums = map[string]pschema.ComplexTypeSpec{\n")
	tokenNames := make([]string, 0, len(byToken))
	for tok := range byToken {
		tokenNames = append(tokenNames, tok)
	}
	sort.Strings(tokenNames)
	for _, tok := range tokenNames {
		fmt.Fprintf(&b, "%q: {\n", tok)
		b.WriteString("ObjectTypeSpec: pschema.ObjectTypeSpec{Type: \"string\"},\n")
		b.WriteString("Enum: []pschema.EnumValueSpec{\n")
		// Two proto values mangling to one member name would emit a duplicate
		// TypeScript enum member, so catch it here rather than in tsc.
		seen := map[string]string{}
		for _, v := range byToken[tok].Values {
			name, err := enumMemberName(v)
			if err != nil {
				return nil, fmt.Errorf("enum %q: %w", byToken[tok].FullName, err)
			}
			if prev, clash := seen[name]; clash {
				return nil, fmt.Errorf(
					"enum %q: values %q and %q both yield member name %q",
					byToken[tok].FullName, prev, v, name)
			}
			seen[name] = v
			fmt.Fprintf(&b, "{Name: %q, Value: %q},\n", name, v)
		}
		b.WriteString("},\n},\n")
	}
	b.WriteString("}\n")

	return format.Source([]byte(b.String()))
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
cd ~/src/pulumi-tsb
go test ./provider/cmd/gen-resources-tsb/ -v
```

Expected: PASS. A `format.Source` error in the output means the emitted Go is malformed — read the
error, which names the offending line.

- [ ] **Step 5: Commit**

```bash
cd ~/src/pulumi-tsb
git add provider/cmd/gen-resources-tsb/emit.go \
        provider/cmd/gen-resources-tsb/emit_test.go \
        provider/cmd/gen-resources-tsb/main.go
git commit -S -s -m "feat(gen): emit enum types and nested SchemaInfo trees

Sites fold into one tree per resource so sibling paths merge instead of
producing duplicate map keys. Every nesting level costs one .Elem,
matching how tfgen descends."
```

---

### Task 6: Wire the generator together and regenerate

**Files:**
- Modify: `provider/cmd/gen-resources-tsb/main.go`
- Modify (generated): `provider/resources_gen.go`
- Modify: `provider/resources.go:41-56`
- Modify: `go.mod`

**Interfaces:**
- Consumes: `walkSchema` (Task 4), `resolveEnums` (Task 3), `generateSource` (Task 5).
- Produces: `provider/resources_gen.go` exporting both `generatedResources` and `generatedEnums`, and a `ProviderInfo` that wires `generatedEnums` into `ExtraTypes`.

- [ ] **Step 1: Collect schemas alongside metadata**

In `provider/cmd/gen-resources-tsb/main.go`, replace `discoverResourceTypes` with a version that also
walks each schema. Add `"github.com/hashicorp/terraform-plugin-framework/resource"` (already imported)
and `"google.golang.org/protobuf/reflect/protoreflect"` to the imports:

```go
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
```

- [ ] **Step 2: Resolve enums and pass everything to the emitter**

Replace the body of `main` in `provider/cmd/gen-resources-tsb/main.go`:

```go
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
```

- [ ] **Step 3: Run the generator**

```bash
cd ~/src/pulumi-tsb
go mod tidy
make generate
```

Expected: a summary line on stderr reporting 48 resources plus non-zero enum and site counts.
**Record those two numbers** — Task 7's invariant check uses them.

If it errors with a reserved module name or a foreign proto package, add the entry to `moduleRemap`
in `naming.go` (that is the designed escape hatch) and rerun.

- [ ] **Step 4: Review the generated output**

```bash
cd ~/src/pulumi-tsb
head -40 provider/resources_gen.go
grep -c 'Type: "tsb:' provider/resources_gen.go
gofmt -l provider/resources_gen.go
```

Expected: both maps present, a non-zero token count, and no gofmt complaints.

- [ ] **Step 5: Wire `generatedEnums` into `ProviderInfo`**

In `provider/resources.go`, add `ExtraTypes` and `ContainsEnums` to the returned `ProviderInfo`:

```go
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
```

- [ ] **Step 6: Verify it builds and tests pass**

```bash
cd ~/src/pulumi-tsb
go build ./...
go test ./... -v
```

Expected: clean build, tests pass.

- [ ] **Step 7: Commit**

```bash
cd ~/src/pulumi-tsb
git add provider/cmd/gen-resources-tsb/main.go provider/resources_gen.go \
        provider/resources.go go.mod go.sum
git commit -S -s -m "feat: generate real Pulumi enum types

gen-resources-tsb now walks each resource schema for enum leaves,
resolves their members from the protobuf registry, and emits both the
enum type definitions and the nested SchemaInfo trees that point
attributes at them."
```

---

### Task 7: Regenerate the schema and guard the invariant

**Files:**
- Create: `hack/check-enums.sh`
- Modify: `Makefile`
- Modify (generated): `provider/cmd/pulumi-resource-tsb/schema.json`

**Interfaces:**
- Consumes: `generatedEnums` wired into `ProviderInfo` (Task 6).
- Produces: a `make check-enums` target that fails if enum refs go missing, wired into `make check`.

- [ ] **Step 1: Regenerate the Pulumi schema**

```bash
cd ~/src/pulumi-tsb
make schema
```

- [ ] **Step 2: Confirm enums actually landed**

This is the load-bearing check. A `Fields`/`Elem` tree that fails to match the shim structure produces
**no error** — the tokens are simply never applied — so the count is the only signal:

```bash
cd ~/src/pulumi-tsb
python3 -c "
import json
s = json.load(open('provider/cmd/pulumi-resource-tsb/schema.json'))
enums = [k for k, v in s['types'].items() if 'enum' in v]
print('enum types:', len(enums))
print('sample:', sorted(enums)[:5])
"
```

Expected: a count matching the enum total from Task 6 Step 3, and tokens like
`tsb:auth/TLSMode:TLSMode`. **A count of 0 means the tokens never applied — stop and debug the
`Fields` tree before continuing.**

- [ ] **Step 3: Write the invariant check**

Create `hack/check-enums.sh`:

```bash
#!/usr/bin/env bash

# Copyright (c) Tetrate, Inc 2026 All Rights Reserved.
# Verifies that the generated Pulumi schema still carries the enum types the
# resource generator emitted. A Fields/Elem tree that stops matching the bridge
# shim silently drops every token without erroring, so the count is the only
# signal that anything broke.
set -euo pipefail

cd "$(dirname "$0")/.."

# One ObjectTypeSpec line is emitted per enum, and unlike the map keys it is
# not subject to gofmt's column alignment, so it counts reliably.
EXPECTED="$(grep -c 'ObjectTypeSpec: pschema.ObjectTypeSpec{Type: "string"}' provider/resources_gen.go)"
ACTUAL="$(python3 -c "
import json
s = json.load(open('provider/cmd/pulumi-resource-tsb/schema.json'))
print(sum(1 for v in s['types'].values() if 'enum' in v))
")"

if [ "$EXPECTED" -eq 0 ]; then
  echo "check-enums: resources_gen.go declares no enums; did generation run?" >&2
  exit 1
fi

if [ "$ACTUAL" != "$EXPECTED" ]; then
  echo "check-enums: schema.json has $ACTUAL enum types, expected $EXPECTED" >&2
  echo "check-enums: the SchemaInfo Fields/Elem tree likely stopped matching the bridge shim" >&2
  exit 1
fi

echo "check-enums: $ACTUAL enum types present"
```

```bash
chmod +x hack/check-enums.sh
```

- [ ] **Step 4: Verify the check passes, and that it can fail**

```bash
cd ~/src/pulumi-tsb
./hack/check-enums.sh
```

Expected: `check-enums: <N> enum types present`.

Prove it detects a regression:

```bash
cd ~/src/pulumi-tsb
python3 - <<'EOF'
import json
p = 'provider/cmd/pulumi-resource-tsb/schema.json'
s = json.load(open(p))
k = next(k for k, v in s['types'].items() if 'enum' in v)
del s['types'][k]
json.dump(s, open(p, 'w'), indent=2)
EOF
./hack/check-enums.sh || echo "correctly detected the regression"
make schema   # restore
./hack/check-enums.sh
```

Expected: the tampered run fails, then the restored run passes.

- [ ] **Step 5: Wire the checks into the Makefile**

In `Makefile`, add a typecheck target and extend `check`:

```make
# typechecks the generated TypeScript SDK; strict enums make this load-bearing,
# since a bad enum token surfaces as a compile error rather than a bad value
sdk.typecheck: sdk.nodejs
	npm install --silent
	npx tsc --noEmit

# depends on schema so the comparison is against a freshly generated schema.json
# rather than a stale one, which would fail with a confusing count mismatch
check-enums: schema
	@./hack/check-enums.sh

check: generate licenser check-enums
	[ -z "`git status -uno --porcelain`" ] || (git status && echo 'Check failed. This could be a failed check or dirty git state.'; exit 1)
```

Note `check` gains `check-enums` before the dirty-tree assertion, so a dropped enum fails loudly
rather than showing up as an unexplained diff.

- [ ] **Step 6: Regenerate the SDK and typecheck it**

```bash
cd ~/src/pulumi-tsb
make sdk
make sdk.typecheck
```

Expected: the SDK regenerates and compiles. Confirm enums reached TypeScript:

```bash
ls sdk/types/enums 2>/dev/null || find sdk -name '*.ts' | xargs grep -l 'TLSMode' | head
```

- [ ] **Step 7: Run the full check**

```bash
cd ~/src/pulumi-tsb
make check
```

Expected: passes with a clean tree.

- [ ] **Step 8: Commit**

```bash
cd ~/src/pulumi-tsb
git add hack/check-enums.sh Makefile provider/cmd/pulumi-resource-tsb/schema.json sdk package.json
git commit -S -s -m "feat: regenerate schema and SDK with enum types, guard the invariant

A SchemaInfo tree that stops matching the bridge shim drops every enum
token without erroring, so check-enums compares the schema's enum count
against what the generator emitted. Also typechecks the SDK, which
strict enums make load-bearing."
```

- [ ] **Step 9: Confirm before pushing**

Ask the user before pushing. Single remote (`origin` = `tetratelabs/pulumi-tsb`).

---

## Follow-up, not in scope

- The SDK README should note that a server newer than the compiled descriptors can return an enum value protobuf-go renders as a decimal string (e.g. `"7"`), which is outside the declared enum type. TypeScript enums erase to strings at runtime, so this is a type-level inaccuracy rather than a crash. See "Accepted consequences" in the spec.
- This is a **breaking SDK change**: string literals stop type-checking on enum properties. Release notes should say so, and the version bump should reflect it.
