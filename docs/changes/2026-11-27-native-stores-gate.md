---
date: 2026-11-27
title: a gate names every place native code persists, and fails on a new one
class: yellow
---
What: `scripts/check-native-stores` reads the Swift under `mobile/AutoradioKit/`, `mobile/exact-package/apple/` and `mobile/AutoradioLab/modules/` and counts the lines that persist (file writes, directory creation, copy and move, downloads, `UserDefaults` sets, Keychain adds). `scripts/native-stores.txt` owns each one with a ruling: `wiped:<registration>`, `kept:<reason>` or `unruled`. The gate fails on a persistence line the list does not own, on an entry with no line left, and on a token that is gone; `--selftest` plants both in a temp dir. It runs in `scripts/gates`. `docs/evidence/native-stores-inventory-2026-11-27.md` is the inventory: 86 lines, 24 wiped, 50 kept, 12 unruled. No Swift changed.

Why: the sign-out wipe is a hand-kept list of registrations; a new native file, key or Keychain item that holds account data was never noticed (failure-list G2, D6; problem item 4). The gate cannot say a store is safe; it makes adding one change the list in the same diff.

Proof: `python3 -I scripts/check-native-stores --selftest` -> `0 failure(s)` (ten cases); `python3 -I scripts/check-native-stores` -> `native stores hold`. Not proved: that a "wiped" entry is emptied (that is the sign-out tests), or any CI run: CI has no step for the gate yet.

Replaced: nothing; the native wipe was unguarded.

Undo: revert this commit; the list and the gate go, the Swift is untouched.
