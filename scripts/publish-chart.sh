#!/bin/sh
set -eu

chart_dir=$1
repository=$2

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

name=$(awk -F': *' '$1 == "name" {print $2; exit}' "$chart_dir/Chart.yaml" | tr -d '"')
version=$(awk -F': *' '$1 == "version" {print $2; exit}' "$chart_dir/Chart.yaml" | tr -d '"')
if [ -z "$name" ] || [ -z "$version" ]; then
	echo "Chart.yaml in $chart_dir needs a name and version" >&2
	exit 1
fi

helm package "$chart_dir" --destination "$work" >/dev/null
package="$work/$name-$version.tgz"
tar -xzf "$package" -C "$work"

mkdir -p "$work/published"
if pull_output=$(helm pull "$repository/$name" --version "$version" --destination "$work/published" --untar 2>&1); then
	if diff -r "$work/$name" "$work/published/$name" >/dev/null; then
		echo "Chart $name $version is already published"
		exit 0
	fi
	echo "Chart $name $version is already published with different content" >&2
	exit 1
fi

if ! printf '%s\n' "$pull_output" | grep -qiE 'not found|manifest unknown|404'; then
	printf '%s\n' "$pull_output" >&2
	echo "Could not check chart $name $version in $repository" >&2
	exit 1
fi

helm push "$package" "$repository"
