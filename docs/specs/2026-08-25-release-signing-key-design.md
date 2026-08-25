# Release Signing Key Design

**Status:** Accepted

**Issue:** #7

## Scope

This design establishes the trust and operating model for the OpenPGP key that signs Terraform Registry
release checksums. It documents the production key, escrow and recovery boundaries, verification evidence,
and the owner checkpoints required before the first real release. It does not change the release workflow,
create a GitHub Environment or Registry signing-key registration, store any secret material, or publish a
release.

## Trust model and key contract

One owner controls the production signing key. The private key, its passphrase, and its revocation
certificate are confidential recovery material; access to any one does not authorize sharing it, and none
belongs in Git, GitHub Issues, pull requests, workflow artifacts, logs, or a shared vault.

The key contract is fixed for the current production key:

- UID: `terraform-provider-kintone release signing <786618+rychhr@users.noreply.github.com>`
- Fingerprint: `E94B0DA8102D1D1AB8A5D01E925F019641552B8E`
- RSA 4096-bit primary key, no expiration
- release-checksum signing use, with the primary-key certification capability shown as `[SC]`; encryption
  and authentication use are out of scope

Terraform Registry requires a signed provider release and the matching public GPG key; it validates the
release signature and Terraform verifies the result during installation. The Registry accepts RSA signing
keys and requires the public key in ASCII armor at User Settings > Signing Keys.
[HashiCorp's publishing guidance](https://developer.hashicorp.com/terraform/registry/providers/publishing)
defines that contract.

## Escrow, recovery, and verification

The owner's Personal vault in 1Password is the primary escrow and remains owner-only. It contains three
separate items: a Password item for the passphrase, a Secure Note for the private key, and a Secure Note for
the revocation certificate. This separation prevents routine use of one item from disclosing every recovery
artifact. 1Password documents that people with vault access can view and copy its items, so the vault must
not be shared. [1Password's vault-access guidance](https://support.1password.com/create-share-vaults/)
explains that boundary.

An independently encrypted offline recovery copy protects against loss of the primary escrow. It is an
AES-256 encrypted copy of the private-key export; its recovery passphrase is stored in a separate recovery
record. The recovery copy, its passphrase record, and the 1Password escrow must not share a single failure
mode or reveal their exact physical locations in repository material.

Key generation and every restore check use a new, permission-restricted temporary `GNUPGHOME`, with the
passphrase entered interactively rather than supplied in a command line, environment variable, or file. A
restore is accepted only after importing into an empty keyring, matching the full fingerprint above, signing
a disposable test payload, and verifying that signature in a third keyring containing only the exported
public key. GnuPG documents both the creation of a revocation certificate and the requirement to import it
before it revokes a key. [GnuPG OpenPGP key management](https://gnupg.org/documentation/manuals/gnupg/OpenPGP-Key-Management.html)
describes those semantics.

The production key was generated and escrowed under this model. A restoration into an empty keyring,
fingerprint comparison, test signature, and verification from a separate public-only keyring have passed.
The independent offline recovery copy remains an owner checkpoint; this design does not claim that it is
complete.

## Release authorization boundary

The release workflow must consume `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` only as GitHub Environment
secrets in a dedicated release environment. That environment requires the owner as its sole reviewer and has
`Allow administrators to bypass configured protection rules` disabled; the release job must explicitly
reference it. GitHub does not make environment secrets available to a job until required review is approved,
so this is the approval boundary for releasing a private key to a runner. Administrators bypass Environment
protection rules by default, so disabling that option is required for the owner-only boundary.
[GitHub's secure-use guidance](https://docs.github.com/en/actions/reference/security/secure-use) and
[Environment configuration guidance](https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/manage-environments)
document those controls.

Creating the environment or secrets alone is insufficient: the workflow change that binds the release job to
the environment is separate repository work. Until both the external configuration and the workflow binding
are complete and reviewed, the approval boundary is not active.

## Registry registration, rotation, and compromise

The owner registers only the ASCII-armored public key with the Terraform Registry. Registry registration is
an external owner operation and is not complete merely because this design is merged. It precedes the first
real release.

For normal rotation, the owner generates and validates a replacement using this same model, creates its
escrow and independent recovery copy, uploads the replacement public key before signing a release with it,
updates the protected Environment secrets after approval, and records the new fingerprint in this design and
the runbook. Existing Registry public keys are retained so consumers can verify older releases.
[HashiCorp's Registry FAQ](https://developer.hashicorp.com/terraform/registry/faq) confirms that old public
keys should remain registered when a signing key changes.

For a suspected compromise, stop release signing, remove the affected GitHub Environment secrets or disable
the environment, preserve evidence without copying secrets into tickets or logs, use the protected
revocation certificate to revoke and publish the affected public key where applicable, contact HashiCorp for
Registry guidance, and rotate to a replacement key. HashiCorp directs maintainers to contact it after a GPG
key or GitHub-account compromise. The old Registry public key is not removed because consumers may still
need it for historical releases.

## First-release completion gate

The first real release remains blocked until the owner has completed and independently read back all of the
following: the offline AES-256 recovery copy and separate recovery-passphrase record; protected GitHub
Environment and its owner approval; workflow binding to that Environment; Terraform Registry public-key
registration; and the final release workflow and artifact verification. A successful local restore test and
the presence of repository documentation are necessary evidence, not substitutes for those owner operations.

## References

- [Publish providers to the Terraform Registry](https://developer.hashicorp.com/terraform/registry/providers/publishing)
- [Terraform Registry FAQ: changing GPG keys](https://developer.hashicorp.com/terraform/registry/faq)
- [GnuPG OpenPGP key management](https://gnupg.org/documentation/manuals/gnupg/OpenPGP-Key-Management.html)
- [GitHub Actions secure-use reference](https://docs.github.com/en/actions/reference/security/secure-use)
- [1Password vault access](https://support.1password.com/create-share-vaults/)
