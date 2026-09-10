#!/usr/bin/env python3
"""Corpus enumerator for the mission-2 V1 inventory gate (query_version 1).

Prints JSON {"prod": [...], "bad_ext": [...], "test_support": [...]} where
prod is the frozen-corpus file set: tracked, non-test production sources with
covered extensions. Tracked symlinks (e.g. the nix `result` link) are skipped.
Extensionless scripts/* are ops/test shell tooling (negative-swept clean);
any other extensionless or unknown-extension file in scope is reported in
bad_ext and fails the gate until classified in a Define update.

Usage: v1-inventory-corpus.py <repo-root>
"""
import json
import subprocess
import sys

COVERED = {".go", ".ts", ".js", ".svelte", ".yaml", ".yml", ".json",
           ".toml", ".sql", ".html", ".sh", ".nix"}
TOOLING = {".md", ".mjs", ".py", ".mod", ".sum", ".lock", ".css", ".png",
           ".icns", ".plist", ".pbxproj", ".entitlements", ".swift",
           ".texture", ".envrc", ".gitignore"}


def istest(f):
    return ("_test." in f or ".test." in f or "/tests/" in f
            or f.startswith("frontend/tests/"))


def main():
    root = sys.argv[1]
    entries = subprocess.run(["git", "ls-files", "-s"], capture_output=True,
                             text=True, cwd=root).stdout.splitlines()
    files = [e.split("\t", 1)[1] for e in entries
             if not e.startswith("120000")]
    prod, bad_ext, test_support = [], [], []
    for f in files:
        if (f.startswith(("docs/", "specs/", "tmp/", ".git/"))
                or "__pycache__" in f or istest(f)):
            continue
        base = f.rsplit("/", 1)[-1]
        ext = "." + base.rsplit(".", 1)[-1] if "." in base else ""
        if ext in COVERED:
            if f == "internal/base/testkit/scenarios.go":
                test_support.append(f)
            else:
                prod.append(f)
        elif ext not in TOOLING and not f.startswith(".github/"):
            if ext == "" and f.startswith("scripts/"):
                pass  # extensionless ops/test shell tooling; swept clean
            else:
                bad_ext.append(f)
    print(json.dumps({"prod": sorted(prod), "bad_ext": sorted(bad_ext),
                      "test_support": sorted(test_support)}))


if __name__ == "__main__":
    main()
