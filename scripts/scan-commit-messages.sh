#!/usr/bin/env bash
#
# Scan a set of commit messages against .gitleaks.toml.
#
#     scripts/scan-commit-messages.sh <git-rev-list-arguments>...
#     scripts/scan-commit-messages.sh origin/main..HEAD
#     scripts/scan-commit-messages.sh --max-count=1 HEAD
#
# The arguments are handed to `git rev-list` unchanged, so anything that names a
# set of commits works.
#
# `gitleaks git` scans diffs, not commit messages, so a secret or an agent
# session link that appears only in a message is invisible to it — measured with
# gitleaks 8.30.1. `gitleaks dir` scans the files it is handed, so this script
# writes each message to its own file and hands gitleaks the directory. The file
# name carries the commit hash, so a hit names the commit it came from.
#
# Exit status: 0 when every message is clean, 1 when gitleaks matched something,
# 2 when the scan could not be run at all. That last case matters — a guard that
# cannot resolve its arguments must not report success.

set -euo pipefail

if [ "$#" -eq 0 ]; then
	echo "usage: $(basename "$0") <git-rev-list-arguments>..." >&2
	exit 2
fi

repo_root="$(git rev-parse --show-toplevel)"
config="${GITLEAKS_CONFIG:-${repo_root}/.gitleaks.toml}"

if [ ! -f "$config" ]; then
	echo "error: gitleaks configuration not found at ${config}" >&2
	exit 2
fi

messages_dir="$(mktemp -d)"
# Kept outside messages_dir so that gitleaks does not scan the list itself.
revs_file="$(mktemp)"
trap 'rm -rf "${messages_dir}" "${revs_file}"' EXIT

# `git rev-list` runs on its own rather than inside the loop's redirection: a
# process substitution swallows the exit status, and a failure to resolve the
# arguments would then look exactly like an empty range.
if ! git rev-list "$@" -- >"$revs_file"; then
	echo "error: could not resolve $* to a set of commits" >&2
	exit 2
fi

count=0
while read -r sha; do
	[ -n "$sha" ] || continue
	git log -1 --format=%B "$sha" >"${messages_dir}/${sha}.commit-message.txt"
	count=$((count + 1))
done <"$revs_file"

if [ "$count" -eq 0 ]; then
	echo "No commits selected by $*; nothing to scan."
	exit 0
fi

echo "Scanning ${count} commit message(s) selected by $*."
gitleaks dir "$messages_dir" --config "$config" --redact --verbose --no-banner
