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
