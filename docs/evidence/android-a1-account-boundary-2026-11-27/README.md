# The Android association file and accepted origin are live on staging (2026-11-27)

Device: Motorola `ZY22H5K8QR`, package `news.choir.autoradio.lab.one`, A0 debug
certificate SHA-256
`dfe9767c642a2f864b297049bde97c0836cb58d5ce516ffe7a7da11f6fec9ea7`.

**What was run.** `curl` against `https://choir.news` after CI run
`45917811642` deployed `774e87d2f9140eef626772ecec15e1d58d2d1152`:

1. `x-choir-build-commit` on the site is that commit;
2. `GET /.well-known/assetlinks.json` answers 200 with the exact document:
   only `delegate_permission/common.get_login_creds`, the lab-one package and
   the certificate fingerprint above;
3. `/health` answers `status: ok`, service `proxy`.

Raw output: `staging-assetlinks.txt`.

## What this proves

The staging proxy serves the Android association file from the same canonical
record that derives the accepted Credential Manager origin
(`android:apk-key-hash:3-l2fGQqL4ZLKXBJvel8CDbLWNXOUW_-en2hH2_snqc`), and the
deployed commit is the reviewed one. The Go route tests assert the exact bytes,
GET/HEAD, no redirect and the refusal of neighbour paths; the auth ceremony
tests accept creation and discoverable assertion from that origin and refuse a
wrong APK hash.

## What is not proved

The phone journey itself: the d@e passkey sheet, a relaunch reading
authenticated Account data, sign-out, and a cold relaunch still signed out.
`scripts/proofs/android-a1-account-boundary.py` is the fail-closed proof for
that run. It needs the owner present for Android's passkey/biometric sheet: the
Motorola had no enrolled fingerprint or face at the A0 read, so this is an
owner action, not something `adb` may drive. The proof also has not yet been
run, so no `summary.json` exists.
