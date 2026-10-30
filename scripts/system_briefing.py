#!/usr/bin/env python3
"""Gather a JSON source pack for a Choir system briefing (v0).

A system briefing is a read-aloud programme for the owner: what changed,
the state of the system with evidence labels, decisions waiting, pathways,
and concepts (docs/briefings/README.md; design in
docs/archive/reports/autoradio-programming-engine-2026-10-14.md section 5). This
script is the gatherer only. It makes no model calls; a writer (a person or
an agent) reads the pack and the sources it points to and writes the
briefing.

Usage:
    python3 -I scripts/system_briefing.py                      # last 24 hours
    python3 -I scripts/system_briefing.py --since 2026-10-14T00:00:00Z
    python3 -I scripts/system_briefing.py --since <git ref> --out pack.json
    python3 -I scripts/system_briefing.py --fetch --no-network

What the pack holds (each key is documented in pack["about"]):
    commits         commits on the remote branch since the cutoff, each with
                    files, a kind (problems / reports / code / docs_other /
                    mixed), an area, and whether staging already runs it;
    active          docs/ACTIVE.md, verbatim;
    goals           the active goal files' `now` cards: the mission graph's
                    entrypoint nodes, goal files linked from ACTIVE.md, and
                    goal files touched since the cutoff;
    calibration     docs/calibration/claude-ledger.md (settled and open rows
                    parsed) and docs/calibration/goal-ledger.md;
    problems        problem docs touched since the cutoff, with change type
                    and status line;
    open_questions  open-question and owner-decision sections from reports
                    touched since the cutoff;
    branches        local branches not merged into the remote branch, with
                    their commit subjects, and the worktree list;
    staging         build.deployed_commit and platform.deployed_commit from
                    https://choir.news/health, nothing else;
    learn           the docs/learn primers (titles only), the concept
                    inventory a briefing's concept chapter draws on.

Safety: the script reads only tracked Markdown and YAML under docs/, plus
git metadata. It refuses any path that looks like a secret or env file, and
from the health endpoint it keeps only the two deployed-commit fields.
Documents are read from the remote branch's tree (git show), not from the
working tree, so the pack reports what landed. The
default output path is outside the repository, because the pack carries
local paths and branch names and the repository is public.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path

HEALTH_URL = "https://choir.news/health"
DEFAULT_REMOTE = "origin/main"
# Only tracked, public documents are read: docs/ Markdown and YAML. Env files,
# keys and anything else are refused by extension and name.
READABLE_SUFFIXES = (".md", ".yaml", ".yml")
SECRET_NAMES = re.compile(
    r"(^|/)\.env|(^|/)\.envrc|\.(pem|key|p12)$|(^|/)id_(rsa|ed25519)", re.IGNORECASE
)
OPEN_Q_HEADING = re.compile(
    r"^(#{2,4})\s+(?:\d+(?:\.\d+)*\.?\s+)?(.*(?:open question|open decision|"
    r"owner decision|decisions? (?:for|waiting|needed|open)|questions? for the owner|"
    r"waiting on the owner|for the owner|unresolved).*)$",
    re.IGNORECASE,
)
DECISION_MEMO_HEADING = re.compile(r"^(#{2,3})\s+(options|recommendation)\b.*$", re.IGNORECASE)
RESIDUAL_HEADING = re.compile(r"^(#{2,3})\s+(?:\d+\.\s+)?(.*\bresiduals?\b.*)$", re.IGNORECASE)
STATUS_LINE =re.compile(r"(\*\*)?Status(\*\*)?\s*[:.](\*\*)?", re.IGNORECASE)
CONVENTIONAL = re.compile(r"^(?P<type>[a-z]+)(?:\((?P<scope>[^)]+)\))?!?:\s")


class Repo:
    def __init__(self, root: Path, remote: str):
        self.root = root
        self.remote = remote

    def git(self, *args: str, check: bool = True) -> str:
        proc = subprocess.run(
            ["git", "-C", str(self.root), *args],
            capture_output=True,
            text=True,
        )
        if check and proc.returncode != 0:
            raise RuntimeError(f"git {' '.join(args)}: {proc.stderr.strip()}")
        return proc.stdout

    def ok(self, *args: str) -> bool:
        return (
            subprocess.run(
                ["git", "-C", str(self.root), *args], capture_output=True
            ).returncode
            == 0
        )

    def read_doc(self, rel: str) -> str | None:
        """Read a tracked file under docs/ at the remote branch's tip.

        Reading from git (not the worktree) keeps the pack about what landed,
        and the docs/ prefix plus the secret filter keep env and key files out.
        """
        if (
            not rel.startswith("docs/")
            or not rel.endswith(READABLE_SUFFIXES)
            or SECRET_NAMES.search(rel)
            or ".." in rel
        ):
            raise PermissionError(f"refusing to read {rel}")
        proc = subprocess.run(
            ["git", "-C", str(self.root), "show", f"{self.remote}:{rel}"],
            capture_output=True,
            text=True,
        )
        return proc.stdout if proc.returncode == 0 else None


def parse_since(repo: Repo, since: str | None) -> dict:
    """Resolve --since to either a timestamp or a git ref."""
    now = dt.datetime.now(dt.timezone.utc)
    if not since:
        t = now - dt.timedelta(hours=24)
        return {"kind": "time", "value": t.isoformat(), "label": "last 24 hours"}
    try:
        t = dt.datetime.fromisoformat(since.replace("Z", "+00:00"))
        if t.tzinfo is None:
            t = t.replace(tzinfo=dt.timezone.utc)
        return {"kind": "time", "value": t.isoformat(), "label": since}
    except ValueError:
        pass
    if repo.ok("rev-parse", "--verify", "--quiet", f"{since}^{{commit}}"):
        sha = repo.git("rev-parse", f"{since}^{{commit}}").strip()
        return {"kind": "ref", "value": sha, "label": since}
    raise SystemExit(f"--since: not an ISO time or a git ref: {since}")


def range_args(repo: Repo, since: dict) -> list[str]:
    if since["kind"] == "ref":
        return [f"{since['value']}..{repo.remote}"]
    return [repo.remote, f"--since={since['value']}"]


def file_kind(path: str) -> str:
    if path.startswith("docs/problems/"):
        return "problems"
    if path.startswith("docs/reports/"):
        return "reports"
    if path.startswith("docs/") or path.endswith(".md"):
        return "docs_other"
    return "code"


def commit_area(subject: str, files: list[str]) -> str:
    m = CONVENTIONAL.match(subject)
    if m and m.group("scope"):
        return m.group("scope").strip()
    code = [f for f in files if file_kind(f) == "code"]
    pick = code or files
    if not pick:
        return "unknown"
    parts = pick[0].split("/")
    if parts[0] in ("internal", "cmd", "frontend") and len(parts) > 2:
        return f"{parts[0]}/{parts[1]}"
    if parts[0] == "docs" and len(parts) > 2:
        return f"docs/{parts[1]}"
    return parts[0]


def gather_commits(repo: Repo, since: dict, deployed: str | None) -> dict:
    sep = "\x1e"
    out = repo.git(
        "log",
        *range_args(repo, since),
        f"--format={sep}%H%x1f%h%x1f%aI%x1f%cI%x1f%s",
        "--name-status",
        "-M",
    )
    not_deployed: set[str] | None = None
    if deployed and repo.ok("cat-file", "-e", f"{deployed}^{{commit}}"):
        not_deployed = set(
            repo.git("rev-list", f"{deployed}..{repo.remote}").split()
        )
    commits = []
    for block in out.split(sep):
        block = block.strip("\n")
        if not block:
            continue
        head, *rest = block.split("\n")
        sha, short, adate, cdate, subject = head.split("\x1f", 4)
        changes = []
        for line in rest:
            if not line.strip():
                continue
            cols = line.split("\t")
            status = cols[0]
            path = cols[-1]
            entry = {"status": status[0], "path": path}
            if status.startswith("R") and len(cols) == 3:
                entry["from"] = cols[1]
            changes.append(entry)
        files = [c["path"] for c in changes]
        kinds = sorted({file_kind(f) for f in files})
        kind = kinds[0] if len(kinds) == 1 else ("mixed" if kinds else "empty")
        on_staging = None
        if not_deployed is not None:
            on_staging = sha not in not_deployed
        commits.append(
            {
                "sha": sha,
                "short": short,
                "author_date": adate,
                "commit_date": cdate,
                "subject": subject,
                "kind": kind,
                "kinds": kinds,
                "area": commit_area(subject, files),
                "on_staging": on_staging,
                "changes": changes,
            }
        )
    by_area: dict[str, list[str]] = {}
    by_kind: dict[str, list[str]] = {}
    for c in commits:
        by_area.setdefault(c["area"], []).append(c["short"])
        by_kind.setdefault(c["kind"], []).append(c["short"])
    by_area = dict(sorted(by_area.items(), key=lambda kv: (-len(kv[1]), kv[0])))
    code_not_on_staging = [
        c["short"]
        for c in commits
        if c["on_staging"] is False and "code" in c["kinds"]
    ]
    return {
        "count": len(commits),
        "by_area": by_area,
        "by_kind": by_kind,
        "code_on_main_not_on_staging": code_not_on_staging,
        "items": commits,
    }


def touched_paths(commits: dict, prefix: str) -> dict[str, dict]:
    """Paths under prefix touched in the window: newest change type wins,
    and a file moved out of prefix (archived) is reported with its new home."""
    seen: dict[str, dict] = {}
    for c in reversed(commits["items"]):  # oldest first
        for ch in c["changes"]:
            p, frm = ch["path"], ch.get("from")
            if frm and frm.startswith(prefix) and not p.startswith(prefix):
                seen[frm] = {"change": "moved", "moved_to": p, "last": c["short"]}
                continue
            if not p.startswith(prefix) or not p.endswith(".md"):
                continue
            prev = seen.get(p)
            change = ch["status"]
            if prev and prev["change"] == "A":
                change = "A"  # added in the window, then edited
            seen[p] = {"change": change, "last": c["short"]}
    return dict(sorted(seen.items()))


def status_line(text: str) -> str | None:
    lines = text.splitlines()
    for i, line in enumerate(lines[:60]):
        m = STATUS_LINE.search(line)
        if not m:
            continue
        para = [line[m.start():]]
        for nxt in lines[i + 1 : i + 8]:
            if not nxt.strip() or nxt.startswith("#"):
                break
            para.append(nxt.strip())
        s = " ".join(x.strip() for x in para)
        return s[:600]
    # a "## Status" section
    for i, line in enumerate(lines):
        if re.match(r"^#{2,3}\s+(current\s+)?status\b", line, re.I):
            body = []
            for nxt in lines[i + 1 : i + 10]:
                if nxt.startswith("#"):
                    break
                if nxt.strip():
                    body.append(nxt.strip())
            return " ".join(body)[:600] or None
    return None


def title_of(text: str) -> str | None:
    for line in text.splitlines()[:10]:
        if line.startswith("# "):
            return line[2:].strip()
    return None


def gather_problems(repo: Repo, commits: dict) -> list[dict]:
    out = []
    for path, info in touched_paths(commits, "docs/problems/").items():
        item = {"path": path, **info}
        text = repo.read_doc(info.get("moved_to", path)) if (
            info.get("moved_to", path).startswith("docs/")
        ) else None
        if text:
            item["title"] = title_of(text)
            item["status_line"] = status_line(text)
        out.append(item)
    return out


def sections_matching(text: str, heading: re.Pattern) -> list[dict]:
    lines = text.splitlines()
    out = []
    i = 0
    while i < len(lines):
        m = heading.match(lines[i])
        if not m:
            i += 1
            continue
        level = len(m.group(1))
        body = []
        j = i + 1
        while j < len(lines):
            h = re.match(r"^(#{1,6})\s", lines[j])
            if h and len(h.group(1)) <= level:
                break
            body.append(lines[j])
            j += 1
        out.append({"heading": lines[i].lstrip("#").strip(), "text": "\n".join(body).strip()})
        i = j
    return out


def gather_open_questions(repo: Repo, commits: dict) -> list[dict]:
    out = []
    for path, info in touched_paths(commits, "docs/reports/").items():
        if info["change"] in ("D", "moved"):
            continue
        text = repo.read_doc(path)
        if not text:
            continue
        secs = [dict(s, kind="open_question") for s in sections_matching(text, OPEN_Q_HEADING)]
        title = title_of(text) or ""
        if title.lower().startswith("decision"):
            # a decision memo: its options and recommendation are the question
            secs += [dict(s, kind="decision_memo") for s in sections_matching(text, DECISION_MEMO_HEADING)]
        seen = {s["heading"] for s in secs}
        secs += [
            dict(s, kind="residual")
            for s in sections_matching(text, RESIDUAL_HEADING)
            if s["heading"] not in seen
        ]
        if secs:
            out.append({"path": path, "change": info["change"], "title": title_of(text), "sections": secs})
    return out


def frontmatter_block(text: str, key: str) -> str | None:
    """Return the raw YAML block for a top-level key in a goal file's
    frontmatter (kept raw so the pack is faithful and needs no YAML library)."""
    m = re.match(r"^---\n(.*?)\n---\n", text, re.S)
    if not m:
        return None
    lines = m.group(1).splitlines()
    out, inside = [], False
    for line in lines:
        if re.match(rf"^{re.escape(key)}:", line):
            inside = True
            out.append(line)
            continue
        if inside:
            if line and not line[0].isspace() and not line.startswith("#"):
                break
            out.append(line)
    return "\n".join(out).rstrip() or None


def graph_nodes(text: str) -> list[dict]:
    """Minimal parse of docs/mission-graph.yaml node entries."""
    nodes, cur = [], None
    for line in text.splitlines():
        m = re.match(r"^  - id:\s*(\S+)", line)
        if m:
            cur = {"id": m.group(1)}
            nodes.append(cur)
            continue
        if cur is None:
            continue
        m = re.match(r"^    (path|status|entrypoint|execution_mode|kind|title):\s*(.*)$", line)
        if m:
            val = m.group(2)
            note = None
            if " #" in val:
                val, note = val.split(" #", 1)
                cur[f"{m.group(1)}_note"] = note.strip()
            cur[m.group(1)] = val.strip()
    return nodes


def gather_goals(repo: Repo, commits: dict, active: str | None) -> list[dict]:
    graph = repo.read_doc("docs/mission-graph.yaml") or ""
    nodes = graph_nodes(graph)
    by_path = {n.get("path"): n for n in nodes if n.get("path")}
    reasons: dict[str, set[str]] = {}
    for n in nodes:
        if n.get("entrypoint") == "true" and n.get("path"):
            reasons.setdefault(n["path"], set()).add("graph_entrypoint")
    if active:
        for rel in re.findall(r"\(?(definitions/[A-Za-z0-9_.\-]+\.md)", active):
            reasons.setdefault("docs/" + rel, set()).add("linked_from_ACTIVE")
    for path, info in touched_paths(commits, "docs/definitions/").items():
        if info["change"] not in ("D", "moved"):
            reasons.setdefault(path, set()).add("touched_in_window")
    out = []
    for path in sorted(reasons):
        text = repo.read_doc(path)
        if not text:
            continue
        node = by_path.get(path, {})
        out.append(
            {
                "path": path,
                "why": sorted(reasons[path]),
                "graph": {k: v for k, v in node.items() if k != "path"} or None,
                "title": title_of(text.split("\n---\n", 1)[-1]) if text.startswith("---") else title_of(text),
                "now": frontmatter_block(text, "now"),
            }
        )
    return out


def parse_md_tables(text: str) -> dict[str, list[dict]]:
    """Parse each '## heading' section's first Markdown table into rows."""
    out: dict[str, list[dict]] = {}
    section = "preamble"
    header: list[str] | None = None
    for line in text.splitlines():
        if line.startswith("## "):
            section, header = line[3:].strip(), None
            continue
        if not line.startswith("|"):
            header = None if not line.strip() else header
            continue
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if header is None:
            header = cells
            continue
        if all(re.fullmatch(r":?-+:?", c) for c in cells):
            continue
        out.setdefault(section, []).append(dict(zip(header, cells)))
    return out


def gather_calibration(repo: Repo) -> dict:
    claude = repo.read_doc("docs/calibration/claude-ledger.md") or ""
    goal = repo.read_doc("docs/calibration/goal-ledger.md") or ""
    tables = parse_md_tables(claude)
    settled = [r for s, rows in tables.items() if s.lower().startswith("settled") for r in rows]
    open_ = [r for s, rows in tables.items() if s.lower().startswith("open") for r in rows]
    brier = []
    for r in settled:
        try:
            p = float(r.get("p", ""))
        except ValueError:
            continue
        o = r.get("Outcome", "").lower()
        if o.startswith("true"):
            brier.append((p - 1) ** 2)
        elif o.startswith("false"):
            brier.append(p ** 2)
    return {
        "claude_ledger": {
            "path": "docs/calibration/claude-ledger.md",
            "settled": settled,
            "open": open_,
            "brier_true_false_rows": {
                "n": len(brier),
                "mean": round(sum(brier) / len(brier), 4) if brier else None,
                "note": "rows whose outcome starts with true/false only; 'partly' rows excluded",
            },
            "text": claude,
        },
        "goal_ledger": {"path": "docs/calibration/goal-ledger.md", "text": goal},
    }


def gather_branches(repo: Repo, since: dict) -> dict:
    names = [
        b.strip().lstrip("+* ").strip()
        for b in repo.git("branch", "--no-merged", repo.remote).splitlines()
        if b.strip()
    ]
    cutoff = None
    if since["kind"] == "time":
        cutoff = dt.datetime.fromisoformat(since["value"])
    else:
        cutoff = dt.datetime.fromisoformat(
            repo.git("log", "-1", "--format=%cI", since["value"]).strip()
        )
    items = []
    for name in names:
        # --cherry-pick drops commits whose patch already landed on the remote
        # (rebased or cherry-picked work), so "unmerged" means not landed.
        log = repo.git(
            "log", "--cherry-pick", "--right-only", "--no-merges",
            f"{repo.remote}...{name}", "--format=%h\x1f%cI\x1f%s", check=False,
        )
        subjects = []
        newest = None
        for line in log.splitlines():
            if not line.strip():
                continue
            h, when, s = line.split("\x1f", 2)
            subjects.append({"sha": h, "date": when, "subject": s})
            t = dt.datetime.fromisoformat(when)
            newest = t if newest is None or t > newest else newest
        if not subjects:
            continue  # every commit's patch is already on the remote
        items.append(
            {
                "branch": name,
                "unlanded_commits": len(subjects),
                "newest_commit": newest.isoformat() if newest else None,
                "active_in_window": bool(newest and newest >= cutoff),
                "subjects": subjects[:30],
            }
        )
    active = [b for b in items if b["active_in_window"]]
    active.sort(key=lambda b: b["newest_commit"] or "", reverse=True)
    stale = [
        {"branch": b["branch"], "unlanded_commits": b["unlanded_commits"], "newest_commit": b["newest_commit"]}
        for b in items
        if not b["active_in_window"]
    ]
    worktrees = []
    cur: dict = {}
    for line in repo.git("worktree", "list", "--porcelain").splitlines() + [""]:
        if not line.strip():
            if cur:
                worktrees.append(cur)
            cur = {}
            continue
        k, _, v = line.partition(" ")
        cur[k] = v or True
    return {
        "unmerged_count": len(names),
        "with_unlanded_patches": len(items),
        "active_in_window": active,
        "stale": stale,
        "worktrees": worktrees,
    }


def gather_staging(no_network: bool) -> dict:
    if no_network:
        return {"skipped": "no-network"}
    try:
        proc = subprocess.run(
            ["curl", "-s", "--max-time", "20", HEALTH_URL],
            capture_output=True,
            text=True,
            timeout=30,
        )
        data = json.loads(proc.stdout)
    except Exception as exc:  # network or JSON failure is recorded, not fatal
        return {"error": f"{type(exc).__name__}: {exc}"[:200]}
    build = data.get("build") or {}
    platform = data.get("platform") or {}
    return {
        "url": HEALTH_URL,
        "fetched_at": dt.datetime.now(dt.timezone.utc).isoformat(),
        "build_deployed_commit": build.get("deployed_commit"),
        "platform_deployed_commit": platform.get("deployed_commit"),
    }


def gather_learn(repo: Repo) -> list[dict]:
    out = []
    listing = repo.git("ls-tree", "--name-only", f"{repo.remote}:docs/learn", check=False)
    for name in listing.split():
        if not name.endswith(".md"):
            continue
        text = repo.read_doc(f"docs/learn/{name}") or ""
        out.append({"path": f"docs/learn/{name}", "title": title_of(text)})
    return out


ABOUT = {
    "commits": "Commits on the remote branch since the cutoff. kind: problems | reports | code | docs_other | mixed. on_staging: true if the commit is an ancestor of staging's build.deployed_commit (docs-only commits never deploy; read on_staging only for code).",
    "active": "docs/ACTIVE.md verbatim (the curated work view).",
    "goals": "Goal files with their raw `now` card: mission-graph entrypoints, goal files linked from ACTIVE.md, and goal files touched since the cutoff.",
    "calibration": "Claude's calibration ledger (settled and open rows) and the goal ledger. Settle open rows only with the evidence each row names.",
    "problems": "Problem docs touched since the cutoff. change: A added, M modified, moved (archived). The status line is the doc's own and may be stale: check it against later commits.",
    "open_questions": "Sections from reports touched since the cutoff, verbatim. kind: open_question (open questions, owner decisions), decision_memo (options and recommendation of a 'Decision:' memo), residual (named residuals: open work, usually not an owner decision).",
    "branches": "Local branches not merged into the remote branch, keeping only commits whose patch is not already on it (git log --cherry-pick). active_in_window if the newest such commit is after the cutoff. Built but not landed; a branch may also be abandoned.",
    "staging": "Only build.deployed_commit and platform.deployed_commit from the health endpoint.",
    "learn": "The docs/learn primer inventory.",
}


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--since", help="ISO time (e.g. 2026-10-14T00:00:00Z) or git ref; default: 24 hours ago")
    ap.add_argument("--out", help="output path; default: a file under the system temp dir")
    ap.add_argument("--repo", default=str(Path(__file__).resolve().parent.parent))
    ap.add_argument("--remote", default=DEFAULT_REMOTE, help="branch the briefing reports on (default origin/main)")
    ap.add_argument("--fetch", action="store_true", help="git fetch the remote first")
    ap.add_argument("--no-network", action="store_true", help="skip the staging health probe")
    args = ap.parse_args()

    repo = Repo(Path(args.repo), args.remote)
    if args.fetch:
        repo.git("fetch", "--quiet", args.remote.split("/", 1)[0])
    since = parse_since(repo, args.since)
    staging = gather_staging(args.no_network)
    commits = gather_commits(repo, since, staging.get("build_deployed_commit"))
    active = repo.read_doc("docs/ACTIVE.md")
    pack = {
        "schema": "choir-system-briefing-pack/1",
        "generated_at": dt.datetime.now(dt.timezone.utc).isoformat(),
        "remote": args.remote,
        "remote_head": repo.git("rev-parse", args.remote).strip(),
        "since": since,
        "about": ABOUT,
        "staging": staging,
        "commits": commits,
        "active": {"path": "docs/ACTIVE.md", "text": active},
        "goals": gather_goals(repo, commits, active),
        "calibration": gather_calibration(repo),
        "problems": gather_problems(repo, commits),
        "open_questions": gather_open_questions(repo, commits),
        "branches": gather_branches(repo, since),
        "learn": gather_learn(repo),
    }
    if args.out:
        out = Path(args.out)
    else:
        stamp = dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
        out = Path(tempfile.gettempdir()) / "choir-briefing" / f"pack-{stamp}.json"
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps(pack, indent=1, ensure_ascii=False))
    summary = {
        "out": str(out),
        "since": since["label"],
        "commits": commits["count"],
        "code_on_main_not_on_staging": len(commits["code_on_main_not_on_staging"]),
        "goals": len(pack["goals"]),
        "problems": len(pack["problems"]),
        "reports_with_open_questions": len(pack["open_questions"]),
        "branches_active_in_window": len(pack["branches"]["active_in_window"]),
        "staging": staging.get("build_deployed_commit") or staging,
    }
    print(json.dumps(summary, indent=1))
    return 0


if __name__ == "__main__":
    sys.exit(main())
