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
