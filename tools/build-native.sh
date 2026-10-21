#!/usr/bin/env bash
# Build a native (.so) goIsekai plugin.
#
# Toolchain tiers, best first:
#
#   1. zig c++ + static musl  -> NEEDED=[]      the .so carries its own libc,
#                                                  so it needs no libc at all on
#                                                  the host (no libc.so.6, no
#                                                  ld-linux, no musl either).
#   2. zig c++ (glibc)         -> NEEDED=libc.so.6
#   3. g++                     -> NEEDED=libstdc++.so.6, libgcc_s.so.1, libc.so.6
#
# Tier 1 is the interesting one. A .so can never be a fully static ELF
# executable, but it can be a fully self-contained shared object: musl's
# libc.a is PIC, so linking it in leaves the shared object with no DT_NEEDED
# entry at all. That is why tier 1 uses the HOST's musl (musl-tools) and not
# zig's own: zig's bundled musl libc.a is not PIC (R_X86_64_32S against local
# symbols) and cannot go into a .so.
#
# Tier 1's .so still uses the host's malloc/free: the dynamic linker binds
# those to the host's libc, because RTLD_LOCAL keeps the .so's own copies out
# of the global scope. Do NOT add -Bsymbolic or a --version-script here: both
# force the .so to call its own musl malloc, whose allocator was never
# initialised (musl's __init_libc does not run for a dlopen'd .so), and the
# first allocation then segfaults.
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

MUSL_A=/usr/lib/musl/lib/libc.a
MUSL_TARGET="$(uname -m)-linux-musl"

# RapidJSON is header-only and lives in the system include dir. It must come
# AFTER libc++'s own headers, or <cstring> picks up glibc's <string.h> and
# breaks ("didn't find libc++'s <wchar.h> header"). -idirafter, not -I.
inc=()
[ -d /usr/include/rapidjson ] && inc=(-idirafter /usr/include)

musl_version() {
	local v
	v=$(strings /usr/lib/ld-musl-*.so.1 2>/dev/null |
		grep -oE '^1\.[0-9]+\.[0-9]+$' | head -1)
	echo "${v:-unknown}"
}

build_musl() {
	[ -f "$MUSL_A" ] || return 1
	command -v zig >/dev/null 2>&1 || return 1
	local obj
	obj="$(mktemp -d)/main.o"
	zig c++ -target "$MUSL_TARGET" "${inc[@]}" \
		-ffunction-sections -fdata-sections \
		"${flags[@]}" \
		-c "${srcs[@]}" -o "$obj" || return 1
	zig c++ -target "$MUSL_TARGET" -shared -fPIC \
		-Wl,--gc-sections \
		-Wl,-Bstatic "$MUSL_A" -Wl,-Bdynamic \
		"$obj" -o "$out" || return 1
	strip "$out" 2>/dev/null || true
}

build_zig() {
	command -v zig >/dev/null 2>&1 || return 1
	zig c++ "${flags[@]}" -o "$out" "${srcs[@]}"
}

build_gpp() {
	command -v g++ >/dev/null 2>&1 || return 1
	g++ "${flags[@]}" -o "$out" "${srcs[@]}"
}

if build_musl; then
	tool="zig c++ musl"
	ver="$(musl_version)"
elif build_zig; then
	echo "-- musl static unavailable or failed, fallback: zig c++ (glibc)" >&2
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
