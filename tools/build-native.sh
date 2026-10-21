#!/usr/bin/env bash
# Build a native (.so) goIsekai plugin.
#
# Toolchain: zig c++ is preferred, g++ is the fallback. zig c++ links its own
# libc++ and libgcc statically, so the result depends on libc.so.6 ONLY - no
# libstdc++.so.6 / libgcc_s.so.1 to find on the host. The cost is size
# (roughly twice the g++ build) and a cold build of ~38 s; warm builds are
# <100 ms because zig caches. g++ stays the fallback so a machine without zig
# still builds.
#
# A fully static .so is NOT reachable: `zig c++ -static -shared` emits an ar
# archive instead of a shared object, and a .so needs PIC relocations in the
# first place. Static libc++ is the closest target that works, and that is
# what zig gives by default.
#
# The toolchain that actually built the artifact is recorded in the plugin's
# plugin.json ("built_with"), so the plugins page shows it instead of the
# reader having to stat the bytes.
#
# Usage: tools/build-native.sh <plugin-dir> <out.so> [cxx flags...]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dir="${1:?usage: build-native.sh <plugin-dir> <out.so> [flags...]}"
out="${2:?usage: build-native.sh <plugin-dir> <out.so> [flags...]}"
shift 2
flags=("$@")

cd "$dir"

srcs=()
for f in *.cpp; do
	srcs+=("$f")
done
if [ ${#srcs[@]} -eq 0 ]; then
	echo "no .cpp source in $dir" >&2
	exit 1
fi

build_zig() {
	command -v zig >/dev/null 2>&1 || return 1
	zig c++ "${flags[@]}" -o "$out" "${srcs[@]}"
}

build_gpp() {
	command -v g++ >/dev/null 2>&1 || return 1
	g++ "${flags[@]}" -o "$out" "${srcs[@]}"
}

if build_zig; then
	tool="zig"
	ver="$(zig version)"
elif build_gpp; then
	echo "-- zig c++ unavailable or failed, fallback: g++" >&2
	tool="g++"
	ver="$(g++ -dumpfullversion -dumpversion)"
else
	echo "no C++ compiler found (need zig or g++)" >&2
	exit 1
fi

"$ROOT/tools/stamp-built-with.sh" "$dir" "$tool" "$ver"
echo "built $out ($(stat -c%s "$out") bytes) with $tool $ver"
