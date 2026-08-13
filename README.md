# TSB Provider for Pulumi

Manage [Tetrate Service Bridge (TSB)](https://tetrate.io/tetrate-service-bridge/) resources with Pulumi.

Generated from the [tsb](https://github.com/tetratelabs/terraform-provider-tsb) terraform provider.

## Installing

The package is published to GitHub Packages, which requires authentication for
reads even though this repository is public. Add an `.npmrc` next to your
`package.json`:

```
@tetratelabs:registry=https://npm.pkg.github.com
//npm.pkg.github.com/:_authToken=${GITHUB_TOKEN}
```

Set `GITHUB_TOKEN` to a classic personal access token with the `read:packages`
scope, then:

```
npm install @tetratelabs/pulumi-tsb
```

Installing runs `pulumi plugin install`, which downloads the provider binary
from this repository's GitHub Releases. That download is unauthenticated — the
token is only needed for the npm package itself.

## Enums

Properties backed by a TSB API enum are typed, so the compiler catches a bad
value rather than the server rejecting it at apply time:

```ts
import * as tsb from "@tetratelabs/pulumi-tsb";

mode: tsb.auth.TLSMode.Mutual   // ok
mode: "mutual"                  // compile error
```

The enum types live in modules named after the proto package they come from,
because the same leaf name can mean different things in different packages —
`tsb.auth.TLSMode` has four values, `tsb.profile.TLSMode` has three.

**One caveat on outputs.** These properties are both inputs and outputs, and the
declared type is derived from the API descriptors this package was built
against. A TSB server newer than those descriptors can return an enum value it
knows and this package does not; protobuf renders such a value as its number in
string form, e.g. `"7"`. That value is outside the declared type, but TypeScript
enums are strings at runtime, so it flows through your program untouched and
round-trips back to the server correctly — it is a type-level inaccuracy, not a
failure. Upgrading this package to one built against newer descriptors resolves
it.

## Updating

To update the terraform provider version, edit 3 files:
* `Makefile`
* `go.mod`
* `provider/resources.go`

Then run `make generate` to pick up any resource additions/removals from the new
terraform provider version, review the diff to `provider/resources_gen.go`, and
run `make` to rebuild everything. If any version is mismatched, `make` fails and
prints which file is out of sync.

## Publishing

Run `git tag v{new version}` and push. The release workflow builds the provider
binaries with GoReleaser, then publishes the TypeScript package to GitHub
Packages using the workflow's built-in `GITHUB_TOKEN` — no secret to configure.

Run `make tagcheck` first to check that the tag matches the provider version,
and `./hack/smoke-package.sh` to check that the tarball is installable.
