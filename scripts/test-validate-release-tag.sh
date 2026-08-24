#!/bin/sh

set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
validator="$script_dir/validate-release-tag.sh"
fixture_root=$(mktemp -d "${TMPDIR:-/tmp}/kintone-release-tag-test.XXXXXX")
trap 'rm -rf "$fixture_root"' EXIT HUP INT TERM

repo="$fixture_root/repo"
git init -q -b main "$repo"
git -C "$repo" config user.name "Release Test"
git -C "$repo" config user.email "release-test@example.invalid"
printf '%s\n' baseline >"$repo/fixture.txt"
git -C "$repo" add fixture.txt
git -C "$repo" commit -q -m "test: add baseline"

assert_accept() {
	tag=$1
	git -C "$repo" tag "$tag"
	if ! output=$(cd "$repo" && "$validator" "$tag" main 2>&1); then
		printf 'expected %s to be accepted, got:\n%s\n' "$tag" "$output" >&2
		exit 1
	fi
}

assert_reject() {
	tag=$1
	created=0
	if git check-ref-format "refs/tags/$tag" >/dev/null 2>&1; then
		git -C "$repo" tag -f "$tag" main
		created=1
	fi
	if output=$(cd "$repo" && "$validator" "$tag" main 2>&1); then
		printf 'expected %s to be rejected, got success:\n%s\n' "$tag" "$output" >&2
		exit 1
	fi
	if [ "$created" -eq 1 ]; then
		git -C "$repo" tag -d "$tag" >/dev/null
	fi
}

for tag in v0.1.0 v1.2.3-rc.1 v2.0.0+build.7 v3.4.5-beta.2+linux.amd64; do
	assert_accept "$tag"
done

for tag in 1.2.3 v1.2 v01.2.3 v1.02.3 v1.2.03 v1.2.3-01 v1.2.3- v1.2.3+build..7 v1.2.3_beta; do
	assert_reject "$tag"
done

git -C "$repo" switch -q -c feature
printf '%s\n' feature >>"$repo/fixture.txt"
git -C "$repo" commit -q -am "test: add feature"
git -C "$repo" tag v4.0.0
git -C "$repo" switch -q main
if output=$(cd "$repo" && "$validator" v4.0.0 main 2>&1); then
	printf 'expected unreachable tag to be rejected, got success:\n%s\n' "$output" >&2
	exit 1
fi

git -C "$repo" tag v5.0.0
git -C "$repo" branch v5.0.0 main
if output=$(cd "$repo" && "$validator" v5.0.0 main 2>&1); then
	printf 'expected local branch collision to be rejected, got success:\n%s\n' "$output" >&2
	exit 1
fi
git -C "$repo" branch -D v5.0.0 >/dev/null

git -C "$repo" tag v6.0.0
git -C "$repo" update-ref refs/remotes/origin/v6.0.0 refs/heads/main
if output=$(cd "$repo" && "$validator" v6.0.0 main 2>&1); then
	printf 'expected remote branch collision to be rejected, got success:\n%s\n' "$output" >&2
	exit 1
fi

printf '%s\n' "release tag validator tests passed"
