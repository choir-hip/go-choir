#!/bin/sh
# Build libhashshim.so with the installed NDK (a compile on the Mac, nothing is run).
# Usage: shim/build.sh <output.so>
set -eu
NDK=${NDK:-$(ls -d "$HOME"/Library/Android/sdk/ndk/* | tail -1)}
CC=$(ls "$NDK"/toolchains/llvm/prebuilt/*/bin/aarch64-linux-android28-clang)
exec "$CC" -shared -fPIC -O2 -o "$1" "$(dirname "$0")/hash_memory_shim.c"
