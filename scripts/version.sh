#!/bin/sh
set -eu

version=$(git describe --tags --always --dirty 2>/dev/null) || {
	echo dev
	exit 0
}

echo "${version#v}"
