#!/usr/bin/env python3
"""Read-only host CPU sampler for Choir computers (runs ON the host).

usage: ssh node-b 'python3 - SECONDS' < scripts/cpu_host_sample.py

Over one window it reports, as JSON:
  - host busy cores and host guest-mode cores (/proc/stat);
  - CPU of each go-choir service cgroup (cpu.stat usage_usec);
  - for every Firecracker process: per-thread user/sys/guest cores and
    context switches (vCPU threads vs the VMM event-loop thread vs fc_api),
    KVM per-VM debugfs counters per second (exits, halt exits, halt-poll
    cores, irq injections, l1d flushes) and the process's block I/O.

It reads /proc/<pid>/{stat,status,io,cmdline} (the cmdline only to find
the --id of a firecracker process), cgroup cpu.stat files and
/sys/kernel/debug/kvm. It reads no guest memory, no environment and no
secrets, and changes nothing. Used by
docs/archive/reports/computer-cpu-efficiency-2026-10-14.md.
"""
import os, time, json, glob, sys
WIN = float(sys.argv[1]) if len(sys.argv) > 1 else 60
HZ = os.sysconf('SC_CLK_TCK')
SERVICES = ['go-choir-auth','go-choir-corpus-dolt','go-choir-corpusd','go-choir-gateway','go-choir-goproxy',
            'go-choir-maild','go-choir-platform-dolt','go-choir-proxy','go-choir-vmctl']
def stat(path):
    s = open(path).read()
    rp = s.rfind(')')
    comm = s[s.find('(')+1:rp]
    f = s[rp+2:].split()
    # f[0]=state (field 3); utime field 14 -> f[11]; stime f[12]; guest_time field 43 -> f[40]
    return comm, int(f[11]), int(f[12]), int(f[40])
def ctx(path):
    v = n = 0
    for line in open(path):
        if line.startswith('voluntary_ctxt_switches'): v = int(line.split()[1])
        if line.startswith('nonvoluntary_ctxt_switches'): n = int(line.split()[1])
    return v, n
def fc_pids():
    out = {}
    for p in glob.glob('/proc/[0-9]*/cmdline'):
        try:
            c = open(p,'rb').read().split(b'\0')
        except Exception: continue
        if c and c[0].endswith(b'firecracker') and b'--id' in c:
            out[p.split('/')[2]] = c[c.index(b'--id')+1].decode()
    return out
def svc_cpu():
    r = {}
    for s in SERVICES:
        try:
            for line in open(f'/sys/fs/cgroup/system.slice/{s}.service/cpu.stat'):
                if line.startswith('usage_usec'): r[s] = int(line.split()[1])
        except Exception: pass
    return r
KVMK = ['exits','halt_exits','halt_wakeup','halt_attempted_poll','halt_successful_poll','halt_poll_success_ns',
        'halt_poll_fail_ns','halt_wait_ns','io_exits','mmio_exits','irq_exits','irq_injections','signal_exits',
        'request_irq_exits','nmi_window_exits','irq_window_exits','insn_emulation','preemption_reported',
        'directed_yield_attempted','pf_taken','tlb_flush','host_state_reload','fpu_reload','l1d_flush']
def kvm(pid):
    d = glob.glob(f'/sys/kernel/debug/kvm/{pid}-*')
    if not d: return {}
    r = {}
    for k in KVMK:
        try: r[k] = int(open(f'{d[0]}/{k}').read())
        except Exception: pass
    return r
def snap():
    t = time.time()
    procstat = open('/proc/stat').readline().split()[1:]
    vms = {}
    for pid, vid in fc_pids().items():
        th = {}
        for tdir in glob.glob(f'/proc/{pid}/task/*'):
            tid = tdir.split('/')[-1]
            try:
                comm, u, s, g = stat(f'{tdir}/stat'); v, n = ctx(f'{tdir}/status')
                th[tid] = dict(comm=comm, u=u, s=s, g=g, vcs=v, nvcs=n)
            except Exception: pass
        try:
            io = dict(l.split(': ') for l in open(f'/proc/{pid}/io').read().strip().split('\n'))
        except Exception: io = {}
        vms[vid] = dict(pid=pid, threads=th, kvm=kvm(pid), io={k:int(v) for k,v in io.items()})
    return dict(t=t, procstat=[int(x) for x in procstat], vms=vms, svc=svc_cpu())
a = snap(); time.sleep(WIN); b = snap()
dt = b['t'] - a['t']
res = dict(window_s=round(dt,1), at=time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(a['t'])))
tot = [y-x for x,y in zip(a['procstat'], b['procstat'])]
alltk = sum(tot[:8]); idle = tot[3]+tot[4]
res['host_busy_cores'] = round((alltk-idle)/HZ/dt, 3)
res['host_guest_cores'] = round(tot[8]/HZ/dt, 3) if len(tot) > 8 else None
res['services_cores'] = {k: round((b['svc'][k]-a['svc'].get(k,0))/1e6/dt, 4) for k in b['svc']}
res['vms'] = {}
for vid, vb in b['vms'].items():
    va = a['vms'].get(vid)
    if not va: continue
    th = {}
    for tid, x in vb['threads'].items():
        y = va['threads'].get(tid)
        if not y: continue
        th[f"{x['comm']}#{tid}"] = dict(user=round((x['u']-y['u'])/HZ/dt,4), sys=round((x['s']-y['s'])/HZ/dt,4),
            guest=round((x['g']-y['g'])/HZ/dt,4), vcs_per_s=round((x['vcs']-y['vcs'])/dt,1), nvcs_per_s=round((x['nvcs']-y['nvcs'])/dt,1))
    k = {kk: round((vb['kvm'][kk]-va['kvm'].get(kk,0))/dt, 1) for kk in vb['kvm']}
    for kk in ('halt_poll_success_ns','halt_poll_fail_ns','halt_wait_ns'):
        if kk in k: k[kk.replace('_ns','_cores')] = round(k.pop(kk)/1e9, 4)
    io = {kk: round((vb['io'][kk]-va['io'].get(kk,0))/dt,1) for kk in vb['io'] if kk in ('rchar','wchar','syscr','syscw','read_bytes','write_bytes')}
    res['vms'][vid] = dict(pid=vb['pid'], threads=th, kvm_per_s=k, io_per_s=io)
print(json.dumps(res, indent=1))
