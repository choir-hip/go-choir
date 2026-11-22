# Runs on Node B under sudo, reading stdin (capture text hashes, one per line)
# and writing, for each, "H <hash> <n>\n" and n bytes: the first PREFIX bytes of
# the capture body file (read only; nothing is written). n = -1 when the file
# is absent. The caller runs it under nice and ionice.
import sys, os
ROOT = "/var/lib/go-choir/platform-artifacts/capture-bodies"
PREFIX = int(sys.argv[1]) if len(sys.argv) > 1 else 32768
out = sys.stdout.buffer
for line in sys.stdin:
    h = line.strip().removeprefix("sha256:")
    if len(h) != 64 or any(c not in "0123456789abcdef" for c in h):
        continue
    try:
        with open(os.path.join(ROOT, h[:2], h), "rb") as f:
            b = f.read(PREFIX)
        out.write(b"H %s %d\n" % (h.encode(), len(b)))
        out.write(b)
    except OSError:
        out.write(b"H %s -1\n" % h.encode())
out.flush()
