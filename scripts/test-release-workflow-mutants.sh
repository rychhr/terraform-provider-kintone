#!/bin/sh

set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
subject="$script_dir/test-release-workflow.sh"
workflow="$script_dir/../.github/workflows/release.yml"
fixture_root=$(mktemp -d "${TMPDIR:-/tmp}/kintone-release-workflow-mutants.XXXXXX")
trap 'rm -rf "$fixture_root"' EXIT HUP INT TERM

fail() {
	printf 'release workflow mutation test failed: %s\n' "$1" >&2
	exit 1
}

assert_rejects() {
	name=$1
	fixture=$2
	if output=$(RELEASE_WORKFLOW="$fixture" "$subject" 2>&1); then
		printf '%s\n' "expected $name mutation to be rejected, got success:" >&2
		printf '%s\n' "$output" >&2
		exit 1
	fi
}

bracket_secret_in_validate="$fixture_root/bracket-secret-in-validate.yml"
awk '
	$0 == "      - name: Validate release tag" {
		print "      - name: Incorrect signing secret use"
		print "        env:"
		print "          LEAK: ${{ secrets[\047GPG_PRIVATE_KEY\047] }}"
		print "        run: true"
	}
	{ print }
' "$workflow" >"$bracket_secret_in_validate"
assert_rejects bracket-secret-in-validate "$bracket_secret_in_validate"

wrong_import_id="$fixture_root/wrong-import-id.yml"
sed 's/id: import_gpg/id: incorrect_gpg_import/' "$workflow" >"$wrong_import_id"
assert_rejects wrong-import-id "$wrong_import_id"

wrong_import_action="$fixture_root/wrong-import-action.yml"
sed 's/crazy-max\/ghaction-import-gpg@2dc316deee8e90f13e1a351ab510b4d5bc0c82cd/crazy-max\/ghaction-import-gpg@0000000000000000000000000000000000000000/' \
	"$workflow" >"$wrong_import_action"
assert_rejects wrong-import-action "$wrong_import_action"

wrong_private_key_input="$fixture_root/wrong-private-key-input.yml"
sed 's/gpg_private_key: ${{ secrets.GPG_PRIVATE_KEY }}/private_key: ${{ secrets.GPG_PRIVATE_KEY }}/' \
	"$workflow" >"$wrong_private_key_input"
assert_rejects wrong-private-key-input "$wrong_private_key_input"

wrong_passphrase_input="$fixture_root/wrong-passphrase-input.yml"
sed 's/passphrase: ${{ secrets.GPG_PASSPHRASE }}/passphrase: ${{ secrets.GPG_PRIVATE_KEY }}/' \
	"$workflow" >"$wrong_passphrase_input"
assert_rejects wrong-passphrase-input "$wrong_passphrase_input"

wrong_fingerprint_output="$fixture_root/wrong-fingerprint-output.yml"
sed 's/steps.import_gpg.outputs.fingerprint/steps.incorrect_gpg_import.outputs.fingerprint/' \
	"$workflow" >"$wrong_fingerprint_output"
assert_rejects wrong-fingerprint-output "$wrong_fingerprint_output"

printf '%s\n' 'release workflow mutation tests passed'
