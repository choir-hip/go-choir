#!/usr/bin/env python3
"""S0: pages read per header statement under three row layouts.

Layouts: A today's (body mid-row), B body last, C header/body split.
Counts bytes read through read syscalls (/proc/self/io rchar) per statement
with a minimal page cache and mmap off, so every page fetch is a pread.
Synthetic data only.
"""
import json, os, random, sqlite3, sys, time

N = int(sys.argv[1]) if len(sys.argv) > 1 else 200_000
DIR = sys.argv[2] if len(sys.argv) > 2 else "/tmp/s0"
os.makedirs(DIR, exist_ok=True)
random.seed(7)

EV_KINDS = ["telemetry.tick", "run.step", "run.done", "texture.rev", "mail.in", "tool.call"]
OWNERS = [f"owner-{i}" for i in range(4)]
PHASES = ["", "start", "end"]


def body_size():
    r = random.random()
    if r < 0.40: return random.randint(200, 900)      # small
    if r < 0.75: return random.randint(1000, 3900)    # fits a leaf page, no overflow
    if r < 0.95: return random.randint(4000, 20000)   # overflow
    return random.randint(20000, 120000)              # large


def rows():
    t0 = 1_700_000_000
    for i in range(N):
        kind = random.choice(EV_KINDS)
        meta = json.dumps({"kind": kind, "phase": random.choice(PHASES), "run": f"r{i % 5000}"})
        ts = time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime(t0 + i * 3))
        yield (f"obj-{i:08d}", "choir.event", random.choice(OWNERS), "comp-1", f"v{i}",
               f"h{i:016x}", os.urandom(body_size()), meta, ts, ts, 0, "")


COLS_HDR = "canonical_id, object_kind, owner_id, computer_id, version_id, content_hash"
COLS_TAIL = "metadata, created_at, updated_at, tombstone, superseded_by"
DDL = {
    "A": f"""CREATE TABLE og_objects (canonical_id TEXT PRIMARY KEY, object_kind TEXT, owner_id TEXT,
        computer_id TEXT, version_id TEXT, content_hash TEXT, body BLOB, metadata TEXT, created_at TEXT,
        updated_at TEXT, tombstone INTEGER, superseded_by TEXT)""",
    "B": f"""CREATE TABLE og_objects (canonical_id TEXT PRIMARY KEY, object_kind TEXT, owner_id TEXT,
        computer_id TEXT, version_id TEXT, content_hash TEXT, metadata TEXT, created_at TEXT,
        updated_at TEXT, tombstone INTEGER, superseded_by TEXT, body BLOB)""",
    "C": f"""CREATE TABLE og_objects (canonical_id TEXT PRIMARY KEY, object_kind TEXT, owner_id TEXT,
        computer_id TEXT, version_id TEXT, content_hash TEXT, metadata TEXT, created_at TEXT,
        updated_at TEXT, tombstone INTEGER, superseded_by TEXT, event_kind TEXT, event_phase TEXT);
        CREATE TABLE og_bodies (obj INTEGER PRIMARY KEY, body BLOB)""",
}
# Today's real indexes that matter here (no workaround expression indexes),
# plus "A+" = today with the 2026-11-01 workaround expression indexes.
BASE_IDX = """CREATE INDEX idx_kind_owner_updated ON og_objects(object_kind, owner_id, updated_at);
CREATE INDEX idx_updated ON og_objects(updated_at);"""
KIND = "CASE WHEN json_valid(metadata) THEN json_extract(metadata,'$.kind') END"
PHASE = "CASE WHEN json_valid(metadata) THEN json_extract(metadata,'$.phase') END"
WORKAROUND = f"""CREATE INDEX idx_ev_kind_updated ON og_objects(({KIND}), updated_at) WHERE object_kind='choir.event';
CREATE INDEX idx_ev_phase_updated ON og_objects(({KIND}),({PHASE}), updated_at) WHERE object_kind='choir.event';"""
C_IDX = "CREATE INDEX idx_ev_typed ON og_objects(event_kind, event_phase, updated_at);"


def build(layout):
    path = f"{DIR}/{layout}.db"
    if os.path.exists(path):
        return path
    db = sqlite3.connect(path)
    db.execute("PRAGMA page_size=4096"); db.execute("PRAGMA journal_mode=OFF"); db.execute("PRAGMA synchronous=OFF")
    base = "C" if layout == "C" else ("B" if layout == "B" else "A")
    db.executescript(DDL[base]); db.executescript(BASE_IDX)
    if layout == "A+": db.executescript(WORKAROUND)
    if layout == "C": db.executescript(C_IDX)
    for r in rows():
        cid, ok, ow, co, ve, ha, body, meta, cr, up, tb, sb = r
        if base == "A":
            db.execute("INSERT INTO og_objects VALUES (?,?,?,?,?,?,?,?,?,?,?,?)", r)
        elif base == "B":
            db.execute("INSERT INTO og_objects VALUES (?,?,?,?,?,?,?,?,?,?,?,?)", (cid, ok, ow, co, ve, ha, meta, cr, up, tb, sb, body))
        else:
            m = json.loads(meta)
            db.execute("INSERT INTO og_objects VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)", (cid, ok, ow, co, ve, ha, meta, cr, up, tb, sb, m["kind"], m["phase"]))
            db.execute("INSERT INTO og_bodies VALUES (last_insert_rowid(),?)", (body,))
    db.commit(); db.execute("ANALYZE"); db.commit(); db.close()
    return path


def rchar():
    with open("/proc/self/io") as f:
        for line in f:
            if line.startswith("rchar:"):
                return int(line.split()[1])


T_MID = time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime(1_700_000_000 + N * 3 // 2))
T_END = time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime(1_700_000_000 + N * 3 // 2 + 3600))


def queries(layout):
    kind = "event_kind" if layout == "C" else KIND
    phase = "event_phase" if layout == "C" else PHASE
    return {
        # retirement slice: telemetry kinds in a one-hour window, no phase
        "retire_slice": (f"SELECT canonical_id FROM og_objects WHERE object_kind='choir.event' AND ({kind}) IN ('telemetry.tick','tool.call') AND ({phase})='' AND updated_at >= ? AND updated_at < ?", (T_MID, T_END)),
        # newest by kind/owner, header fields only
        "kind_owner_newest": (f"SELECT canonical_id, version_id, metadata, updated_at FROM og_objects WHERE object_kind='choir.event' AND owner_id='owner-1' ORDER BY updated_at DESC LIMIT 200", ()),
        # boot-style census over every header
        "census_by_kind": (f"SELECT ({kind}), count(*) FROM og_objects WHERE tombstone=0 GROUP BY 1", ()),
        # refs without body over one owner (the 'refs without body' helpers)
        "refs_one_owner": ("SELECT canonical_id, content_hash, updated_at FROM og_objects WHERE object_kind='choir.event' AND owner_id='owner-2' AND superseded_by=''", ()),
    }


BODY_Q = {"A": "SELECT body FROM og_objects WHERE canonical_id=?", "B": "SELECT body FROM og_objects WHERE canonical_id=?",
          "A+": "SELECT body FROM og_objects WHERE canonical_id=?", "C": "SELECT b.body FROM og_objects o JOIN og_bodies b ON b.obj = o.rowid WHERE o.canonical_id=?"}


def measure(layout, path):
    db = sqlite3.connect(path)
    db.execute("PRAGMA cache_size=" + os.environ.get("S0_CACHE", "-2000")); db.execute("PRAGMA mmap_size=0")
    out = {"layout": layout, "file_mib": round(os.path.getsize(path) / 2**20, 1)}
    for name, (sql, args) in queries(layout).items():
        plan = " | ".join(r[3] for r in db.execute("EXPLAIN QUERY PLAN " + sql, args))
        a = rchar(); t = time.perf_counter()
        n = len(db.execute(sql, args).fetchall())
        out[name] = {"rows": n, "read_mib": round((rchar() - a) / 2**20, 2), "ms": round((time.perf_counter() - t) * 1000), "plan": plan}
    ids = [f"obj-{random.randrange(N):08d}" for _ in range(500)]
    a = rchar()
    got = sum(len(db.execute(BODY_Q[layout], (i,)).fetchone()[0]) for i in ids)
    out["body_point_500"] = {"body_mib": round(got / 2**20, 2), "read_mib": round((rchar() - a) / 2**20, 2)}
    db.close()
    return out


for L in os.environ.get("S0_LAYOUTS", "A A+ B C").split():
    print(json.dumps(measure(L, build(L))), flush=True)
