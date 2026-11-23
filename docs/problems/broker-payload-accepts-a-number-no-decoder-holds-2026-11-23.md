# The broker's payload check accepts a number no decoder holds

Date: 2026-11-23. Class of the fix: orange (the validator's behavior changes).

## Problem

`ValidatePayload` (`internal/yaegikernel/broker_bounds.go`) accepted a
payload holding a number literal that is valid JSON grammar but overflows
float64, such as `{"":1e700}`. `json.Valid` and the token walk
(`json.Decoder` with `UseNumber`) both pass it; `json.Unmarshal` into `any` or
`float64` refuses it. The admitted payload then fails later and differently in
each consumer that decodes numbers: the per-field bound checks (which skip a
value they cannot decode, so a bound field could carry such a value past the
bound), `SourceAPIPayload.Params` (`map[string]any`), and any re-encode.

## Evidence

- CI run 45183432665 on commit 42fa0d9f4, job "Desk Protocol Lock", step
  "Broker protocol fuzz (fixed time)": `--- FAIL: FuzzValidatePayload`,
  `broker_fuzz_test.go:136: accepted payload is not JSON: "{\"\":1e700}"`,
  input `testdata/fuzz/FuzzValidatePayload/e15a31d61d94fd92`. The commit did
  not touch the package; the time-boxed fuzzer found a latent defect.
- Reproduced locally before the fix by the committed seed and by
  `TestValidatePayloadNumbersAndNeighbours` (every overflow case failed, no neighbour case did).

## Belief

The validator was wrong, not the test's oracle. Its own contract is "valid
JSON ... before any handler decodes it", and at least one handler type decodes
into `any`. The rule now: a number literal that `strconv.ParseFloat` rejects
(overflow, either sign) is refused with the literal truncated in the message;
nothing is rounded to infinity. Underflow (`1e-700`) parses to 0 and is held.
Long integer literals that fit float64 (300 digits) are held. The empty key,
duplicate keys and invalid UTF-8 (which `json.Unmarshal` replaces with U+FFFD)
were already accepted and decode, so they are unchanged.

## Remaining error

- A number is checked as float64 even where a handler reads an int64: an
  integer between 2^53 and 1.8e308 is held here and the typed handler still
  judges its own range. That is the handler's decode error, as before.
- Only the broker's payload check changed; other JSON entry points were not
  audited for the same overflow.
