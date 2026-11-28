#!/bin/sh
# How the real-data proof signs the simulator in (sourced by proof-real.sh after proof-common.sh).
# Plan: docs/design/proof-sign-in-after-the-key-moved-2026-11-28.md, option A. Needs fail, SIM, SCRATCH, APP, EXACT_SIM and KEYFILE (the QA key's file).
#
# The account key rests in Exact's secret store and shared code is its only reader (api.ts storeSource). In an agent drive that store is a file
# per secret under <container>/Library/Caches/exact/<app id>/agent/default/secrets/ (host/apple store.rs, picker.rs), kept between drives. So the proof
# writes the two files a sign-in writes (app-signin.ts KEY_NAME `choir.key`, KEY_ID_NAME `choir.key-id`; keepKey) and clears what would make the next
# launch wipe or sign out (account.ts reconcile): `choir.computer-id` (keepKey forgets it; shared code finds the computer itself), and under .../data
# `account-stamp` (a stamp that differs from the digest of the new key is "another account": it wipes every shared store), `signout-pending` and `pending-revoke`.
#
# The key is DISPOSABLE. The QA key is only the credential that mints it: the sign-out drill revokes whatever key the app holds (account.ts revokeDirect),
# so seeding the QA key itself would kill it. Mint:    POST /auth/api-keys   (internal/auth/handlers.go:1941, HandleCreateAPIKey)
#   needs manage:keys or admin on the calling key (handlers.go:1996, apiKeyMayManageKeys at :1903), a label (:1964); omitted scopes mean the caller's own
#   delegable scopes (:1973, defaultKeyScopes :1766); answers 201 with the key's `id` and its `secret`, once (:2037, createAPIKeyResponse :1818).
# Revoke:  DELETE /auth/api-keys/{id}   (handlers.go:2094, HandleRevokeAPIKey): a key may revoke itself; a sibling needs manage:keys or admin (:2113-2133).
#   The exit cleanup revokes with the QA key, which holds manage:keys, so it works whether or not the sign-out drill already revoked the minted key
#   (404 then: fine).
#
# No key is ever on a command line, in a variable, in a log, an error or an `sh -x` trace: curl reads the Authorization header from a 0600 file
# (`-H @file`) and the JSON body from a file; the reply goes to a file; python writes the secret and id to files; the seed is `cp`. The server's reply is
# never printed. Only the key's id (not secret) is shown.
#
# Not here, on purpose: AutoradioKit/ProofKey.swift and its two call sites stay. They are Swift, they wait on owner decision D6 (where the key is held),
# and nothing reads what they write in a drive. A later change deletes them.

PROOF_BASE_URL="${PROOF_BASE_URL:-https://choir.news}"
PROOF_KEY_DONE=""

# proof_mint: mint the disposable key once. Files in $SCRATCH (0600): minted.key, minted.id, qa.hdr (the QA key as a header, kept for the revoke).
proof_mint() {
  [ -s "$SCRATCH/minted.id" ] && return 0
  [ -r "$KEYFILE" ] || fail "sign-in" "cannot read the test key file"
  (
    umask 077
    { printf 'Authorization: Bearer '; tr -d ' \r\n' < "$KEYFILE"; printf '\nContent-Type: application/json\n'; } > "$SCRATCH/qa.hdr"
    exp=$(date -u -v+3H '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date -u -d '+3 hours' '+%Y-%m-%dT%H:%M:%SZ')
    printf '{"label":"proof-%s","expires_at":"%s"}' "$(date '+%H%M%S')" "$exp" > "$SCRATCH/mint.json"
  ) || fail "sign-in" "could not write the mint request"
  code=$(curl -sS --max-time 30 -o "$SCRATCH/mint.reply" -w '%{http_code}' -X POST -H "@$SCRATCH/qa.hdr" --data-binary "@$SCRATCH/mint.json" "$PROOF_BASE_URL/auth/api-keys" 2>/dev/null)
  if [ "$code" != 201 ]; then rm -f "$SCRATCH/mint.reply"; fail "sign-in: the test key may not mint keys ($code)"; fi
  if ! ( umask 077; /usr/bin/python3 -I -c '
import json, sys
d = json.load(open(sys.argv[1]))
s, i = d.get("secret", ""), d.get("id", "")
if not (isinstance(s, str) and s.startswith("choir_sk_") and isinstance(i, str) and i): sys.exit(1)
open(sys.argv[2], "w").write(s); open(sys.argv[3], "w").write(i)
' "$SCRATCH/mint.reply" "$SCRATCH/minted.key" "$SCRATCH/minted.id" ) 2>/dev/null; then
    rm -f "$SCRATCH/mint.reply" "$SCRATCH/minted.key" "$SCRATCH/minted.id"
    fail "sign-in: the mint reply held no key or no id"
  fi
  rm -f "$SCRATCH/mint.reply" "$SCRATCH/mint.json"
  PROOF_KEY_ID=$(cat "$SCRATCH/minted.id")
  echo "sign-in: minted a disposable key for this run (id $PROOF_KEY_ID, three hours); it is revoked at exit"
}

# the scratch trees of every store name (the proof passes no --storage, so `default`; a glob keeps a renamed store from being missed)
proof_scratch_roots() { for r in "$1"/Library/Caches/exact/*/agent/*; do [ -d "$r" ] && echo "$r"; done; }

# proof_seed <container>: this install holds the minted key for the test account, with nothing left that would wipe or sign it out.
proof_seed() {
  proof_mint
  root="$1/Library/Caches/exact/$APP/agent/default"
  ( umask 077; mkdir -p "$root/secrets" "$root/data" ) || fail "sign-in" "could not make the scratch secret store"
  chmod 700 "$root/secrets"
  cp "$SCRATCH/minted.key" "$root/secrets/choir.key" && cp "$SCRATCH/minted.id" "$root/secrets/choir.key-id" || fail "sign-in" "could not write the seeded key"
  chmod 600 "$root/secrets/choir.key" "$root/secrets/choir.key-id"
  rm -f "$root/secrets/choir.computer-id"
  for r in $(proof_scratch_roots "$1"); do rm -f "$r/data/account-stamp" "$r/data/signout-pending" "$r/data/pending-revoke"; done
}

# proof_unseed <container>: a launch with no key at all (the signed-out drives).
proof_unseed() {
  for r in $(proof_scratch_roots "$1"); do
    rm -f "$r/secrets/choir.key" "$r/secrets/choir.key-id" "$r/secrets/choir.computer-id" "$r/data/account-stamp" "$r/data/signout-pending" "$r/data/pending-revoke"
  done
}

# proof_key_cleanup: the EXIT trap. Removes the seeded files and revokes the minted key by id (once). A failed revoke is said loudly, with the id.
proof_key_cleanup() {
  [ -n "$PROOF_KEY_DONE" ] && return 0
  PROOF_KEY_DONE=1
  [ -s "$SCRATCH/minted.id" ] || return 0
  id=$(cat "$SCRATCH/minted.id")
  c=$(SIM get_app_container "$EXACT_SIM" "$APP" data 2>/dev/null)
  [ -n "$c" ] && proof_unseed "$c"
  code=$(curl -sS --max-time 30 -o /dev/null -w '%{http_code}' -X DELETE -H "@$SCRATCH/qa.hdr" "$PROOF_BASE_URL/auth/api-keys/$id" 2>/dev/null)
  case "$code" in
    204|200|404) echo "sign-in: the disposable key $id is revoked (or was already)";;
    *) echo "REVOKE FAILED: the disposable key $id is still live on the server until it expires (status ${code:-none}); revoke it by id with the QA key (DELETE /auth/api-keys/$id)" >&2; return 1;;
  esac
}

# proof_assert_signed_in <drive dump>: the app launched signed in (the account page and the app's own console line agree).
proof_assert_signed_in() {
  if [ ! -s "$1" ] || ! grep -qE '\[settings-account-signed\][^"]*"Yes"' "$1" || ! grep -q 'api: session found=true key=true' "$1"; then
    [ -s "$1" ] && grep -m3 'api: session ' "$1" | cut -c1-160
    fail "sign-in: the app is signed out"
  fi
  echo "ok: sign-in: the app launched signed in with the seeded key"
}

# proof_assert_signed_out <drive dump>: a drive started with no key shows signed out, and no key file is left in the scratch tree.
proof_assert_signed_out() {
  grep -qE '\[settings-account-signed\][^"]*"No"' "$1" 2>/dev/null || fail "sign-in: the signed-out drive was not signed out"
  grep -q 'api: session found=true' "$1" && fail "sign-in: the signed-out drive found a key"
  c=$(SIM get_app_container "$EXACT_SIM" "$APP" data 2>/dev/null)
  [ -n "$c" ] && [ -n "$(find "$c/Library/Caches/exact" -path '*/secrets/choir.key' 2>/dev/null)" ] && fail "sign-in: a key file is left in the scratch tree of a signed-out drive"
  echo "ok: sign-in: the signed-out drive was signed out and no key file is left"
}
