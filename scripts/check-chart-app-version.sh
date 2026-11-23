#!/bin/sh
set -eu

if [ "$#" -ne 2 ]; then
	echo "Usage: $0 <chart dir> <app version>" >&2
	exit 1
fi

chart_dir=$1
app_version=$2

chart_app_version=$(awk -F': *' '$1 == "appVersion" {print $2; exit}' "$chart_dir/Chart.yaml" | tr -d '"')
if [ -z "$chart_app_version" ] || [ -z "$app_version" ]; then
	echo "A chart appVersion in $chart_dir and an app version are needed" >&2
	exit 1
fi

if [ "$chart_app_version" != "$app_version" ]; then
	echo "Chart appVersion $chart_app_version differs from app version $app_version" >&2
	exit 1
fi
