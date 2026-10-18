#!/usr/bin/env bash
# Builds the roliascan native .so plugin.
#
# Links against system libraries (libcurl, libxml2, OpenSSL) plus header-only
# RapidJSON — all present on a normal Linux dev box (`libcurl4-openssl-dev`,
# `libxml2-dev`, `libssl-dev`, `rapidjson-dev`).
set -euo pipefail
cd "$(dirname "$0")"

g++ -std=c++17 -O2 -fPIC -shared -fvisibility=hidden \
	main.cpp -o main.so \
	-lcurl -lxml2 -lcrypto \
	-I/usr/include/libxml2

echo "built main.so ($(stat -c%s main.so) bytes)"
