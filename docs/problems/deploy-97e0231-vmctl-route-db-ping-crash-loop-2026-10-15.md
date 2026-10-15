# Deploy of 97e023100 (P4): vmctl crash loop and failed deploy (2026-10-15)

Pointer only. CI run 38717028854 (P4, `97e023100`): the nixos switch ran about
29 minutes and vmctl exited status 1 every 30 s from 10:42:23 to 10:45:43
(`ping ComputerVersion route database: context deadline exceeded`, then
start-limit-hit); the deploy was recorded incomplete. Root cause, evidence and
repair: [platform-dolt-deadlock-hangs-checkpointd-and-crashloops-vmctl-2026-10-15](platform-dolt-deadlock-hangs-checkpointd-and-crashloops-vmctl-2026-10-15.md).
