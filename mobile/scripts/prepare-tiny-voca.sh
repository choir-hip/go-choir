#!/bin/sh
# Tiny Voca's native speech assets. Build outside the Exact checkout and stage
# into its app only at build time. No Python or JavaScript runs on the phone.
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
CACHE="$HOME/.cache/autoradio-tiny-voca/native-v1"
PORT=426d644dba5953a2971c8fa27eeac5df77c0c2bb
CPP=4363a483002df5b0c3eb83ae93addf0073de388c
HF=5fe06641d0afecb3c04bc9faca2cf64d9b2b1600
check() { /usr/bin/python3 -I "$HERE/scripts/check-tiny-voca-assets.py" "$@"; }
fail() { echo "prepare-tiny-voca: $*" >&2; exit 1; }
if [ "${1:-}" = --stage ]; then
  shift
  [ "$#" -eq 2 ] || fail 'usage: prepare-tiny-voca.sh --stage ios|macos <app directory>'
  case "$1" in ios|macos) ;; *) fail 'expected ios or macos' ;; esac
  src="$CACHE/$1/TinyVoca"
  check "$src" "$1" || fail 'run mobile/scripts/prepare-tiny-voca.sh outside the Mac build lock first'
  mkdir -p "$2/tiny-voca-resources"
  dst="$2/tiny-voca-resources/TinyVoca"
  rm -rf "$dst"
  cp -R "$src" "$dst"
  check "$dst" "$1"
  # The Dizione XCFramework is linked, not a resource: it goes where Exact links modules from.
  X="$CACHE/$1/dizione.xcframework"
  [ -d "$X" ] || fail "run mobile/scripts/prepare-tiny-voca.sh outside the Mac build lock first (no $X)"
  mkdir -p "$(dirname "$2")/.deps/xcframeworks"
  rm -rf "$(dirname "$2")/.deps/xcframeworks/dizione.xcframework"
  cp -R "$X" "$(dirname "$2")/.deps/xcframeworks/"
  exit 0
fi
[ "$#" -eq 0 ] || fail 'usage: prepare-tiny-voca.sh'
if check "$CACHE/ios/TinyVoca" ios >/dev/null 2>&1 && check "$CACHE/macos/TinyVoca" macos >/dev/null 2>&1; then
  printf '%s\n' "$CACHE/macos/TinyVoca"; exit 0
fi
[ "$(uname -s)/$(uname -m)" = Darwin/arm64 ] || fail 'requires Apple silicon and Xcode'
UV="${UV:-$(command -v uv || true)}"
[ -n "$UV" ] || UV="$HOME/.local/bin/uv"
[ -x "$UV" ] || fail 'install uv, or set UV to its path'
mkdir -p "$CACHE"
WORK="$(mktemp -d "$CACHE/.build.XXXXXXXX")"
trap 'rm -rf "$WORK"' EXIT
trap 'exit 1' HUP INT TERM
git -c advice.detachedHead=false clone -q --revision="$PORT" https://github.com/razorback16/tiny-voca-ane.git "$WORK/port"
git -c advice.detachedHead=false clone -q --revision=4bf796d29ea775edd5a7cc5ee3a6806b3c3b91f6 https://github.com/n4ze3m/tiny-voca.git "$WORK/model"
curl -fsSL "https://codeload.github.com/0xShug0/audio.cpp/tar.gz/$CPP" -o "$WORK/audio.cpp.tar.gz"
FULL="audio.cpp-$CPP/src/community_models/tiny_voca/dizione"
mkdir -p "$WORK/dizione" "$WORK/assets/phonemizer"
tar -xzf "$WORK/audio.cpp.tar.gz" --strip-components 4 -C "$WORK/dizione" "$FULL"
fetch() {
  /usr/bin/curl --retry 3 -fsSL "https://huggingface.co/n4ze3m/tiny-voca/resolve/$HF/$1" -o "$WORK/$2"
  printf '%s  %s\n' "$3" "$WORK/$2" | /usr/bin/shasum -a 256 -c >&2
}
fetch tiny-voca.pt tiny-voca.pt ebb22871524c5732520fb1652e6aefcec708a7452e83bdd00adcf6181db5ef70
fetch LICENSE LICENSE.apache-2.0 cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30
fetch phonemizer/LICENSE.cmudict LICENSE.cmudict bd4ce8e44170a5f9f481310ca85c51de3c4f851a65e679b40e603b143bd3542a
fetch phonemizer/LICENSE.homograph-data LICENSE.homograph-data cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30
fetch phonemizer/sources.json phonemizer-sources.json f4c54924d30a5bc0bc1e3d7b058adb924484b3fab8541bc8d8bcecfb028fdbad
for file in lexicon.tsv align.json g2p.json g2p.npz homographs.json; do
  /usr/bin/curl --retry 3 -fsSL "https://huggingface.co/n4ze3m/tiny-voca/resolve/$HF/phonemizer/$file" -o "$WORK/assets/phonemizer/$file"
done
[ "$(git -C "$WORK/port" rev-parse HEAD)" = "$PORT" ] || fail 'port pin mismatch'
cmp "$WORK/LICENSE.apache-2.0" "$WORK/model/LICENSE"
cmp "$WORK/LICENSE.apache-2.0" "$WORK/port/LICENSE"
"$UV" venv --python 3.12 "$WORK/venv"
"$UV" pip sync --python "$WORK/venv/bin/python" "$HERE/scripts/tiny-voca-build-requirements.txt"
/usr/bin/lockf -k -t 7200 "$HOME/.cache/autoradio-mac-build.lock" /bin/sh -eu -c '
  w="$1"; here="$2"; cache="$3"
  check() { /usr/bin/python3 -I "$here/scripts/check-tiny-voca-assets.py" "$@"; }
  if check "$cache/ios/TinyVoca" ios >/dev/null 2>&1 && check "$cache/macos/TinyVoca" macos >/dev/null 2>&1; then exit 0; fi
  export PYTHONPATH="$w/port/src:$w/model/src" PYTHONDONTWRITEBYTECODE=1 OMP_NUM_THREADS=4
  "$w/venv/bin/python" "$here/scripts/tiny-voca-coreml.py" "$w/tiny-voca.pt" "$w/export"
  "$here/scripts/build-dizione-xcframework.sh" "$w/dizione/dizione" "$w/export"
  mkdir -p "$here/.deps/xcframeworks" "$here/AutoradioLab/modules/apple"
  rm -rf "$here/.deps/xcframeworks/dizione.xcframework" "$here/AutoradioLab/modules/apple/dizione.xcframework"
  cp -R "$w/export/dizione.xcframework" "$here/.deps/xcframeworks/"
  cp -R "$w/export/dizione.xcframework" "$here/AutoradioLab/modules/apple/"
  cp "$w/export/dizione-module.modulemap" "$here/AutoradioLab/modules/apple/"
  for platform in macos ios; do
    case "$platform" in macos) sdk=macosx; target=macOS; floor=15.0 ;; ios) sdk=iphoneos; target=iOS; floor=18.0 ;; esac
    dest="$w/$platform/TinyVoca"
    mkdir -p "$dest/Licenses"
    for model in Encoder Decoder Vocoder; do
      /usr/bin/xcrun --sdk "$sdk" coremlcompiler compile "$w/export/$model.mlpackage" "$dest" --platform "$target" --deployment-target "$floor"
    done
    cp -R "$w/assets/phonemizer/." "$dest/phonemizer/"
    cp "$w/export/tiny-voca-coreml.json" "$dest/"
    cp "$w"/LICENSE.* "$w/phonemizer-sources.json" "$dest/Licenses/"
    cp "$w/port/NOTICE" "$dest/Licenses/NOTICE"
    check "$dest" "$platform" --record
    mkdir -p "$cache/$platform"
    rm -rf "$cache/$platform/TinyVoca"
    mv "$dest" "$cache/$platform/TinyVoca"
  done
' sh "$WORK" "$HERE" "$CACHE"
printf '%s\n' "$CACHE/macos/TinyVoca"
