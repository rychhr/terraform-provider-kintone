# Release Pipeline Design

## Goal

Build the complete Terraform Registry artifact set from a version tag, sign its checksum file, publish it
as a GitHub draft release, and verify the draft before any human publishes it. The same configuration must
also support an unsigned local snapshot for release-candidate inspection.

## Scope

This change adds the Registry manifest, GoReleaser configuration, release workflow, tag validator, artifact
verifier, and their shell regression tests. It does not create a real tag, import the production signing
key locally, publish a GitHub release, or publish a provider version to the Terraform Registry. Provider
schemas, Terraform state, and the kintone API client are unchanged.

## Artifact contract

Every non-snapshot release contains exactly one ZIP archive for each supported target in the following
matrix:

| Operating system | Architectures |
| --- | --- |
| `freebsd` | `amd64`, `386`, `arm`, `arm64` |
| `windows` | `amd64`, `386`, `arm64` |
| `linux` | `amd64`, `386`, `arm`, `arm64` |
| `darwin` | `amd64`, `arm64` |

The unsupported `darwin/386`, `darwin/arm`, and `windows/arm` pairs are excluded. Archives are named
`terraform-provider-kintone_{VERSION}_{OS}_{ARCH}.zip`. Each archive contains one root-level executable
named `terraform-provider-kintone_v{VERSION}`; Windows alone adds `.exe`.

The release also contains:

- `terraform-provider-kintone_{VERSION}_manifest.json`, copied from
  `terraform-registry-manifest.json` and declaring manifest version `1` and protocol version `6.0`;
- `terraform-provider-kintone_{VERSION}_SHA256SUMS`, containing exactly the 13 archives and the manifest;
  and
- `terraform-provider-kintone_{VERSION}_SHA256SUMS.sig`, a binary detached GPG signature of the checksum
  file.

GoReleaser sets `project_name: terraform-provider-kintone` explicitly so a repository or module-path
change cannot silently alter artifact names. Builds set `CGO_ENABLED=0`, use ZIP archives on every platform,
and inject the tag version into `main.version` with linker flags.

## Tag validation

The workflow runs only for pushed refs matching `refs/tags/v*`, but the trigger is not the security or
correctness boundary. Before any signing secret is imported, `scripts/validate-release-tag.sh` validates
the event tag and the checked-out history.

The tag must start with `v` and the remainder must be a complete SemVer 2.0 version. Release versions,
prerelease identifiers, and build metadata are accepted. Numeric identifiers and core version fields must
not contain leading zeroes, and identifiers must contain only ASCII alphanumerics and hyphens where SemVer
allows them.

The tag must exist, resolve to a commit, and be reachable from the supplied `main` ref. A local branch or a
remote-tracking branch whose short name equals the tag is rejected because the Terraform Registry requires
the release tag name not to collide with a branch name. Missing refs and ambiguous or non-commit objects
fail closed.

## Signing and secret handling

The release workflow imports `GPG_PRIVATE_KEY` with `GPG_PASSPHRASE` only after tag validation succeeds.
The import action exposes the key fingerprint to GoReleaser, which invokes GnuPG in batch mode and produces
a detached, non-armored signature for the checksum artifact. The workflow never prints either secret and
requires only the repository-scoped `GITHUB_TOKEN` with `contents: write`.

The verifier requires the signature by default, rejects ASCII armor, and runs `gpg --verify` against the
checksum file. A signature that exists but cannot be verified by a key in the active GnuPG keyring is a
failure.

## Draft publication and ordering

The workflow has no manual trigger and runs only for `v*` tag pushes. Checkout fetches full history, Go is
selected from `go.mod`, and the steps execute in this order:

1. validate the tag and its ancestry;
2. import the GPG private key;
3. run GoReleaser v2.18.0;
4. verify the generated `dist/` artifacts in signature-required mode.

GoReleaser always creates a draft GitHub release and does not generate a changelog. The workflow never
publishes or undrafts a release. A failed post-upload verification may therefore leave a draft containing
invalid or incomplete assets; a maintainer inspects or deletes that draft manually. Keeping the failed
draft is safer than adding automated cleanup that could target the wrong release.

Concurrency is grouped by tag ref and does not cancel an in-progress release. Workflow permissions are
limited to `contents: write`.

## Snapshot behavior

`make release-snapshot` runs the same build and packaging configuration without publishing. The Make target
passes GoReleaser's `--skip=sign` flag and sets `VERIFY_RELEASE_SIGNATURE=0` for the verifier. This is the
only supported signature bypass. All archive, manifest, checksum-entry, and digest checks still run in
snapshot mode. Tagged releases do not pass either bypass and therefore require checksum signing.

Snapshot artifact names use the snapshot version selected by GoReleaser. The verifier derives that version
from the checksum filename rather than assuming a release tag.

## Artifact verification

`scripts/verify-release-artifacts.sh DIST_DIR` derives the version from the single expected checksum file
and independently verifies the public artifact contract. It rejects missing or additional target archives,
duplicate checksum entries, additional provider release assets, malformed manifest JSON, unexpected ZIP
contents, and digest mismatches. The checksum file must name exactly the 13 archives and the manifest.

GoReleaser's `extra_files` gives the manifest its release name in the checksum and GitHub upload without
materializing that renamed path in `dist/`. When that path is absent, the verifier resolves only the
manifest content from `terraform-registry-manifest.json` next to the `dist/` directory. It still requires
the release name in the checksum and verifies the source file's JSON and digest against that entry.

GoReleaser may create internal metadata such as `artifacts.json`, `config.yaml`, and per-build directories
inside `dist/`. These are not provider release assets and are ignored by the public-asset inventory check.
Any root-level file beginning with `terraform-provider-kintone_` remains subject to the strict contract.

## Failure modes

- An invalid, missing, unreachable, or branch-colliding tag stops the workflow before secrets are read.
- A missing or unusable signing key stops GoReleaser before a valid release set exists.
- A build, archive, checksum, or upload failure leaves the workflow failed and may leave a draft release.
- A verifier failure leaves the workflow failed and the draft unpublished for manual inspection.
- Snapshot verification may omit only the signature; all other contract failures remain fatal.

All shell scripts use temporary directories for fixtures, avoid printing secret material, and exit non-zero
on malformed inputs or missing external commands.

## Validation

Shell regression tests create isolated Git repositories and synthetic release artifacts. Tag tests cover
valid releases, prereleases, build metadata, malformed SemVer, unreachable commits, and branch collisions.
Artifact tests cover a valid binary signature plus each documented archive, checksum, manifest, and
signature failure mode. The repository-level checks are GoReleaser v2.18.0 `check` and snapshot builds,
actionlint v1.7.12, build, unit tests, lint, whitespace checks, and all three documented secret scans.

## References

- [Publishing providers to the Terraform Registry](https://developer.hashicorp.com/terraform/registry/providers/publishing)
- [GoReleaser GitHub Actions integration](https://www.goreleaser.com/customization/ci/actions/)
- [GoReleaser artifact metadata](https://goreleaser.com/customization/general/artifacts/)
- [GoReleaser checksum signing](https://goreleaser.com/customization/sign/sign/)
