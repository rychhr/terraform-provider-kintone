#!/bin/sh

set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
verifier="$script_dir/verify-release-artifacts.sh"
# GnuPG creates Unix-domain sockets below GNUPGHOME. Keep the path short enough
# for macOS as well as Linux runners.
fixture_root=$(mktemp -d "/tmp/kra.XXXXXX")
trap 'rm -rf "$fixture_root"' EXIT HUP INT TERM

export GNUPGHOME="$fixture_root/gnupg"
mkdir -m 700 "$GNUPGHOME"
gpg --batch --passphrase '' --quick-generate-key \
	'Release Artifact Test <release-artifact-test@example.invalid>' rsa2048 sign 0 >/dev/null 2>&1

project=terraform-provider-kintone
version=0.1.0
binary="${project}_v${version}"
manifest="${project}_${version}_manifest.json"
checksums="${project}_${version}_SHA256SUMS"
targets='freebsd_amd64 freebsd_386 freebsd_arm freebsd_arm64
windows_amd64 windows_386 windows_arm64
linux_amd64 linux_386 linux_arm linux_arm64
darwin_amd64 darwin_arm64'

refresh_metadata() {
	dist=$1
	(
		cd "$dist"
		shasum -a 256 "$manifest" "${project}_${version}"_*.zip | LC_ALL=C sort -k 2 >"$checksums"
	)
	gpg --batch --yes --output "$dist/$checksums.sig" --detach-sign "$dist/$checksums"
}

create_valid_fixture() {
	dist=$1
	mkdir -p "$dist" "$fixture_root/bin"
	printf '%s\n' '{"version":1,"metadata":{"protocol_versions":["6.0"]}}' >"$dist/$manifest"
	for target in $targets; do
		case "$target" in
			windows_*) archive_binary="$binary.exe" ;;
			*) archive_binary=$binary ;;
		esac
		printf 'provider fixture for %s\n' "$target" >"$fixture_root/bin/$archive_binary"
		chmod +x "$fixture_root/bin/$archive_binary"
		zip -j -q "$dist/${project}_${version}_${target}.zip" "$fixture_root/bin/$archive_binary"
	done
	refresh_metadata "$dist"
}

copy_fixture() {
	name=$1
	destination="$fixture_root/$name"
	mkdir "$destination"
	cp -R "$valid/." "$destination/"
	printf '%s\n' "$destination"
}

assert_invalid() {
	name=$1
	dist=$2
	if output=$("$verifier" "$dist" 2>&1); then
		printf 'expected %s fixture to fail, got success:\n%s\n' "$name" "$output" >&2
		exit 1
	fi
}

valid="$fixture_root/valid"
create_valid_fixture "$valid"

if ! output=$("$verifier" "$valid" 2>&1); then
	printf 'expected valid fixture to pass, got:\n%s\n' "$output" >&2
	exit 1
fi

missing_archive=$(copy_fixture missing-archive)
rm "$missing_archive/${project}_${version}_linux_arm64.zip"
refresh_metadata "$missing_archive"
assert_invalid missing-archive "$missing_archive"

extra_target=$(copy_fixture extra-target)
printf '%s\n' extra >"$fixture_root/bin/$binary"
zip -j -q "$extra_target/${project}_${version}_linux_riscv64.zip" "$fixture_root/bin/$binary"
refresh_metadata "$extra_target"
assert_invalid extra-target "$extra_target"

multiple_entries=$(copy_fixture multiple-entries)
printf '%s\n' unexpected >"$fixture_root/bin/README.txt"
zip -j -q "$multiple_entries/${project}_${version}_linux_amd64.zip" "$fixture_root/bin/README.txt"
refresh_metadata "$multiple_entries"
assert_invalid multiple-entries "$multiple_entries"

tampered_checksum=$(copy_fixture tampered-checksum)
awk 'NR == 1 {$1 = "0000000000000000000000000000000000000000000000000000000000000000"} {print}' \
	"$tampered_checksum/$checksums" >"$tampered_checksum/$checksums.tmp"
mv "$tampered_checksum/$checksums.tmp" "$tampered_checksum/$checksums"
gpg --batch --yes --output "$tampered_checksum/$checksums.sig" \
	--detach-sign "$tampered_checksum/$checksums"
assert_invalid tampered-checksum "$tampered_checksum"

wrong_manifest=$(copy_fixture wrong-manifest)
printf '%s\n' '{"version":1,"metadata":{"protocol_versions":["5.0"]}}' >"$wrong_manifest/$manifest"
refresh_metadata "$wrong_manifest"
assert_invalid wrong-manifest "$wrong_manifest"

armored_signature=$(copy_fixture armored-signature)
rm "$armored_signature/$checksums.sig"
gpg --batch --yes --armor --output "$armored_signature/$checksums.sig" \
	--detach-sign "$armored_signature/$checksums"
assert_invalid armored-signature "$armored_signature"

missing_signature=$(copy_fixture missing-signature)
rm "$missing_signature/$checksums.sig"
assert_invalid missing-signature "$missing_signature"

if ! output=$(VERIFY_RELEASE_SIGNATURE=0 "$verifier" "$missing_signature" 2>&1); then
	printf 'expected unsigned snapshot fixture to pass, got:\n%s\n' "$output" >&2
	exit 1
fi

goreleaser_extra_file=$(copy_fixture goreleaser-extra-file)
cp "$goreleaser_extra_file/$manifest" "$fixture_root/terraform-registry-manifest.json"
rm "$goreleaser_extra_file/$manifest" "$goreleaser_extra_file/$checksums.sig"
if ! output=$(VERIFY_RELEASE_SIGNATURE=0 "$verifier" "$goreleaser_extra_file" 2>&1); then
	printf 'expected GoReleaser renamed extra-file fixture to pass, got:\n%s\n' "$output" >&2
	exit 1
fi

printf '%s\n' "release artifact verifier tests passed"
