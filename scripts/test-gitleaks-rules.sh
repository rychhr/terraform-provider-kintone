#!/usr/bin/env bash
#
# Regression test for .gitleaks.toml.
#
#     scripts/test-gitleaks-rules.sh
#
# Asserts that the ruleset catches what it is there to catch and leaves alone
# what it must leave alone. The fixtures are assembled from fragments at run
# time and live in a temporary directory: a file in this repository holding a
# real-form session link or a real-form access key would be flagged by the very
# scanner under test.
#
# `AKIAIOSFODNN7EXAMPLE` is allowlisted in the gitleaks default configuration
# and is useless as a fixture, hence the made-up key below.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
config="${repo_root}/.gitleaks.toml"

work_dir="$(mktemp -d)"
trap 'rm -rf "${work_dir}"' EXIT

failures=0

pass() { printf 'ok   %s\n' "$1"; }
fail() {
	printf 'FAIL %s\n' "$1"
	failures=$((failures + 1))
}

# Runs gitleaks over a path and checks the exit status against what is expected.
# expect_hit=1 means the scan must report a leak.
expect_scan() {
	local name="$1" expect_hit="$2" path="$3"
	local status=0
	gitleaks dir "$path" --config "$config" --redact --verbose --no-banner || status=$?
	if [ "$status" -eq "$expect_hit" ]; then
		pass "$name"
	else
		fail "$name (exit ${status}, expected ${expect_hit})"
	fi
}

session_link="https://claude.ai/code/""session_""0123456789abcdefABCDEF"
mixed_case_link="https://Claude.AI/code/""session_""0123456789abcdefABCDEF"
# The default `aws-access-token` rule matches `AKIA` followed by sixteen
# characters from the base32 alphabet, so the body below avoids 0, 1, 8 and 9.
access_key="AKIA""Z3K5QW2M7X4T6L5V"

mkdir -p "${work_dir}/content"
printf 'notes\n\nSee %s for the session.\n' "$session_link" \
	>"${work_dir}/content/session-link.txt"
expect_scan "session link in file content is detected" 1 "${work_dir}/content"

mkdir -p "${work_dir}/mixed-case"
printf 'See %s for the session.\n' "$mixed_case_link" \
	>"${work_dir}/mixed-case/session-link.txt"
expect_scan "a link written with a mixed-case host is detected" 1 "${work_dir}/mixed-case"

mkdir -p "${work_dir}/aws"
printf 'aws_access_key_id = %s\n' "$access_key" >"${work_dir}/aws/credentials"
expect_scan "non-example access key is detected by the default ruleset" 1 "${work_dir}/aws"

# The forms AGENTS.md quotes while stating the rule must not match, or every
# commit touching AGENTS.md would be rejected. The short illustrative id pins
# the `{10,}` floor; the placeholder pins the character class.
mkdir -p "${work_dir}/quoted"
# The backticks below are markdown code spans copied from AGENTS.md, not command
# substitution, so the single quotes are deliberate.
# shellcheck disable=SC2016
{
	printf -- '- **Never publish an agent session link.** No `https://claude.ai/code/session_...` URL and no\n'
	printf -- '  `Claude-Session:` trailer belongs in a commit message.\n'
	printf -- '  Grep the body for `claude.ai/code/session` before publishing.\n'
	printf -- '  An id is written `claude.ai/code/session_ID` in examples.\n'
} >"${work_dir}/quoted/agents-excerpt.md"
expect_scan "the placeholder forms quoted in AGENTS.md are not flagged" 0 "${work_dir}/quoted"

# Only tracked content, because `gitleaks dir` does not honour .gitignore and
# an ignored .env.local holds real development credentials. A fresh checkout is
# also what the CI content scan sees.
tracked_dir="${work_dir}/tracked"
mkdir -p "$tracked_dir"
git -C "$repo_root" archive HEAD | tar -x -C "$tracked_dir"
expect_scan "the committed tree is clean" 0 "$tracked_dir"

# A message-only hit: no file in the scratch tree carries the link, so only the
# commit-message scan can see it. scan-commit-messages.sh resolves commits in
# the current repository, so it runs from inside the scratch repository.
msg_repo="${work_dir}/message-repo"
mkdir -p "$msg_repo"
git init --quiet "$msg_repo"

# The identity and signing overrides keep the test independent of whatever
# global git configuration the machine carries; commit.gpgsign would otherwise
# fail or block on a passphrase prompt.
scratch_commit() {
	printf '%s\n' "$1" >>"${msg_repo}/file.txt"
	git -C "$msg_repo" add file.txt
	git -C "$msg_repo" \
		-c user.name=test -c user.email=test@example.invalid \
		-c commit.gpgsign=false \
		commit --quiet --no-verify -m "$2"
}

scratch_commit "line 1" "chore: base commit"
base="$(git -C "$msg_repo" rev-parse HEAD)"
scratch_commit "line 2" "chore: an ordinary commit

Nothing forbidden in this message."
clean="$(git -C "$msg_repo" rev-parse HEAD)"
scratch_commit "line 3" "chore: a commit that leaks

Claude-Session: ${session_link}"

scan_commits() {
	local name="$1" expected="$2"
	shift 2
	local status=0
	(cd "$msg_repo" && GITLEAKS_CONFIG="$config" \
		"${repo_root}/scripts/scan-commit-messages.sh" "$@") || status=$?
	if [ "$status" -eq "$expected" ]; then
		pass "$name"
	else
		fail "$name (exit ${status}, expected ${expected})"
	fi
}

scan_commits "session link in a commit message is detected" 1 "${clean}..HEAD"
scan_commits "an ordinary commit message is not flagged" 0 "${base}..${clean}"
scan_commits "an empty range is not an error" 0 "${clean}..${clean}"
scan_commits "a root commit can be scanned on its own" 0 --max-count=1 "$base"
# A range that cannot be resolved must not look like a clean scan.
scan_commits "an unresolvable range is an error, not a pass" 2 \
	"0000000000000000000000000000000000000001..HEAD"

if [ "$failures" -ne 0 ]; then
	printf '\n%d assertion(s) failed.\n' "$failures" >&2
	exit 1
fi

printf '\nAll assertions passed.\n'
