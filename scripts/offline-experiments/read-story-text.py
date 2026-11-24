# Runs on Node B under sudo (from scripts/offline-experiments/export-digest), reading stdin lines "<version_id>\t<manifest body_ref>"
# and writing one JSON line per version: {"version_id", "title", "text"}; text is the first LIMIT bytes of the story text blob. Read only.
# is the first LIMIT bytes of the story text blob the manifest names. Read only.
import sys, os, json
ROOT = "/var/lib/go-choir/platform-artifacts"
LIMIT = 6000
def safe(ref):
    p = os.path.normpath(os.path.join(ROOT, ref.lstrip("/")))
    return p if p.startswith(ROOT + "/") else None
for line in sys.stdin:
    vid, _, ref = line.rstrip("\n").partition("\t")
    out = {"version_id": vid, "title": "", "text": ""}
    try:
        p = safe(ref)
        if p:
            with open(p, "rb") as f:
                man = json.loads(f.read())
            out["title"] = man.get("title", "") or ""
            sp = safe(man.get("storage_ref", "") or "")
            if sp:
                with open(sp, "rb") as f:
                    out["text"] = f.read(LIMIT).decode("utf-8", "replace")
    except Exception as e:
        out["error"] = str(e)[:100]
    sys.stdout.write(json.dumps(out, ensure_ascii=False) + "\n")
