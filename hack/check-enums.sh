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
#
# Both counts checked below have a floor, not just the "no $ref" check: a
# schema where the "Possible values:" phrase itself has vanished from every
# description would trivially satisfy "every property with that phrase has a
# $ref" (there are zero such properties) while telling us nothing. The floor
# on the property count exists specifically to catch that vacuous case.
set -euo pipefail

cd "$(dirname "$0")/.."

SCHEMA="provider/cmd/pulumi-resource-tsb/schema.json"

# The two floors below are MEASURED EXPECTATIONS, not derived constants: they
# are the exact counts observed on a schema.json known to be correct (every
# enum site actually wired through). They are not computed from the
# generator's site count (that number, printed by `make generate` as "N enum
# sites", is close but not equal, because a handful of sites fan out to more
# than one $ref or documented property) — comparing against a borrowed number
# from a different measurement is what left 18 refs of accidental, undesigned
# slack in a previous version of this floor.
#
# A legitimate upstream change (proto comments reworded to add/remove enum
# documentation, enum-typed fields added or removed from the .proto) can
# legitimately move these numbers. If that happens, this script will fail
# even though the Fields/Elem tree is fine. To re-measure after confirming
# such a change is legitimate (e.g. by reading the schema.json diff, not just
# by making the failure go away): run this script against the newly
# regenerated schema.json, read the two counts it prints on success
# ("N \"Possible values:\" properties..." and "M enum $refs present"), and set
# MIN_POSSIBLE_VALUES_PROPERTIES / MIN_ENUM_REFS below to those exact numbers.
MIN_POSSIBLE_VALUES_PROPERTIES=334  # floor for count of "Possible values:" properties
MIN_ENUM_REFS=334                   # floor for count of $refs to enum types

if [ ! -f "$SCHEMA" ]; then
  echo "check-enums: $SCHEMA not found; run 'make schema' first" >&2
  exit 1
fi

python3 - "$SCHEMA" "$MIN_POSSIBLE_VALUES_PROPERTIES" "$MIN_ENUM_REFS" <<'PYEOF'
import json
import sys

schema_path = sys.argv[1]
min_possible_values_properties = int(sys.argv[2])
min_enum_refs = int(sys.argv[3])

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
    print("check-enums: this means the Fields/Elem SchemaInfo tree probably broke:", file=sys.stderr)
    print("check-enums: it stopped matching the bridge shim and these properties", file=sys.stderr)
    print("check-enums: silently degraded back to plain string:", file=sys.stderr)
    for path, desc in missing[:20]:
        print(f"check-enums:   {path}: {desc}", file=sys.stderr)
    if len(missing) > 20:
        print(f"check-enums:   ... and {len(missing) - 20} more", file=sys.stderr)

if possible_values_total < min_possible_values_properties:
    ok = False
    print(f"check-enums: only {possible_values_total} properties document \"Possible values:\",", file=sys.stderr)
    print(f"check-enums: expected at least {min_possible_values_properties};", file=sys.stderr)
    if missing:
        print("check-enums: properties are also missing $refs above, which is strong evidence", file=sys.stderr)
        print("check-enums: the Fields/Elem SchemaInfo tree broke rather than upstream changing.", file=sys.stderr)
    else:
        print("check-enums: every remaining such property still carries a $ref, so this is more", file=sys.stderr)
        print("check-enums: likely upstream legitimately changed (proto comments reworded so the", file=sys.stderr)
        print('check-enums: phrase disappeared, or enum-documented properties were removed) than', file=sys.stderr)
        print("check-enums: the Fields/Elem tree breaking. If you've confirmed that by reading the", file=sys.stderr)
        print("check-enums: schema.json diff, re-measure and update MIN_POSSIBLE_VALUES_PROPERTIES", file=sys.stderr)
        print("check-enums: in hack/check-enums.sh; do not just lower it to make this pass.", file=sys.stderr)

if total_enum_refs < min_enum_refs:
    ok = False
    print(f"check-enums: schema.json has only {total_enum_refs} $refs to enum types,", file=sys.stderr)
    print(f"check-enums: expected at least {min_enum_refs};", file=sys.stderr)
    if missing:
        print("check-enums: properties are also missing $refs above, which is strong evidence", file=sys.stderr)
        print("check-enums: the Fields/Elem SchemaInfo tree broke rather than upstream changing.", file=sys.stderr)
    else:
        print("check-enums: no property with \"Possible values:\" is missing its $ref, so this is", file=sys.stderr)
        print("check-enums: more likely upstream legitimately removing enum attribute sites than", file=sys.stderr)
        print("check-enums: the Fields/Elem tree breaking. If you've confirmed that by reading the", file=sys.stderr)
        print("check-enums: schema.json diff, re-measure and update MIN_ENUM_REFS in", file=sys.stderr)
        print("check-enums: hack/check-enums.sh; do not just lower it to make this pass.", file=sys.stderr)

if not ok:
    sys.exit(1)

print(f'check-enums: {possible_values_total} "Possible values:" properties all carry an enum $ref (>= {min_possible_values_properties} required)')
print(f"check-enums: {total_enum_refs} enum $refs present (>= {min_enum_refs} required)")
PYEOF
