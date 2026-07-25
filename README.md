# TSB Provider for Pulumi

Manage [Tetrate Service Bridge (TSB)](https://tetrate.io/tetrate-service-bridge/) resources with Pulumi.

Generated from the [tsb](https://github.com/tetratelabs/terraform-provider-tsb) terraform provider.

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

Run `git tag v{new version}` and push.
You can run `make tagcheck` to double check that the tag matches the provider version.
