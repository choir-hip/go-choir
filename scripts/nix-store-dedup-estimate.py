"""Estimate what `nix-store --optimise` would free in a Nix store. Read-only.

    nice -n 19 ionice -c3 python3 -I scripts/nix-store-dedup-estimate.py [--root /nix/store] [--json]

docs/problems/node-b-nix-store-and-host-disk-policy-2026-10-22.md, item 1.
Walks the store (not /nix/store/.links, nix's own link farm), groups regular
files by size, and hashes only the files whose size another inode shares.
Identical content held by k distinct inodes frees (k - 1) x size once
optimised. Files that already share an inode (link count above 1, the weekly
nix.optimise's work) count as already linked. Symlinks and directories are
never counted. Nothing is written.
"""

import argparse
import hashlib
import json
import os
import sys
from collections import defaultdict


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def estimate(root):
    by_size = defaultdict(dict)  # size -> {(dev, ino): path}
    files = total = linked = 0
    seen_linked = set()
    for dirpath, dirnames, filenames in os.walk(root):
        if dirpath == root and ".links" in dirnames:
            dirnames.remove(".links")
        for name in filenames:
            path = os.path.join(dirpath, name)
            try:
                st = os.lstat(path)
            except OSError:
                continue
            if not os.path.stat.S_ISREG(st.st_mode):
                continue
            files += 1
            key = (st.st_dev, st.st_ino)
            if st.st_nlink > 1 and key not in seen_linked:
                seen_linked.add(key)
                linked += st.st_size
            if key in by_size[st.st_size]:
                continue
            total += st.st_size
            by_size[st.st_size][key] = path
    reclaimable = groups = 0
    for size, inodes in by_size.items():
        if size == 0 or len(inodes) < 2:
            continue
        by_hash = defaultdict(int)
        for path in inodes.values():
            try:
                by_hash[sha256(path)] += 1
            except OSError:
                continue
        for count in by_hash.values():
            if count > 1:
                groups += 1
                reclaimable += (count - 1) * size
    return {
        "root": root,
        "files": files,
        "distinct_inode_bytes": total,
        "already_linked_bytes": linked,
        "duplicate_groups": groups,
        "reclaimable_bytes": reclaimable,
    }


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--root", default="/nix/store")
    ap.add_argument("--json", action="store_true")
    args = ap.parse_args()
    r = estimate(os.path.abspath(args.root))
    if args.json:
        json.dump(r, sys.stdout)
        print()
        return
    gib = 1 << 30
    print(f"store {r['root']}: {r['files']} files, {r['distinct_inode_bytes'] / gib:.1f} GiB in distinct inodes")
    print(f"already linked (shared inodes): {r['already_linked_bytes'] / gib:.1f} GiB")
    print(f"an optimise would free about {r['reclaimable_bytes'] / gib:.2f} GiB ({r['duplicate_groups']} duplicate groups)")


if __name__ == "__main__":
    main()
