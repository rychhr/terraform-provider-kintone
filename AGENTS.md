# Repository Guidelines

## Overview

Terraform provider for kintone, published to the Terraform Registry as `rychhr/kintone`. Built on
terraform-plugin-framework (plugin protocol v6) and licensed under MPL-2.0.

**Status: pre-implementation.** The repository holds tooling configuration and a provider skeleton that
compiles — no resources, data sources, or provider configuration yet. It is a clean
rewrite of a private proof-of-concept: no code is copied across. What carries over is verified knowledge of
the kintone REST API and the singleton-settings-resource design, restated in English.

v0.1.0 is deliberately a minimal core, so that the Registry publishing path is exercised end to end before
feature work begins:

- provider configuration (password authentication and API-token authentication)
- `kintone_app` — the app itself and its general settings
- `data.kintone_app` / `data.kintone_apps`

Later minor releases add form fields and layout, views, process management, ACLs, and notification
resources. Each minor gets its own design document before implementation starts.

Every artifact in this repository — code comments, diagnostics, documentation, commit messages, issues, and
pull requests — is written in English. Chat responses to the user are in Japanese.

## Commands

The `GNUmakefile` is ported from the proof-of-concept. `release-check` and `release-snapshot` fail until
`.goreleaser.yaml` and `scripts/verify-release-artifacts.sh` land; the other targets work today.

| Command | Purpose |
| --- | --- |
| `make build` | `go build -v ./...` |
| `make test` | unit tests (`go test -v -count=1 ./...`) |
| `make testacc` | acceptance tests (`TF_ACC=1`, 30m timeout, `./internal/provider/`) |
| `make lint` | `golangci-lint run` |
| `make docs` | regenerate provider documentation with `tfplugindocs` |
| `make release-check` | `goreleaser check` |
| `make release-snapshot` | local snapshot build, then `scripts/verify-release-artifacts.sh dist` |

Run a single test with `go test -v -count=1 -run 'TestName' ./internal/kintone/`.

## Architecture

Two strictly separated layers:

- `internal/kintone/` — Terraform-independent HTTP client for the kintone REST API, hand-rolled on
  `net/http` and unit-tested with `httptest`. All API call-order logic (preview write → deploy → poll)
  lives here, never in the provider layer.
- `internal/provider/` — plugin-framework glue: provider definition, resources, and data sources.

Two consequences that are easy to get wrong:

- The `kintone_` prefix on resource type names comes from `resp.TypeName = "kintone"` in the provider
  definition, not from the Registry address.
- Build the shared foundation for settings resources **first**, then layer individual resources on top. The
  proof-of-concept duplicated preview GET/PUT boilerplate across every settings resource and only partially
  factored it out afterwards. Inverting that order is a deliberate design change, not an accident.

Settings that exist at most once per app are modeled as standalone singleton resources whose import ID is
the app ID. That design worked in the proof-of-concept and is kept.

Resource and attribute names cannot be changed once published to the Registry. `CONTRIBUTING.md` will hold the
naming convention; decide names against that convention rather than case by case.

## kintone API constraints that shape the code

These are non-negotiable properties of the kintone API. Every resource must respect them.

1. **Two-phase writes.** App settings are written to the *preview* environment, then deployed via
   `POST /k/v1/preview/app/deploy.json` and polled until `PROCESSING` becomes `SUCCESS` or `FAIL`. Reads
   for drift detection use the *live* environment as the source of truth.
2. **No app deletion API.** A resource `Delete` must only remove state and warn about manual cleanup. Never
   implement physical deletion.
3. **App creation requires password authentication** (`X-Cybozu-Authorization: base64(login:password)`);
   API tokens cannot create apps. Users with 2FA enabled cannot authenticate this way, so a dedicated
   service account is assumed.
4. **Concurrency.** Operations on the same app must be serialized by a per-app mutex. The client retries
   429 responses with exponential backoff for all methods. It retries 5xx responses, transport errors, and
   response-read errors only for idempotent methods, so that replaying a create-style POST cannot duplicate
   side effects.

Keep API IDs and revisions as strings — kintone returns numeric values as JSON strings.

## Setting-resource state safety

For settings resources backed by preview GET/PUT APIs:

- Treat a Terraform stable key, the current API update key, and a planned name or code as three distinct
  identities. Test that a rename preserves the stable key through the immediate read-back.
- When a PUT requires a complete nested object, preserve unknown `Optional + Computed` child values from
  prior state or from the API. Never collapse null or unknown values into zero values, and cover
  sibling-only updates with a regression test.
- Determine destructive transitions from their semantic before/after values rather than from the state
  representation alone. This includes compatibility forms such as `options` and stable option settings.
- Before the first mutation, validate rename targets against the current preview state and against the
  mutation order. A later planned deletion does not free a target needed by an earlier update — stage the
  changes safely or reject the plan.

## Testing

Unit tests use `httptest` for isolated client and CRUD coverage. Acceptance tests use
`terraform-plugin-testing` and require `TF_ACC=1` plus dedicated development credentials
(`KINTONE_DEV_BASE_URL`, `KINTONE_DEV_USERNAME`, `KINTONE_DEV_PASSWORD`) and the explicit guard
`KINTONE_DEV_ALLOW_ACCEPTANCE_TESTS=1`. The test harness maps those development credentials onto the
provider's `KINTONE_BASE_URL`, `KINTONE_USERNAME`, and `KINTONE_PASSWORD` inside the test process and never
falls back to generic credentials.

Acceptance tests create real apps. Prefix every acceptance-test app name with `tfacc-`, and because kintone
has no app deletion API, record the created app names and report them for manual cleanup. Never run
acceptance tests against production, and never print credentials in logs or diagnostics.

## Local development against an unreleased build

`dev_overrides` derives the binary name from the **last element of the source address**: Terraform looks for
an executable named `terraform-provider-<TYPE>`, where `<TYPE>` is that last element. For the address
`rychhr/kintone` the required binary name is therefore `terraform-provider-kintone`. Keep the address and
the binary name in sync whenever build instructions change.

Credentials are loaded from an ignored `.env.local` through `direnv`; the repository will provide an
`.env.example` template. Never commit credentials. Use a dedicated kintone service account, since password
authentication is required for app creation.

## Release and Registry requirements

- `.goreleaser.yaml` must set `project_name: terraform-provider-kintone` explicitly. GoReleaser derives
  `ProjectName` from the **repository name**, not from the Go module path, so without an explicit setting a
  repository rename silently changes artifact names and breaks release verification.
- `scripts/verify-release-artifacts.sh` must expect `terraform-provider-kintone_*` artifact names.
- The build matrix follows terraform-provider-scaffolding-framework: `goos` of freebsd, windows, linux, and
  darwin against `goarch` of amd64, 386, arm, and arm64, with `darwin/386` and `darwin/arm` excluded via
  `ignore` because Go does not support them. Writing the full cross product without those exclusions breaks
  the build.
- Releases are created as drafts. Verify the assets with `scripts/verify-release-artifacts.sh` before
  publishing.

The Registry requires these assets per release:

| Asset | Format |
| --- | --- |
| Binary archive | `terraform-provider-kintone_{VERSION}_{OS}_{ARCH}.zip`, containing a binary named `terraform-provider-kintone_v{VERSION}` |
| Checksums | `terraform-provider-kintone_{VERSION}_SHA256SUMS` |
| Signature | `terraform-provider-kintone_{VERSION}_SHA256SUMS.sig` — a binary detached signature, *not* ASCII armored |
| Manifest | `terraform-provider-kintone_{VERSION}_manifest.json` |

`terraform-registry-manifest.json` declares `version: 1` and `metadata.protocol_versions: ["6.0"]`.

Never place a private signing key or its passphrase in the repository, an issue, a pull request, a workflow
artifact, or a log.

## Workflow

- Never push directly to `main`. Work on a branch and open a pull request.
- Name branches `<type>/<short-kebab-case-summary>` using the Conventional Commit type that best describes
  the change (`feat/`, `fix/`, `docs/`, `test/`, `refactor/`, `chore/`, `ci/`, `build/`, `perf/`). Describe
  the change, not the author or the tool — no `agent/` prefixes, usernames, or agent names.
- Write commit messages in English using Conventional Commits. Keep commits focused.
- **Never publish an agent session link.** No `https://claude.ai/code/session_...` URL and no
  `Claude-Session:` trailer belongs in a commit message, a pull request body or comment, an issue body or
  comment, or a file in this repository. Agent tooling adds these by default; strip them. They name the tool
  rather than the change, for the same reason branch names may not, and this repository is public — a link
  published once stays retrievable by SHA even after the text is edited.
- Pull requests and issues are written in English. A pull request should summarize the behavior change,
  reference the relevant issue or plan task, list the verification commands that were run, and call out
  credentials, manual cleanup, generated documentation, or state-migration effects. Include HCL examples
  when schemas or resource behavior change.
- Specs and plans are conclusion-based documents. Never record review-fix history or round-by-round
  changelogs inside them; log each review round's resolution as a pull request comment (what changed, plus
  the commit hash) and squash the branch into logical commits before merge.
- Settle a review finding that hinges on unverified kintone API behavior by measuring it against the
  development environment first (a disposable `tfacc-` app and a small script), then record the measured
  fact in the document or code comment — never the guess.

## Pushing to a public repository

This repository is public, and a push is irreversible in practice. Once an object reaches GitHub it stays
retrievable by its SHA even after the branch is deleted, the commit is amended, or the history is
rewritten, and it may already have been copied by forks, the API, or search indexes. Rotating an exposed
credential is the only real remedy; removing the commit is not one.

- **Never push automatically.** Obtain the user's explicit approval in the current turn before running
  `git push` — including the first push of a new branch, a `--force` push, a tag push, and any command that
  pushes implicitly, such as `gh pr create` on an unpushed branch. Permission to edit files, an approval
  granted for an earlier push, or a command sandbox approval does not authorize a new push.
- Before asking for that approval, review exactly what would become public: run `git status --porcelain`
  and `git diff --cached`, and confirm the change set carries no credential, internal planning document,
  private repository name, local absolute path, environment-specific hostname, or agent session link. Verify
  that files excluded by `.gitignore` — `.env.local` above all — are still untracked rather than assuming
  they are.
- The same review covers the text that accompanies the push. Before `gh pr create`, `gh pr edit`, or
  `gh issue edit`, grep the body you are about to publish for `claude.ai/code/session` and remove what you
  find.
- Review every file, not just the diff, before the first push of the repository. A file that was safe while
  the repository was empty is not automatically safe once it is published.

## Remote infrastructure change safety

Read-only commands such as `terraform plan` may be run within the task scope. Before running
`terraform apply`, `terraform destroy`, or any other command that changes remote infrastructure, present the
target environment, the affected resources, and the planned changes, then obtain the user's explicit
approval in the current turn. Permission to edit source files, a previous apply approval, or a command
sandbox approval does not authorize a new remote change.
