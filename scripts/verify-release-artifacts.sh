#!/bin/sh

set -eu

usage() {
	printf 'usage: %s DIST_DIR\n' "${0##*/}" >&2
	exit 2
}

fail() {
	printf 'release artifact verification failed: %s\n' "$1" >&2
	exit 1
}

require_command() {
	command -v "$1" >/dev/null 2>&1 || fail "required command '$1' was not found"
}

[ "$#" -eq 1 ] || usage

dist=${1%/}
[ -d "$dist" ] || fail "dist directory '$dist' does not exist"

verify_signature=${VERIFY_RELEASE_SIGNATURE:-1}
case "$verify_signature" in
	0 | 1) ;;
	*) fail "VERIFY_RELEASE_SIGNATURE must be 0 or 1" ;;
esac

require_command basename
require_command awk
require_command cat
require_command cmp
require_command dirname
require_command grep
require_command jq
require_command mktemp
require_command shasum
require_command sort
require_command unzip
if [ "$verify_signature" -eq 1 ]; then
	require_command gpg
fi

project=terraform-provider-kintone

set -- "$dist"/${project}_*_SHA256SUMS
if [ "$#" -ne 1 ] || [ ! -f "$1" ]; then
	fail "expected exactly one ${project}_*_SHA256SUMS file"
fi
checksum_path=$1
checksum_name=$(basename "$checksum_path")
version=${checksum_name#"${project}"_}
version=${version%_SHA256SUMS}
[ -n "$version" ] && [ "$version" != "$checksum_name" ] || fail "could not derive a version from '$checksum_name'"

manifest_name="${project}_${version}_manifest.json"
signature_name="$checksum_name.sig"
signature_path="$dist/$signature_name"
binary_name="${project}_v${version}"
targets='freebsd_amd64 freebsd_386 freebsd_arm freebsd_arm64
windows_amd64 windows_386 windows_arm64
linux_amd64 linux_386 linux_arm linux_arm64
darwin_amd64 darwin_arm64'

manifest_path="$dist/$manifest_name"
manifest_is_extra_file=0
if [ ! -f "$manifest_path" ]; then
	manifest_path="$(dirname "$dist")/terraform-registry-manifest.json"
	[ -f "$manifest_path" ] || fail "manifest '$manifest_name' is missing"
	manifest_is_extra_file=1
fi

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/kintone-release-verify.XXXXXX")
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM
expected_checksums="$work_dir/expected-checksums"
expected_assets="$work_dir/expected-assets"
actual_checksum_names="$work_dir/actual-checksum-names"
actual_assets="$work_dir/actual-assets"

: >"$expected_checksums"
printf '%s\n' "$manifest_name" >>"$expected_checksums"
for target in $targets; do
	printf '%s\n' "${project}_${version}_${target}.zip" >>"$expected_checksums"
done
LC_ALL=C sort -o "$expected_checksums" "$expected_checksums"

{
	cat "$expected_checksums"
	printf '%s\n' "$checksum_name"
	if [ -f "$signature_path" ]; then
		printf '%s\n' "$signature_name"
	fi
} | LC_ALL=C sort >"$expected_assets"

: >"$actual_assets"
for asset_path in "$dist"/"${project}"_*; do
	[ -f "$asset_path" ] || continue
	basename "$asset_path" >>"$actual_assets"
done
if [ "$manifest_is_extra_file" -eq 1 ]; then
	# GoReleaser extra_files records and uploads the renamed manifest without
	# materializing the renamed path in dist.
	printf '%s\n' "$manifest_name" >>"$actual_assets"
fi
LC_ALL=C sort -o "$actual_assets" "$actual_assets"
if ! cmp -s "$expected_assets" "$actual_assets"; then
	fail "provider release asset inventory does not match the expected 13 archives, manifest, checksums, and signature"
fi

if ! jq -e '
  type == "object" and
  keys == ["metadata", "version"] and
  .version == 1 and
  (.metadata | type == "object") and
  (.metadata | keys == ["protocol_versions"]) and
  .metadata.protocol_versions == ["6.0"]
' "$manifest_path" >/dev/null; then
	fail "manifest '$manifest_name' must declare only version 1 and protocol version 6.0"
fi

for target in $targets; do
	archive_name="${project}_${version}_${target}.zip"
	archive_path="$dist/$archive_name"
	[ -f "$archive_path" ] || fail "archive '$archive_name' is missing"
	case "$target" in
		windows_*) expected_binary="$binary_name.exe" ;;
		*) expected_binary=$binary_name ;;
	esac
	if ! archive_entries=$(unzip -Z1 "$archive_path" 2>/dev/null); then
		fail "archive '$archive_name' is not a readable ZIP file"
	fi
	if [ "$archive_entries" != "$expected_binary" ]; then
		fail "archive '$archive_name' must contain only root entry '$expected_binary'"
	fi
done

: >"$actual_checksum_names"
while IFS=' ' read -r expected_hash filename extra; do
	[ -n "$expected_hash" ] && [ -n "$filename" ] && [ -z "${extra:-}" ] || \
		fail "checksum file '$checksum_name' contains a malformed entry"
	[ "${#expected_hash}" -eq 64 ] || fail "checksum for '$filename' is not a SHA-256 digest"
	case "$expected_hash" in
		*[!0-9a-f]*) fail "checksum for '$filename' is not lowercase hexadecimal" ;;
	esac
	printf '%s\n' "$filename" >>"$actual_checksum_names"
	checksum_target="$dist/$filename"
	if [ "$filename" = "$manifest_name" ] && [ "$manifest_is_extra_file" -eq 1 ]; then
		checksum_target=$manifest_path
	fi
	[ -f "$checksum_target" ] || fail "checksum entry '$filename' does not name an existing file"
	actual_hash=$(shasum -a 256 "$checksum_target" | awk '{print $1}')
	[ "$actual_hash" = "$expected_hash" ] || fail "checksum mismatch for '$filename'"
done <"$checksum_path"
LC_ALL=C sort -o "$actual_checksum_names" "$actual_checksum_names"
if ! cmp -s "$expected_checksums" "$actual_checksum_names"; then
	fail "checksum file must contain each of the 13 archives and the manifest exactly once"
fi

if [ "$verify_signature" -eq 1 ]; then
	[ -f "$signature_path" ] || fail "signature '$signature_name' is missing"
	if LC_ALL=C grep -q -- '-----BEGIN PGP SIGNATURE-----' "$signature_path" 2>/dev/null; then
		fail "signature '$signature_name' must be binary, not ASCII armored"
	fi
	if ! gpg --batch --verify "$signature_path" "$checksum_path"; then
		fail "signature '$signature_name' did not verify"
	fi
fi

printf "verified Terraform Registry artifacts for version %s\n" "$version"
