# Release Signing Key Implementation Plan

**Goal:** Establish an owner-controlled signing key, a recoverable escrow model, and a review-gated release
path for Terraform Registry artifacts.

**Architecture:** Repository documentation fixes the public trust anchor and the required operational
boundaries. Owner-only systems retain the private material, protect release secrets behind a GitHub
Environment approval, and register only the public key with the Terraform Registry.

**Spec:** `docs/specs/2026-08-25-release-signing-key-design.md`

## Completed repository work

This task has recorded the production key contract, owner-only trust model, primary 1Password escrow,
independent offline recovery-copy requirement, restore-verification procedure, Environment boundary,
Registry registration prerequisite, rotation and compromise process, and first-release completion gate.
It also links the operations runbook from contributor and agent guidance and explicitly prohibits committing
or publishing the private key, passphrase, and revocation certificate.

The repository release workflow now binds its release job to the `release` Environment. Its read-only
`validate` predecessor validates tags before signing secrets are available, and the release job checks the
exact normalized production fingerprint before GoReleaser runs.

The external Environment and Registry public-key registration are also complete. The Environment's measured
configuration has sole reviewer `rychhr` (GitHub ID `786618`), `prevent_self_review=false`,
`can_admins_bypass=false`, a `v*` tag deployment policy, and secret names `GPG_PRIVATE_KEY` and
`GPG_PASSPHRASE`; secret values were not read back. The Registry long GPG Key ID
`925F019641552B8E` was checked against the final 16 hexadecimal digits of the verified full public-export
fingerprint. These completed controls do not complete the offline recovery, workflow-merge, or release
checkpoints.

## Remaining owner operations

1. Create and verify the independently stored AES-256 encrypted offline recovery copy, retaining its
   recovery passphrase in a separate recovery record. Do not record physical storage locations in the
   repository.
2. Before the first release, generate a disposable test tag only if the release process specifically
   authorizes it, approve the Environment request as owner, and confirm the resulting checksum signature is
   made by the expected fingerprint. Do not publish or push a tag without the explicit approval required by
   repository policy.

## Repository workflow contract

The release job is bound to the `release` Environment and depends on a secrets-free, read-only tag-validation
job. After approval makes Environment secrets available, it imports the signing key and rejects a normalized
full fingerprint that differs from the production contract before invoking GoReleaser.

The workflow commit must be merged before this control can protect an actual release. Keep the workflow
secret names aligned with the Environment: `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE`.
Do not duplicate them as repository or organization secrets.

Update the design and runbook in the same change whenever the active fingerprint, escrow structure, or
release authorization boundary changes. Preserve tag validation before secret access, binary detached-signature
verification, and the prohibition on printing secret values.

## Rotation and incident readiness

Normal rotation stages and verifies a replacement key before use, uploads its public key to the Registry,
updates the protected Environment secrets, and keeps prior Registry public keys available for existing
releases. A suspected compromise stops signing, blocks access to the affected Environment secrets, uses the
revocation and Registry-contact procedure in the runbook, and replaces the key rather than trying to reuse
the affected private material.

An annual owner audit confirms the key identity and fingerprint, item separation and owner-only vault access,
offline-recovery readability, the sole Environment reviewer, disabled administrator bypass, secret scope,
Registry key presence, and the restore-verification exercise. The audit records only dates and pass/fail
conclusions in an owner-controlled record; it never records secrets or physical storage locations.

## First-release gate

No real provider release is authorized by this plan alone. The owner must complete the offline recovery
operation, the Environment-bound workflow must be merged and verified, and the draft artifact must pass
signature verification before a human publishes it. The offline recovery and first real release remain
incomplete until the owner explicitly performs and verifies them.

## References

- [GitHub Actions secure-use reference](https://docs.github.com/en/actions/reference/security/secure-use)
- [Publish providers to the Terraform Registry](https://developer.hashicorp.com/terraform/registry/providers/publishing)
- [Terraform Registry FAQ: changing GPG keys](https://developer.hashicorp.com/terraform/registry/faq)
