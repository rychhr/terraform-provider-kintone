# Pinned so that documentation regenerates reproducibly without adding
# tfplugindocs to go.mod.
TFPLUGINDOCS_VERSION ?= v0.25.0

default: build

# Compile every package.
build:
	go build -v ./...

# Unit tests. -count=1 keeps the test cache from hiding a regression.
test:
	go test -v -count=1 ./...

# Acceptance tests. These create real kintone apps and, because kintone has no
# app deletion API, leave them behind for manual cleanup. They require
# KINTONE_DEV_BASE_URL, KINTONE_DEV_USERNAME, KINTONE_DEV_PASSWORD and the
# explicit guard KINTONE_DEV_ALLOW_ACCEPTANCE_TESTS=1. Never run them against a
# production subdomain.
testacc:
	TF_ACC=1 go test -v -count=1 -timeout 30m ./internal/provider/

lint:
	golangci-lint run

# Regenerate the provider documentation published to the Terraform Registry.
docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@$(TFPLUGINDOCS_VERSION) generate --provider-name kintone

# Validate the GoReleaser configuration without building or publishing assets.
release-check:
	goreleaser check

# Build and verify the complete local artifact set. Snapshots are deliberately
# unsigned; release workflow verification requires the detached GPG signature.
release-snapshot:
	goreleaser release --snapshot --clean --skip=sign
	VERIFY_RELEASE_SIGNATURE=0 scripts/verify-release-artifacts.sh dist

.PHONY: default build test testacc lint docs release-check release-snapshot
