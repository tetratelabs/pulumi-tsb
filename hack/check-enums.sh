#!/usr/bin/env bash

# Copyright (c) Tetrate, Inc 2026 All Rights Reserved.
# Verifies that the generated Pulumi schema actually APPLIES the enum types the
# resource generator declared, not merely that it declared them.
#
# ExtraTypes injects the 52 declared enum type definitions into schema.json
# unconditionally (pkg/tfgen/generate_schema.go), regardless of whether the
# Fields/Elem SchemaInfo tree in provider/resources_gen.go successfully
# attached any of them to a property. That tree is the one place total token
# loss can happen silently: if it stops matching the bridge's internal shape,
# every property quietly degrades back to plain "string" while schema.json
# still lists all 52 enum types, looking exactly like success.
#
# The real signal is on the property side: every property whose description
# documents "Possible values:" must carry a $ref to one of those enum types,
# and the total number of such $refs across the schema must be at least as
# large as the measured number of enum attribute sites the generator walked.
set -euo pipefail

cd "$(dirname "$0")/.."

SCHEMA="provider/cmd/pulumi-resource-tsb/schema.json"
MIN_ENUM_REFS=316

if [ ! -f "$SCHEMA" ]; then
  echo "check-enums: $SCHEMA not found; run 'make schema' first" >&2
  exit 1
fi

python3 - "$SCHEMA" "$MIN_ENUM_REFS" <<'PYEOF'
import json
import sys

schema_path, min_enum_refs = sys.argv[1], int(sys.argv[2])

with open(schema_path) as f:
    schema = json.load(f)

types = schema.get("types", {})
enum_ref_targets = {"#/types/" + tok for tok, v in types.items() if "enum" in v}

if not enum_ref_targets:
    print("check-enums: schema.json declares no enum types; did generation run?", file=sys.stderr)
    sys.exit(1)


def node_enum_ref(node):
    """Return the enum $ref reachable directly from this property node, if any.

    Covers a scalar property ($ref straight on the node), a list property
    (items.$ref), and a map property (additionalProperties.$ref).
    """
    if not isinstance(node, dict):
        return None
    if node.get("$ref") in enum_ref_targets:
        return node["$ref"]
    items = node.get("items")
    if isinstance(items, dict) and items.get("$ref") in enum_ref_targets:
        return items["$ref"]
    additional = node.get("additionalProperties")
    if isinstance(additional, dict) and additional.get("$ref") in enum_ref_targets:
        return additional["$ref"]
    return None


possible_values_total = 0
missing = []


def walk(obj, path=""):
    global possible_values_total
    if isinstance(obj, dict):
        desc = obj.get("description")
        if isinstance(desc, str) and "Possible values:" in desc:
            possible_values_total += 1
            if node_enum_ref(obj) is None:
                missing.append((path, desc.split("Possible values:")[0].strip()[:80]))
        for k, v in obj.items():
            walk(v, f"{path}/{k}")
    elif isinstance(obj, list):
        for i, v in enumerate(obj):
            walk(v, f"{path}[{i}]")


walk(schema)

total_enum_refs = 0


def count_refs(obj):
    global total_enum_refs
    if isinstance(obj, dict):
        if obj.get("$ref") in enum_ref_targets:
            total_enum_refs += 1
        for v in obj.values():
            count_refs(v)
    elif isinstance(obj, list):
        for v in obj:
            count_refs(v)


count_refs(schema)

ok = True

if missing:
    ok = False
    print(f"check-enums: {len(missing)} of {possible_values_total} properties document", file=sys.stderr)
    print('check-enums: "Possible values:" but carry no $ref to an enum type;', file=sys.stderr)
    print("check-enums: the Fields/Elem SchemaInfo tree likely stopped matching the bridge shim", file=sys.stderr)
    print("check-enums: and these properties silently degraded back to plain string:", file=sys.stderr)
    for path, desc in missing[:20]:
        print(f"check-enums:   {path}: {desc}", file=sys.stderr)
    if len(missing) > 20:
        print(f"check-enums:   ... and {len(missing) - 20} more", file=sys.stderr)

if total_enum_refs < min_enum_refs:
    ok = False
    print(f"check-enums: schema.json has only {total_enum_refs} $refs to enum types,", file=sys.stderr)
    print(f"check-enums: expected at least {min_enum_refs} (the measured enum attribute site count);", file=sys.stderr)
    print("check-enums: the Fields/Elem SchemaInfo tree likely stopped matching the bridge shim", file=sys.stderr)
    print("check-enums: and most or all enum tokens were never applied to a property", file=sys.stderr)

if not ok:
    sys.exit(1)

print(f'check-enums: {possible_values_total} "Possible values:" properties all carry an enum $ref')
print(f"check-enums: {total_enum_refs} enum $refs present (>= {min_enum_refs} required)")
PYEOF
