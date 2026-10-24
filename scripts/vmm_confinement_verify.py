#!/usr/bin/env python3
"""Read-only proof of VMM confinement on Node B (VMM jailer slice 1).

Design: docs/problems/firecracker-runs-as-root-without-jailer-2026-10-14.md
("Design: slice 1"); contract: docs/reports/computer-isolation-security-
review-2026-10-14.md section 4.2.

For every running Firecracker (argv[0] is a firecracker binary with --id) it
reads /proc/<pid>/status (Uid, Gid, CapEff, CapPrm, CapBnd, NoNewPrivs,
Seccomp, speculation lines), /proc/<pid>/root, /proc/<pid>/ns/{mnt,pid,net},
/proc/<pid>/cgroup and the leaf's pids.max/cpu.weight, the number of
environment entries (never their values), and the VM's confinement.json.
It also reads the host: the iptables OUTPUT no-IP rule for VMM uids, and
vmctl's own capability bounding set (CAP_SYS_PTRACE must be gone).

Each VMM is judged against its contract:
  jailer: Uid/Gid in the VMM range (all four fields), CapEff = CapPrm = 0,
          NoNewPrivs 1, Seccomp 2, root = .../firecracker/<vm>/root, mnt and
          pid namespaces differ from init and from every other VMM, cgroup
          leaf .../vms/<vm>, speculation force-mitigated, empty environment.
          Host, while one runs: every jailed leaf is outside vmctl's unit
          and the unit root's cgroup.subtree_control is empty.
  direct: uid 0, CapEff and CapBnd at most CAP_NET_ADMIN, NoNewPrivs 1,
          Seccomp 2, empty environment (the pre-jailer path, tightened).

It changes nothing. Output: one JSON receipt on stdout (and --out). Exit 1
when a judged VMM or host check fails, or --expect is not met.

usage:
  scripts/vmm_confinement_verify.py --ssh node-b [--vm VM_ID ...]
        [--expect jailer|direct|any] [--out FILE]
  (on the host itself, as root: omit --ssh)
"""
import argparse
import json
import os
import subprocess
import sys
import time

CAP_NAMES = {
    0: "chown", 1: "dac_override", 2: "dac_read_search", 3: "fowner", 4: "fsetid",
    5: "kill", 6: "setgid", 7: "setuid", 8: "setpcap", 9: "linux_immutable",
    10: "net_bind_service", 11: "net_broadcast", 12: "net_admin", 13: "net_raw",
    14: "ipc_lock", 15: "ipc_owner", 16: "sys_module", 17: "sys_rawio",
    18: "sys_chroot", 19: "sys_ptrace", 20: "sys_pacct", 21: "sys_admin",
    22: "sys_boot", 23: "sys_nice", 24: "sys_resource", 25: "sys_time",
    26: "sys_tty_config", 27: "mknod", 28: "lease", 29: "audit_write",
    30: "audit_control", 31: "setfcap", 32: "mac_override", 33: "mac_admin",
    34: "syslog", 35: "wake_alarm", 36: "block_suspend", 37: "audit_read",
    38: "perfmon", 39: "bpf", 40: "checkpoint_restore",
}
CAP_NET_ADMIN = 1 << 12
CAP_SYS_PTRACE = 1 << 19
NOIP_COMMENT = "go-choir-vmm-noip"


# --- collection (runs on the host, as root) ---------------------------------

def read(path, default=""):
    try:
        with open(path, "rb") as f:
            return f.read().decode("utf-8", "replace")
    except OSError:
        return default


def readlink(path):
    try:
        return os.readlink(path)
    except OSError as err:
        return "error:" + err.strerror


def stat_id(path):
    try:
        st = os.stat(path)
    except OSError:
        return None
    return [st.st_dev, st.st_ino]


def listdir(path):
    try:
        return os.listdir(path)
    except OSError:
        return []


def status_fields(pid):
    return status_fields_path("/proc/%d/status" % pid)


def status_fields_path(path):
    out = {}
    for line in read(path).splitlines():
        key, _, value = line.partition(":")
        if key in ("Uid", "Gid", "CapEff", "CapPrm", "CapBnd", "CapInh", "CapAmb", "NoNewPrivs",
                   "Seccomp", "Seccomp_filters", "Speculation_Store_Bypass",
                   "SpeculationIndirectBranch", "NSpid", "Cpus_allowed_list"):
            out[key] = value.strip()
    return out


def vm_id_from_cmdline(args):
    if not args or "firecracker" not in os.path.basename(args[0]):
        return ""
    for i, arg in enumerate(args):
        if arg == "--id" and i + 1 < len(args):
            return args[i + 1]
        if arg.startswith("--id="):
            return arg[len("--id="):]
    return ""


def unit_cgroup(cgroup):
    """The nearest *.service or *.scope element of a cgroup path."""
    parts = cgroup.strip("/").split("/")
    for i in range(len(parts) - 1, -1, -1):
        if parts[i].endswith(".service") or parts[i].endswith(".scope"):
            return "/" + "/".join(parts[:i + 1])
    return ""


def collect(state_dir):
    init_ns = {ns: readlink("/proc/1/ns/" + ns) for ns in ("mnt", "pid", "net")}
    vmms = []
    for name in os.listdir("/proc"):
        if not name.isdigit():
            continue
        pid = int(name)
        args = [a for a in read("/proc/%d/cmdline" % pid).split("\0") if a]
        vm_id = vm_id_from_cmdline(args)
        if not vm_id:
            continue
        cgroup = ""
        for line in read("/proc/%d/cgroup" % pid).splitlines():
            if line.startswith("0::"):
                cgroup = line[3:].strip()
        leaf = "/sys/fs/cgroup" + cgroup
        environ = read("/proc/%d/environ" % pid, None)
        confinement = read(os.path.join(state_dir, vm_id, "confinement.json"))
        try:
            confinement = json.loads(confinement) if confinement else None
        except ValueError:
            confinement = {"error": "unparseable"}
        if isinstance(confinement, dict):
            confinement = {k: confinement.get(k) for k in ("mode", "reason", "uid", "jail_root", "cgroup", "pid", "launch_thread", "jail_files")}
        # readlink(/proc/<pid>/root) reads "/" for a process that
        # pivot_rooted in another mount namespace; the inode behind it is
        # the proof: it must be the VM's jail root and not the host's /.
        root_id = stat_id("/proc/%d/root" % pid)
        jail_root = os.path.join(state_dir, ".jail", "firecracker", vm_id, "root")
        vmms.append({
            "vm_id": vm_id,
            "pid": pid,
            "argv0": args[0],
            "status": status_fields(pid),
            "root": readlink("/proc/%d/root" % pid),
            "root_is_jail": root_id is not None and root_id == stat_id(jail_root),
            "root_is_host_root": root_id is not None and root_id == stat_id("/"),
            "thread_seccomp": sorted({status_fields_path("/proc/%d/task/%s/status" % (pid, tid)).get("Seccomp", "")
                                      for tid in listdir("/proc/%d/task" % pid)}),
            "ns": {ns: readlink("/proc/%d/ns/%s" % (pid, ns)) for ns in ("mnt", "pid", "net")},
            "cgroup": cgroup,
            "cgroup_limits": {k: read(os.path.join(leaf, k)).strip() for k in ("pids.max", "cpu.weight", "memory.max", "memory.swap.max")},
            # Count only: values may hold secrets and are never read out.
            "environ_entries": None if environ is None else len([e for e in environ.split("\0") if e]),
            "confinement_record": confinement,
        })
    vmctl_pid = 0
    for name in os.listdir("/proc"):
        if name.isdigit():
            args = [a for a in read("/proc/%s/cmdline" % name).split("\0") if a]
            if args and os.path.basename(args[0]) == "vmctl":
                vmctl_pid = int(name)
    vmctl = {}
    if vmctl_pid:
        cgroup = ""
        for line in read("/proc/%d/cgroup" % vmctl_pid).splitlines():
            if line.startswith("0::"):
                cgroup = line[3:].strip()
        unit = unit_cgroup(cgroup)
        vmctl = {"pid": vmctl_pid, "status": status_fields(vmctl_pid), "cgroup": cgroup, "unit_cgroup": unit,
                 "unit_subtree_control": read("/sys/fs/cgroup" + unit + "/cgroup.subtree_control").strip() if unit else ""}
    rules = {}
    for tool in ("iptables", "ip6tables"):
        try:
            out = subprocess.run([tool, "-S", "OUTPUT"], capture_output=True, text=True, timeout=20).stdout
        except (OSError, subprocess.SubprocessError) as err:
            out = "error:" + str(err)
        rules[tool] = [line for line in out.splitlines() if NOIP_COMMENT in line]
    return {
        "collected_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "host": os.uname().nodename,
        "kernel": os.uname().release,
        "init_ns": init_ns,
        "vmms": vmms,
        "vmctl": vmctl,
        "noip_rules": rules,
        "dev_kvm_mode": oct(os.stat("/dev/kvm").st_mode & 0o7777) if os.path.exists("/dev/kvm") else "",
    }


# --- judgment ---------------------------------------------------------------

def cap_names(hexmask):
    try:
        mask = int(hexmask, 16)
    except (TypeError, ValueError):
        return ["unparseable:" + str(hexmask)]
    return [CAP_NAMES.get(i, "cap_%d" % i) for i in range(64) if mask >> i & 1]


def judge_vmm(v, all_vmms, init_ns, uid_base, uid_count):
    st = v["status"]
    uids = [int(x) for x in st.get("Uid", "").split()] or [-1]
    gids = [int(x) for x in st.get("Gid", "").split()] or [-1]
    checks = []

    def check(name, ok, got):
        checks.append({"check": name, "ok": bool(ok), "got": got})

    jailed = uid_base <= uids[0] < uid_base + uid_count
    v["mode"] = "jailer" if jailed else ("direct" if uids[0] == 0 else "unknown")
    v["cap_eff_names"] = cap_names(st.get("CapEff"))
    v["cap_bnd_names"] = cap_names(st.get("CapBnd"))
    others = [o for o in all_vmms if o is not v]
    if jailed:
        uid = uids[0]
        check("uid_in_vmm_range_all_fields", all(u == uid for u in uids) and len(uids) == 4, st.get("Uid"))
        check("gid_equals_uid", all(g == uid for g in gids), st.get("Gid"))
        check("cap_eff_zero", int(st.get("CapEff", "1"), 16) == 0, st.get("CapEff"))
        check("cap_prm_zero", int(st.get("CapPrm", "1"), 16) == 0, st.get("CapPrm"))
        check("no_new_privs", st.get("NoNewPrivs") == "1", st.get("NoNewPrivs"))
        check("seccomp_filter", st.get("Seccomp") == "2", st.get("Seccomp"))
        check("root_is_own_jail", v["root_is_jail"] and not v["root_is_host_root"],
              {"readlink": v["root"], "same_inode_as_jail": v["root_is_jail"]})
        check("all_threads_seccomp", v["thread_seccomp"] == ["2"], v["thread_seccomp"])
        check("mnt_ns_not_init", v["ns"]["mnt"] != init_ns["mnt"], v["ns"]["mnt"])
        check("pid_ns_not_init", v["ns"]["pid"] != init_ns["pid"], v["ns"]["pid"])
        check("mnt_ns_unique", all(o["ns"]["mnt"] != v["ns"]["mnt"] for o in others), v["ns"]["mnt"])
        check("pid_ns_unique", all(o["ns"]["pid"] != v["ns"]["pid"] for o in others), v["ns"]["pid"])
        check("cgroup_own_leaf", v["cgroup"].endswith("/vms/" + v["vm_id"]) and all(o["cgroup"] != v["cgroup"] for o in others), v["cgroup"])
        check("cgroup_pids_max_set", v["cgroup_limits"].get("pids.max") not in ("", "max"), v["cgroup_limits"].get("pids.max"))
        check("ssbd_forced", "force" in st.get("Speculation_Store_Bypass", ""), st.get("Speculation_Store_Bypass"))
        check("ibpb_stibp_forced", "force" in st.get("SpeculationIndirectBranch", ""), st.get("SpeculationIndirectBranch"))
        check("environment_empty", v["environ_entries"] == 0, v["environ_entries"])
        rec = v.get("confinement_record") or {}
        check("record_matches_process", rec.get("mode") == "jailer" and rec.get("uid") == uid, {"mode": rec.get("mode"), "uid": rec.get("uid")})
    elif uids[0] == 0:
        eff = int(st.get("CapEff", "0"), 16)
        bnd = int(st.get("CapBnd", "0"), 16)
        check("cap_eff_at_most_net_admin", eff & ~CAP_NET_ADMIN == 0, st.get("CapEff"))
        check("cap_bnd_at_most_net_admin", bnd & ~CAP_NET_ADMIN == 0, st.get("CapBnd"))
        check("no_sys_ptrace", eff & CAP_SYS_PTRACE == 0, st.get("CapEff"))
        check("no_new_privs", st.get("NoNewPrivs") == "1", st.get("NoNewPrivs"))
        check("seccomp_filter", st.get("Seccomp") == "2", st.get("Seccomp"))
        check("environment_empty", v["environ_entries"] == 0, v["environ_entries"])
    else:
        check("uid_known", False, st.get("Uid"))
    v["checks"] = checks
    v["ok"] = all(c["ok"] for c in checks)
    return v


def judge(data, args):
    vmms = data["vmms"]
    if args.vm:
        missing = [vm for vm in args.vm if vm not in {v["vm_id"] for v in vmms}]
        selected = [v for v in vmms if v["vm_id"] in args.vm]
    else:
        missing = []
        selected = vmms
    for v in vmms:
        judge_vmm(v, vmms, data["init_ns"], args.uid_base, args.uid_count)
    host_checks = []
    rules = data["noip_rules"].get("iptables", [])
    want_range = "%d-%d" % (args.uid_base, args.uid_base + args.uid_count - 1)
    any_jailed = any(v["mode"] == "jailer" for v in vmms)
    host_checks.append({
        "check": "noip_rule_v4",
        # Required only while a jailed VMM runs.
        "ok": (not any_jailed) or any(want_range in r and "DROP" in r for r in rules),
        "got": rules,
    })
    vmctl = data.get("vmctl") or {}
    if vmctl:
        bnd = vmctl["status"].get("CapBnd", "0")
        host_checks.append({"check": "vmctl_without_sys_ptrace", "ok": int(bnd, 16) & CAP_SYS_PTRACE == 0, "got": cap_names(bnd)})
        # The VM slice (docs/problems/vmctl-restart-fails-while-a-jailed-vm-
        # runs-2026-10-24.md): jailed leaves outside vmctl's unit, and the
        # unit root without controller bits, else a vmctl restart wedges.
        # Required only while a jailed VMM runs.
        unit = vmctl.get("unit_cgroup", "")
        inside = [v["cgroup"] for v in vmms if v["mode"] == "jailer" and unit and
                  (v["cgroup"] == unit or v["cgroup"].startswith(unit + "/"))]
        host_checks.append({"check": "jailed_cgroups_outside_vmctl_unit", "ok": (not any_jailed) or (bool(unit) and not inside),
                            "got": {"unit": unit, "inside": inside}})
        host_checks.append({"check": "vmctl_unit_root_no_controllers",
                            "ok": (not any_jailed) or vmctl.get("unit_subtree_control", "") == "",
                            "got": vmctl.get("unit_subtree_control", "")})
    expect_ok = True
    if args.expect != "any":
        expect_ok = bool(selected) and all(v["mode"] == args.expect for v in selected) and not missing
    ok = all(v["ok"] for v in selected) and all(c["ok"] for c in host_checks) and expect_ok
    return {
        "kind": "vmm_confinement_receipt",
        "ok": ok,
        "expect": args.expect,
        "expect_ok": expect_ok,
        "missing_vms": missing,
        "summary": [{"vm_id": v["vm_id"], "pid": v["pid"], "mode": v["mode"], "ok": v["ok"],
                     "failed": [c["check"] for c in v["checks"] if not c["ok"]]} for v in selected],
        "host_checks": host_checks,
        "collected": data,
    }


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--ssh", help="collect on this ssh host (read-only) instead of locally")
    ap.add_argument("--vm", action="append", default=[], help="judge only this VM id (repeatable)")
    ap.add_argument("--expect", choices=["jailer", "direct", "any"], default="any")
    ap.add_argument("--uid-base", type=int, default=900000)
    ap.add_argument("--uid-count", type=int, default=4096)
    ap.add_argument("--state-dir", default="/var/lib/go-choir/vm-state")
    ap.add_argument("--out")
    ap.add_argument("--collect", action="store_true", help=argparse.SUPPRESS)
    args = ap.parse_args()

    if args.collect:
        json.dump(collect(args.state_dir), sys.stdout)
        return 0
    if args.ssh:
        with open(os.path.abspath(__file__)) as f:
            source = f.read()
        proc = subprocess.run(["ssh", args.ssh, "python3", "-I", "-", "--collect", "--state-dir", args.state_dir],
                              input=source, capture_output=True, text=True, timeout=120)
        if proc.returncode != 0:
            print(json.dumps({"kind": "vmm_confinement_receipt", "ok": False, "error": proc.stderr.strip()[-2000:]}))
            return 2
        data = json.loads(proc.stdout)
    else:
        data = collect(args.state_dir)
    receipt = judge(data, args)
    text = json.dumps(receipt, indent=2, sort_keys=True)
    print(text)
    if args.out:
        with open(args.out, "w") as f:
            f.write(text + "\n")
    return 0 if receipt["ok"] else 1


if __name__ == "__main__":
    sys.exit(main())
