# Release artifacts and development builds

Read this before changing build or release configuration, verifying release assets, or using an
unreleased provider with Terraform. Run commands from the repository root. Follow
[AGENTS.md](../../AGENTS.md) for push and remote-change approval, and the
[release signing contract](release-signing-key.md) for signing controls and key transitions.

## Local development against an unreleased build

`dev_overrides` derives the binary name from the **last element of the source address**: Terraform looks for
an executable named `terraform-provider-<TYPE>`, where `<TYPE>` is that last element. For the address
`rychhr/kintone` the required binary name is therefore `terraform-provider-kintone`. Keep the address and
the binary name in sync whenever build instructions change.

Credentials are loaded from an ignored `.env.local` through `direnv`; the repository will provide an
`.env.example` template. Never commit credentials. Use a dedicated kintone service account, since password
authentication is required for app creation.

## Release and Registry requirements

- `.goreleaser.yaml` must set `project_name: terraform-provider-kintone` explicitly. GoReleaser derives
  `ProjectName` from the **repository name**, not from the Go module path, so without an explicit setting a
  repository rename silently changes artifact names and breaks release verification.
- `scripts/verify-release-artifacts.sh` must expect `terraform-provider-kintone_*` artifact names.
- The build matrix follows terraform-provider-scaffolding-framework: `goos` of freebsd, windows, linux, and
  darwin against `goarch` of amd64, 386, arm, and arm64, with `darwin/386`, `darwin/arm`, and `windows/arm`
  excluded via `ignore` because Go does not support them. Writing the full cross product without those
  exclusions breaks the build.
- Releases are created as drafts. Verify the assets with `scripts/verify-release-artifacts.sh` before
  publishing.

The Registry requires these assets per release:

| Asset | Format |
| --- | --- |
| Binary archive | `terraform-provider-kintone_{VERSION}_{OS}_{ARCH}.zip`, containing a binary named `terraform-provider-kintone_v{VERSION}` |
| Checksums | `terraform-provider-kintone_{VERSION}_SHA256SUMS` |
| Signature | `terraform-provider-kintone_{VERSION}_SHA256SUMS.sig` — a binary detached signature, *not* ASCII armored |
| Manifest | `terraform-provider-kintone_{VERSION}_manifest.json` |

`terraform-registry-manifest.json` declares `version: 1` and `metadata.protocol_versions: ["6.0"]`.
