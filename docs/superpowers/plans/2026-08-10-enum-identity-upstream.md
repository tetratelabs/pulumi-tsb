# Enum Identity Upstream (Phase 1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `protoc-gen-terraform`'s `EnumType` carry the proto full name of the enum it came from, so downstream consumers can recover enum identity instead of reconstructing it.

**Architecture:** `EnumType` already exists purely as a decorator marking proto origin; it gains a `FullName` field and an `EnumTypeFor` constructor. The generator emits `EnumTypeFor("…")` at the two places it currently emits the bare `EnumType` var — the singular-enum `CustomType` and the list/map `ElementType`. Then the API module is regenerated.

**Tech Stack:** Go, `google.golang.org/protobuf` (protogen/protoreflect), `github.com/dave/jennifer/jen` for code emission, `buf` v1.53.0 for generation, `testify/require` for tests.

**Repo:** `tetrateio/tetrate`, at `~/src/tetrate`. All paths below are relative to that repo root.

## Global Constraints

- Every new or modified file keeps the existing header: `// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.`
- Commits are signed and carry a sign-off trailer: `git commit -S -s`. Never add `Co-Authored-By` or AI attribution.
- Work on a branch. Nothing is pushed without explicit confirmation.
- `EnumType.Equal` must stay blind to `FullName`. The Terraform framework compares attribute types for wire compatibility, and changing this would break existing consumers.
- The `protoreflect.EnumKind` entry in `primitiveSchemaTypeMap` must **not** be deleted. `pkg/generate/schema.go:124` uses that map as a membership test to decide `MapAttribute` vs `MapNestedAttribute`; removing the entry turns every `map<string, SomeEnum>` into a nested object.
- Generated `*.pb.terraform.go` files are committed. Regeneration output is reviewed, not blindly accepted.

---

### Task 1: `EnumType` carries proto identity

**Files:**
- Modify: `api/protoc-plugins/protoc-gen-terraform/pkg/types/basetypes/enum_type.go`
- Modify: `api/protoc-plugins/protoc-gen-terraform/pkg/types/enum.go`
- Test: `api/protoc-plugins/protoc-gen-terraform/pkg/types/basetypes/enum_type_test.go` (create)

**Interfaces:**
- Consumes: nothing.
- Produces: `basetypes.EnumType{ StringType; FullName protoreflect.FullName }` and
  `types.EnumTypeFor(name protoreflect.FullName) basetypes.EnumType`. Tasks 2 and 3 emit calls to
  `EnumTypeFor`; Phase 2 type-asserts against `basetypes.EnumType` and reads `.FullName`.

- [ ] **Step 1: Write the failing test**

Create `api/protoc-plugins/protoc-gen-terraform/pkg/types/basetypes/enum_type_test.go`:

```go
// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package basetypes

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Equal must stay blind to FullName: the Terraform framework compares attribute
// types for wire compatibility, and two enums are as wire-compatible as two
// strings. Downstream consumers read FullName directly instead.
func TestEnumTypeEqualIgnoresFullName(t *testing.T) {
	a := EnumType{FullName: "test.Mode"}
	b := EnumType{FullName: "other.Mode"}

	if !a.Equal(b) {
		t.Error("Equal(EnumType with different FullName) = false, want true")
	}
	if !a.Equal(basetypes.StringType{}) {
		t.Error("Equal(StringType) = false, want true")
	}
}

func TestEnumTypeStringNamesTheEnum(t *testing.T) {
	got := EnumType{FullName: "test.Mode"}.String()
	want := "prototypes.EnumType(test.Mode)"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd ~/src/tetrate/api/protoc-plugins/protoc-gen-terraform
go test ./pkg/types/... -run TestEnumType -v
```

Expected: FAIL — `unknown field FullName in struct literal`.

- [ ] **Step 3: Add the field**

Replace the body of `pkg/types/basetypes/enum_type.go` below the header with:

```go
package basetypes

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var (
	_ basetypes.StringTypable = EnumType{}
)

// EnumType is a Terraform attribute type that decorates StringType so generated
// schemas can carry the fact that a string field was originally a proto enum —
// and which enum it was.
//
// FullName is the proto full name of the originating enum (e.g.
// "tetrateio.api.tsb.auth.v2.TLSMode"). Consumers projecting this schema into
// another type system need the identity, not just the fact of enum-ness: two
// enums in different proto packages can share a leaf name while declaring
// different values (tsb.auth.v2.TLSMode has four, tsb.profile.v2.TLSMode has
// three).
type EnumType struct {
	basetypes.StringType

	FullName protoreflect.FullName
}

// String overrides StringType.String so schema dumps surface this as an
// enum-flavoured type rather than a plain string, and name the enum.
func (e EnumType) String() string {
	return "prototypes.EnumType(" + string(e.FullName) + ")"
}

// Equal treats EnumType and basetypes.StringType as interchangeable: an enum
// is functionally a string with a decorator marking its proto origin. It stays
// deliberately blind to FullName — the framework compares attribute types for
// wire compatibility, and two enums are as wire-compatible as two strings.
func (e EnumType) Equal(o attr.Type) bool {
	_, isEnum := o.(EnumType)
	_, isString := o.(basetypes.StringType)
	return isEnum || isString
}
```

- [ ] **Step 4: Add the constructor**

Replace the body of `pkg/types/enum.go` below the header with:

```go
package types

import (
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/tetrateio/tetrate/api/protoc-plugins/protoc-gen-terraform/pkg/types/basetypes"
)

// EnumType is the zero value of the enum-flavoured Terraform attribute type.
// Generated schemas call EnumTypeFor instead so the attribute names its enum;
// this var is retained for hand-written consumers that only need the marker.
var EnumType = basetypes.EnumType{}

// EnumTypeFor returns the enum-flavoured Terraform attribute type for the proto
// enum named name. It is referenced by generated schemas wherever a proto field
// or collection element was originally an enum.
func EnumTypeFor(name protoreflect.FullName) basetypes.EnumType {
	return basetypes.EnumType{FullName: name}
}
```

- [ ] **Step 5: Run the tests to verify they pass**

```bash
cd ~/src/tetrate/api/protoc-plugins/protoc-gen-terraform
go test ./pkg/... -v
go build ./...
```

Expected: PASS, and a clean build.

- [ ] **Step 6: Commit**

```bash
cd ~/src/tetrate
git add api/protoc-plugins/protoc-gen-terraform/pkg/types
git commit -S -s -m "feat(protoc-gen-terraform): EnumType carries proto full name

EnumType exists to mark that a string attribute was originally a proto
enum, but not which one. Downstream consumers projecting these schemas
into another type system cannot recover the identity: two enums in
different packages can share a leaf name and declare different values.

Add FullName plus an EnumTypeFor constructor. Equal stays blind to the
new field, so this is behaviour-preserving for existing consumers."
```

---

### Task 2: Singular enum attributes emit `EnumTypeFor`

**Files:**
- Modify: `api/protoc-plugins/protoc-gen-terraform/pkg/generate/enum.go:51-71`
- Test: `api/protoc-plugins/protoc-gen-terraform/test/schema_test.go` (append)

**Interfaces:**
- Consumes: `types.EnumTypeFor` from Task 1.
- Produces: generated singular enum attributes whose `CustomType` is
  `types.EnumTypeFor("<proto full name>")`. Task 4 relies on this for the API-wide regeneration.

- [ ] **Step 1: Write the failing test**

Append to `api/protoc-plugins/protoc-gen-terraform/test/schema_test.go`. Add these two imports to the
existing import block:

```go
	"google.golang.org/protobuf/reflect/protoreflect"

	prototypes "github.com/tetrateio/tetrate/api/protoc-plugins/protoc-gen-terraform/pkg/types/basetypes"
```

Then append:

```go
// enumTypeOf asserts that a's Terraform type is an EnumType and returns it.
func enumTypeOf(t *testing.T, a schema.Attribute) prototypes.EnumType {
	t.Helper()
	et, ok := a.GetType().(prototypes.EnumType)
	require.True(t, ok, "attribute type is %T, want prototypes.EnumType", a.GetType())
	return et
}

func TestSingularEnumIdentity(t *testing.T) {
	s := TestMessageSchema()

	cases := map[string]protoreflect.FullName{
		"enum":          "test.Mode",
		"tagged_enum":   "test.Mode",
		"inline_enum":   "test.TestMessage.InlineMode",
		"imported_enum": "google.protobuf.Syntax",
	}
	for attrName, want := range cases {
		require.Equal(t, want, enumTypeOf(t, s.Attributes[attrName]).FullName,
			"attribute %q", attrName)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd ~/src/tetrate/api/protoc-plugins/protoc-gen-terraform
make test
```

Expected: FAIL — `TestSingularEnumIdentity` reports `FullName` is `""`, because the regenerated
schema still emits the bare `types1.EnumType` var.

- [ ] **Step 3: Thread the descriptor into `applyEnumDefault`**

In `pkg/generate/enum.go`, change the call site inside `applyEnum` from:

```go
	if f.Desc.Kind() == protoreflect.EnumKind && !f.Desc.IsList() {
		applyEnumDefault(d, f, pkg)
	}
```

to:

```go
	if f.Desc.Kind() == protoreflect.EnumKind && !f.Desc.IsList() {
		applyEnumDefault(d, f, pkg, ed)
	}
```

Change the signature and the `CustomType` line of `applyEnumDefault`:

```go
// applyEnumDefault emits the Default and CustomType entries for a singular enum
// field. Enums default to the 0 int value, unless overridden via comments. ed is
// the field's enum descriptor, used to name the enum in CustomType.
func applyEnumDefault(d j.Dict, f *protogen.Field, pkg string, ed protoreflect.EnumDescriptor) {
```

and, at the end of that function, replace:

```go
	// We also need to label it using a custom type.
	d[j.Id("CustomType")] = j.Qual(ProtoTypes, "EnumType")
```

with:

```go
	// We also need to label it using a custom type that names the originating
	// enum, so consumers projecting this schema elsewhere can recover identity.
	d[j.Id("CustomType")] = j.Qual(ProtoTypes, "EnumTypeFor").
		Call(j.Lit(string(ed.FullName())))
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
cd ~/src/tetrate/api/protoc-plugins/protoc-gen-terraform
make test
```

Expected: PASS. Confirm the regenerated fixture changed as intended:

```bash
grep -n 'CustomType' test/primary.pb.terraform.go | head -5
```

Expected output includes `CustomType:  types1.EnumTypeFor("test.Mode"),`.

- [ ] **Step 5: Commit**

```bash
cd ~/src/tetrate
git add api/protoc-plugins/protoc-gen-terraform/pkg/generate/enum.go \
        api/protoc-plugins/protoc-gen-terraform/test
git commit -S -s -m "feat(protoc-gen-terraform): name the enum in singular CustomType

Singular enum attributes now emit EnumTypeFor(\"<full name>\") rather
than the identity-free EnumType var."
```

---

### Task 3: List and map enum elements emit `EnumTypeFor`

**Files:**
- Modify: `api/protoc-plugins/protoc-gen-terraform/pkg/generate/schema.go:136`, `:144`, `:202-213`
- Test: `api/protoc-plugins/protoc-gen-terraform/test/schema_test.go` (append)

**Interfaces:**
- Consumes: `types.EnumTypeFor` (Task 1), `enumDescriptor(f)` (already in `pkg/generate/enum.go`), and the
  `protoreflect` / `prototypes` imports plus the `enumTypeOf` helper that Task 2 added to
  `test/schema_test.go`. Task 2 must land first.
- Produces: `elementSchemaType(f *protogen.Field, k protoreflect.Kind) *j.Statement`, and generated
  `ListAttribute`/`MapAttribute` whose `ElementType` names its enum.

- [ ] **Step 1: Write the failing test**

Append to `api/protoc-plugins/protoc-gen-terraform/test/schema_test.go`:

```go
func TestCollectionEnumIdentity(t *testing.T) {
	s := TestMessageSchema()

	listCases := map[string]protoreflect.FullName{
		"enum_list":          "test.Mode",
		"inline_enum_list":   "test.TestMessage.InlineMode",
		"imported_enum_list": "google.protobuf.Syntax",
	}
	for attrName, want := range listCases {
		a, ok := s.Attributes[attrName].(schema.ListAttribute)
		require.True(t, ok, "attribute %q is %T, want ListAttribute", attrName, s.Attributes[attrName])
		et, ok := a.ElementType.(prototypes.EnumType)
		require.True(t, ok, "attribute %q element is %T, want EnumType", attrName, a.ElementType)
		require.Equal(t, want, et.FullName, "attribute %q", attrName)
	}

	mapCases := map[string]protoreflect.FullName{
		"enum_map":          "test.Mode",
		"inline_enum_map":   "test.TestMessage.InlineMode",
		"imported_enum_map": "google.protobuf.Syntax",
	}
	for attrName, want := range mapCases {
		a, ok := s.Attributes[attrName].(schema.MapAttribute)
		require.True(t, ok, "attribute %q is %T, want MapAttribute", attrName, s.Attributes[attrName])
		et, ok := a.ElementType.(prototypes.EnumType)
		require.True(t, ok, "attribute %q element is %T, want EnumType", attrName, a.ElementType)
		require.Equal(t, want, et.FullName, "attribute %q", attrName)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd ~/src/tetrate/api/protoc-plugins/protoc-gen-terraform
make test
```

Expected: FAIL — element `FullName` is `""` for all six attributes.

- [ ] **Step 3: Add the field-aware element type helper**

In `pkg/generate/schema.go`, add immediately above `var primitiveSchemaTypeMap`:

```go
// elementSchemaType returns the ElementType entry for a list or map element of
// kind k on field f.
//
// Enums cannot be served from primitiveSchemaTypeMap: that table is keyed by
// kind alone and so has no way to name the specific enum. Every other kind
// comes straight from the table.
func elementSchemaType(f *protogen.Field, k protoreflect.Kind) *j.Statement {
	if k == protoreflect.EnumKind {
		return j.Qual(ProtoTypes, "EnumTypeFor").
			Call(j.Lit(string(enumDescriptor(f).FullName())))
	}
	return primitiveSchemaTypeMap[k]
}
```

- [ ] **Step 4: Route both element lookups through the helper**

At `pkg/generate/schema.go:136`, change:

```go
					s[j.Id("ElementType")] = primitiveSchemaTypeMap[f.Desc.MapValue().Kind()]
```

to:

```go
					s[j.Id("ElementType")] = elementSchemaType(f, f.Desc.MapValue().Kind())
```

At `pkg/generate/schema.go:144`, change:

```go
			s[j.Id("ElementType")] = primitiveSchemaTypeMap[f.Desc.Kind()]
```

to:

```go
			s[j.Id("ElementType")] = elementSchemaType(f, f.Desc.Kind())
```

Then document the trap on the map itself, replacing the `var primitiveSchemaTypeMap` declaration line
with:

```go
// primitiveSchemaTypeMap maps a proto kind to the Terraform type used for a
// collection element of that kind.
//
// The EnumKind entry's *value* is dead — enum elements are resolved by
// elementSchemaType, which can name the enum. The entry itself must stay: the
// map-field branch above uses this map as a membership test to decide
// MapAttribute vs MapNestedAttribute, so deleting it would turn every
// map<string, SomeEnum> into a nested object.
var primitiveSchemaTypeMap = map[protoreflect.Kind]*j.Statement{
```

- [ ] **Step 5: Run the tests to verify they pass**

```bash
cd ~/src/tetrate/api/protoc-plugins/protoc-gen-terraform
make test
```

Expected: PASS, including the earlier `TestSchema` map/list subtests (which assert
`map<string, SomeEnum>` is still a `MapAttribute`, guarding the membership trap).

Confirm the fixture:

```bash
grep -n 'ElementType' test/primary.pb.terraform.go | head -6
```

Expected output includes `ElementType: types1.EnumTypeFor("test.Mode"),`.

- [ ] **Step 6: Commit**

```bash
cd ~/src/tetrate
git add api/protoc-plugins/protoc-gen-terraform/pkg/generate/schema.go \
        api/protoc-plugins/protoc-gen-terraform/test
git commit -S -s -m "feat(protoc-gen-terraform): name the enum in collection ElementType

Repeated and map-valued enum elements now emit EnumTypeFor(\"<full
name>\"). The kind-keyed primitiveSchemaTypeMap cannot express this
because it has no field context, so enums route through a field-aware
helper; the map entry stays because it doubles as a membership test."
```

---

### Task 4: Regenerate the API module

**Files:**
- Modify (generated): `api/**/*.pb.terraform.go` — 96 files

**Interfaces:**
- Consumes: the emitter changes from Tasks 2 and 3.
- Produces: an API module where every enum attribute and element names its enum. This is what
  `terraform-provider-tsb` picks up, and what Phase 2 reads.

- [ ] **Step 1: Regenerate only the Terraform output**

```bash
cd ~/src/tetrate/api
make protoc-plugins/protoc-gen-terraform/bin/protoc-gen-terraform
go run github.com/bufbuild/buf/cmd/buf@v1.53.0 generate --template buf.gen.terraform.yaml
```

Using the terraform template alone (rather than `make gen-protos`) keeps the diff reviewable — no
go-grpc, k8s, or jsonschema churn.

- [ ] **Step 2: Verify the conversion is total**

```bash
cd ~/src/tetrate/api
echo "EnumTypeFor calls: $(grep -rho 'EnumTypeFor(' --include='*.pb.terraform.go' . | wc -l)"
echo "bare EnumType refs: $(grep -rEho '\btypes[0-9]*\.EnumType\b' --include='*.pb.terraform.go' . | wc -l)"
```

Expected: roughly 659 `EnumTypeFor` calls, and **0** bare `EnumType` references. A non-zero second
number means an emission path was missed — do not proceed.

- [ ] **Step 3: Verify the module still builds and tests pass**

```bash
cd ~/src/tetrate/api
go build ./...
go test ./... 2>&1 | tail -20
```

Expected: clean build; tests pass.

- [ ] **Step 4: Spot-check a known collision pair**

```bash
cd ~/src/tetrate/api
grep -rho 'EnumTypeFor("[^"]*TLSMode")' --include='*.pb.terraform.go' . | sort -u
```

Expected: both `tetrateio.api.tsb.auth.v2.TLSMode` and `tetrateio.api.tsb.profile.v2.TLSMode` appear
as distinct strings. This is the case that motivates the whole change — the two enums declare
different values, and leaf-name naming would have merged them.

- [ ] **Step 5: Commit**

```bash
cd ~/src/tetrate
git add api
git commit -S -s -m "chore(api): regenerate terraform schemas with enum identity

Every enum attribute and collection element now names its proto enum.
Generated-only change; no proto or hand-written source touched."
```

- [ ] **Step 6: Confirm before pushing**

Ask the user before pushing. The repo has multiple remotes (`origin` = `tetrateio/tetrate`, plus
personal forks including `chirauki`); confirm which to push to and open the PR from there.

---

## Handoff to Phase 2

Once this merges and `terraform-provider-tsb` bumps `github.com/tetrateio/tetrate` and tags a release,
proceed to `docs/superpowers/plans/2026-08-10-pulumi-enum-types.md` in the `pulumi-tsb` repo.
