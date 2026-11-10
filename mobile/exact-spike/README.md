# exact-spike: the Exact audio-seam spike

A minimal Exact iOS app with one Swift package of ours that takes the audio
session only through Exact's claim, plays a 440 Hz tone and taps the
microphone, logging the category and mode. It is the real code behind the
examples in [docs/reference/EXACT.md](../../docs/reference/EXACT.md) and
[docs/reference/exact/](../../docs/reference/exact/).

- `package/`: the package `@choir/spike` (`exact.json` interface,
  `apple/exact.json` implementation, `apple/Spike.swift`).
- `app/`: the app (`app.contract`, `app.ts`, `app.json`, `strings/en.json`).

Pinned Exact commit: `76cd50f9e863129b3fc75db1673d6ad4bbf79258`.

## Build (simulator), as run on 2026-11-10

```sh
git clone https://github.com/expo/exact exact-checkout && cd exact-checkout
git checkout 76cd50f9e863129b3fc75db1673d6ad4bbf79258
# Bun must be 1.4.2 exactly:
curl -fsSL https://bun.sh/install | BUN_INSTALL=$HOME/.bun-1.4.2 bash -s bun-v1.4.2
export BUN=$HOME/.bun-1.4.2/bin/bun
# Lean Hermes install (skips Android/Linux/Windows bundles that `setup` fetches):
for t in aarch64-apple-darwin aarch64-apple-ios aarch64-apple-ios-sim; do
  cargo run --manifest-path vendor/ibex/crates/hermes-lean-sys-installer/Cargo.toml -- --target $t
done
# Make an app with Exact's own scaffold, then overlay these files:
$BUN scripts/exact.mjs new ../spikeapp && cd ../spikeapp
cp <repo>/mobile/exact-spike/app/{app.contract,app.json,app.ts} .
mkdir -p strings && cp <repo>/mobile/exact-spike/app/strings/en.json strings/
cp -R <repo>/mobile/exact-spike/package ../spikepkg
# package.json dependencies: { "@choir/spike": "file:../spikepkg" }
$BUN install
$BUN exact.mjs ios                       # first build about 15 minutes
$BUN exact.mjs agent ios tree "tap listen" "clock settle" state
```

Run Apple tools outside the Nix shell (`env -i HOME=$HOME PATH=/usr/bin:/bin:/usr/sbin:/sbin:$HOME/.cargo/bin DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer ...`).

## Device

Not run: needs a provisioning profile for `news.choir.exactspike` on the
Mac, which needs the owner signed in under Xcode > Settings > Accounts. Then
`$BUN exact.mjs ios --device --phone <udid>`. See
[docs/reference/exact/build-and-ship.md](../../docs/reference/exact/build-and-ship.md).
