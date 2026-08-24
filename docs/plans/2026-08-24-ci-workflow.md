# CI Workflow Implementation Plan

> **For maintainers:** Implement this plan on `ci/implement-ci-workflow` from `main`.

**Goal:** Add pull-request and `main` branch CI for build, unit tests, lint, and generated-documentation
drift while retaining a deliberately disabled acceptance-test matrix.

**Architecture:** Keep each validation concern in a separate GitHub Actions job so failures are distinct and
independent work runs in parallel. Every Go job uses the repository's Go version and dependency checksum
files, and all executable behavior remains behind the existing Make targets.

**Tooling:** GitHub Actions, Go 1.26.4 from `go.mod`, golangci-lint v2.12.2, actionlint v1.7.12,
tfplugindocs v0.25.0, and Terraform CLI 1.0/1.15 for the disabled acceptance matrix.

---

## Task 1: Add the continuous integration workflow

**Files:**

- Create: `.github/actionlint.yaml`
- Create: `.github/workflows/ci.yml`

1. Run actionlint against the absent workflow and confirm it fails because the file does not exist.
2. Add `pull_request` and pushes to `main` as the only workflow triggers.
3. Restrict workflow permissions to `contents: read`.
4. Group concurrency by workflow and ref, cancelling only superseded pull-request runs.
5. Add 10-minute `build`, `test`, and `lint` jobs using `actions/checkout@v7` and
   `actions/setup-go@v7` with `go.mod` and `go.sum`.
6. Run `make build` and `make test` in their jobs.
7. Run `golangci/golangci-lint-action@v9` with golangci-lint pinned to v2.12.2 in the lint job.
8. Add a 15-minute `docs` job that runs `make docs`, rejects tracked changes below `docs/`, and rejects
   untracked files below `docs/`.
9. Add a 45-minute acceptance-test matrix for Terraform `1.0.*` and `1.15.*`, with `fail-fast: false`,
   `hashicorp/setup-terraform@v4`, `terraform_wrapper: false`, and `make testacc`.
10. Disable acceptance execution unconditionally with `if: ${{ false }}`. Do not configure credentials or
    the acceptance-test guard.
11. Add a path-scoped actionlint exception for the intentional constant-false condition so other workflow
    diagnostics remain active.
12. Run actionlint v1.7.12 and confirm the workflow is valid.

## Task 2: Generate and commit provider documentation

**Files:**

- Create: `docs/index.md`
- Preserve: `docs/design/`

1. Run `make docs` using the Makefile's tfplugindocs v0.25.0 pin.
2. Inspect the generated provider page for expected provider and schema content.
3. Re-run `make docs` and confirm `git status --porcelain --untracked-files=all -- docs/` is empty after
   the generated page is staged for comparison.
4. Confirm no existing design document changed.

## Task 3: Exercise CI failure detection

**Files:**

- Temporarily modify and restore: `docs/index.md`
- Temporarily create and remove: a malformed Go source file

1. Add a temporary tracked change to `docs/index.md` and confirm the workflow's tracked drift command exits
   non-zero, then restore the file.
2. Create a temporary untracked file below `docs/` and confirm the workflow's untracked-file command exits
   non-zero, then remove the file.
3. Create a temporary malformed Go source file and confirm `make build` exits non-zero, then remove it.
4. Run `make build` again and confirm the clean source tree builds.

## Task 4: Run local verification and security checks

1. Run `make build`.
2. Run `make test`.
3. Run `make lint` with golangci-lint v2.12.2.
4. Run `make docs` and confirm no documentation drift remains.
5. Run `git diff --check`.
6. Run `scripts/scan-commit-messages.sh origin/main..HEAD`.
7. Run `gitleaks git . --config .gitleaks.toml --redact --log-opts=origin/main..HEAD`.
8. Run `gitleaks dir . --config .gitleaks.toml --redact`.

## Task 5: Commit and prepare the pull request

1. Commit the plan, workflow, actionlint configuration, and generated documentation as
   `ci: add build test and lint workflow`.
2. Before requesting push approval, inspect `git status --porcelain`, the full public diff, ignored local
   environment files, and the proposed pull-request body.
3. Obtain explicit approval in the current turn before any push or implicit push from `gh pr create`.
4. After approval, push the branch and open an English pull request against `main` that references Issue
   #3, explains generated documentation and the skipped acceptance job, states that no credentials are
   configured, and lists the verification commands.
5. Confirm `build`, `test`, `lint`, `docs`, and `gitleaks` pass and `acceptance` is skipped.
6. After merge, verify `main`, update and close Issue #3, and leave Issue #18 unchanged.
