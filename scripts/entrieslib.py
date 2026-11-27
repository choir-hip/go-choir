"""Shared reader for docs/changes/ and docs/calibration/claims/ files. Loaded by path
(python3 -I does not put scripts/ on sys.path) by check-entries, changes and ledger.

Front matter is single-line `key: value` scalars between two `---` lines, nothing else."""
import datetime, math, re

TS_RE = re.compile(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}Z$")
DATE_RE = re.compile(r"^\d{4}-\d{2}-\d{2}$")
NAME_TS_RE = re.compile(r"^\d{8}T\d{4}Z$")
KEY_RE = re.compile(r"^([a-z_]+):[ \t]*(.*)$")


def parse_ts(s):
    """YYYY-MM-DDTHH:MMZ (zero padded, UTC) as a datetime, else None."""
    if not isinstance(s, str) or not TS_RE.match(s):
        return None
    try:
        return datetime.datetime.strptime(s, "%Y-%m-%dT%H:%MZ")
    except ValueError:
        return None


def parse_date(s):
    if not isinstance(s, str) or not DATE_RE.match(s):
        return None
    try:
        return datetime.datetime.strptime(s, "%Y-%m-%d")
    except ValueError:
        return None


def parse_name_ts(s):
    """The YYYYMMDDTHHMMZ of a claim file name as a datetime, else None."""
    if not NAME_TS_RE.match(s):
        return None
    try:
        return datetime.datetime.strptime(s, "%Y%m%dT%H%MZ")
    except ValueError:
        return None


def finite(s):
    try:
        v = float(s)
    except (TypeError, ValueError):
        return None
    return v if math.isfinite(v) else None


def unquote(v):
    if len(v) >= 2 and v[0] == v[-1] and v[0] in "'\"":
        return v[1:-1]
    return v


def parse_front(text):
    """(fields, body, problems). fields is None when there is no front matter."""
    lines = text.split("\n")
    if not lines or lines[0].strip() != "---":
        return None, text, ["no front matter"]
    for i in range(1, len(lines)):
        if lines[i].strip() == "---":
            f, probs = {}, []
            for l in lines[1:i]:
                m = KEY_RE.match(l)
                if not m:
                    probs.append("front matter line %r is not 'key: value' (no blank, indented or continuation lines)" % l)
                    continue
                v = m.group(2).strip()
                if re.match(r"^[|>][+-]?\d*$", v):
                    probs.append("front matter %s: a block scalar (%s) is not allowed" % (m.group(1), v))
                    continue
                if m.group(1) in f:
                    probs.append("front matter key %s is repeated" % m.group(1))
                f[m.group(1)] = unquote(v)
            return f, "\n".join(lines[i + 1:]), probs
    return None, text, ["front matter is not closed by ---"]


def entry_names(d):
    """The names to read in an entry directory, sorted: no README.md and no dotfile.
    The one place the validator and both readers agree on what counts."""
    import os
    if not os.path.isdir(d):
        return []
    return sorted(n for n in os.listdir(d) if n != "README.md" and not n.startswith("."))


def norm_body(b):
    """A body without leading and trailing blank lines or trailing spaces on lines."""
    return "\n".join(l.rstrip() for l in b.split("\n")).strip("\n")
