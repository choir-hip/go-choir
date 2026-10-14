#!/bin/bash
# List the syscalls a headless-shell screenshot and a video recording make, to
# compare with the capsule seccomp allowlist (internal/capsule/seccomp.go).
set -u
cd /w
apt-get -qq update >/dev/null 2>&1 && apt-get -qq install -y strace >/dev/null 2>&1
node server.mjs & sleep 1
mkdir -p /sys/fs/cgroup/st && echo $((2*1024*1024*1024)) > /sys/fs/cgroup/st/memory.max 2>/dev/null
for scn in one-shot video-30s; do
  strace -f -qq -c -o /w/out/strace-$scn.txt node scenario-plain.mjs $scn >/dev/null 2>&1
  awk 'NR>2 && $NF!="total" && $NF!~/^-/ {print $NF}' /w/out/strace-$scn.txt | sort -u > /w/out/syscalls-$scn.txt
  echo "$scn: $(wc -l < /w/out/syscalls-$scn.txt) distinct syscalls"
done
