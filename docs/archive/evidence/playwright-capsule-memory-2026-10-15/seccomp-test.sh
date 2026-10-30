#!/bin/bash
# Runs under a Docker seccomp profile built from the capsule allowlist.
cd /w
node server.mjs & sleep 1
for scn in one-shot video-30s; do
  if timeout 120 node scenario-plain.mjs $scn > /w/out/seccomp-$scn.log 2>&1; then echo "$scn: ok"; else echo "$scn: FAILED rc=$?"; head -c 600 /w/out/seccomp-$scn.log; echo; fi
done
