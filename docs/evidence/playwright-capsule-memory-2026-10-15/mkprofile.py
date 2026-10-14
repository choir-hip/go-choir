import json, re, sys
src = open(sys.argv[1]).read()
block = re.search(r'capsuleAllowedSyscalls = \[\]string\{(.*?)\n\}', src, re.S).group(1)
allowed = sorted(set(re.findall(r'"([a-z0-9_]+)"', block)))
extra = sys.argv[3].split(',') if len(sys.argv) > 3 and sys.argv[3] else []
prof = {
    "defaultAction": "SCMP_ACT_ERRNO",
    "defaultErrnoRet": 1,
    "architectures": ["SCMP_ARCH_AARCH64", "SCMP_ARCH_X86_64"],
    "syscalls": [
        {"names": allowed + extra, "action": "SCMP_ACT_ALLOW"},
        # WorkloadInetSeccompFilter: socket() only for AF_UNIX, AF_INET, AF_INET6
        {"names": ["socket"], "action": "SCMP_ACT_ALLOW", "args": [{"index": 0, "value": 1, "op": "SCMP_CMP_EQ"}]},
        {"names": ["socket"], "action": "SCMP_ACT_ALLOW", "args": [{"index": 0, "value": 2, "op": "SCMP_CMP_EQ"}]},
        {"names": ["socket"], "action": "SCMP_ACT_ALLOW", "args": [{"index": 0, "value": 10, "op": "SCMP_CMP_EQ"}]},
    ],
}
json.dump(prof, open(sys.argv[2], "w"), indent=1)
print(len(allowed) + len(extra), "names")
