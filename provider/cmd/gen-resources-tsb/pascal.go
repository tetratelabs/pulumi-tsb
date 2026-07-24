// Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

package main

import "strings"

// pascalCase converts a snake_case string into PascalCase by upper-casing
// the first letter of each underscore-separated segment and leaving the
// rest of each segment untouched (so acronyms like "api" become "Api",
// not "API" — a deliberate, unambiguous convention rather than an
// acronym exception list).
func pascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}
