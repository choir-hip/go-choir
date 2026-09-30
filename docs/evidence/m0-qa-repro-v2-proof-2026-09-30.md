# Deployed QA-repro proof — research/emit → texture v2 (2026-09-30)

Objective: prove the desk-to-desk supervision path the QA stall exposed is
closed on staging — an owner prompt produces texture v1, a research/emit leg,
then texture v2, on the deployed build. This is the M-SUB / M1 deployed
acceptance anchor.

Deployed build: build.commit = 9e3d6948 (M0a choir.* egress verbs +
emission-drain fix + handle auth + research messaging-authority repair).
deployed_commit field lagged at 2404e7d2 (deploy ledger, separate from the
running build — verified via /health build.commit).

Environment fault encountered and recovered non-destructively this session:
platform-dolt ballooning to ~28GB anon-rss on the 31GB Node B box triggers the
kernel OOM-killer, which kills the retained-computer firecracker on a ~20-40min
cycle; each reboot runs boot passivation that parks in-flight desk cells. A
secondary symptom was vmctl's stale guest route (resolved via POST
/internal/vmctl/refresh). Evidence: dmesg "Out of memory: Killed process ...
(dolt) total-vm:48GB", vmmanager "exited with error: signal: killed".

Result — trajectory 2491cb20-ec91-5b56-b2c7-c7a1d03482e7, document
211d4458-aa10-5920-97d6-6dbb071e2e3c, prompt "What's new in ai today":

  rev  c47924d7-1ba5  author=user      2026-09-30T12:03:32  (owner prompt)
  rev  22dcee8e-a31a  author=appagent  2026-09-30T12:10:50  (texture v1)
  rev  3f010a8a-e3e9  author=appagent  2026-09-30T12:38:53  (texture v2)

Verification command (read-only):
  curl -H "Authorization: Bearer $CHOIR_API_KEY" \
       -H "X-Choir-Computer: computer-03335285269bdba4f94377e56879f9e6" \
       https://choir.news/api/texture/documents/211d4458-aa10-5920-97d6-6dbb071e2e3c/revisions
  -> 3 revisions, 2 post-owner (appagent): v1 + v2.

Conclusion: PASS — the supervision path closed end-to-end on staging. The stall
the mission targeted (research unable to surface evidence to texture) is fixed:
the v2 revision was authored by texture AFTER the emit/research leg on the
9e3d6948 build.

Residual: the OOM-driven reboot cycle means deployed proofs must complete inside
one ~20-40min uptime window; sustained acceptance still wants the platform-dolt
memory fix. This proof landed across one reboot boundary via the texture-durable
revision (v1 survived, v2 authored post-restart).
