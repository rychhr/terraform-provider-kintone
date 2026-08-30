# Release Signing Key Operations

## Public contract and private records

This document describes the public signing identity and the controls required for release signing. Keep
maintainer-specific access records, escrow configuration, recovery locations, and completion evidence in
private maintainer records, not in this repository or public issues. Never include a private key,
passphrase, revocation certificate, or plaintext recovery export in Git, tickets, chat, workflow artifacts,
logs, or shell history.

## Production key identity

| Property | Value |
| --- | --- |
| UID | `terraform-provider-kintone release signing <786618+rychhr@users.noreply.github.com>` |
| Full fingerprint | `E94B0DA8102D1D1AB8A5D01E925F019641552B8E` |
| Algorithm | RSA 4096-bit |
| Expiration | None |
| Intended use | Sign release checksums only; no encryption or authentication use |
| Primary-key capability | `[SC]` (signing and primary-key certification) |

Compare the full fingerprint of a public-key export before registration or use. A Registry-displayed
Key ID is a registration read-back aid, not proof of full-fingerprint equality. Upload only the
ASCII-armored public key, never a private-key export or revocation certificate.
[HashiCorp's provider publishing documentation](https://developer.hashicorp.com/terraform/registry/providers/publishing)
defines the Registry artifact and public-key requirements.

## Recovery requirements

Private signing material must be accessible only to the designated release owner. Keep the private-key export,
its passphrase, and the revocation certificate separately protected. Maintain an independently encrypted
AES-256 offline recovery bundle and a separate recovery-passphrase record. Recovery must remain possible
when the primary credential service or device is unavailable; do not keep the only means of decrypting a
backup on that backup medium.

Verify recovery after creation or replacement and at least annually:

1. Decrypt the recovery bundle in an owner-controlled, permission-restricted temporary location.
2. Import only the private-key export into a new empty temporary keyring. Do not import a revocation
   certificate during a recovery-signing test. Enter local passphrases interactively, not through command
   arguments, environment variables, or files.
3. Compare the restored key's complete fingerprint with the intended production identity.
4. Sign a disposable payload and verify it in a separate temporary keyring containing only the public key.
5. Remove temporary plaintext, test files, and keyrings after confirming their exact paths. Ordinary file
   deletion is not proof of physical media sanitization; avoid synchronized, shared, or backed-up plaintext
   work locations.

Record the result privately. A successful restore exercise proves recovery of the tested backup, not that
a real release workflow or Registry download has been verified.

## Release authorization boundary

The signing job must use the dedicated `release` GitHub Environment. Configure the release owner as its
sole required reviewer, allow that owner to review their own release, disable administrator bypass, and
restrict deployments to the `v*` tag pattern. Keep `GPG_PRIVATE_KEY`
and `GPG_PASSPHRASE` as Environment secrets, not repository- or organization-level duplicates. Record the
actual reviewer assignment and access checks privately.

The read-only `validate` job checks the tag without signing secrets. Only its dependent Environment-bound
job has `contents: write`. After approval, that job imports the key and compares its normalized full
fingerprint with the expected value before GoReleaser runs. Do not print secret values or read them back
to demonstrate that they exist.

GitHub withholds Environment secrets until the required review succeeds.
[GitHub's secure-use reference](https://docs.github.com/en/actions/reference/security/secure-use) documents
this boundary. Approvers must independently check the tag, workflow revision, and intended release before
releasing secrets to a runner. Review-gated signing is effective only for workflow revisions that contain
the Environment binding and fingerprint guard.

## Normal rotation

1. Pause release approvals and ensure no release using the old key remains in flight. Keep signing paused
   throughout the workflow and secret transition.
2. Prepare a replacement on an owner-controlled system, record its full fingerprint, establish protected
   escrow and independent offline recovery, and verify restoration from both sources.
3. Register the replacement public key before signing a release with it. Retain historical Registry public
   keys so existing provider versions remain verifiable.
4. In one reviewed change, update `EXPECTED_GPG_FINGERPRINT` in `.github/workflows/release.yml`,
   `expected_fingerprint` in `scripts/test-release-workflow.sh`, and the public identity in this document.
   Update any other documentation that embeds the old identity. Preserve the mismatch guard; do not make
   it accept both keys merely to ease the transition. Run `make test-release` and merge the change before
   issuing a release tag for the replacement key.
5. Update the protected Environment's private-key and passphrase pair from the replacement material
   verified in step 2 while approvals remain paused. Do not read back or expose secret values.
6. Resume approvals only after the merged workflow trust anchor and intended Environment key agree.
   Approve the next intended release, verify the imported fingerprint and draft checksum signature, and
   inspect logs and assets before publishing. Do not publish a disposable provider version merely to test
   a key transition.

[HashiCorp's Registry FAQ](https://developer.hashicorp.com/terraform/registry/faq) describes replacement-key
registration and retention of historical public keys.

## Suspected compromise

Stop signing and block access to the affected Environment secrets. Preserve necessary evidence privately
without copying secret material into incident tickets or logs. Once revocation is authorized, use the
protected revocation certificate in an isolated owner-controlled keyring and distribute the resulting
revoked public key through the appropriate channels. This is an incident action, not a routine restore test.
Contact HashiCorp for Registry-specific guidance and rotate to a replacement key. Coordinate treatment of
historical Registry keys with HashiCorp rather than deleting them and breaking older provider versions.

## Preparation and release verification

Signing preparation requires documented access and recovery controls, the intended public-key registration,
the protected Environment configuration, and a reviewed, tested workflow merged to `main`.

Preparation does not authorize a tag push or release publication, and passing fixture tests is not evidence
of a production signature. For an actual release, separately confirm Environment approval, the full
imported fingerprint, binary detached checksum signatures, and the absence of secrets in logs and assets.
Verify draft assets with `scripts/verify-release-artifacts.sh` and a trusted public-only keyring before
publishing. After publication, verify the Registry-provided signing key and provider download. Keep these
operational results in private maintainer records.
