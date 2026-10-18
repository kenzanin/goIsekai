#!/usr/bin/env bash
# Builds the madarascans native .so plugin.
#
# Only header-only RapidJSON is needed: HTTP, text stripping and HTML handling
# all go through the host bridge (native_abi.h), so there is no libcurl/libxml2
# link here.
set -euo pipefail
cd "$(dirname "$0")"

g++ -std=c++17 -O2 -fPIC -shared -fvisibility=hidden \
	main.cpp -o main.so

echo "built main.so ($(stat -c%s main.so) bytes)"
