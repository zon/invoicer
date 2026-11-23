#!/bin/sh
set -eu

if [ "$#" -ne 4 ]; then
	echo "Usage: $0 <image> <version> <commit> <local-image>" >&2
	exit 1
fi

image=$1
version=$2
commit=$3
local_image=$4

short=$(printf '%s' "$commit" | cut -c1-7)
if [ -z "$short" ]; then
	echo "A commit is needed for the image tag" >&2
	exit 1
fi
if [ -z "$version" ]; then
	echo "An app version is needed for the image tag" >&2
	exit 1
fi

sha_ref="$image:sha-$short"
version_ref="$image:$version"

if inspect_output=$(skopeo inspect "docker://$sha_ref" 2>&1); then
	skopeo copy "docker://$sha_ref" "docker://$version_ref"
	exit 0
fi

if ! printf '%s\n' "$inspect_output" | grep -qiE 'not found|manifest unknown|404'; then
	printf '%s\n' "$inspect_output" >&2
	echo "Could not check image $sha_ref" >&2
	exit 1
fi

podman push "$local_image" "docker://$sha_ref"
skopeo copy "docker://$sha_ref" "docker://$version_ref"
