#!/bin/sh

set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
workflow=${RELEASE_WORKFLOW:-"$script_dir/../.github/workflows/release.yml"}
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

step_block() {
	job_name=$1
	step_name=$2
	awk -v job_name="$job_name" -v step_name="$step_name" '
		$0 == "  " job_name ":" { in_job = 1; next }
		in_job && $0 ~ /^  [[:alnum:]_-]+:$/ { exit }
		in_job && $0 == "      - name: " step_name { in_step = 1 }
		in_step && $0 ~ /^      - / && $0 != "      - name: " step_name { exit }
		in_step { print }
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
assert_absent "$validate_job" 'GPG_' 'validate job must not reference signing secrets'

assert_contains "$release_job" 'needs: validate' 'release job must depend on tag validation'
assert_contains "$release_job" 'contents: write' 'release job must have contents: write permission'
assert_contains "$release_job" 'environment: release' 'release job must use the release environment'

import_step=$(step_block release 'Import GPG key')
[ -n "$import_step" ] || fail 'release job must contain the GPG import step'
assert_contains "$import_step" 'id: import_gpg' 'GPG import step must have the import_gpg ID'
assert_contains "$import_step" 'uses: crazy-max/ghaction-import-gpg@2dc316deee8e90f13e1a351ab510b4d5bc0c82cd' 'GPG import step must use the pinned import action'
assert_contains "$import_step" 'gpg_private_key: ${{ secrets.GPG_PRIVATE_KEY }}' 'GPG import step must receive GPG_PRIVATE_KEY'
assert_contains "$import_step" 'passphrase: ${{ secrets.GPG_PASSPHRASE }}' 'GPG import step must receive GPG_PASSPHRASE'

fingerprint_step=$(step_block release 'Verify imported GPG fingerprint')
[ -n "$fingerprint_step" ] || fail 'release job must contain the fingerprint verification step'
assert_contains "$fingerprint_step" "EXPECTED_GPG_FINGERPRINT: $expected_fingerprint" 'fingerprint verification must record the production fingerprint'
assert_contains "$fingerprint_step" 'IMPORTED_GPG_FINGERPRINT: ${{ steps.import_gpg.outputs.fingerprint }}' 'fingerprint verification must read the import action fingerprint'
assert_contains "$fingerprint_step" "tr -d '[:space:]'" 'fingerprint verification must remove whitespace'
assert_contains "$fingerprint_step" "tr '[:lower:]' '[:upper:]'" 'fingerprint verification must normalize case'
assert_contains "$fingerprint_step" 'imported_fingerprint=$(normalize_fingerprint "$IMPORTED_GPG_FINGERPRINT")' 'fingerprint verification must normalize the imported fingerprint'
assert_contains "$fingerprint_step" 'expected_fingerprint=$(normalize_fingerprint "$EXPECTED_GPG_FINGERPRINT")' 'fingerprint verification must normalize the expected fingerprint'
assert_contains "$fingerprint_step" '"$imported_fingerprint" != "$expected_fingerprint"' 'fingerprint verification must reject an unexpected imported fingerprint'

signing_secret_references=$(grep -nE 'GPG_(PRIVATE_KEY|PASSPHRASE)' "$workflow" || true)
signing_secret_reference_count=$(printf '%s\n' "$signing_secret_references" | sed '/^$/d' | wc -l | tr -d '[:space:]')
[ "$signing_secret_reference_count" -eq 2 ] || fail 'only the GPG import step may reference signing secrets'
import_signing_secret_references=$(printf '%s\n' "$import_step" | grep -E 'GPG_(PRIVATE_KEY|PASSPHRASE)' || true)
import_signing_secret_reference_count=$(printf '%s\n' "$import_signing_secret_references" | sed '/^$/d' | wc -l | tr -d '[:space:]')
[ "$import_signing_secret_reference_count" -eq 2 ] || fail 'the GPG import step must contain both signing secret references'

import_step_line=$(grep -nF 'id: import_gpg' "$workflow" | cut -d: -f1)
import_action_line=$(grep -nF 'uses: crazy-max/ghaction-import-gpg@2dc316deee8e90f13e1a351ab510b4d5bc0c82cd' "$workflow" | cut -d: -f1)
fingerprint_check_line=$(grep -nF '"$imported_fingerprint" != "$expected_fingerprint"' "$workflow" | cut -d: -f1)
goreleaser_line=$(grep -nF 'goreleaser/goreleaser-action@' "$workflow" | cut -d: -f1)
[ -n "$import_step_line" ] || fail 'GPG import step ID is missing'
[ -n "$import_action_line" ] || fail 'pinned GPG import action is missing'
[ -n "$fingerprint_check_line" ] || fail 'fingerprint comparison is missing'
[ -n "$goreleaser_line" ] || fail 'GoReleaser action is missing'
[ "$import_step_line" -lt "$fingerprint_check_line" ] || fail 'fingerprint comparison must follow the GPG import step'
[ "$import_action_line" -lt "$fingerprint_check_line" ] || fail 'fingerprint comparison must follow the pinned GPG import action'
[ "$fingerprint_check_line" -lt "$goreleaser_line" ] || fail 'fingerprint comparison must run before GoReleaser'

printf '%s\n' 'release workflow tests passed'

if [ -z "${RELEASE_WORKFLOW:-}" ]; then
	"$script_dir/test-release-workflow-mutants.sh"
fi
