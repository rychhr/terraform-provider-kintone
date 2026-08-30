# Release Signing Integration Plan

## Scope

Integrate a review-gated signing path without changing provider schemas, Terraform state, or release asset
names. This document records implementation boundaries, not maintainer progress or external configuration
evidence. The [design](../specs/2026-08-25-release-signing-key-design.md) explains the trust model; the
[operations contract](../operations/release-signing-key.md) describes recovery and key transitions.

## Workflow integration

Keep tag validation in a secrets-free job with read-only permissions. The dependent signing job references
the `release` Environment and has the write permission needed to create a draft release. After GPG import,
compare the normalized full fingerprint against the reviewed production trust anchor before invoking
GoReleaser. A mismatch stops the job without signing.

Preserve the release artifact contract: 13 platform archives, a manifest, SHA-256 checksums, and a binary
detached checksum signature. Post-build verification remains mandatory; it must not silently use the
unsigned-snapshot bypass for a tagged release.

## Regression checks

`make test-release` is the shared local and CI entry point for:

- workflow permission, Environment, secret-placement, import, and fingerprint-ordering checks, including
  mutations that must be rejected;
- tag validation using isolated Git repositories; and
- artifact verification using disposable signing keys and synthetic release archives.

These checks need no production credentials, publish no releases, and do not run acceptance tests against
kintone. Their fixtures cannot establish that production Environment secret values are usable.

## Documentation boundary

Publish the signing identity, workflow contract, validation commands, and reusable safety requirements.
Keep credential-service configuration, reviewer access evidence, recovery records, locations, and dated
completion checklists in private maintainer records. No secret material belongs in either kind of document.

## Readiness and release gates

Review and merge the Environment-bound workflow only after its guard tests and public-content review pass.
External access, recovery, and public-key registration checks establish signing readiness independently of
an actual release run.

A subsequent release requires explicit authorization for tag publication and Environment approval, draft
artifact and log inspection, and verification of the Registry-provided signing key and provider download.
Merging signing preparation does not waive those release gates or authorize publication.
