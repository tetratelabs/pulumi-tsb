#!/usr/bin/env bash
# Verifies that the tarball `npm publish` would upload is installable and
# importable. Run from anywhere; it operates on the repo root.
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$PWD"

PKG_VERSION="$(node -p "require('./package.json').version")"
TARBALL="$ROOT/tetratelabs-pulumi-tsb-${PKG_VERSION}.tgz"
SMOKE_DIR="$(mktemp -d)"

cleanup() { rm -rf "$SMOKE_DIR" "$TARBALL"; }
trap cleanup EXIT

# `npm pack` runs the `prepare` script, which populates dist/ via tsc.
npm pack --silent >/dev/null
if [ ! -f "$TARBALL" ]; then
  echo "FAIL: expected tarball at $TARBALL"
  exit 1
fi

echo "--- tarball contents ---"
tar -tzf "$TARBALL"
echo "-----------------------"

if ! tar -tzf "$TARBALL" | grep -q '^package/dist/index\.js$'; then
  echo "FAIL: dist/index.js missing from tarball"
  exit 1
fi
if ! tar -tzf "$TARBALL" | grep -q '^package/sdk/scripts/install-pulumi-plugin\.js$'; then
  echo "FAIL: sdk/scripts/install-pulumi-plugin.js missing from tarball"
  exit 1
fi
if tar -tzf "$TARBALL" | grep -q '^package/provider/'; then
  echo "FAIL: Go provider sources leaked into the tarball"
  exit 1
fi

cd "$SMOKE_DIR"
npm init -y >/dev/null
# --ignore-scripts skips the `install` hook, which downloads the provider
# binary from the GitHub Release for this tag. That release may not exist yet
# and is not what this test covers.
npm install --ignore-scripts --no-audit --no-fund "$TARBALL" >/dev/null

node -e '
  const pkg = require("@tetratelabs/pulumi-tsb");
  if (typeof pkg.Cluster === "undefined") {
    throw new Error("expected Cluster export to be present");
  }
  const { getVersion } = require("@tetratelabs/pulumi-tsb/dist/sdk/utilities");
  const got = getVersion();
  const want = require("@tetratelabs/pulumi-tsb/package.json").version;
  if (got !== want) {
    throw new Error("getVersion() returned " + got + ", want " + want);
  }
  console.log("OK: imported package, getVersion() = " + got);
'
