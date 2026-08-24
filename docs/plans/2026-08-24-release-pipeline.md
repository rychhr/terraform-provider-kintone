# Release Pipeline Implementation Plan

> **For maintainers:** Implement this plan on `ci/build-release-pipeline` after the design change is merged.

**Goal:** Build, sign, draft-publish, and independently verify the Terraform Registry release assets for
the provider.

**Architecture:** GoReleaser owns compilation, ZIP packaging, checksums, signing, and draft upload. Small
shell programs enforce tag provenance and independently validate the resulting public asset contract;
GitHub Actions composes those pieces without automatically publishing a release.

**Tooling:** Go 1.26.4 from `go.mod`, GoReleaser v2.18.0, GnuPG, POSIX shell, GitHub Actions, and actionlint
v1.7.12.

**Spec:** `docs/specs/2026-08-24-release-pipeline-design.md`

---

## Task 1: Validate release tags

**Files:**

- Create: `scripts/test-validate-release-tag.sh`
- Create: `scripts/validate-release-tag.sh`

1. Add fixtures for valid release, prerelease, and build-metadata tags in an isolated Git repository.
2. Run the test and confirm it fails because the validator does not exist.
3. Implement `scripts/validate-release-tag.sh TAG MAIN_REF` with a `v`-prefixed SemVer 2.0 check.
4. Require the tag to resolve to a commit reachable from `MAIN_REF`.
5. Reject local and remote-tracking branches whose short name is identical to the tag.
6. Add rejecting fixtures for prefix omission, incomplete versions, leading zeroes, invalid prerelease
   identifiers, unreachable tag commits, and branch collisions.
7. Run the test suite and confirm all fixtures pass.

## Task 2: Verify release artifacts

**Files:**

- Create: `scripts/test-verify-release-artifacts.sh`
- Create: `scripts/verify-release-artifacts.sh`

1. Build a valid synthetic 13-target artifact set in a temporary directory, including the protocol `6.0`
   manifest, exact checksum inventory, and a throwaway RSA key and binary detached signature.
2. Run the test and confirm it fails because the verifier does not exist.
3. Implement version discovery, exact target inventory, ZIP entry validation, manifest validation, exact
   checksum inventory, and SHA-256 recalculation.
4. Require a non-armored signature and successful `gpg --verify` by default.
5. Allow only `VERIFY_RELEASE_SIGNATURE=0` to skip signature presence and verification.
6. Add independent mutations for a missing archive, extra target, multiple ZIP entries, digest tampering,
   malformed manifest, armored signature, missing signature, and unsigned snapshot mode.
7. Run the test suite and confirm every mutation is detected.

## Task 3: Configure GoReleaser and local release targets

**Files:**

- Create: `terraform-registry-manifest.json`
- Create: `.goreleaser.yaml`
- Modify: `.gitignore`
- Modify: `GNUmakefile`
- Modify: `AGENTS.md`

1. Add the Registry manifest with format version `1` and protocol version `6.0`.
2. Configure the explicit project name, `CGO_ENABLED=0`, linker version injection, 13-target build matrix,
   ZIP archive names, executable names, renamed manifest, exact checksum name, checksum signature, draft
   releases, and disabled changelog.
3. Disable the signing pipe only in `make release-snapshot` with GoReleaser's `--skip=sign` flag while
   preserving release signing.
4. Ignore `/dist/` and run the snapshot verifier with `VERIFY_RELEASE_SIGNATURE=0`.
5. Update stale repository guidance that says release targets are not yet implemented.
6. Run GoReleaser v2.18.0 `check` and fix configuration errors without weakening the artifact contract.
7. Run `make release-snapshot` and confirm 13 archives plus the manifest pass unsigned verification.

## Task 4: Add the release workflow

**Files:**

- Create: `.github/workflows/release.yml`

1. Trigger only on pushed `v*` tags and grant only `contents: write`.
2. Group concurrency by tag ref without cancelling an in-progress release.
3. Check out full history and configure Go from `go.mod`.
4. Run the tag validator before any step references signing secrets.
5. Import `GPG_PRIVATE_KEY` using `GPG_PASSPHRASE` and expose its fingerprint to GoReleaser.
6. Run GoReleaser v2.18.0 with `release --clean` and the repository `GITHUB_TOKEN`.
7. Run the artifact verifier in its default signature-required mode after upload.
8. Pin each third-party action to the selected immutable commit SHA and document its release tag in a
   comment.
9. Run actionlint v1.7.12 against the workflow.

## Task 5: Verify and prepare for review

1. Run `scripts/test-validate-release-tag.sh`.
2. Run `scripts/test-verify-release-artifacts.sh`.
3. Run `make release-check` with GoReleaser v2.18.0.
4. Run `make release-snapshot` with GoReleaser v2.18.0.
5. Run actionlint v1.7.12, `make build`, `make test`, `make lint`, and `git diff --check`.
6. Run the commit-message, commit-diff, and whole-tree gitleaks scans with redaction.
7. Commit the implementation as `ci: build the release pipeline`.
8. Inspect the complete public diff and ignored local files before requesting push approval.
9. After explicit approval in the current turn, push and open an English pull request that closes Issue
   #4, lists verification commands, states that releases remain drafts, and states that no credentials are
   included.
