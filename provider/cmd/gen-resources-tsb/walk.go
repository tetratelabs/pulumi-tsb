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
