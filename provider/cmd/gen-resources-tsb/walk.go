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
	// Blocks are a second, parallel home for schema-level fields alongside
	// Attributes; walkAttributes never looks at them. Upstream emits none
	// today, but an enum tucked inside a block would vanish from this walk
	// with no diagnostic, so treat any appearance as unsupported rather than
	// silently ignoring it.
	if len(s.Blocks) > 0 {
		names := make([]string, 0, len(s.Blocks))
		for n := range s.Blocks {
			names = append(names, n)
		}
		sort.Strings(names)
		return nil, fmt.Errorf(
			"%s: schema has %d block(s) (%s) that walkSchema does not inspect; teach walkSchema to descend into blocks rather than skipping them",
			resourceType, len(names), strings.Join(names, ", "))
	}

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
func pathKey(p []pathSegment) string {
	names := make([]string, len(p))
	for i, seg := range p {
		names[i] = seg.Name
	}
	return strings.Join(names, ".")
}

func walkAttributes(res string, prefix []pathSegment, attrs map[string]schema.Attribute, out *[]enumSite) error {
	names := make([]string, 0, len(attrs))
	for n := range attrs {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		// Copy rather than append in place: sibling branches must not share
		// backing storage with this path.
		path := append(append([]pathSegment{}, prefix...), pathSegment{Name: name})
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
func walkAttribute(res string, path []pathSegment, a schema.Attribute, out *[]enumSite) error {
	switch t := a.(type) {
	case schema.StringAttribute:
		if et, ok := t.GetType().(prototypes.EnumType); ok {
			*out = append(*out, enumSite{Resource: res, Path: path, Enum: et.FullName})
		}
		return nil

	case schema.BoolAttribute, schema.Int64Attribute, schema.Float64Attribute:
		return nil

	case schema.ListAttribute:
		return appendElementSite(res, path, "ListAttribute", t.ElementType, t.CustomType, out)

	case schema.MapAttribute:
		return appendElementSite(res, path, "MapAttribute", t.ElementType, t.CustomType, out)

	case schema.SingleNestedAttribute:
		return walkAttributes(res, path, t.Attributes, out)

	case schema.ListNestedAttribute:
		// See pathSegment.Collection: the pf shim needs two .Elem hops to
		// reach a List/MapNestedAttribute's nested fields, versus one for a
		// SingleNestedAttribute.
		path[len(path)-1].Collection = true
		return walkAttributes(res, path, t.NestedObject.Attributes, out)

	case schema.MapNestedAttribute:
		path[len(path)-1].Collection = true
		return walkAttributes(res, path, t.NestedObject.Attributes, out)

	default:
		return fmt.Errorf(
			"%s: attribute %q has unhandled kind %T; teach walkAttribute this kind rather than skipping it",
			res, pathKey(path), a)
	}
}

// appendElementSite records a collection whose ELEMENT is an enum. The token
// attaches one level deeper than a scalar attribute's, hence Element.
//
// A List/MapAttribute with plain-string elements is common and legitimate,
// so a non-enum ElementType is not an error. But two shapes mean enum-ness
// could be hiding from us where we cannot see it, and both must fail loudly
// rather than silently degrade to string:
//
//   - ElementType is nil: there is nothing to inspect. The framework requires
//     ElementType to be set unless CustomType supplies an equivalent, so nil
//     here (with no CustomType either) means the attribute is malformed or
//     this walk's assumptions about the shape no longer hold.
//   - CustomType is non-nil: it overrides ElementType for the framework's own
//     purposes, and this walk has no logic to inspect a CustomType. If
//     upstream ever re-expressed a repeated enum as a CustomType (as it
//     already does for scalar enum leaves via StringAttribute.CustomType),
//     this walk would need to be taught that shape rather than ignore it.
func appendElementSite(res string, path []pathSegment, kind string, et attr.Type, customType any, out *[]enumSite) error {
	if customType != nil {
		return fmt.Errorf(
			"%s: %s %q has a CustomType (%T) this walk does not understand; teach appendElementSite this shape rather than ignoring it, since it could be hiding an enum element",
			res, kind, pathKey(path), customType)
	}
	if et == nil {
		return fmt.Errorf(
			"%s: %s %q has a nil ElementType and no CustomType; there is nothing to inspect for enum-ness",
			res, kind, pathKey(path))
	}
	if e, ok := et.(prototypes.EnumType); ok {
		*out = append(*out, enumSite{Resource: res, Path: path, Element: true, Enum: e.FullName})
	}
	return nil
}
