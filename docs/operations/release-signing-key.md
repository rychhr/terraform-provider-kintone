# Release Signing Key Operations

## Purpose and non-negotiable boundary

This runbook is for the sole owner of the Terraform Registry release signing key. It records the operational
procedure and public trust anchor without containing any secret value or physical storage location. Do not
copy the private key, its passphrase, the revocation certificate, recovery passphrases, or any plaintext
export into Git, GitHub, chat, tickets, pull requests, workflow artifacts, logs, shell history, or a shared
vault.

## Production key identity

| Property | Value |
| --- | --- |
| UID | `terraform-provider-kintone release signing <786618+rychhr@users.noreply.github.com>` |
| Full fingerprint | `E94B0DA8102D1D1AB8A5D01E925F019641552B8E` |
| Algorithm | RSA 4096-bit |
| Expiration | None |
| Intended use | Sign release checksums only; no encryption or authentication use |
| Primary-key capability | `[SC]` (signing and the required primary-key certification capability) |
| Access | Owner only |

Always compare the full fingerprint, never a short key ID. The Terraform Registry requires the matching
public key and a signed release checksum file; its signing-key instructions require an ASCII-armored public
key at User Settings > Signing Keys. [HashiCorp's provider publishing documentation](https://developer.hashicorp.com/terraform/registry/providers/publishing)
is the authoritative Registry procedure.

## Generation or replacement-key preparation

Perform key generation only on an owner-controlled system. Create a new temporary directory, restrict it to
the owner, set `GNUPGHOME` to that directory, and use GnuPG's interactive full key-generation flow. Select
an RSA sign-only key, 4096 bits, no expiration, and this exact UID:

```text
terraform-provider-kintone release signing <786618+rychhr@users.noreply.github.com>
```

Enter the passphrase only at GnuPG's interactive prompt. Do not pass it through command-line arguments,
environment variables, shell input redirection, a file, or an automated generator. Confirm the public
listing has the expected sign/certification capability and record the full fingerprint before any export.
GnuPG's `--full-generate-key` is the interactive flow, and GnuPG creates a revocation certificate alongside
a generated key. [GnuPG OpenPGP key management](https://gnupg.org/documentation/manuals/gnupg/OpenPGP-Key-Management.html)
documents both operations.

For the current production key, the required fingerprint is
`E94B0DA8102D1D1AB8A5D01E925F019641552B8E`. A different fingerprint means a replacement key: do not add
it to the release Environment or Registry until it completes the rotation procedure below.

## Escrow and offline recovery

The current primary escrow is the owner's unshared Personal vault in 1Password. It has these three separate
items:

- one Password item holding only the GPG passphrase;
- one Secure Note holding only the private-key export; and
- one Secure Note holding only the revocation certificate.

Confirm that no person or group other than the owner can access that vault. People who can access a 1Password
vault can view and copy its items, so sharing the vault would break this model.
[1Password's vault-access documentation](https://support.1password.com/create-share-vaults/) describes the
access behavior.

Create an independent offline recovery copy by encrypting a private-key export with AES-256. Store its
recovery passphrase in a separately controlled recovery record. Do not place that passphrase in the same
1Password item, offline medium, or record as the encrypted copy. Do not document exact physical locations.
The recovery copy and primary escrow must be independent so a single service or device failure cannot
eliminate both.

After exporting, remove every plaintext export and the temporary GnuPG home directory by confirming their
exact temporary paths and deleting only those paths. Do not rely on a deletion command as proof of media
sanitization; avoid creating plaintext in synchronized, backed-up, shared, or public locations.

## Restore-verification exercise

Run this exercise after initial escrow, after an escrow or recovery change, and at least annually.

1. Create a fresh, permission-restricted temporary `GNUPGHOME`; verify that it contains no existing keyring.
2. Restore the private-key export from the escrow under test into that empty keyring. Enter the passphrase
   interactively. Do not put it in a command, environment variable, input file, or log.
3. List the imported public identity and compare its complete fingerprint exactly to
   `E94B0DA8102D1D1AB8A5D01E925F019641552B8E`. Stop on any difference.
4. Create a disposable test payload and make a detached test signature with the restored private key.
5. Create a third fresh temporary `GNUPGHOME`, import only an ASCII-armored public-key export, and verify
   the detached test signature there. The third keyring must not contain secret keys.
6. Confirm that the result identifies the expected full fingerprint and that verification succeeds.
7. Remove the disposable payload, signature, plaintext exports, and all temporary keyrings after checking
   each deletion target. Keep no test keyring for convenience.

This verification has passed for the current primary escrow: restoration into an empty keyring, full
fingerprint comparison, test signing, and verification from a third public-only keyring all succeeded. It
does not prove that the separate offline recovery copy exists or is readable; that is a separate owner
checkpoint.

## Configure the GitHub release Environment

The release job must use a dedicated GitHub Environment, not repository- or organization-level signing
secrets. Configure that Environment as follows:

1. Add `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` as Environment secrets only. Do not enter their values in a
   workflow file, issue, pull request, or log.
2. Add the owner as the required reviewer for the Environment and restrict deployment branches or tags to
   the release policy.
3. Read the Environment settings back and verify both the reviewer requirement and the two secret names.
4. Ensure the release job explicitly declares that Environment before it references either secret. Environment
   configuration alone does not protect a job that has not been bound to it.
5. Approve a release only after independently checking the tag, workflow revision, and intended release.

GitHub documents that a job cannot access Environment secrets until the required reviewer approves it, and
that secrets should be passed as inputs or environment variables rather than exposed in logs.
[GitHub's secure-use reference](https://docs.github.com/en/actions/reference/security/secure-use) and
[secret-use guidance](https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-secrets)
are authoritative. The repository workflow does not yet declare this Environment, so the approval boundary
is not active until the follow-on workflow change is merged and verified.

## Register the public key with Terraform Registry

1. In an isolated temporary keyring, export only the current public key in ASCII armor.
2. Compare the exported public key's full fingerprint with
   `E94B0DA8102D1D1AB8A5D01E925F019641552B8E`.
3. In Terraform Registry, open User Settings > Signing Keys and add the ASCII-armored public key.
4. Read the Registry entry back and compare its fingerprint with the full production fingerprint.
5. Delete the temporary public-key export and temporary keyring.

Never upload a private-key export or revocation certificate to the Registry. Registration has not been
claimed complete by this runbook; it remains an owner checkpoint before the first real release.

## Normal rotation

1. Generate a replacement in a fresh temporary `GNUPGHOME` using the generation procedure and record its
   new full fingerprint.
2. Establish separate owner-only 1Password items and a separately encrypted AES-256 offline recovery copy
   with a separate recovery-passphrase record.
3. Complete the restore-verification exercise for both the primary escrow and the offline recovery copy.
4. Add the replacement public key to Terraform Registry before using it. Keep older Registry public keys so
   consumers can verify historical releases.
5. Update the protected GitHub Environment secrets after reviewing the replacement fingerprint. Bind only
   the new pair for future releases.
6. Update this runbook and the design with the replacement UID and fingerprint in the same reviewed change.
7. Sign a controlled release only after the Environment approval and artifact verification succeed.

The Registry supports adding a replacement key and advises retaining prior keys for existing provider
versions. [HashiCorp's Registry FAQ](https://developer.hashicorp.com/terraform/registry/faq) is the
authoritative rotation guidance.

## Suspected compromise

1. Stop signing and block the affected GitHub Environment from releasing secrets. Remove or replace affected
   Environment secrets after preserving the minimum evidence needed for investigation.
2. Do not paste the private key, passphrase, revocation certificate, or recovery data into an incident issue,
   ticket, chat, workflow log, or commit.
3. Import the protected revocation certificate into an owner-controlled temporary keyring only when the
   compromise decision is confirmed, then publish the resulting revoked public key through the appropriate
   public distribution channel. GnuPG documents that a revocation certificate must be imported to revoke the
   key.
4. Contact HashiCorp for Registry guidance if the GPG key or GitHub account is compromised, then generate and
   rotate to a replacement key under this runbook.
5. Retain the historical Registry public key; removing it can prevent verification of old provider versions.

## Annual audit

At least annually, the owner records a date and pass/fail conclusion in an owner-controlled record and
checks all of the following without recording secret values or storage locations:

- the UID, full fingerprint, RSA 4096-bit sign-only/no-expiry contract, and `[SC]` capability;
- owner-only access and the continued separation of the three 1Password items;
- readability and independent placement of the AES-256 recovery copy and separate recovery-passphrase
  record;
- a fresh restore-verification exercise for the primary escrow and offline recovery copy;
- GitHub Environment secret scope, required owner approval, and release-job Environment binding;
- Terraform Registry public-key registration and historical-key retention; and
- this runbook's rotation and compromise contacts and procedures.
