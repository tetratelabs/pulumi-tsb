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
