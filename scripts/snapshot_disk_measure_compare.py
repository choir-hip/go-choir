#!/usr/bin/env python3
"""Compare the three snapshot disk measures on one or more files (read-only).

For each path, prints one line with:
  st_blocks     st_blocks*512, the uncompressed allocated size;
  fiemap_bound  vmmanager's unprivileged measure: FIEMAP extents, each
                compressed (ENCODED) extent bounded by the distance to the
                next higher physical start among the file's extents;
  tree_search   the exact compressed size from BTRFS_IOC_TREE_SEARCH, as
                compsize computes it (needs CAP_SYS_ADMIN; "err ..." if not).

It only opens files read-only and issues read-only ioctls. Use it on Node B
(as root, which has CAP_SYS_ADMIN outside vmctl's unit) to check a
hibernate's logged disk_bytes against the truth:

  ssh node-b 'python3 -I - /var/lib/go-choir/vm-state/<vm>/snapshot/mem' \
      < scripts/snapshot_disk_measure_compare.py

Problem: docs/problems/snapshot-budget-falls-back-to-logical-bytes-2026-10-14.md.
"""
import bisect
import fcntl
import os
import struct
import sys

FS_IOC_FIEMAP = 0xC020660B
BTRFS_IOC_TREE_SEARCH = 0xD0009411
LAST, UNKNOWN, DELALLOC, ENCODED, INLINE = 0x1, 0x2, 0x4, 0x8, 0x200
NO_START = INLINE | UNKNOWN | DELALLOC
U64_MAX = 2**64 - 1


def fiemap(fd):
    exts, start, batch = [], 0, 4096
    while True:
        hdr = struct.pack('=QQIIII', start, U64_MAX, 1, 0, batch, 0)
        buf = bytearray(hdr + b'\0' * (56 * batch))
        fcntl.ioctl(fd, FS_IOC_FIEMAP, buf, True)
        mapped = struct.unpack_from('=I', buf, 20)[0]
        if mapped == 0:
            return exts
        for i in range(mapped):
            lo, ph, ln, _, _, fl = struct.unpack_from('=QQQQQI', buf, 32 + 56 * i)
            exts.append((lo, ph, ln, fl))
        lo, ph, ln, fl = exts[-1]
        if fl & LAST or lo + ln <= start:
            return exts
        start = lo + ln


def fiemap_bound(exts):
    starts = sorted(e[1] for e in exts if not e[3] & NO_START and e[1])
    seen, total = set(), 0
    for lo, ph, ln, fl in exts:
        if fl & NO_START or not ph:
            total += ln
            continue
        if ph in seen:
            continue
        seen.add(ph)
        size = ln
        if fl & ENCODED:
            j = bisect.bisect_right(starts, ph)
            if j < len(starts):
                size = min(ln, starts[j] - ph)
        total += size
    return total


def tree_search(fd, ino):
    total, seen, minoff = 0, set(), 0
    while True:
        args = bytearray(4096)
        struct.pack_into('=QQQQQQQIII', args, 0, 0, ino, ino, minoff, U64_MAX, 0, U64_MAX, 108, 108, 4096)
        fcntl.ioctl(fd, BTRFS_IOC_TREE_SEARCH, args, True)
        nr = struct.unpack_from('=I', args, 64)[0]
        if nr == 0:
            return total
        off, last = 104, 0
        for _ in range(nr):
            _, oid, key_off, typ, ln = struct.unpack_from('=QQQII', args, off)
            off += 32
            item = args[off:off + ln]
            off += ln
            last = key_off
            if oid != ino or typ != 108:
                continue
            if item[20] == 0:
                total += ln - 21
            elif item[20] in (1, 2):
                bytenr, disk_num = struct.unpack_from('<QQ', item, 21)
                if bytenr and bytenr not in seen:
                    seen.add(bytenr)
                    total += disk_num
        if last == U64_MAX:
            return total
        minoff = last + 1


def main(paths):
    for path in paths:
        fd = os.open(path, os.O_RDONLY)
        try:
            st = os.fstat(fd)
            exts = fiemap(fd)
            try:
                truth = str(tree_search(fd, st.st_ino))
            except OSError as err:
                truth = 'err %s' % err
            encoded = sum(1 for e in exts if e[3] & ENCODED)
            print('%s size=%d st_blocks=%d fiemap_bound=%d tree_search=%s extents=%d encoded=%d'
                  % (path, st.st_size, st.st_blocks * 512, min(fiemap_bound(exts), st.st_blocks * 512),
                     truth, len(exts), encoded))
        finally:
            os.close(fd)


if __name__ == '__main__':
    if len(sys.argv) < 2:
        sys.exit('usage: snapshot_disk_measure_compare.py FILE...')
    main(sys.argv[1:])
