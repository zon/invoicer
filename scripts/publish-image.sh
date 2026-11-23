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

# digest_of prints the digest from a skopeo inspect report.
digest_of() {
	sed -n 's/.*"Digest":[[:space:]]*"\([^"]*\)".*/\1/p'
}

# is_missing reports whether a skopeo inspect error means the image is absent.
is_missing() {
	printf '%s\n' "$1" | grep -qiE 'not found|manifest unknown|404'
}

version_digest=
if version_inspect=$(skopeo inspect "docker://$version_ref" 2>&1); then
	version_digest=$(printf '%s\n' "$version_inspect" | digest_of)
	if [ -z "$version_digest" ]; then
		echo "Could not read the digest of $version_ref" >&2
		exit 1
	fi
elif ! is_missing "$version_inspect"; then
	printf '%s\n' "$version_inspect" >&2
	echo "Could not check image $version_ref" >&2
	exit 1
fi

if sha_inspect=$(skopeo inspect "docker://$sha_ref" 2>&1); then
	sha_digest=$(printf '%s\n' "$sha_inspect" | digest_of)
	if [ -z "$sha_digest" ]; then
		echo "Could not read the digest of $sha_ref" >&2
		exit 1
	fi
	if [ -n "$version_digest" ] && [ "$sha_digest" != "$version_digest" ]; then
		echo "$version_ref is already published at $version_digest, which differs from $sha_ref at $sha_digest" >&2
		exit 1
	fi
	if [ -n "$version_digest" ]; then
		echo "Image $version_ref is already published at $version_digest"
		exit 0
	fi
	skopeo copy "docker://$sha_ref" "docker://$version_ref"
	exit 0
fi

if ! is_missing "$sha_inspect"; then
	printf '%s\n' "$sha_inspect" >&2
	echo "Could not check image $sha_ref" >&2
	exit 1
fi

if [ -n "$version_digest" ]; then
	echo "$version_ref is already published at $version_digest, but $sha_ref does not exist" >&2
	exit 1
fi

podman push "$local_image" "docker://$sha_ref"
skopeo copy "docker://$sha_ref" "docker://$version_ref"
