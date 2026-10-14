#!/bin/bash
# Runs inside the privileged measurement container. Each scenario gets a fresh
# cgroup leaf shaped like an engineering capsule (memory.max 2 GiB, no swap,
# cpu.max 1 CPU unless CPU_MAX is set, pids.max 256).
#
# Layout under /w: this directory's files, plus site/ = a built frontend/dist
# with heavy.html copied into it, plus `npm i playwright-core@1.59.1`.
# Host command (docs/reports/playwright-in-capsules-memory-2026-10-15.md):
#   docker run --rm --privileged --cgroupns=private --memory=4g --cpus=2 \
#     --shm-size=1g -e REPS=3 -v "$DIR":/w \
#     mcr.microsoft.com/playwright:v1.59.1-noble bash /w/run.sh
set -u
cd /w
mkdir -p /sys/fs/cgroup/init out
for p in $(cat /sys/fs/cgroup/cgroup.procs); do echo $p > /sys/fs/cgroup/init/cgroup.procs 2>/dev/null; done
echo "+memory +cpu +pids" > /sys/fs/cgroup/cgroup.subtree_control
node server.mjs & sleep 1
REPS=${REPS:-3}
for scn in ${SCENARIOS:-one-shot new-headless heavy five-pages four-parallel video-30s}; do
  for r in $(seq 1 $REPS); do
    cg=cap-$scn-$r
    mkdir /sys/fs/cgroup/$cg
    echo $((2*1024*1024*1024)) > /sys/fs/cgroup/$cg/memory.max
    echo 0 > /sys/fs/cgroup/$cg/memory.swap.max
    echo "${CPU_MAX:-100000 100000}" > /sys/fs/cgroup/$cg/cpu.max
    echo ${PIDS_MAX:-256} > /sys/fs/cgroup/$cg/pids.max
    # cold page cache for the browser binary on rep 1 only
    if [ "$r" = 1 ]; then sync; echo 3 > /proc/sys/vm/drop_caches; fi
    sh -c "echo \$\$ > /sys/fs/cgroup/$cg/cgroup.procs; exec node scenario.mjs $scn $cg out" | sed "s/^{/{\"rep\":$r,/"
    sleep 1
  done
done
