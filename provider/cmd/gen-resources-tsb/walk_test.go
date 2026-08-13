// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
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
		"entries": schema.MapNestedAttribute{
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{"mode": enumAttr("test.EntryMode")},
			},
		},
	}}

	got, err := walkSchema("tsb_thing", s)
	if err != nil {
		t.Fatalf("walkSchema returned error: %v", err)
	}

	want := []enumSite{
		{Resource: "tsb_thing", Path: []pathSegment{{Name: "entries", Collection: true}, {Name: "mode"}}, Enum: "test.EntryMode"},
		{Resource: "tsb_thing", Path: []pathSegment{{Name: "labels"}}, Element: true, Enum: "test.Label"},
		{Resource: "tsb_thing", Path: []pathSegment{{Name: "mode"}}, Enum: "test.Mode"},
		{Resource: "tsb_thing", Path: []pathSegment{{Name: "modes"}}, Element: true, Enum: "test.Mode"},
		{Resource: "tsb_thing", Path: []pathSegment{{Name: "rules", Collection: true}, {Name: "action"}}, Enum: "test.Action"},
		{Resource: "tsb_thing", Path: []pathSegment{{Name: "spec"}, {Name: "tls"}, {Name: "mode"}}, Enum: "test.TLSMode"},
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

// A List/MapAttribute with a nil ElementType and no CustomType gives the walk
// nothing to inspect. If upstream ever emitted a repeated enum this way, the
// site would silently vanish; this must fail loudly instead.
func TestWalkSchemaRejectsNilElementType(t *testing.T) {
	s := schema.Schema{Attributes: map[string]schema.Attribute{
		"tags": schema.ListAttribute{},
	}}
	_, err := walkSchema("tsb_thing", s)
	if err == nil {
		t.Fatal("walkSchema(ListAttribute{ElementType: nil}) = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "nil ElementType") {
		t.Errorf("error = %q, want it to mention nil ElementType", err.Error())
	}
}

func TestWalkSchemaRejectsNilElementTypeOnMap(t *testing.T) {
	s := schema.Schema{Attributes: map[string]schema.Attribute{
		"labels": schema.MapAttribute{},
	}}
	_, err := walkSchema("tsb_thing", s)
	if err == nil {
		t.Fatal("walkSchema(MapAttribute{ElementType: nil}) = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "nil ElementType") {
		t.Errorf("error = %q, want it to mention nil ElementType", err.Error())
	}
}

// A List/MapAttribute carrying a CustomType is a shape this walk has no logic
// to inspect. If upstream ever re-expressed a repeated enum this way (as it
// already does for scalar enum leaves via StringAttribute.CustomType), the
// walk must refuse rather than silently drop the site.
func TestWalkSchemaRejectsUnknownCustomTypeOnList(t *testing.T) {
	s := schema.Schema{Attributes: map[string]schema.Attribute{
		"tags": schema.ListAttribute{
			ElementType: types.StringType,
			CustomType:  fakeListCustomType{},
		},
	}}
	_, err := walkSchema("tsb_thing", s)
	if err == nil {
		t.Fatal("walkSchema(ListAttribute{CustomType: ...}) = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "CustomType") {
		t.Errorf("error = %q, want it to mention CustomType", err.Error())
	}
}

func TestWalkSchemaRejectsUnknownCustomTypeOnMap(t *testing.T) {
	s := schema.Schema{Attributes: map[string]schema.Attribute{
		"labels": schema.MapAttribute{
			ElementType: types.StringType,
			CustomType:  fakeMapCustomType{},
		},
	}}
	_, err := walkSchema("tsb_thing", s)
	if err == nil {
		t.Fatal("walkSchema(MapAttribute{CustomType: ...}) = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "CustomType") {
		t.Errorf("error = %q, want it to mention CustomType", err.Error())
	}
}

// A non-empty Blocks map is a second, parallel home for schema fields that
// walkAttributes never looks at. Upstream emits none today, but silence here
// is not a guarantee, so its mere presence must stop the build.
func TestWalkSchemaRejectsNonEmptyBlocks(t *testing.T) {
	s := schema.Schema{
		Attributes: map[string]schema.Attribute{"name": schema.StringAttribute{}},
		Blocks:     map[string]schema.Block{"nested": schema.ListNestedBlock{}},
	}
	_, err := walkSchema("tsb_thing", s)
	if err == nil {
		t.Fatal("walkSchema(Blocks: non-empty) = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "block") {
		t.Errorf("error = %q, want it to mention blocks", err.Error())
	}
}

// fakeListCustomType and fakeMapCustomType are minimal stand-ins for a
// CustomType this walk has never seen, satisfying just enough of the
// basetypes interfaces to compile into a schema.Attribute literal.
type fakeListCustomType struct {
	basetypes.ListType
}

type fakeMapCustomType struct {
	basetypes.MapType
}
