#!/usr/bin/env bash
# Fuzz the broker protocol (MA6.3) for a fixed time per target. The checked-in
# seeds in internal/yaegikernel/testdata/fuzz and internal/agentcore/testdata/fuzz
# also run as ordinary tests under `go test`. A crasher lands in
# testdata/fuzz/<Target>/ and fails the run; keep it there as a regression
# seed once fixed.
#   scripts/fuzz-broker-protocol.sh [fuzztime]   (default 20s per target)
set -euo pipefail
cd "$(dirname "$0")/.."
time_per=${1:-20s}
# The wire: decode, frame reader, the worker's multiplexer, each verb's
# payload bounds and its handler validator, and the daemon's admission check.
for target in FuzzBrokerRequestDecode FuzzValidatePayload FuzzReadFrame FuzzWorkerMuxReadLoop \
              FuzzSearchFrame FuzzSessionFrameDecode FuzzAdmitFrame FuzzHandlerPayloads; do
  echo "== yaegikernel $target ($time_per)"
  go test ./internal/yaegikernel -run '^$' -fuzz "^${target}\$" -fuzztime="$time_per"
done
# The host handlers themselves, with their confused-deputy properties.
for target in FuzzDeskEgressHandlers; do
  echo "== agentcore $target ($time_per)"
  go test ./internal/agentcore -run '^$' -fuzz "^${target}\$" -fuzztime="$time_per"
done
