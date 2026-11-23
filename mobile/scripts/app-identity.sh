#!/bin/sh
# The app's build identity, read from AutoradioLab/app.json and nowhere else.   app-identity.sh id|name|variant|version
# With no choice the app is the PRODUCTION app as app.json describes it: news.choir.autoradio, "Autoradio 1", the id the store build is uploaded under.
# The two development apps on the owner's phone are Exact variants (Exact's own mechanism: the id is app.id + the variant's idSuffix, the name its own,
# every derived id follows: bundle id, App Groups, data directory):
#   lab      news.choir.autoradio.lab      "Autoradio"       (the first development app)
#   lab-one  news.choir.autoradio.lab.one  "Autoradio 1 Dev"  (the second; named "Autoradio 1" until the production app took that name: Exact refuses two apps
#                                                              of one installed name, scripts/variants.mjs)
# Choose one by environment:
#   AR_VARIANT=lab-one                        by name, or
#   AR_APP_ID=news.choir.autoradio.lab.one    by id (AR_APP_NAME, when set, must be that app's name)
# An unknown variant (the retired name `one` included), an id nobody has, a name that is not the chosen one, or two choices that disagree: exit 1 naming what is declared.
# `version` is app.json's `version` (CFBundleShortVersionString; archive-store.sh gives it to Exact as EXACT_VERSION).
# This is a build identity, not a product switch: nothing in the app branches on it. exact.sh hands the variant to Exact as EXACT_VARIANT.
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
exec /usr/bin/python3 -I - "${1:-id}" "${AR_APP_JSON:-$HERE/AutoradioLab/app.json}" <<'PY'
import json, os, sys
what, path = sys.argv[1], sys.argv[2]
def die(m): sys.stderr.write("app-identity: " + m + "\n"); sys.exit(1)
if what not in ("id", "name", "variant", "version"): die("usage: app-identity.sh id|name|variant|version")
m = json.load(open(path))
base, bname = m["app"]["id"], m["app"]["name"]
vs = m.get("variants", {})
ids = {n: (base + v["idSuffix"], v["name"]) for n, v in vs.items()}
declared = "; ".join([f"(production, no variant) {base} \"{bname}\""] + [f"{n} {i} \"{nm}\"" for n, (i, nm) in ids.items()])
var, aid, anm = (os.environ.get(k, "") for k in ("AR_VARIANT", "AR_APP_ID", "AR_APP_NAME"))
if var and var not in ids: die(f"AR_VARIANT={var} is not declared in app.json. Declared: {declared}")
if aid and aid != base:
    hit = [n for n, (i, _) in ids.items() if i == aid]
    if not hit: die(f"AR_APP_ID={aid} is no declared id. Declared: {declared}")
    if var and var != hit[0]: die(f"AR_VARIANT={var} and AR_APP_ID={aid} name different apps")
    var = hit[0]
elif aid and var:
    die(f"AR_VARIANT={var} and AR_APP_ID={aid} name different apps")
i, nm = ids[var] if var else (base, bname)
if anm and anm != nm: die(f"AR_APP_NAME={anm} is not the name of {i} (\"{nm}\")")
ver = m.get("version", "")
if what == "version" and not ver: die("app.json has no version")
print({"id": i, "name": nm, "variant": var, "version": ver}[what])
PY
