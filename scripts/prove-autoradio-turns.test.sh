#!/usr/bin/env bash
# Test of scripts/prove-autoradio-turns against fake servers. Failure list,
# written before the repairs:
#   1. a run passes although the server reported a tape write failure
#      (record_error or report_error in done);
#   2. a data line after a blank line, with no event line of its own, is read
#      under the previous event's name (one_ask keeps the name);
#   3. the good case does not pass (river, reasoning off, cached share shown);
#   4. a Bedrock-only run, or an unconfirmed reasoning state, passes.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd -P)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/prove-test.XXXXXX")
trap 'kill $(jobs -p) 2>/dev/null; rm -rf "$tmp"' EXIT
printf 'dummy-key\n' >"$tmp/key"

cat >"$tmp/fake.py" <<'PY'
import http.server, json, sys
port, provider, reasoning, extra = int(sys.argv[1]), sys.argv[2], sys.argv[3], sys.argv[4]
class H(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    def do_POST(self):
        self.rfile.read(int(self.headers["Content-Length"]))
        self.send_response(200); self.send_header("Content-Type", "text/event-stream"); self.send_header("Connection", "close"); self.end_headers()
        w = self.wfile
        w.write(b'event: speech\ndata: {"seq":0,"text":"Hi."}\n\n')
        if extra == "stray":
            w.write(b'data: {"seq":1,"text":"stray with no event name"}\n\n')
        d = {"status": "completed", "provider": provider, "model": "m", "applied": {"reasoning": reasoning, "cache_break": "accepted"},
             "usage": {"input_tokens": 1000, "cached_input_tokens": 800},
             "timings_ms": {"record_written": 3, "upstream_first_token": 5, "first_sentence": 6, "done": 9}}
        if extra == "record_error":
            d["record_error"] = "disk full"
        if extra == "report_error":
            d["report_error"] = "disk full"
        w.write(("event: done\ndata: " + json.dumps(d) + "\n\n").encode()); w.flush()
    def log_message(self, *a): pass
http.server.HTTPServer(("127.0.0.1", port), H).serve_forever()
PY

fail=0
run() { # name port provider reasoning extra want_exit [grep-pattern]
  local name=$1 port=$2 provider=$3 reasoning=$4 extra=$5 want=$6 pat=${7:-}
  python3 "$tmp/fake.py" "$port" "$provider" "$reasoning" "$extra" >/dev/null 2>&1 &
  local pid=$!
  sleep 0.7
  local out rc
  out=$(python3 "$here/prove-autoradio-turns" --host "127.0.0.1:$port" --insecure-http --api-key-file "$tmp/key" --count 2 2>&1); rc=$?
  kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null
  if [ "$rc" != "$want" ]; then echo "FAIL $name: exit $rc, want $want"; echo "$out" | tail -6; fail=1; return; fi
  if [ -n "$pat" ] && ! grep -q -- "$pat" <<<"$out"; then echo "FAIL $name: output lacks '$pat'"; echo "$out" | tail -6; fail=1; return; fi
  echo "ok   $name"
}

run "good run passes"                  18971 river   off     none         0 "PASS"
run "record_error fails the run"       18972 river   off     record_error 1 "FAIL"
run "report_error fails the run"       18973 river   off     report_error 1 "FAIL"
run "bedrock-only run fails"           18974 bedrock off     none         1 "River never answered"
run "unconfirmed reasoning fails"      18975 river   unknown none         1 "reasoning was not confirmed off"
run "a stray data line is not a speech" 18976 river  off     stray        0 "sent"
# The stray line must not be counted as a second sentence.
python3 "$tmp/fake.py" 18977 river off stray >/dev/null 2>&1 &
pid=$!; sleep 0.7
out=$(python3 "$here/prove-autoradio-turns" --host 127.0.0.1:18977 --insecure-http --api-key-file "$tmp/key" --count 1 2>&1)
kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null
if ! grep -E '^ +1 .* completed' <<<"$out" | grep -qE '  1  completed'; then echo "FAIL stray data line counted as speech"; echo "$out" | head -4; fail=1; else echo "ok   stray data line not counted"; fi

[ "$fail" = 0 ] && echo "ALL PASS"
exit "$fail"
