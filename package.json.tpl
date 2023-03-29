{
  "name": "@tetratelabs/pulumi-tsb",
  "version": "${VERSION}",
  "description": ". Based on terraform-provider-tsb: version v${PROVIDER_VERSION}",
  "main": "ts_bin/index.js",
  "types": "ts_bin/index.d.ts",
  "scripts": {
    "build": "tsc",
    "prepare": "npm run build",
    "install": "node sdk/scripts/install-pulumi-plugin.js resource tsb ${VERSION} --server github://api.github.com/tetratelabs"
  },
  "dependencies": {
    "@pulumi/pulumi": "^3.0.0"
  },
  "devDependencies": {
    "@types/node": "^10.0.0",
    "typescript": "^4.3.5"
  },
  "pulumi": {
    "resource": true,
    "name": "tsb"
  }
}
