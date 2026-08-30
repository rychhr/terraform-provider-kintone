# Release Signing Key Design

## Scope

Define the trust and authorization boundaries for the OpenPGP key used to sign Terraform Registry release
checksums. The public identity and reusable operating requirements are in the
[operations contract](../operations/release-signing-key.md). This design contains no maintainer progress
log, credential-service configuration, or private recovery material.

## Trust model

Only the designated release owner may access private signing material or approve its release to a runner. The
private key, its passphrase, and its revocation certificate are independently protected recovery material;
none belongs in Git, issues, pull requests, workflow artifacts, or logs. An independently encrypted offline
backup and separately controlled decryption records protect against loss of the primary credential service
or device.

Recovery verification imports a private-key export into a fresh, restricted keyring, compares its complete
fingerprint, signs a disposable payload, and verifies that signature from a separate public-only keyring.
It does not apply the revocation certificate. Temporary plaintext and keyrings are removed afterwards;
ordinary deletion does not establish physical media sanitization. Operational evidence is recorded privately.

## Release authorization

The workflow's `validate` job has read-only permissions and no Environment or signing-secret references.
It validates the version tag and its ancestry before the signing job may proceed.

The dependent `release` job references the protected `release` Environment and has `contents: write`.
The release owner is the sole required reviewer, with self-review allowed, controlling access to
Environment secrets named `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE`. Administrator bypass must be disabled
and deployment policy restricted to `v*` tags. Actual reviewer
assignments and configuration evidence belong in private access records.

After import, the workflow normalizes whitespace and letter case and compares the full imported fingerprint
against its reviewed production trust anchor. A mismatch exits before GoReleaser. The regression suite
independently pins the expected identity and rejects incorrect import wiring and ordering. Public key IDs
are useful registration metadata, but are not a replacement for full-fingerprint comparison.

GitHub withholds Environment secrets until required approval succeeds.
[GitHub's secure-use guidance](https://docs.github.com/en/actions/reference/security/secure-use) defines
this boundary. The workflow never interprets a successful local test or pull-request merge as release
approval.

## Rotation and incident boundaries

A replacement must have verified escrow and offline recovery before use. Register its public key before
signing with it. Keep release approvals paused while coordinating the workflow's fingerprint, the regression
test's expected fingerprint, public identity documentation, and Environment secret pair. Merge the reviewed
trust-anchor change before tagging a release with the replacement; never remove the mismatch check to make
rotation pass. The operations contract specifies the order and post-transition verification.

Retain historical Registry public keys for existing provider versions. For suspected compromise, stop
signing, block affected secret access, preserve minimal private evidence, and follow the authorized
revocation and HashiCorp-contact process before resuming with a replacement. Revocation is not part of a
routine restoration test.

## Verification boundaries

`make test-release` runs workflow guard and mutation checks, tag-validator tests, and artifact-verifier
tests locally and in CI without production secrets. The artifact tests use disposable keys and verify the
required binary detached-signature format.

Signing readiness consists of completed access, recovery, and registration checks plus the tested,
Environment-bound workflow merged to `main`. Actual release verification is a separate gate: approve the
intended workflow, confirm the imported fingerprint, inspect and verify draft assets and logs, then verify
Registry signing-key metadata and provider installation after publication. Neither gate substitutes for
the other.

## References

- [Publish providers to the Terraform Registry](https://developer.hashicorp.com/terraform/registry/providers/publishing)
- [Terraform Registry FAQ](https://developer.hashicorp.com/terraform/registry/faq)
- [GitHub Actions secure-use reference](https://docs.github.com/en/actions/reference/security/secure-use)
