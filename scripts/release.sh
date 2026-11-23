#!/bin/sh
set -eu

fail() {
	echo "$1" >&2
	exit 1
}

git fetch --quiet origin main
commit=$(git rev-parse origin/main)

version=$(git show "$commit:VERSION" 2>/dev/null) || fail "origin/main has no VERSION file"
printf '%s\n' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' || fail "VERSION must be MAJOR.MINOR.PATCH, not '$version'"

tag="v$version"
if git ls-remote --exit-code --tags origin "refs/tags/$tag" >/dev/null; then
	fail "Tag $tag exists on origin"
fi

chart_dir=$(mktemp -d)
trap 'rm -rf "$chart_dir"' EXIT
git archive "$commit" deploy/chart | tar -x -C "$chart_dir"
sh "$(dirname "$0")/check-chart-app-version.sh" "$chart_dir/deploy/chart" "$version"

git tag "$tag" "$commit"
git push origin "$tag"
