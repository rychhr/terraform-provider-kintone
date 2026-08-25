# Community Health Design

**Status:** Accepted

**Issue:** #6

## Scope

This repository publishes the community-health documents required for a public Terraform provider:
contribution guidance, a code of conduct, a security policy, and a changelog. The documents establish the
project's contribution and disclosure expectations without changing provider behavior, release automation,
or remote GitHub settings.

## Contribution guidance

`CONTRIBUTING.md` is the contributor entry point. It keeps the established public-interface naming
convention and adds local setup, the repository build, test, lint, and documentation commands,
acceptance-test safeguards, branch and commit conventions, pull-request expectations, English-only
repository artifacts, and secret-handling rules.

Contributors use an ignored `.env.local` file through `direnv` for development credentials and must never
commit or publish credentials, private signing material, or agent session links. Acceptance tests require
the dedicated development credentials and explicit opt-in described in the repository guidance; they must
not run against production and leave created kintone apps for manual cleanup.

## Conduct

`CODE_OF_CONDUCT.md` adopts Contributor Covenant 2.1, including its enforcement guidelines and attribution.
Reports use GitHub's Report content or Support path rather than a maintainer email address. This gives
reporters a private GitHub-operated route while allowing the repository to avoid publishing a personal
contact address.

## Security policy

`SECURITY.md` supports the `main` branch and the latest release. Its scope includes provider code,
credential handling, the HTTP client, release artifacts, and workflows. Vulnerabilities in kintone or
Terraform itself are outside this repository's scope and should be reported to their respective projects.

Security reports must be made privately through GitHub Private Vulnerability Reporting. Public issues, pull
requests, logs, and workflow artifacts must not contain credentials, access tokens, private keys, or other
secrets.

## Changelog policy

`CHANGELOG.md` follows [Keep a Changelog 2.0.0](https://keepachangelog.com/en/2.0.0/) and
[Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html). It begins with an empty `Unreleased`
section because this repository has no released implementation history to reconstruct. Future notable
changes are categorized for people reading the provider's release history.

## Issue intake and GitHub features

Issues and pull requests are public, English-language collaboration channels. They are appropriate for
reproducible defects, documentation, and feature proposals, but not for vulnerability reports or secrets.
Contributors should provide the relevant provider version, Terraform version, kintone environment details
without credentials, a minimal configuration where applicable, observed behavior, and expected behavior.

Dependabot is the selected dependency-update mechanism. Its configuration becomes active when
`.github/dependabot.yml` reaches the default branch.

GitHub Private Vulnerability Reporting is a separate repository setting. Before the default branch adopts
this policy, maintainers must enable Private Vulnerability Reporting and read the repository setting back as
`enabled: true`. Adding the security policy and Issue Form link does not enable that setting. This external
gate preserves the private reporting route required by `SECURITY.md` without inventing a competing fallback
channel.

## Verification

The implementation verifies that local Markdown links resolve, the policies agree on reporting and secret
handling, and the working-tree diff is whitespace-clean. The code of conduct, changelog, and versioning
references use their primary official sources.
