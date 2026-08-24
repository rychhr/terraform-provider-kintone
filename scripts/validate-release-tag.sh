#!/bin/sh

set -eu

usage() {
	printf 'usage: %s TAG MAIN_REF\n' "${0##*/}" >&2
	exit 2
}

fail() {
	printf 'release tag validation failed: %s\n' "$1" >&2
	exit 1
}

[ "$#" -eq 2 ] || usage

tag=$1
main_ref=$2

# SemVer 2.0.0: numeric prerelease identifiers cannot have leading zeroes;
# alphanumeric prerelease and build identifiers use only ASCII letters,
# digits, and hyphens.
number='(0|[1-9][0-9]*)'
prerelease_identifier="(${number}|[0-9]*[A-Za-z-][0-9A-Za-z-]*)"
semver="^v${number}\\.${number}\\.${number}(-${prerelease_identifier}(\\.${prerelease_identifier})*)?(\\+[0-9A-Za-z-]+(\\.[0-9A-Za-z-]+)*)?$"

if ! LC_ALL=C printf '%s\n' "$tag" | grep -Eq "$semver"; then
	fail "tag '$tag' is not a v-prefixed SemVer 2.0 version"
fi

if git show-ref --verify --quiet "refs/heads/$tag"; then
	fail "local branch '$tag' collides with the release tag"
fi

if git for-each-ref --format='%(refname:strip=3)' refs/remotes | grep -Fqx "$tag"; then
	fail "remote-tracking branch '$tag' collides with the release tag"
fi

if ! git show-ref --verify --quiet "refs/tags/$tag"; then
	fail "tag '$tag' does not exist"
fi

if ! tag_commit=$(git rev-parse --verify "$tag^{commit}" 2>/dev/null); then
	fail "tag '$tag' does not resolve to a commit"
fi

if ! main_commit=$(git rev-parse --verify "$main_ref^{commit}" 2>/dev/null); then
	fail "main ref '$main_ref' does not resolve to a commit"
fi

if ! git merge-base --is-ancestor "$tag_commit" "$main_commit"; then
	fail "tag '$tag' is not reachable from '$main_ref'"
fi

printf "release tag '%s' is valid and reachable from '%s'\n" "$tag" "$main_ref"
