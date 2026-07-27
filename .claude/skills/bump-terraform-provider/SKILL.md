---
name: bump-terraform-provider
description: Check tetratelabs/terraform-provider-tsb for a tagged release newer than pulumi-tsb's current PROVIDER_VERSION pin, and if one exists, prep and open the dependency-bump PR (go.mod bump, Makefile/resources.go version sync, full regeneration, verification). Use when asked to check for a new terraform-provider-tsb release, upgrade pulumi-tsb to track one, or prep a pulumi-tsb release PR ahead of tagging.
---

# Bump Terraform Provider

## Overview

`pulumi-tsb` tracks `github.com/tetratelabs/terraform-provider-tsb` via a version
pin duplicated in three places: `go.mod`'s require line, `Makefile`'s
`PROVIDER_VERSION`, and `provider/resources.go`'s `TFProviderVersion` field.
`Makefile`'s `providerversion` target enforces all three agree, and CI's
`build`/`lint` jobs (`make check`) fail if they don't. This skill automates the
whole bump-and-regenerate cycle: check for a newer upstream tag, update the
three pin sites, regenerate everything derived from them, verify, and open a
PR — mirroring the procedure already used for `v0.0.1`-`v0.0.4` and for the
`PROVIDER_VERSION` fix in PR #5.

**Out of scope — do not do these:**
- Tagging or publishing a `pulumi-tsb` release (`vY` tag + `release.yml`).
  That's a separate, deliberate action taken by a human *after* this PR
  merges, per `Makefile`'s `tagcheck` gate.
- Cutting the upstream `terraform-provider-tsb` release itself. That repo's
  tag is a prior, independent action taken directly on that repo.
- Merging the PR this skill opens.

## Step 1: Read the current pin

```bash
cd ~/src/pulumi-tsb
grep '^PROVIDER_VERSION=' Makefile
grep '^VERSION=' Makefile
```

## Step 2: Find the latest upstream tag

Use `git ls-remote` rather than the GitHub tags API — the API's ordering
isn't guaranteed to be semver order.

```bash
git ls-remote --tags --refs https://github.com/tetratelabs/terraform-provider-tsb.git \
  | sed 's#.*refs/tags/v##' | sort -V | tail -1
```

## Step 3: Compare and decide

- **Current `PROVIDER_VERSION` is a clean `X.Y.Z` and equals the latest tag**:
  up to date. Report this and stop — nothing to do.

- **Current `PROVIDER_VERSION` is a pseudo-version**
  (`X.Y.Z-<timestamp>-<hash>` — Go's format when a dependency is pinned to a
  commit with no tag at or after it): STOP and report to the user instead of
  guessing. This means `pulumi-tsb` is already pinned past the newest tagged
  commit, on an untagged one. Reconciling this needs a human judgment call
  (wait for upstream to cut a tag covering that commit, or accept whatever
  the latest tag actually covers if it's not the same commit) — don't pick
  for them. This is the exact situation hit with the `Groups()` commit vs.
  the real `v0.1.1` tag; don't repeat that mismatch silently.

- **Current `PROVIDER_VERSION` is a clean `X.Y.Z` older than the latest tag**:
  proceed to Step 4 with the latest tag as the target version.

## Step 4: Branch

```bash
git checkout main && git pull
git checkout -b bump-terraform-provider-v<TARGET>
```

## Step 5: Bump the dependency

```bash
go get github.com/tetratelabs/terraform-provider-tsb@v<TARGET>
go mod tidy
```

Both `tetratelabs/terraform-provider-tsb` and its indirect
`tetrateio/tetrate` dependency resolve fine here — the local machine already
has `GOPRIVATE=github.com/tetrateio/*,github.com/tetratelabs/*` and a
`git@github.com:` `insteadOf` rewrite configured (same mechanism CI uses via
`webfactory/ssh-agent` + `GOPRIVATE`). If `go mod tidy` hits a
`Permission denied (publickey)` error, the local SSH setup is missing or
broken — stop and report rather than working around it.

## Step 6: Sync the other two pin sites

- `Makefile`: set `PROVIDER_VERSION=<TARGET>` (exact string `go.mod` now has
  after Step 5 — copy it verbatim; don't retype it, pseudo-version hashes are
  easy to transpose).
- `provider/resources.go`: set `TFProviderVersion: "<TARGET>"` in the
  `tfbridge.ProviderInfo{}` literal returned by `Provider()`.

Also decide `Makefile`'s `VERSION` (and the matching `resources.go` `Version`
field) — this is `pulumi-tsb`'s own package version, an independent track
from `PROVIDER_VERSION` (confirmed by history: commit `252fba0` bumped
`PROVIDER_VERSION` 0.0.4→0.0.5 while `VERSION` went 0.0.2→0.0.3 in the same
commit — they move together but aren't required to match numerically):

- Default: bump `VERSION`'s patch number by one from the last released
  `pulumi-tsb` tag (`git tag -l 'v*' | sort -V | tail -1`).
- If the upstream bump is major/minor, or `make build` (Step 7) produces a
  schema diff that looks breaking (removed/renamed resources or fields),
  stop and ask the user how to bump `VERSION` instead of guessing — this is
  a judgment call about pulumi-tsb's own semver, not something to infer from
  upstream's version number.

## Step 7: Regenerate

```bash
make build     # runs: generate -> schema -> bridge -> sdk (sdk includes licenser)
```

This regenerates `provider/resources_gen.go`, the schema, and everything
under `sdk/` (including `package.json` and
`sdk/scripts/install-pulumi-plugin.js`, both derived from `VERSION`/
`PROVIDER_VERSION` via `Makefile`'s `sdk.nodejs` target).

## Step 8: Verify

```bash
go build ./... && go vet ./... && go test ./...
make check
```

`make check` must produce a clean `git status` — that's what CI's `build`
job actually gates on. If it isn't clean, stop and report the diff; don't
force a commit over an inconsistent regeneration.

## Step 9: Commit, push, open the PR

Follow this repo's commit conventions: sign the commit (`git commit -S`),
include a `Signed-off-by` trailer, no `Co-Authored-By`/AI-attribution
footer.

```bash
git add -A
git commit -S -s -m "Bump terraform-provider-tsb to v<TARGET>"
git push origin bump-terraform-provider-v<TARGET>
gh pr create --title "Bump terraform-provider-tsb to v<TARGET>" --body "$(cat <<'EOF'
Bumps the pinned terraform-provider-tsb dependency to v<TARGET> and
regenerates the schema/SDK to match.

- PROVIDER_VERSION: <OLD> -> <TARGET>
- VERSION: <OLD_VERSION> -> <NEW_VERSION>

This PR does not tag or publish a pulumi-tsb release — that happens
separately after merge, per Makefile's tagcheck.
EOF
)"
```

Report the PR URL. Wait for `build`+`lint` CI to go green before telling the
user it's ready to merge; if a check fails, diagnose it the same way PR #5's
failures were diagnosed (read the job logs, don't just rerun blind).

## Guardrails

- Never force-push.
- If a branch or open PR already exists for this same target version, stop
  and ask rather than opening a duplicate.
- Don't guess the `VERSION` bump on a major/minor upstream change — ask.
- Don't silently proceed when `PROVIDER_VERSION` is already a pseudo-version
  past the latest tag (Step 3) — that always needs a human decision.
