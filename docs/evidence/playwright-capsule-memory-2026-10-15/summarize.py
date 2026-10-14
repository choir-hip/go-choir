import json, sys, statistics as st
rows = {}
for p in sys.argv[1:]:
    for line in open(p):
        if not line.strip():
            continue
        r = json.loads(line)
        tag = f"{r.get('cpu_max_cores', 1)}cpu"
        rows.setdefault((r["scenario"], tag), []).append(r)
keys = ["cgroup_peak_over_base_mib", "sampled_max_anon_mib", "sampled_max_file_mib", "chrome_max_pss_mib",
        "after_close_mib", "after_close_file_mib", "after_reclaim_mib", "cpu_s", "wall_s", "cpu_cores_avg",
        "capture_ms", "chrome_max_procs", "cgroup_max_pids", "shot_kib", "video_kib"]
for (scn, tag), rs in rows.items():
    out = {k: (min(r[k] for r in rs), st.median(r[k] for r in rs), max(r[k] for r in rs)) for k in keys}
    print(f"## {scn} [{tag}] n={len(rs)} base={st.median(r['base_mib'] for r in rs)}")
    print("   " + "; ".join(f"{k}={v[1]} ({v[0]}-{v[2]})" for k, v in out.items()))
