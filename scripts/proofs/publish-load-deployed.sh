#!/usr/bin/env bash
# Deployed proof for docs/problems/story-publishes-fail-loading-the-private-
# revision-2026-11-20.md. Read-only against Node B: two ssh reads (the proxy
# journal, the paper's guest console); nothing is written or restarted there.
#
# Prints, per UTC hour: the failing publishes seen by the proxy, counted as
# distinct revisions among its "platform publish fetch revision" lines (a
# publish that fails is now tried up to three times, so lines are not
# publishes; before the fix: 10 to 48 an hour), the guest console's story publish
# outcomes (published, failed and stays a gap, retry lines), and the median
# and p90 of the new guest line
#   texture api: revision load revision=<id> refs=N entities=M refs_ms=A total_ms=B
# (ids and counts only). The console keeps about 2.5 hours; the journal keeps
# days. Pass the deploy time as --since (to the minute, UTC) to count only the
# failing publishes at or after it.
#
#   publish-load-deployed.sh [--hours N] [--since YYYY-MM-DDTHH:MMZ] [--node HOST]
#
# Exit 0 when the timing line exists and, WITH --since, no hour has 2 or more
# failing publishes at or after it (the claim in the problem record). WITHOUT
# --since, exit 0 means only that the timing line exists; it says nothing about
# failures. 1 when a check fails; 2 when a fetch failed.
set -euo pipefail
HOURS=24; SINCE=""; NODE=node-b
PAPER_VM=vm-1669020adcece51c2f5ff9b6df399ef5
while [ $# -gt 0 ]; do
  case "$1" in
    --hours) HOURS="$2"; shift 2 ;;
    --since) SINCE="$2"; shift 2 ;;
    --node) NODE="$2"; shift 2 ;;
    *) echo "usage: $0 [--hours N] [--since T] [--node HOST]" >&2; exit 2 ;;
  esac
done
[[ "$HOURS" =~ ^[0-9]+$ ]] || { echo "--hours must be a number" >&2; exit 2; }
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
ssh -C -o BatchMode=yes -o ConnectTimeout=15 "$NODE" \
  "sudo -n journalctl -u go-choir-proxy --since '${HOURS} hours ago' --no-pager -o short-iso 2>/dev/null | grep 'platform publish fetch revision' | sed -E 's/^([^ ]+) .*revisions\\/([0-9a-f-]+).*/\\1 \\2/'" \
  > "$tmp/proxy.txt" || { echo "journal read failed" >&2; exit 2; }
ssh -C -o BatchMode=yes -o ConnectTimeout=15 "$NODE" \
  "sudo -n sh -c 'cd /var/lib/go-choir/vm-state/${PAPER_VM} && for f in console.log.4 console.log.3 console.log.2 console.log.1 console.log; do [ -f \$f ] && cat \$f; done' 2>/dev/null | grep -E 'story publish:|texture api: revision load'" \
  > "$tmp/console.txt" || { echo "console read failed" >&2; exit 2; }
python3 -I - "$tmp/proxy.txt" "$tmp/console.txt" "$SINCE" <<'PY'
import collections, datetime, re, statistics, sys
proxy, console, since = sys.argv[1], sys.argv[2], sys.argv[3]
cut = None
if since:
    cut = datetime.datetime.strptime(since, "%Y-%m-%dT%H:%MZ").replace(tzinfo=datetime.timezone.utc)
def parse(ts):  # 2026-11-21T05:46:46+0000
    return datetime.datetime.strptime(ts, "%Y-%m-%dT%H:%M:%S%z")
failing = collections.defaultdict(set)   # hour -> revisions
after = collections.defaultdict(set)
for line in open(proxy):
    parts = line.split()
    if len(parts) != 2: continue
    t = parse(parts[0]).astimezone(datetime.timezone.utc)
    h = t.strftime("%Y-%m-%d %H")
    failing[h].add(parts[1])
    if cut and t >= cut: after[h].add(parts[1])
print("proxy: failing publishes per UTC hour (distinct revisions in the journal)")
for h in sorted(failing):
    print("  %s:00Z %3d%s" % (h, len(failing[h]), ("  (at or after --since: %d)" % len(after[h])) if cut else ""))
pub, fail, retry = collections.Counter(), collections.Counter(), collections.Counter()
loads, fallbacks = [], 0
for line in open(console, errors="replace"):
    m = re.search(r"(\d{4}/\d\d/\d\d) (\d\d):\d\d:\d\d (.*)", line)
    if not m: continue
    h = m.group(1).replace("/", "-") + " " + m.group(2)
    body = m.group(3)
    if "story publish:" in body and " published v" in body: pub[h] += 1
    elif "failed and stays a gap" in body: fail[h] += 1
    elif "story publish:" in body and "retrying in" in body: retry[h] += 1
    t = re.search(r"texture api: revision load revision=\S+ refs=(\d+) entities=(\d+) fallback=(\d) refs_ms=(\d+) total_ms=(\d+)", body)
    if t:
        loads.append((h, int(t.group(5)), int(t.group(1)), int(t.group(2))))
        fallbacks += int(t.group(3))
print("guest console: story publish outcomes per UTC hour")
for h in sorted(set(pub) | set(fail) | set(retry)):
    print("  %s:00Z published %3d  failed(gap) %3d  retried %3d" % (h, pub[h], fail[h], retry[h]))
if not loads:
    print("no 'texture api: revision load' line yet: the fix is not deployed or no revision was read")
    sys.exit(1)
ms = sorted(x[1] for x in loads)
p = lambda q: ms[min(len(ms) - 1, int(q * len(ms)))]
print("revision load: n=%d median_ms=%d p90_ms=%d max_ms=%d; refs median=%d entities median=%d; fallback scans=%d" % (
    len(ms), statistics.median(ms), p(0.9), ms[-1],
    statistics.median(x[2] for x in loads), statistics.median(x[3] for x in loads), fallbacks))
if cut:
    bad = [h for h, r in after.items() if len(r) >= 2]
    if bad:
        print("not ok: %d hour(s) at or after --since with 2 or more failing publishes" % len(bad)); sys.exit(1)
    print("ok: timing line present; under 2 failing publishes an hour at or after --since")
else:
    print("ok: timing line present (no --since: failures not checked)")
PY
