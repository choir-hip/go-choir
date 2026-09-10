#!/usr/bin/env bash
# Decode-root guard for mission-2 (landing step 3, decoder matrix companion).
#
# Every production Event/CASRequest/DurableEvent decode must route through one
# of the centralized roots (internal/computerevent/decode.go), the format
# discriminator (DecodeProjectionBatch), or the descriptor seam
# (ParseDescriptor). Receipt decodes and non-computerevent types are out of
# charter scope with reasons in the allowlist below. Tests are exempt.
#
# Usage: scripts/check-decode-roots.sh
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

# Production (non-test) Go files containing a JSON decode whose target or
# context names a charter type.
CANDIDATES="$(git grep -n -E 'json\.Unmarshal|Decoder\)\.Decode|\.Decode\(' -- '*.go' ':!*_test.go' | grep -E 'computerevent\.(Event|CASRequest|DurableEvent)|&event\b|&request\.Event|&record\.Request\.Event|&receipt\b|&batch\b|&page\b|DecodeHistoric|DecodeAdmission|DecodeProjectionBatch|ParseDescriptor' || true)"

# Files routed through the centralized roots (must contain a DecodeHistoric*
# or DecodeAdmission* call site; the exact line is pinned in the decoder matrix).
ROOTED=(
  internal/computerevent/decode.go
  internal/platform/event_artifacts.go
  internal/platform/event_replay.go
  internal/platform/checkpoints.go
  internal/platform/event_handlers.go
  internal/computerevent/http_client.go
  internal/store/computer_events.go
  internal/store/computer_event_recovery.go
  internal/projectionbase/source.go
  internal/projectionbase/source_http.go
)
# Vocabulary-neutral centralized decoders (anchor appears on the decode line).
NEUTRAL=(
  "internal/computerevent/projection_batch.go:DecodeProjectionBatch:format discriminator, vocabulary-neutral:&batch"
  "internal/projectionbase/publisher.go:ParseDescriptor:descriptor seam:Decode(&d)"
)
# Out-of-charter-scope decodes: file:presence-anchor:reason:line-pattern.
# The line pattern must match the decode line itself; the presence anchor
# must remain in the file. Charter scope is Event/CASRequest/DurableEvent.
EXEMPT=(
  "internal/computerevent/decode.go:DecodeHistoricEvent:the roots own internal decodes:decoder.Decode(&"
  "internal/platform/file_cas_http.go:BlobSHA256:local ComputerID/BlobSHA256 binding struct:&descriptor"
  "internal/platform/computer_events.go:computerevent.Receipt:receipt namespace, vocabulary-free:&receipt"
  "internal/platform/credential_envelope.go:computerevent.Receipt:receipt namespace, vocabulary-free:&receipt"
  "internal/platform/lifecycle_control.go:computerevent.Receipt:receipt namespace, vocabulary-free:&receipt"
  "internal/platform/self_development_modes.go:computerevent.Receipt:receipt namespace, vocabulary-free:&receipt"
  "internal/platform/event_artifacts.go:computerevent.Receipt:receipt namespace, vocabulary-free:&receipt"
  "internal/platform/checkpoints.go:computerevent.Receipt:receipt namespace, vocabulary-free:&receipt"
  "internal/store/computer_events.go:computerevent.Receipt:receipt namespace, vocabulary-free:&receipt"
  "internal/store/computer_event_recovery.go:record.Receipt:receipt namespace, vocabulary-free:&record.Receipt"
  "internal/proxy/execution_identity.go:computerevent.Receipt:receipt namespace, vocabulary-free:&receipt"
  "internal/receiptsigner/receiptsigner.go:computerevent.Receipt:receipt namespace, vocabulary-free:&receipt"
  "internal/computerevent/http_client.go:failure struct:anonymous error struct, not a charter type:&failure"
  "cmd/choir/main.go:WorkState:anonymous texture-watch struct, not a charter type:WorkState"
  "internal/buildinfo/buildinfo.go:deployReceipt:deployment receipt, not a charter type:&receipt"
  "internal/capsule/executor.go:CapsuleFateReceipt:capsule-local receipt types, outside Event/CASRequest/DurableEvent scope:&receipt"
  "internal/cycle/storage.go:event.Metadata:cycle metadata map, not a charter type:event.Metadata"
  "internal/maild/webhook.go:resendWebhookEvent:webhook event, not a charter type:&event"
  "internal/provider/provider.go:ContentBlock:provider message events, distinct namespace:&event"
)
FAIL=0
while IFS= read -r line; do
  [[ -z "$line" ]] && continue
  file="${line%%:*}"
  rest="${line#*:}"
  lineno="${rest%%:*}"
  text="${rest#*:}"
  covered=""
  # In rooted files, only root calls pass; any other charter-type decode fails.
  for rooted in "${ROOTED[@]}"; do
    if [[ "$file" == "$rooted" ]]; then
      if [[ "$text" == *DecodeHistoric* || "$text" == *DecodeAdmission* ]]; then
        covered="root:$rooted"
      else
        for entry in "${NEUTRAL[@]}" "${EXEMPT[@]}"; do
          allow_file="${entry%%:*}"
          if [[ "$file" != "$allow_file" ]]; then continue; fi
          nofile="${entry#*:}"
          pat="${entry##*:}"
          if [[ "$text" == *"$pat"* ]]; then covered="exempt:$entry"; break; fi
        done
      fi
      break
    fi
  done
  # Outside rooted files, only neutral/exempt line patterns pass.
  if [[ -z "$covered" ]]; then
    for entry in "${NEUTRAL[@]}" "${EXEMPT[@]}"; do
      allow_file="${entry%%:*}"
      pat="${entry##*:}"
      if [[ "$text" == *"$pat"* ]]; then covered="exempt:$entry"; break; fi
    done
  fi
  if [[ -z "$covered" ]]; then
    echo "decode-roots FAIL: unlisted decode root: $file:$lineno: $(echo "$text" | cut -c1-120)"
    FAIL=1
  fi
done <<< "$CANDIDATES"

# Every listed file must still contain its anchor (no silent deletion).
grep -q "func DecodeHistoricEvent" internal/computerevent/decode.go || { echo "decode-roots FAIL: historic root missing"; FAIL=1; }
grep -q "func DecodeAdmissionCASRequest" internal/computerevent/decode.go || { echo "decode-roots FAIL: admission root missing"; FAIL=1; }
for f in "${ROOTED[@]:1}"; do
  grep -q -E "DecodeHistoric|DecodeAdmission" "$f" || { echo "decode-roots FAIL: root call missing in $f"; FAIL=1; }
done
for entry in "${NEUTRAL[@]}" "${EXEMPT[@]}"; do
  allow_file="${entry%%:*}"
  rest="${entry#*:}"
  anchor="${rest%%:*}"
  grep -q -F "$anchor" "$allow_file" || { echo "decode-roots FAIL: anchor '$anchor' missing in $allow_file"; FAIL=1; }
done

if [[ "$FAIL" != "0" ]]; then
  echo "decode-roots: FAILED" >&2
  exit 1
fi
echo "decode-roots: PASS"
