#!/usr/bin/env bash
# Guest isolation I5 network drill, run as root on Node B.
# docs/definitions/choir-guest-isolation-2026-11-01.md (I5) and its ledger.
#
# Every target is ours: network namespaces on Node B, never an outside host.
#   drill-t  the target, 10.99.0.2 (the only block d@e's drill route may dial)
#   drill-v  the victim guest stand-in, 10.98.0.2
#   drill-f  the flooding guest stand-in, 10.98.0.6 (stand-in mode only)
# Host side of each is a veth: dt-h 10.99.0.1, dv-h 10.98.0.1, df-h 10.98.0.5.
# The host masquerades 10.98.0.0/24 out dt-h, as it does a guest's /30.
#
#   setup               build the namespaces, the cake stand-in, the sinks
#   teardown            remove every piece (also run by the timed revert)
#   standin-conns       netns F opens 10000 flows over 30 s through a copy of
#                       the connlimit rule (same iptables spec as the real one)
#   standin-share MODE  victim bandwidth alone, then with F flooding; MODE is
#                       cake (as nix/node-b.nix) or control (per-flow only)
#   deployed-conns      d@e opens 10000 flows over 30 s through the REAL
#                       CHOIR-GUEST-FWD chain (needs the net-conns guest build)
#   deployed-share MODE d@e floods (net-flood) while the victim measures
#   status              what is built, and the rule/qdisc counters
#
# What it proves and cannot: see the I5 ledger entry. The real eno1/ifb cake
# is never loaded: it sits on the far side of eno1 and every target there is a
# third party. The cake here is the same qdisc line on dt-h (egress) and an
# ifb (ingress), 900mbit, so it shows cake's fairness as configured, not the
# eno1 instance's counters.
#
# Safety: setup arms a timed revert (systemd-run, DRILL_TTL seconds, default
# 1500) that runs teardown; nothing here touches eno1, ifb4eno1, the real
# chains or any real tap's rules except one -I FORWARD jump on df-h.
set -euo pipefail

SELF=$(readlink -f "$0")
WORK=${DRILL_WORK:-/var/lib/go-choir/drill-scratch/net}
TTL=${DRILL_TTL:-1500}
TAG=guest-net-drill
PAPER=computer-1355b83bced1613eb048ee28d66688a8
OWNER=computer-03335285269bdba4f94377e56879f9e6
DAE=${DAE_COMPUTER:-computer-59c902c5d46caecc0a180a8557164e72}
CAKE_OUT="cake bandwidth 900mbit besteffort dual-srchost nat"
CAKE_IN="cake bandwidth 900mbit besteffort dual-dsthost nat ingress"
CAKE_CTL="cake bandwidth 900mbit besteffort flows nat"
CONNS=${CONNS:-10000}
DIR=${DIR:-up}          # up: guest sends (egress cake); down: guest receives

log() { echo "[$(date -u +%H:%M:%SZ)] $*"; }

ipt_rules() { # add (-I/-A) or delete (-D)
  local op=$1
  if [ "$op" = add ]; then
    iptables -N CHOIR-DRILL-FWD 2>/dev/null || true
    iptables -C CHOIR-DRILL-FWD -m conntrack --ctstate NEW -m connlimit --connlimit-above 8192 --connlimit-mask 32 --connlimit-saddr -m comment --comment $TAG -j DROP 2>/dev/null ||
      iptables -A CHOIR-DRILL-FWD -m conntrack --ctstate NEW -m connlimit --connlimit-above 8192 --connlimit-mask 32 --connlimit-saddr -m comment --comment $TAG -j DROP
    iptables -C CHOIR-DRILL-FWD -m comment --comment $TAG -j ACCEPT 2>/dev/null ||
      iptables -A CHOIR-DRILL-FWD -m comment --comment $TAG -j ACCEPT
    iptables -C FORWARD -i df-h -m comment --comment $TAG -j CHOIR-DRILL-FWD 2>/dev/null ||
      iptables -I FORWARD 1 -i df-h -m comment --comment $TAG -j CHOIR-DRILL-FWD
    iptables -t nat -C POSTROUTING -s 10.98.0.0/24 -o dt-h -m comment --comment $TAG -j MASQUERADE 2>/dev/null ||
      iptables -t nat -A POSTROUTING -s 10.98.0.0/24 -o dt-h -m comment --comment $TAG -j MASQUERADE
  else
    iptables -D FORWARD -i df-h -m comment --comment $TAG -j CHOIR-DRILL-FWD 2>/dev/null || true
    iptables -F CHOIR-DRILL-FWD 2>/dev/null || true
    iptables -X CHOIR-DRILL-FWD 2>/dev/null || true
    iptables -t nat -D POSTROUTING -s 10.98.0.0/24 -o dt-h -m comment --comment $TAG -j MASQUERADE 2>/dev/null || true
  fi
}

write_helpers() {
  mkdir -p "$WORK"
  cat >"$WORK/sink.py" <<'PY'
import asyncio, sys
async def sink(r, w):
    try:
        while await r.read(65536):
            pass
    except Exception:
        pass
    w.close()
Z = bytes(65536)
async def source(r, w):
    try:
        while True:
            w.write(Z)
            await w.drain()
    except Exception:
        pass
    w.close()
async def main():
    s1 = await asyncio.start_server(sink, "10.99.0.2", 9000, backlog=16384)
    s2 = await asyncio.start_server(source, "10.99.0.2", 9001, backlog=1024)
    async with s1, s2:
        await asyncio.Event().wait()
asyncio.run(main())
PY
  cat >"$WORK/conns.py" <<'PY'
# conns.py TARGET COUNT PACE_SECONDS HOLD_SECONDS: open COUNT flows, hold them.
import asyncio, sys, time, resource, json
resource.setrlimit(resource.RLIMIT_NOFILE, (65536, 65536))
host, port = sys.argv[1].rsplit(":", 1)
count, pace, hold = int(sys.argv[2]), float(sys.argv[3]), float(sys.argv[4])
async def main():
    sem = asyncio.Semaphore(256)
    held, ok, bad = [], 0, 0
    async def one(i):
        nonlocal ok, bad
        await asyncio.sleep(pace * i / count)
        async with sem:
            try:
                r, w = await asyncio.wait_for(asyncio.open_connection(host, int(port)), 3)
                held.append(w); ok += 1
            except Exception:
                bad += 1
    t0 = time.time()
    await asyncio.gather(*(one(i) for i in range(count)))
    print(json.dumps({"phase": "opened", "opened": ok, "failed": bad, "seconds": round(time.time() - t0, 1)}), flush=True)
    await asyncio.sleep(hold)
    for w in held:
        w.close()
asyncio.run(main())
PY
  cat >"$WORK/bw.py" <<'PY'
# bw.py TARGET STREAMS SECONDS: send zeros (up) or receive (down); print Mbit/s.
import socket, sys, threading, time, json, os
host, port = sys.argv[1].rsplit(":", 1)
streams, secs = int(sys.argv[2]), float(sys.argv[3])
down = port == "9001"
total = [0] * streams
end = time.time() + secs
Z = bytes(65536)
def run(i):
    s = socket.create_connection((host, int(port)), 5)
    while time.time() < end:
        if down:
            n = len(s.recv(65536))
            if not n: break
        else:
            n = s.send(Z)
        total[i] += n
    s.close()
ts = [threading.Thread(target=run, args=(i,)) for i in range(streams)]
t0 = time.time()
[t.start() for t in ts]; [t.join() for t in ts]
el = time.time() - t0
print(json.dumps({"streams": streams, "seconds": round(el, 1), "mbit_s": round(sum(total) * 8 / el / 1e6, 1)}), flush=True)
PY
}

cmd_setup() {
  ip netns list | grep -q '^drill-t' && { log "already set up"; return; }
  ulimit -n 65536
  write_helpers
  # The timed revert comes first, so a half-built setup is still removed.
  systemd-run --quiet --unit=guest-net-drill-revert --on-active="${TTL}s" "$SELF" teardown
  log "timed revert armed: guest-net-drill-revert in ${TTL}s"
  for n in t v f; do ip netns add drill-$n; done
  ip link add dt-h type veth peer name dt-n; ip link set dt-n netns drill-t
  ip link add dv-h type veth peer name dv-n; ip link set dv-n netns drill-v
  ip link add df-h type veth peer name df-n; ip link set df-n netns drill-f
  ip addr add 10.99.0.1/30 dev dt-h; ip addr add 10.98.0.1/30 dev dv-h; ip addr add 10.98.0.5/30 dev df-h
  for d in dt-h dv-h df-h; do ip link set $d up; done
  ip -n drill-t addr add 10.99.0.2/30 dev dt-n
  ip -n drill-v addr add 10.98.0.2/30 dev dv-n
  ip -n drill-f addr add 10.98.0.6/30 dev df-n
  for n in t v f; do ip -n drill-$n link set lo up; ip -n drill-$n link set d$n-n up; done
  ip -n drill-t route add default via 10.99.0.1
  ip -n drill-v route add default via 10.98.0.1
  ip -n drill-f route add default via 10.98.0.5
  for n in t v f; do ip netns exec drill-$n sysctl -qw net.core.somaxconn=16384; done
  ip netns exec drill-t sysctl -qw net.ipv4.tcp_max_syn_backlog=16384
  [ "$(sysctl -n net.ipv4.ip_forward)" = 1 ] || { log "ip_forward is off; refusing"; cmd_teardown; exit 1; }
  ipt_rules add
  cmd_cake cake
  nohup ip netns exec drill-t python3 "$WORK/sink.py" >"$WORK/sink.log" 2>&1 &
  echo $! >"$WORK/sink.pid"
  sleep 1
  log "set up; sink pid $(cat "$WORK/sink.pid")"
}

cmd_cake() { # cake | control : the stand-in qdiscs on dt-h
  local mode=$1 out=$CAKE_OUT in=$CAKE_IN
  [ "$mode" = control ] && { out=$CAKE_CTL; in=$CAKE_CTL; }
  modprobe sch_cake; modprobe ifb numifbs=0
  tc qdisc replace dev dt-h root $out
  ip link show ifb4dt >/dev/null 2>&1 || ip link add ifb4dt type ifb
  ip link set ifb4dt up
  tc qdisc replace dev ifb4dt root $in
  tc qdisc del dev dt-h ingress 2>/dev/null || true
  tc qdisc add dev dt-h handle ffff: ingress
  tc filter add dev dt-h parent ffff: matchall action mirred egress redirect dev ifb4dt
  log "qdiscs ($mode): $(tc qdisc show dev dt-h | head -1)"
}

cmd_teardown() {
  if [ -f "$WORK/sink.pid" ]; then kill "$(cat "$WORK/sink.pid")" 2>/dev/null || true; rm -f "$WORK/sink.pid"; fi
  ipt_rules del
  tc qdisc del dev dt-h root 2>/dev/null || true
  tc qdisc del dev dt-h ingress 2>/dev/null || true
  ip link del ifb4dt 2>/dev/null || true
  for l in dt-h dv-h df-h; do ip link del $l 2>/dev/null || true; done
  for n in t v f; do ip netns pids drill-$n 2>/dev/null | xargs -r kill 2>/dev/null || true; ip netns del drill-$n 2>/dev/null || true; done
  systemctl stop guest-net-drill-revert.timer 2>/dev/null || true
  log "torn down"
}

conn_counters() {
  echo "--- connlimit rule counters (pkts dropped):"
  iptables -L "$1" -n -v -x | grep -E 'conn src' || true
  echo "--- conntrack count/max: $(cat /proc/sys/net/netfilter/nf_conntrack_count)/$(cat /proc/sys/net/netfilter/nf_conntrack_max)"
}

cmd_standin_conns() {
  conn_counters CHOIR-DRILL-FWD
  log "netns F opens $CONNS flows over 30 s to 10.99.0.2:9000, holds 15 s"
  ip netns exec drill-f python3 "$WORK/conns.py" 10.99.0.2:9000 "$CONNS" 30 15 &
  local p=$!
  sleep 40
  echo "--- mid-hold: flows from 10.98.0.6 in conntrack: $(grep -c "src=10.98.0.6 " /proc/net/nf_conntrack)"
  wait $p
  conn_counters CHOIR-DRILL-FWD
}

victim() { ip netns exec drill-v python3 "$WORK/bw.py" 10.99.0.2:$1 "$2" "$3"; }
fport() { [ "$DIR" = down ] && echo 9001 || echo 9000; }

cmd_standin_share() {
  local mode=${1:-cake}
  cmd_cake "$mode"
  log "DIR=$DIR mode=$mode: victim alone (1 stream, 10 s)"
  victim "$(fport)" 1 10
  log "flooder alone (16 streams, 10 s)"
  ip netns exec drill-f python3 "$WORK/bw.py" 10.99.0.2:"$(fport)" 16 10
  log "both: flooder 16 streams for 20 s, victim 1 stream for 10 s from t+5"
  ip netns exec drill-f python3 "$WORK/bw.py" 10.99.0.2:"$(fport)" 16 20 | sed 's/^/flooder /' &
  local p=$!
  sleep 5
  victim "$(fport)" 1 10 | sed 's/^/victim during flood /'
  wait $p
  tc -s qdisc show dev dt-h | head -4
  [ "$mode" = control ] && cmd_cake cake
}

# ---- d@e (the real CHOIR-GUEST-FWD path) ----
dae_port() {
  local d vm port comp
  d=$(grep -l "$DAE" /var/lib/go-choir/vm-state/vm-*/fc-config.json | head -1 | xargs -r dirname | xargs -r basename)
  [ -n "$d" ] || { echo "no VM state for d@e" >&2; exit 1; }
  for comp in $PAPER $OWNER; do
    [ "$DAE" != "$comp" ] || { echo "refusing the paper or the owner's computer" >&2; exit 1; }
  done
  port=$(iptables -t nat -S OUTPUT | sed -n "s/.*--dport \([0-9]*\) .*go-choir-vm-$d .*/\1/p" | head -1)
  [ -n "$port" ] || { echo "d@e ($d) has no host port: not running" >&2; exit 1; }
  echo "$port"
}
dae_call() { # QUERY ; host->guest carries only X-Internal-Caller
  curl -sS --max-time 120 -X POST -H 'X-Internal-Caller: true' "http://127.0.0.1:$(dae_port)/internal/diag/drill?$1"
}

cmd_deployed_conns() {
  conn_counters CHOIR-GUEST-FWD
  log "d@e net-conns count=$CONNS seconds=45 to 10.99.0.2:9000"
  dae_call "kind=net-conns&target=10.99.0.2:9000&count=$CONNS&seconds=45" &
  local p=$!
  sleep 35
  echo "--- mid-hold: sink-side established flows: $(ip netns exec drill-t ss -Htn state established | wc -l)"
  wait $p; echo
  conn_counters CHOIR-GUEST-FWD
}

cmd_deployed_share() {
  local mode=${1:-cake}
  cmd_cake "$mode"
  log "DIR=$DIR mode=$mode: victim alone (1 stream, 10 s)"
  victim "$(fport)" 1 10
  log "d@e net-flood 16 streams 25 s; victim measures 10 s from t+8"
  dae_call "kind=net-flood&target=10.99.0.2:$(fport)&streams=16&seconds=25" | sed 's/^/d@e report /' &
  local p=$!
  sleep 8
  victim "$(fport)" 1 10 | sed 's/^/victim during flood /'
  wait $p; echo
  tc -s qdisc show dev dt-h | head -4
  [ "$mode" = control ] && cmd_cake cake
}

cmd_status() {
  ip netns list; tc qdisc show dev dt-h 2>/dev/null | head -2
  iptables -L CHOIR-DRILL-FWD -n -v -x 2>/dev/null || true
  iptables -L CHOIR-GUEST-FWD -n -v -x | grep -E 'conn src' || true
}

case ${1:-} in
  setup) cmd_setup ;;
  teardown) cmd_teardown ;;
  standin-conns) cmd_standin_conns ;;
  standin-share) cmd_standin_share "${2:-cake}" ;;
  deployed-conns) cmd_deployed_conns ;;
  deployed-share) cmd_deployed_share "${2:-cake}" ;;
  status) cmd_status ;;
  *) sed -n '2,25p' "$0"; exit 2 ;;
esac
