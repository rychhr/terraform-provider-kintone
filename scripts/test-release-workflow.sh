#!/bin/sh

set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
workflow="$script_dir/../.github/workflows/release.yml"
expected_fingerprint='E94B0DA8102D1D1AB8A5D01E925F019641552B8E'

fail() {
	printf 'release workflow test failed: %s\n' "$1" >&2
	exit 1
}

job_block() {
	job_name=$1
	awk -v job_name="$job_name" '
		$0 == "  " job_name ":" { in_job = 1 }
		in_job && $0 ~ /^  [[:alnum:]_-]+:$/ && $0 != "  " job_name ":" { exit }
		in_job { print }
	' "$workflow"
}

assert_contains() {
	haystack=$1
	needle=$2
	description=$3
	printf '%s\n' "$haystack" | grep -F -- "$needle" >/dev/null || fail "$description"
}

assert_absent() {
	haystack=$1
	needle=$2
	description=$3
	if printf '%s\n' "$haystack" | grep -F -- "$needle" >/dev/null; then
		fail "$description"
	fi
}

[ -f "$workflow" ] || fail "release workflow is missing"

job_count=$(awk '
	/^jobs:$/ { in_jobs = 1; next }
	in_jobs && /^  [[:alnum:]_-]+:$/ { count++ }
	END { print count + 0 }
' "$workflow")
[ "$job_count" -eq 2 ] || fail "workflow must contain exactly two jobs"

validate_job=$(job_block validate)
release_job=$(job_block release)

[ -n "$validate_job" ] || fail "validate job is missing"
[ -n "$release_job" ] || fail "release job is missing"

assert_contains "$validate_job" 'contents: read' 'validate job must have contents: read permission'
assert_contains "$validate_job" 'scripts/validate-release-tag.sh "$GITHUB_REF_NAME" origin/main' 'validate job must validate the release tag'
assert_absent "$validate_job" 'environment:' 'validate job must not use an environment'
assert_absent "$validate_job" 'secrets.' 'validate job must not reference secrets'

assert_contains "$release_job" 'needs: validate' 'release job must depend on tag validation'
assert_contains "$release_job" 'contents: write' 'release job must have contents: write permission'
assert_contains "$release_job" 'environment: release' 'release job must use the release environment'
assert_contains "$release_job" 'secrets.GPG_PRIVATE_KEY' 'release job must import GPG_PRIVATE_KEY'
assert_contains "$release_job" 'secrets.GPG_PASSPHRASE' 'release job must import GPG_PASSPHRASE'
assert_contains "$release_job" "EXPECTED_GPG_FINGERPRINT: $expected_fingerprint" 'release job must record the production fingerprint'
assert_contains "$release_job" 'IMPORTED_GPG_FINGERPRINT: ${{ steps.import_gpg.outputs.fingerprint }}' 'release job must read the imported key fingerprint'
assert_contains "$release_job" "tr -d '[:space:]'" 'release job must remove fingerprint whitespace before comparison'
assert_contains "$release_job" "tr '[:lower:]' '[:upper:]'" 'release job must normalize fingerprint case before comparison'
assert_contains "$release_job" 'imported_fingerprint=$(normalize_fingerprint "$IMPORTED_GPG_FINGERPRINT")' 'release job must normalize the imported fingerprint'
assert_contains "$release_job" 'expected_fingerprint=$(normalize_fingerprint "$EXPECTED_GPG_FINGERPRINT")' 'release job must normalize the expected fingerprint'
assert_contains "$release_job" '"$imported_fingerprint" != "$expected_fingerprint"' 'release job must reject an unexpected imported fingerprint'

secret_references=$(grep -nE 'secrets\.GPG_(PRIVATE_KEY|PASSPHRASE)' "$workflow" || true)
secret_reference_count=$(printf '%s\n' "$secret_references" | sed '/^$/d' | wc -l | tr -d '[:space:]')
[ "$secret_reference_count" -eq 2 ] || fail 'only the release job may reference signing secrets'

fingerprint_check_line=$(grep -nF '"$imported_fingerprint" != "$expected_fingerprint"' "$workflow" | cut -d: -f1)
goreleaser_line=$(grep -nF 'goreleaser/goreleaser-action@' "$workflow" | cut -d: -f1)
[ -n "$fingerprint_check_line" ] || fail 'fingerprint comparison is missing'
[ -n "$goreleaser_line" ] || fail 'GoReleaser action is missing'
[ "$fingerprint_check_line" -lt "$goreleaser_line" ] || fail 'fingerprint comparison must run before GoReleaser'

printf '%s\n' 'release workflow tests passed'
