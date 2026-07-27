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

Set `GITHUB_TOKEN` to a personal access token with the `read:packages` scope,
then:

```
npm install @tetratelabs/pulumi-tsb
```

Installing runs `pulumi plugin install`, which downloads the provider binary
from this repository's GitHub Releases. That download is unauthenticated — the
token is only needed for the npm package itself.

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
