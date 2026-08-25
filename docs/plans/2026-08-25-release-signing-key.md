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

This documentation task does not alter GoReleaser, the release workflow, GitHub settings, Terraform Registry
settings, or any release artifact. Its completion must not be interpreted as completion of an external owner
checkpoint.

## Owner-performed external operations

1. Create and verify the independently stored AES-256 encrypted offline recovery copy, retaining its
   recovery passphrase in a separate recovery record. Do not record physical storage locations in the
   repository.
2. Create the dedicated GitHub release Environment; add `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` only there;
   configure the owner as its sole required reviewer; disable `Allow administrators to bypass configured
   protection rules`; and read the reviewer, bypass setting, and secret names back. GitHub documents that
   required review protects Environment secrets from a job until approval, while administrators can bypass
   Environment protection rules unless that setting is disabled.
3. Register the ASCII-armored public key for fingerprint
   `E94B0DA8102D1D1AB8A5D01E925F019641552B8E` in Terraform Registry User Settings > Signing Keys, then
   read the registered public key and fingerprint back.
4. Before the first release, generate a disposable test tag only if the release process specifically
   authorizes it, approve the Environment request as owner, and confirm the resulting checksum signature is
   made by the expected fingerprint. Do not publish or push a tag without the explicit approval required by
   repository policy.

## Follow-on repository work

1. Bind the release job to the dedicated GitHub Environment and add regression coverage that rejects an
   imported signing key whose full fingerprint differs from the contract above.
2. Keep the workflow secret names aligned with the Environment: `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE`.
   Do not duplicate them as repository or organization secrets.
3. Review the release workflow after the Environment-binding changes. Validate the tag before the workflow
   imports secrets, preserve binary detached-signature verification, and ensure no workflow step prints
   secret values.
4. Update the design and runbook in the same change whenever the active fingerprint, escrow structure, or
   release authorization boundary changes.

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

No real provider release is authorized by this plan alone. The owner must complete every external operation,
the Environment-bound workflow must be merged and verified, and the draft artifact must pass signature
verification before a human publishes it. Registry registration, offline recovery, Environment approval, and
the first real release remain incomplete until the owner explicitly performs and verifies them.

## References

- [GitHub Actions secure-use reference](https://docs.github.com/en/actions/reference/security/secure-use)
- [Publish providers to the Terraform Registry](https://developer.hashicorp.com/terraform/registry/providers/publishing)
- [Terraform Registry FAQ: changing GPG keys](https://developer.hashicorp.com/terraform/registry/faq)
