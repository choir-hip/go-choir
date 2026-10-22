#!/usr/bin/env python3
"""What the snapshot budget counts against what the disk holds (read-only).

RES-14. For every <state>/<vm>/snapshot and snapshot.partial directory it
prints one row per snapshot:

  recorded     meta.json disk_bytes and disk_bytes_method: what vmctl's
               budget adds up (internal/vmmanager/snapshot.go:697);
  apparent     sum of st_size (meta.json "bytes" is the same figure);
  allocated    sum of st_blocks*512 (uncompressed allocated extents);
  fiemap_bound vmctl's unprivileged measure, clamped to allocated;
  tree_search  referenced compressed bytes from BTRFS_IOC_TREE_SEARCH, as
               compsize counts them (needs CAP_SYS_ADMIN: run as root);
  exclusive    tree_search minus extents FIEMAP flags SHARED: the bytes a
               deletion would free (fiemap_bound based without root);
  shared_ext   extents flagged SHARED (nonzero: deletion frees less);
  apparent_vs_mem  apparent / (mem_size_mib MiB + vmstate): above 1.01 the
               memory file is larger than the budget's "need" (gap G2).

Then totals (the budget's sum against the disk's), deleted snapshot memory
files still mapped by running processes (gap G1, read through
/proc/<pid>/map_files, root only), and statfs free space against the
16 GiB floor.

Usage, on Node B as root, reading the script from stdin:

  ssh node-b 'python3 -I - /var/lib/go-choir/vm-state' \\
      < scripts/snapshot_budget_bytes_probe.py
  ... add --json for one JSON object.

Read-only: files are opened O_RDONLY; the only ioctls are FS_IOC_FIEMAP
(without FIEMAP_FLAG_SYNC, so it flushes nothing) and the tree search; it
reads /proc and writes nothing. It needs no secret and prints none
(meta.json is parsed for size fields and the VM id only).

Ways it can mislead, and what it does about each:
  - not root: tree_search reads "err ..." and exclusive falls back to the
    FIEMAP bound; G1 rows are not readable. The header says which.
  - not btrfs: tree_search errs (ENOTTY); allocated is then the truth.
  - a snapshot resumed or evicted mid-walk: its row reads "vanished".
  - dirty pages (a capture in progress): FIEMAP reports DELALLOC extents at
    their logical length; the row's delalloc count is nonzero.
  - SHARED is matched per extent by physical range; a shared extent whose
    FIEMAP start lies outside its tree-search range is missed (counted as
    exclusive). Only reflinks or subvolume snapshots produce shared
    extents, and Node B's config creates neither.
  - meta.json missing or unreadable (a partial): recorded reads "-".

Problem: docs/problems/snapshot-budget-falls-back-to-logical-bytes-2026-10-14.md
Station: docs/definitions/choir-launch-fp10-res-14-2026-10-22.md
"""
import bisect
import fcntl
import json
import os
import stat
import struct
import sys

FS_IOC_FIEMAP = 0xC020660B
BTRFS_IOC_TREE_SEARCH = 0xD0009411
LAST, UNKNOWN, DELALLOC, ENCODED, INLINE, SHARED = 0x1, 0x2, 0x4, 0x8, 0x200, 0x2000
NO_START = INLINE | UNKNOWN | DELALLOC
U64_MAX = 2**64 - 1
SNAPSHOT_DIRS = ('snapshot', 'snapshot.partial')
STATE_OVERHEAD = 64 << 20   # snapshotStateOverheadBytes
FLOOR = 16 << 30            # snapshotDiskFloorBytes
BUDGET_DEFAULT_GIB = 48     # VM_SNAPSHOT_BUDGET_GIB on Node B


def fiemap(fd):
    exts, start, batch = [], 0, 4096
    while True:
        hdr = struct.pack('=QQIIII', start, U64_MAX, 0, 0, batch, 0)
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


def fiemap_bound(exts, shared_only=None):
    """vmmanager's fiemapBoundBytes. shared_only=False skips SHARED extents."""
    starts = sorted(e[1] for e in exts if not e[3] & NO_START and e[1])
    seen, total = set(), 0
    for lo, ph, ln, fl in exts:
        if shared_only is False and fl & SHARED:
            continue
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


def tree_extents(fd, ino):
    """Distinct (bytenr, disk_num_bytes) of the inode; inline as (0, len)."""
    out, seen, minoff = [], set(), 0
    while True:
        args = bytearray(4096)
        struct.pack_into('=QQQQQQQIII', args, 0, 0, ino, ino, minoff, U64_MAX, 0, U64_MAX, 108, 108, 4096)
        fcntl.ioctl(fd, BTRFS_IOC_TREE_SEARCH, args, True)
        nr = struct.unpack_from('=I', args, 64)[0]
        if nr == 0:
            return out
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
                out.append((0, ln - 21))
            elif item[20] in (1, 2):
                bytenr, disk_num = struct.unpack_from('<QQ', item, 21)
                if bytenr and bytenr not in seen:
                    seen.add(bytenr)
                    out.append((bytenr, disk_num))
        if last == U64_MAX:
            return out
        minoff = last + 1


def measure_fd(fd):
    st = os.fstat(fd)
    allocated = st.st_blocks * 512
    row = {'apparent': st.st_size, 'allocated': allocated,
           'fiemap_bound': None, 'tree_search': None, 'exclusive': None,
           'shared_ext': 0, 'delalloc_ext': 0, 'extents': 0, 'tree_err': None}
    exts = fiemap(fd)
    row['extents'] = len(exts)
    row['shared_ext'] = sum(1 for e in exts if e[3] & SHARED)
    row['delalloc_ext'] = sum(1 for e in exts if e[3] & DELALLOC)
    row['fiemap_bound'] = min(fiemap_bound(exts), allocated)
    shared_starts = sorted(e[1] for e in exts if e[3] & SHARED and e[1])
    try:
        tex = tree_extents(fd, st.st_ino)
    except OSError as err:
        row['tree_err'] = err.strerror or str(err)
        row['exclusive'] = min(fiemap_bound(exts, shared_only=False), allocated)
        return row
    row['tree_search'] = sum(n for _, n in tex)
    excl = 0
    for bytenr, n in tex:
        if bytenr:
            j = bisect.bisect_left(shared_starts, bytenr)
            if j < len(shared_starts) and shared_starts[j] < bytenr + n:
                continue
        excl += n
    row['exclusive'] = excl
    return row


def add(acc, row):
    for k, v in row.items():
        if isinstance(v, int) and not isinstance(v, bool):
            acc[k] = acc.get(k, 0) + v
        elif v is None and k in ('tree_search', 'exclusive', 'fiemap_bound'):
            acc[k + '_missing'] = True
        elif k == 'tree_err' and v:
            acc['tree_err'] = v


def measure_dir(path):
    acc, files = {}, 0
    for name in sorted(os.listdir(path)):
        p = os.path.join(path, name)
        try:
            fd = os.open(p, os.O_RDONLY | os.O_NOFOLLOW)
        except OSError:
            continue
        try:
            if not stat.S_ISREG(os.fstat(fd).st_mode):
                continue
            add(acc, measure_fd(fd))
            files += 1
        finally:
            os.close(fd)
    acc['files'] = files
    return acc


def read_meta(path):
    try:
        with open(os.path.join(path, 'meta.json')) as f:
            m = json.load(f)
    except (OSError, ValueError):
        return {}
    return {k: m.get(k) for k in ('vm_id', 'bytes', 'disk_bytes', 'disk_bytes_method',
                                  'mem_size_mib', 'created_at', 'consumed', 'confinement')}


def mapped_deleted_mem():
    """Deleted snapshot memory files still mapped by a process (gap G1)."""
    out, seen = [], set()
    for pid in os.listdir('/proc'):
        if not pid.isdigit():
            continue
        try:
            with open('/proc/%s/maps' % pid) as f:
                lines = f.readlines()
            with open('/proc/%s/comm' % pid) as f:
                comm = f.read().strip()
        except OSError:
            continue
        for line in lines:
            if not line.rstrip().endswith('(deleted)') or 'snapshot' not in line:
                continue
            parts = line.split(None, 5)
            rng, name = parts[0], parts[5].strip() if len(parts) > 5 else ''
            row = {'pid': int(pid), 'comm': comm, 'path': name}
            try:
                fd = os.open('/proc/%s/map_files/%s' % (pid, rng), os.O_RDONLY)
            except OSError as err:
                row['err'] = err.strerror or str(err)
                out.append(row)
                continue
            try:
                st = os.fstat(fd)
                if (st.st_dev, st.st_ino) in seen:
                    continue
                seen.add((st.st_dev, st.st_ino))
                row.update(measure_fd(fd))
            except OSError as err:
                row['err'] = err.strerror or str(err)
            finally:
                os.close(fd)
            out.append(row)
    return out


def probe(state_dir, budget_gib):
    rows = []
    for vm in sorted(os.listdir(state_dir)):
        for sub in SNAPSHOT_DIRS:
            d = os.path.join(state_dir, vm, sub)
            if not os.path.isdir(d) or os.path.islink(d):
                continue
            row = {'vm': vm, 'kind': 'committed' if sub == 'snapshot' else 'partial'}
            meta = read_meta(d)
            row['recorded'] = meta.get('disk_bytes')
            row['recorded_method'] = meta.get('disk_bytes_method')
            row['meta_bytes'] = meta.get('bytes')
            row['mem_size_mib'] = meta.get('mem_size_mib')
            row['created_at'] = meta.get('created_at')
            row['confinement'] = meta.get('confinement')
            try:
                row.update(measure_dir(d))
            except OSError as err:
                row['vanished'] = err.strerror or str(err)
                rows.append(row)
                continue
            if row.get('mem_size_mib'):
                row['apparent_vs_mem'] = round(row['apparent'] / ((row['mem_size_mib'] << 20) + STATE_OVERHEAD), 4)
            rows.append(row)
    committed = [r for r in rows if r['kind'] == 'committed' and 'vanished' not in r]
    totals = {
        'committed': len(committed),
        'budget_counts': sum(r['recorded'] or r.get('allocated', 0) for r in committed),
        'apparent': sum(r.get('apparent', 0) for r in committed),
        'allocated': sum(r.get('allocated', 0) for r in committed),
        'tree_search': None if any(r.get('tree_search') is None for r in committed)
        else sum(r['tree_search'] for r in committed),
        'exclusive': sum(r.get('exclusive') or 0 for r in committed),
        'partial_allocated': sum(r.get('allocated', 0) for r in rows if r['kind'] == 'partial'),
        'budget': budget_gib << 30,
    }
    mapped = mapped_deleted_mem()
    totals['mapped_deleted_tree_search'] = sum(m.get('tree_search') or 0 for m in mapped)
    totals['mapped_deleted_allocated'] = sum(m.get('allocated') or 0 for m in mapped)
    sv = os.statvfs(state_dir)
    totals['statfs_avail'] = sv.f_bavail * sv.f_frsize
    totals['floor'] = FLOOR
    return {'state_dir': state_dir, 'euid': os.geteuid(), 'snapshots': rows,
            'mapped_deleted_mem': mapped, 'totals': totals}


def mib(v):
    return '-' if v is None else '%.1f' % (v / 1048576.0)


def print_table(r):
    print('state_dir=%s euid=%d (tree_search %s)' % (
        r['state_dir'], r['euid'], 'exact' if r['euid'] == 0 else 'needs root; exclusive uses the FIEMAP bound'))
    print('sizes in MiB')
    cols = ('vm', 'kind', 'recorded', 'method', 'apparent', 'allocated', 'fiemap_bound',
            'tree_search', 'exclusive', 'shared_ext', 'delalloc', 'apparent_vs_mem', 'created_at')
    print('\t'.join(cols))
    for s in r['snapshots']:
        if 'vanished' in s:
            print('%s\t%s\tvanished: %s' % (s['vm'], s['kind'], s['vanished']))
            continue
        ts = mib(s.get('tree_search')) if s.get('tree_search') is not None else 'err %s' % s.get('tree_err')
        print('\t'.join(str(x) for x in (
            s['vm'], s['kind'], mib(s['recorded']), s['recorded_method'] or '-', mib(s.get('apparent')),
            mib(s.get('allocated')), mib(s.get('fiemap_bound')), ts, mib(s.get('exclusive')),
            s.get('shared_ext', 0), s.get('delalloc_ext', 0), s.get('apparent_vs_mem', '-'),
            s.get('created_at') or '-')))
    for m in r['mapped_deleted_mem']:
        if 'err' in m:
            print('mapped deleted mem: pid=%d comm=%s %s: %s' % (m['pid'], m['comm'], m['path'], m['err']))
        else:
            print('mapped deleted mem: pid=%d comm=%s %s apparent=%s allocated=%s tree_search=%s' % (
                m['pid'], m['comm'], m['path'], mib(m['apparent']), mib(m['allocated']), mib(m.get('tree_search'))))
    t = r['totals']
    print('totals: committed=%d budget_counts=%s budget=%s apparent=%s allocated=%s tree_search=%s exclusive=%s '
          'partial_allocated=%s mapped_deleted_tree_search=%s mapped_deleted_allocated=%s' % (
              t['committed'], mib(t['budget_counts']), mib(t['budget']), mib(t['apparent']), mib(t['allocated']),
              mib(t['tree_search']), mib(t['exclusive']), mib(t['partial_allocated']),
              mib(t['mapped_deleted_tree_search']), mib(t['mapped_deleted_allocated'])))
    print('statfs: avail=%s floor=%s headroom_over_floor=%s' % (
        mib(t['statfs_avail']), mib(t['floor']), mib(t['statfs_avail'] - t['floor'])))


def main(argv):
    as_json = '--json' in argv
    args = [a for a in argv if a != '--json']
    state_dir = args[0] if args else '/var/lib/go-choir/vm-state'
    budget_gib = int(os.environ.get('VM_SNAPSHOT_BUDGET_GIB', BUDGET_DEFAULT_GIB))
    r = probe(state_dir, budget_gib)
    if as_json:
        json.dump(r, sys.stdout, indent=1, sort_keys=True)
        print()
    else:
        print_table(r)


if __name__ == '__main__':
    main(sys.argv[1:])
