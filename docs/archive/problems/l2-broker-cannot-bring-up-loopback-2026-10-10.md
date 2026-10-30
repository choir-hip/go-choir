# L2 capsules never start: the broker cannot bring up loopback (2026-10-10)

Status: documented, fix follows. Mutation class of the fix: `red` (capsule
isolation, capability set).

## Evidence

Staging at d39e05e9, arXiv demo on test computer computer-eb87caee… .
Management cast a CPU-only environment setup (pinned numpy, scipy, torch
CPU, fixed-seed smoke test). The delegated cast admitted and reached spawn,
which confirms the authority fix (328e009d). Every spawn then failed with
`capsule broker readiness timed out` (20:39:22, again at 20:48:18), and the
guest log held only `capsule broker launcher: exit status 1`, because the
launcher dropped the broker's stderr.

Local reproduction: the real broker binary, launched the way the executor
launches it (new user namespace mapped to 65534, new mount and net
namespaces, launcher stage, `--egress=true`), in a privileged Linux
container:

    failed to start egress forwarder: bring lo up: operation not permitted

A process launched that way has an effective capability set equal to its
ambient set (`CapEff 0x2001cb`: SYS_ADMIN, SETPCAP, DAC_OVERRIDE, FOWNER,
CHOWN, SETUID, SETGID). It does not get the full root set in its user
namespace, so CAP_NET_ADMIN is absent. With CAP_NET_ADMIN added to the
ambient set, the same reproduction passes the egress setup and stops at
Landlock, which the local kernel lacks (the guest kernel has it).

## Cause

The L2 plan (design §4.0b) assumed the broker is full root in its user
namespace before it drops capabilities. It holds only the executor's
ambient list. No unit or local test exercised the executor-to-broker launch
with egress; the forwarder test ran as real root in a plain network
namespace.

## Fix shape

Add CAP_NET_ADMIN to the ambient list for ecosystem_proxy capsules only.
The broker's existing `DropBrokerCapabilities` clears it from the permitted
and bounding sets after the forwarder is up, which also clears it from the
ambient set, so neither the broker's serving phase nor any workload holds
it. Also pass the broker's stderr through the launcher (e018e916).
