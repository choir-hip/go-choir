#!/usr/bin/env python3
"""Send System 1 jobs (internal/offlineslice S1Job lines) to OpenRouter's decisions endpoint,
as the gateway does (internal/gateway/jev.go): model ~typesafe/jev-latest, the response body kept
whole. Resumes: jobs already in OUT are skipped. A real 429 or 5xx is retried after the provider's
Retry-After (else 5 s, 15 s, 45 s); no limiter of ours.
The key is read by the loader of scripts/event-blind-sample.py (environment, else the one named .env line); it is never printed or written.
  jev-run.py JOBS OUT [--workers N] [--limit N]"""
import json, os, re, sys, time, threading, urllib.request, urllib.error
from concurrent.futures import ThreadPoolExecutor

URL = "https://openrouter.ai/api/alpha/decisions"
MODEL = "~typesafe/jev-latest"

def key():
    # The loader of scripts/event-blind-sample.py (openrouter_key), copied with only the repo-root
    # path adjusted for this directory: the environment, else the one named line of the repo .env.
    # The key goes only into the request header; it is never printed, logged or stored.
    if os.environ.get("OPENROUTER_API_KEY"):
        return os.environ["OPENROUTER_API_KEY"]
    root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    for line in open(os.path.join(root, ".env")):
        mm = re.match(r'\s*(?:export\s+)?OPENROUTER_API_KEY\s*=\s*"?([^"\s]+)', line)
        if mm:
            return mm.group(1)
    sys.exit("no OPENROUTER_API_KEY in the environment or .env")

def post(k, job):
    body = json.dumps({"model": MODEL, "state": json.loads(job["state"]) if isinstance(job["state"], str) else job["state"], "questions": job["questions"]}).encode()
    delay = [5, 15, 45]
    for attempt in range(4):
        req = urllib.request.Request(URL, data=body, headers={"Authorization": "Bearer " + k, "Content-Type": "application/json", "Accept": "application/json"})
        t0 = time.time()
        try:
            with urllib.request.urlopen(req, timeout=120) as r:
                return r.status, time.time() - t0, r.read().decode()
        except urllib.error.HTTPError as e:
            txt = e.read().decode()[:500]
            if e.code in (429, 500, 502, 503, 504) and attempt < 3:
                ra = e.headers.get("Retry-After")
                time.sleep(float(ra) if ra and ra.replace(".", "").isdigit() else delay[attempt])
                continue
            return e.code, time.time() - t0, txt
        except Exception as e:
            if attempt < 3:
                time.sleep(delay[attempt]); continue
            return 0, time.time() - t0, str(e)[:300]

def main():
    jobs_path, out_path = sys.argv[1], sys.argv[2]
    workers = int(sys.argv[sys.argv.index("--workers") + 1]) if "--workers" in sys.argv else 6
    limit = int(sys.argv[sys.argv.index("--limit") + 1]) if "--limit" in sys.argv else 0
    done = set()
    if os.path.exists(out_path):
        for l in open(out_path):
            try:
                d = json.loads(l)
                if d["status"] == 200:
                    done.add(d["job"])
            except Exception:
                pass
    jobs = [json.loads(l) for l in open(jobs_path)]
    jobs = [j for j in jobs if j["id"] not in done]
    if limit:
        jobs = jobs[:limit]
    k = key()
    lock = threading.Lock()
    out = open(out_path, "a")
    n = [0]
    def run(j):
        st, sec, txt = post(k, j)
        rec = {"job": j["id"], "status": st, "seconds": round(sec, 3)}
        if st == 200:
            rec["body"] = json.loads(txt)
        else:
            rec["err"] = txt
        with lock:
            out.write(json.dumps(rec, ensure_ascii=False) + "\n"); out.flush()
            n[0] += 1
            if n[0] % 20 == 0:
                print(n[0], "of", len(jobs), file=sys.stderr, flush=True)
    with ThreadPoolExecutor(workers) as ex:
        list(ex.map(run, jobs))
    print("sent", len(jobs), file=sys.stderr)

main()
