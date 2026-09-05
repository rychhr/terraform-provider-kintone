# Contributing to the Terraform Provider for kintone

Thank you for contributing. Issues, pull requests, commit messages, documentation, and diagnostics are
written in English. Read [AGENTS.md](AGENTS.md) for the repository workflow, testing requirements, API
constraints, and safety rules. Consult the [secret-scanning procedure](docs/operations/secret-scanning.md)
before installing hooks or scanning changes, and the
[release artifact requirements](docs/operations/release-artifacts.md) before working on builds or releases.
Changes to release signing must preserve
the [public signing contract](docs/operations/release-signing-key.md).

## Development setup

Install the Go version declared in [go.mod](go.mod), Terraform CLI 1.16.0 or later, and the repository tools,
including `golangci-lint`, `tfplugindocs`, GoReleaser, gitleaks, and pre-commit. Install the repository
hooks once per clone:

```sh
pre-commit install --hook-type pre-commit --hook-type commit-msg
```

Development credentials belong in an ignored `.env.local` file loaded through `direnv`. Use a dedicated
kintone service account with only the permissions required for development. Never commit credentials, API
tokens, private signing keys, signing-key passphrases, revocation certificates, or agent session links.

## Build, test, lint, and documentation

The `GNUmakefile` provides the local and CI entry points:

| Command | Purpose |
| --- | --- |
| `make build` | Build the provider (`go build -v ./...`). |
| `make test` | Run unit tests (`go test -v -count=1 ./...`). |
| `make test-release` | Test release workflow guards, tags, and artifacts with disposable fixtures. |
| `make testacc` | Run acceptance tests. |
| `make lint` | Run `golangci-lint`. |
| `make docs` | Regenerate provider documentation with `tfplugindocs`. |
| `make release-check` | Validate the GoReleaser configuration. |
| `make release-snapshot` | Build and verify local release artifacts without a signature. |

`make test-release` requires Git, GnuPG, jq, zip, unzip, and shasum. CI runs the same target. It uses
temporary repositories and disposable signing keys, never production credentials or a kintone environment.

Run a single client test with:

```sh
go test -v -count=1 -run 'TestName' ./internal/kintone/
```

### Acceptance tests

Acceptance tests require explicit approval for the dedicated development environment before each run.
The guard alone is not approval. Never run against production, use generic provider credentials as a
fallback, or print credentials in HCL, logs, or diagnostics.

Prepare the ignored `.env.local` using [.env.example](.env.example):

1. Set `KINTONE_DEV_BASE_URL`, `KINTONE_DEV_USERNAME`, and `KINTONE_DEV_PASSWORD` for the dedicated
   development service account.
2. Select an existing, published app in that same environment exclusively for token-test reuse. Set its
   positive decimal ID as `KINTONE_DEV_TOKEN_APP_ID`. Supply its API tokens through
   `KINTONE_DEV_API_TOKENS` (comma-separated, at most nine), with permission to read app metadata and live
   general settings. Deploy any token configuration in advance through the normal approved setup process.
3. Before execution, present the target environment, expected app creations and updates, and manual
   cleanup needs, and obtain explicit approval under [AGENTS.md](AGENTS.md#remote-infrastructure-change-safety).
   Set `KINTONE_DEV_ALLOW_ACCEPTANCE_TESTS=1` only for that approved run.

For each approved CLI-version run, select Terraform 1.16.0 or the latest stable version on `PATH`, record
`terraform version`, and run `make testacc`. The test process rejects missing development configuration,
a missing or invalid token app ID, or a missing guard before starting the suite; it does not silently skip
individual live tests. `make testacc` sets `TF_ACC=1`. Ordinary local tests and CI use `TF_ACC=0` and local
HTTP fixtures; run `TF_ACC=0 make test` under both CLI versions to verify compatibility without live access.

Each successful full acceptance run creates one `tfacc-` app and performs two deployments: creation and
one combined description/number-precision update. Precision starts at `total_digits=12`,
`decimal_places=3`, and `rounding_mode=HALF_EVEN`; the update configures only `decimal_places=4` and checks
that the other values survive and the next plan is empty. Across both CLI-version runs, expect two new
apps and four deployments. Retain the reported app names and IDs for manual cleanup even when a run fails;
kintone has no app deletion API.

The API-token test reads `data.kintone_app` for the preselected app with password authentication excluded.
It checks the ID and retrieved settings without creating, updating, or deploying an app. Reuse that app for
later runs; the suite never deletes it. Record the exact CLI versions, results, and cleanup inventory after
execution. The CI acceptance job remains disabled until protected live-test automation is configured.

## Naming public provider interfaces

Resource types, data-source types, and schema attributes are public Terraform interfaces. After a name is
published, changing it breaks existing configurations and state, so choose names with these rules before
adding a schema. The design rationale and rejected alternatives are in
[the naming-convention design](docs/specs/2026-08-24-naming-convention-design.md).

### General form

- Use English, lowercase `snake_case` names.
- Write acronyms as lowercase words: `api`, `acl`, `id`, and `url`.
- Use nouns for managed payloads. Do not name a resource after an API operation or endpoint.
- Qualify a concept by placing its direct target first: `app_acl`, `record_acl`, and `field_acl`.

### Resource and data-source types

- Name a resource for one managed object in the singular. For example, the app resource is
  `kintone_app`.
- Name a singleton resource that manages a collection-wide payload in the plural. For example,
  `kintone_form_fields` would manage the complete set of fields for one app, not one field. A singleton
  means that the settings surface exists at most once per app; it does not make the managed payload
  singular.
- Name a data source for one object in the singular and a data source that returns a list in the plural.
  For example, use `data.kintone_app` and `data.kintone_apps`.
- The `kintone_` provider prefix is part of every public resource and data-source type. The remainder of
  the type name follows the rules above.

### Attribute names

- Use singular names for a single value and plural names for a collection.
- Name a reference identifier `<object>_id`, such as `space_id` or `thread_id`.
- Name an affirmative boolean state `*_enabled`, such as `comments_enabled`. Do not use negative forms
  such as `comments_disabled`.

### Resource boundaries

`kintone_app` owns app identity, create-only placement, and the app's general settings. Do not split one of
those concerns into another resource merely because the kintone API exposes a different endpoint.

An independently managed settings surface that exists at most once per app is a standalone singleton
resource. Its import ID is the app ID. Give a settings surface its own resource only when it has an
independent management boundary; a separate API endpoint alone is not sufficient.

Later-roadmap names in this document are illustrative only. They are not reserved public interfaces until
their own design work explicitly reserves them.

## v0.1.0 names reserved for Issue #11

Issue #11 must use the following public names for the v0.1.0 app schema:

- Managed resource: `kintone_app`
- Single data source: `kintone_app`
- List data source: `kintone_apps`
- Identity and lifecycle attributes: `id`, `name`, `description`, `space_id`, `thread_id`, `revision`
- General settings: `theme`, `title_field`, `number_precision`, `first_month_of_fiscal_year`
- Title-field children: `selection_mode`, `field_code`
- Number-precision children: `total_digits`, `decimal_places`, `rounding_mode`
- Feature toggles: `thumbnails_enabled`, `bulk_deletion_enabled`, `comments_enabled`,
  `record_duplication_enabled`, `inline_record_editing_enabled`

Provider configuration attributes are `base_url`, `username`, `password`, and `api_tokens`, as defined in
the [provider-layer design](docs/specs/2026-09-06-provider-layer-design.md). Data-source filter and output
attribute names must be chosen under this convention when their schemas are designed.

## Workflow

Create a branch named `<type>/<short-kebab-case-summary>`, where `<type>` is the Conventional Commit type
that best describes the change: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`, `ci`, `build`, or
`perf`. Describe the change, not the author or a tool; do not use agent names, usernames, or `agent/`
prefixes.

Write focused commit messages in English using the Conventional Commits format. Do not include agent session
links or session trailers in commits, pull requests, issues, or repository files.

Never push directly to `main`. Open a pull request from a branch. The pull request must be in English and
summarize the behavior change, reference the relevant issue or plan task, list verification commands, and
call out credentials, manual cleanup, generated documentation, and state-migration effects where relevant.
Include HCL examples when schemas or resource behavior change.

## Issues and pull requests

Use public issues for reproducible defects, documentation improvements, and feature proposals. Do not use
them to report vulnerabilities; follow [SECURITY.md](SECURITY.md) instead. Include the provider and
Terraform versions, a minimal configuration when applicable, the observed and expected behavior, and
environment details with all credentials and other sensitive values removed.

Before requesting review, run the checks appropriate to the change and inspect the complete diff. Regenerate
documentation with `make docs` when provider schemas or documentation inputs change. Pull requests must not
include generated secrets, local paths, environment-specific hostnames, private repository names, or
agent-session links.

## Secret handling

Keep `.env.local` and all credentials out of version control. Never place a private signing key, its
passphrase, or its revocation certificate in the repository, an issue, a pull request, a workflow artifact,
or a log. The [public signing contract](docs/operations/release-signing-key.md) describes workflow and
key-transition requirements; keep actual access, escrow, and recovery records private. Run the documented
gitleaks checks with `--redact`; `gitleaks dir` can read ignored local environment files.

If a scanner reports a genuine secret, remove it and rotate the credential. Do not silence the finding. A
false positive that must remain requires a documented repository-wide allowance as described in
[the secret-scanning procedure](docs/operations/secret-scanning.md#bypassing-a-hook).
