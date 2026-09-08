#!/bin/sh
set -eu

repository="louis-bourgault/ssg"
asset="ssg_Linux_x86_64.tar.gz"
version="${SSG_VERSION:-latest}"

if [ "$version" = "latest" ]; then
    download_url="https://github.com/${repository}/releases/latest/download/${asset}"
else
    download_url="https://github.com/${repository}/releases/download/${version}/${asset}"
fi

working_directory=$(mktemp -d)
trap 'rm -rf "$working_directory"' EXIT HUP INT TERM

echo "Downloading SSG ${version}..."
curl --fail --silent --show-error --location \
    "$download_url" \
    --output "$working_directory/$asset"
tar -xzf "$working_directory/$asset" -C "$working_directory"

if [ ! -f "$working_directory/ssg" ]; then
    echo "The SSG release archive did not contain the ssg executable." >&2
    exit 1
fi

chmod +x "$working_directory/ssg"
"$working_directory/ssg"
