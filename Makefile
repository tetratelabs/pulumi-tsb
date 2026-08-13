# Copyright (c) Tetrate, Inc 2026 All Rights Reserved.

# Terraform provider version
PROVIDER_VERSION=0.1.4

# Pulumi bridged provider version (this package)
VERSION=0.1.4

default: build

build: generate schema bridge sdk

GEN=provider/cmd/gen-resources-tsb
TFGEN=provider/cmd/pulumi-tfgen-tsb
BRIDGE=provider/cmd/pulumi-resource-tsb

# regenerates provider/resources_gen.go from the live terraform-provider-tsb
generate:
	go run ./$(GEN) -o provider/resources_gen.go

# generates the provider schema
schema: providerversion
	cd $(TFGEN) && go run main.go schema -o ../pulumi-resource-tsb

licenser:
	go run github.com/liamawhite/licenser@v0.7.0 apply -r -t header.txt -m "Copyright (c) Tetrate" "Tetrate, Inc"

# generates the typescript package for using the provider
# this requires the provider to be installed in $PATH
sdk: schema sdk.nodejs licenser

sdk.nodejs:
	go run ./$(TFGEN) nodejs -o sdk
	sed -e 's/$${VERSION}/${VERSION}/g' \
		-e 's/$${PROVIDER_VERSION}/${PROVIDER_VERSION}/g' package.json.tpl > package.json
	rm sdk/package.json sdk/tsconfig.json
	sed -i -e "s|require('\./package\.json')|require('../../package.json')|" sdk/utilities.ts
	grep -qF "require('../../package.json')" sdk/utilities.ts || (echo utilities.ts package.json path rewrite failed && false)
	mkdir -p sdk/scripts
	sed -e 's/$${VERSION}/'v${VERSION}/ install-pulumi-plugin.js > sdk/scripts/install-pulumi-plugin.js

# builds the pulumi terraform bridge
bridge: schema
	cd $(BRIDGE) && go build

# installs the bridge
install: bridge
	cd $(BRIDGE) && go install

providerversion:
	grep -q "github.com/tetratelabs/terraform-provider-tsb\s\s*v${PROVIDER_VERSION}" go.mod || (echo go.mod tf provider version does not match && false)
	grep -q "\sTFProviderVersion:.*${PROVIDER_VERSION}" provider/resources.go || (echo provider/resources.go tf provider version does not match && false)
	grep -q "\sVersion:.*${VERSION}" provider/resources.go || (echo provider/resources.go version does not match && false)

versioncheck: providerversion
	grep -q '"version": "'${VERSION} package.json || (echo package.json version does not match && false)
	grep -q "v${VERSION}" sdk/scripts/install-pulumi-plugin.js || (echo sdk/scripts/install-pulumi-plugin.js version does not match && false)

tagcheck: versioncheck
	git tag --points-at HEAD | grep -q v${VERSION} || (echo tag does not match specified version && false)

check: generate licenser
	[ -z "`git status -uno --porcelain`" ] || (git status && echo 'Check failed. This could be a failed check or dirty git state.'; exit 1)