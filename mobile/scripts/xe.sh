#!/bin/sh
# Run a command outside the Nix dev shell with the Xcode toolchain.
exec env -i HOME="$HOME" PATH=/usr/bin:/bin:/usr/sbin:/sbin DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer "$@"
