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

// pathSegment is one hop from a resource root toward an enum leaf.
//
// Collection marks that this hop passed through a ListNestedAttribute or
// MapNestedAttribute rather than a SingleNestedAttribute. The pf shim
// represents a SingleNestedAttribute directly as Type=Map/Elem=Resource (one
// .Elem hop reaches the nested field map), but it represents a
// List/MapNestedAttribute as List-or-Map/Elem=Schema wrapping that same
// Type=Map/Elem=Resource shape (two .Elem hops). A SchemaInfo tree that
// always emits one hop silently fails tfgen's structural validation on every
// List/MapNestedAttribute ancestor — confirmed by walking the shim schema
// for every resource, where SingleNested always measured one hop and
// List/MapNested always measured two.
type pathSegment struct {
	Name       string
	Collection bool
}

// enumSite is one attribute path in one resource whose leaf is an enum.
type enumSite struct {
	// Resource is the Terraform type name, e.g. "tsb_cluster".
	Resource string
	// Path is the attribute names from the resource root to the enum leaf,
	// each carrying whether it passed through a List/MapNestedAttribute.
	Path []pathSegment
	// Element reports that the leaf is a list or map ELEMENT rather than a
	// scalar attribute, which changes where the token attaches.
	Element bool
	// Enum is the proto full name of the enum at the leaf.
	Enum protoreflect.FullName
}
