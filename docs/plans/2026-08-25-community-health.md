# Community Health Implementation Plan

**Goal:** Publish the contribution, conduct, security, and changelog policies required for a healthy public
Terraform provider community.

**Architecture:** Keep day-to-day contributor requirements in `CONTRIBUTING.md`, adopt the standard
Contributor Covenant in a dedicated conduct document, define repository security support and private
reporting in a dedicated security policy, and maintain human-readable release notes in a dedicated
changelog. Record the policy decisions in the accompanying design specification.

**Spec:** `docs/specs/2026-08-25-community-health-design.md`

---

## Task 1: Add the community health documents

**Files:**

- Create: `docs/specs/2026-08-25-community-health-design.md`
- Create: `docs/plans/2026-08-25-community-health.md`
- Modify: `CONTRIBUTING.md`
- Create: `CODE_OF_CONDUCT.md`
- Create: `SECURITY.md`
- Create: `CHANGELOG.md`

1. Preserve the public-interface naming convention in `CONTRIBUTING.md` and add development setup,
   repository commands, acceptance-test safeguards, branch and Conventional Commit conventions,
   pull-request expectations, English-only artifacts, and secret-handling requirements.
2. Adopt Contributor Covenant 2.1 with attribution and direct conduct reports to GitHub Report content or
   Support rather than a maintainer email address.
3. Support `main` and the latest release in `SECURITY.md`; include provider code, credential handling, the
   HTTP client, release artifacts, and workflows; exclude kintone and Terraform vulnerabilities; require
   private reports and prohibit secrets in public collaboration channels and generated output.
4. Start `CHANGELOG.md` with an empty `Unreleased` section under the Keep a Changelog 2.0.0 and Semantic
   Versioning 2.0.0 policies. Do not reconstruct or invent release history.
5. Document the approved scope, issue intake, Dependabot, Private Vulnerability Reporting, and verification
   conclusions in the design specification.
6. Verify Markdown links and cross-document consistency, run `git diff --check`, review the complete diff,
   and commit the policy documents.

---

## Task 2: Add issue intake and dependency automation

**Files:**

- Create: `.github/ISSUE_TEMPLATE/bug_report.yml`
- Create: `.github/ISSUE_TEMPLATE/feature_request.yml`
- Create: `.github/ISSUE_TEMPLATE/config.yml`
- Create: `.github/dependabot.yml`

1. Add GitHub Issue Forms for bug reports and feature requests. Both forms collect the provider version,
   Terraform version, a minimal redacted configuration, and a required confirmation that no credentials are
   included. The bug form collects reproduction steps, expected behavior, actual behavior, and optional logs;
   it applies the existing `bug` label. The feature form collects the use case, proposed behavior,
   alternatives, and expected HCL; it applies the existing `enhancement` label.
2. Configure the Issue Form chooser to disable blank issues and link security reports to the repository's
   GitHub Private Vulnerability Reporting page. The link is not an alternative reporting channel and remains
   dependent on the separately enabled Private Vulnerability Reporting setting.
3. Configure Dependabot with weekly root-directory updates for `gomod` and `github-actions`, without
   grouping or auto-merge behavior. This automation becomes active when the configuration reaches the default
   branch.
4. Validate all four YAML files with Ruby's YAML parser, including the required Issue Form keys and unique
   input IDs, expected labels, credential confirmation, the disabled blank-issue setting and security link,
   and the two Dependabot entries.
5. Commit the GitHub configuration as `chore: add repository contribution automation`.

---

## Task 3: Verify the complete change set and adoption gate

1. Verify local Markdown links and cross-document consistency.
2. Run Ruby YAML validation for both Issue Forms, chooser configuration, and Dependabot configuration.
3. Run `git diff --check` and review the complete diff for whitespace errors and unintended files.
4. Run `make build`, `make test`, `make lint`, and `make docs`; after documentation generation, confirm there
   is no generated-documentation diff. Use task-specific `/tmp` Go caches if the sandbox blocks the default
   cache location. Do not run acceptance tests.
5. Run redacted gitleaks scans for the working tree and `origin/main..HEAD`, then scan the two commit
   messages in that range.
6. Before the default branch adopts this change, separately enable GitHub Private Vulnerability Reporting and
   read the setting back as `enabled: true`. This is an external GitHub-setting gate; it is not performed by
   the repository change, its merge, or Dependabot activation. Confirm after default-branch adoption that
   Dependabot is active and that Private Vulnerability Reporting remains enabled.
