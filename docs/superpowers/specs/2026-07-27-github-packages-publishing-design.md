# Publishing the TypeScript SDK to GitHub Packages

Date: 2026-07-27

## Problem

The `npmpublish` job in `.github/workflows/release.yml` publishes
`@tetratelabs/pulumi-tsb` to `registry.npmjs.org` using an `NPM_TOKEN` secret.
That secret is not available, so releases of the TypeScript package cannot be
cut.

Investigating the job surfaced three further defects that would keep the package
unusable even with a working credential:

1. The job runs `cd dist && npm publish`, but `dist/` holds only the `tsc`
   output — no `package.json`. `npm publish` cannot run there.
2. `package.json` has no `main` field. Node falls back to `index.js` at the
   package root, which does not exist in the published tree, so
   `require('@tetratelabs/pulumi-tsb')` fails.
3. `Makefile`'s `sdk.nodejs` target rewrites `sdk/utilities.ts` to
   `require('../package.json')`. That statement executes from `dist/sdk/`, where
   `../package.json` resolves to `dist/package.json` — a path that does not
   exist. `getVersion()` throws at runtime.

There is also no `files` allowlist or `.npmignore`, so a publish from the repo
root would ship the entire Go provider tree.

## Goal

Release `@tetratelabs/pulumi-tsb` to GitHub Packages on tag push, and make the
published tarball actually installable and importable.

## Non-goals

- Publishing to `registry.npmjs.org`. Consumers are Tetrate engineers and
  partners who already hold GitHub credentials, so the authentication that
  GitHub Packages requires is acceptable.
- Changing how the provider binary is distributed. It already ships as a GitHub
  Release asset via `--server github://api.github.com/tetratelabs`, and the
  repository is public, so that download needs no token.
- Fixing the BSD/GNU `sed -i -e` divergence in the `Makefile`, which leaves a
  stray `sdk/utilities.ts-e` file on macOS. Pre-existing and unrelated.

## Design

### Registry and authentication

`tetratelabs/pulumi-tsb` is owned by the `tetratelabs` org and the package is
already scoped `@tetratelabs`, so GitHub Packages accepts it without a rename.

Authentication uses the workflow's built-in `GITHUB_TOKEN` rather than a new
secret. The job declares `packages: write`; no org-level provisioning is needed.

Because `permissions` is currently set once at the top of the workflow
(`contents: write`, required by `goreleaser`), it moves to per-job blocks so
`npmpublish` can request `packages: write` while `goreleaser` keeps
`contents: write`.

### Job ordering

`goreleaser` and `npmpublish` currently run in parallel. The package's `install`
lifecycle script downloads the provider binary from the GitHub Release for the
same tag, so a package published before the release assets exist is
uninstallable. `npmpublish` gains `needs: goreleaser`, closing that window at
the cost of a slower release.

### Package metadata

`package.json.tpl` — the source of the generated `package.json` — gains:

| Field | Value | Reason |
|---|---|---|
| `main` | `dist/index.js` | Without it `require()` of the package fails. |
| `repository` | `git+https://github.com/tetratelabs/pulumi-tsb.git` | Links the package to the repo on the GitHub Packages page. |
| `publishConfig.registry` | `https://npm.pkg.github.com` | A manual `npm publish` outside CI targets the right registry. |
| `files` | `["dist/", "sdk/scripts/"]` | Excludes the Go provider tree. `sdk/scripts/` is required because the `install` script runs `node sdk/scripts/install-pulumi-plugin.js`. |

`types` already points at `dist/index.d.ts` and is unchanged.

`package.json` and `package-lock.json` are regenerated from the template and
committed, since CI runs `npm ci` against the committed files rather than
regenerating them.

### Runtime version lookup

The `sed` in `Makefile`'s `sdk.nodejs` target changes its replacement from
`../package.json` to `../../package.json`, so the compiled
`dist/sdk/utilities.js` reaches the package root.

`sdk/utilities.ts` is the only file performing this lookup, and it compiles to
exactly one level below `dist/`, so a fixed relative depth is correct.

### Publish job

```yaml
  npmpublish:
    needs: goreleaser
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
      - uses: actions/checkout@v4
      - run: make tagcheck
      - uses: actions/setup-node@v4
        with:
          node-version: '20.x'
          registry-url: 'https://npm.pkg.github.com'
          scope: '@tetratelabs'
      - run: npm ci
      - run: ./hack/smoke-package.sh
      - run: npm publish
        env:
          NODE_AUTH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

The `cd dist` is dropped: `npm publish` runs at the repo root, where the real
`package.json` lives, and the `files` allowlist decides what ships.

`dist/` is populated by the existing `prepare` script, which npm runs during
both `npm ci` and `npm publish`, so the allowlisted directory is never empty.
`prepare` does not run for consumers installing the tarball, so they do not
need TypeScript.

### Verification

A new `hack/smoke-package.sh` runs in CI immediately before `npm publish`. It:

1. Runs `npm pack` to build the tarball that would be published.
2. Installs that tarball into a scratch project with `--ignore-scripts`, which
   skips the `pulumi plugin install` step — the release binary may not be
   reachable from the runner, and plugin installation is not what this checks.
3. Requires the package and asserts a known resource export is present.
4. Calls `getVersion()` from `dist/sdk/utilities` and asserts it returns the
   package version.

Steps 3 and 4 fail against the current tree, covering all three packaging
defects. The script is a standalone file rather than inline YAML so it can be
run locally before tagging.

### Consumer documentation

The README gains an install section covering the `.npmrc` that consumers need:

```
@tetratelabs:registry=https://npm.pkg.github.com
//npm.pkg.github.com/:_authToken=${GITHUB_TOKEN}
```

The text states explicitly that GitHub Packages requires authentication even
though the repository is public, that the token must be a classic personal
access token with `read:packages` (GitHub Packages' npm registry rejects
fine-grained tokens), and that the provider binary download needs no token.
The existing "Publishing" section is updated to drop the npmjs reference.

## Risks

- **First publish under a new registry.** The package has never published
  successfully, so there is no prior version to conflict with. If a version of
  `@tetratelabs/pulumi-tsb` does exist on npmjs, it is stale and unaffected;
  the two registries are independent.
- **`needs: goreleaser` couples the jobs.** A goreleaser failure now blocks the
  package publish. That is the intended behaviour, but it means a partial
  release requires re-running both jobs.
- **Consumers must configure `.npmrc`.** Accepted, and documented in the README.
