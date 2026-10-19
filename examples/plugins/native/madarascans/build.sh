#!/usr/bin/env bash
# Builds the madarascans native .so plugin.
#
# Only header-only RapidJSON is needed: HTTP, text stripping and HTML handling
# all go through the host bridge (native_abi.h), so there is no libcurl/libxml2
# link here.
set -euo pipefail
cd "$(dirname "$0")"

# -s strips the symbol/relocation tables the loader never reads (~11% smaller,
# and the ABI exports live in .dynsym, so they survive).
g++ -std=c++17 -O2 -fPIC -shared -fvisibility=hidden -Wl,-s \
	main.cpp -o main.so

echo "built main.so ($(stat -c%s main.so) bytes)"

# Record the toolchain + version for the plugins page.
"../../../../tools/stamp-built-with.sh" . native "$(g++ -dumpfullversion -dumpversion)"

