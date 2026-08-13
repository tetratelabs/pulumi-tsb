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
