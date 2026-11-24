#!/usr/bin/python3 -I
"""check-store-ipa.py <script name> <ipa> <bundle id> [<version> [<build number>]]

Reads a store .ipa back and says whether it is what archive-store.sh makes: the production bundle id, a version and a build number (the ones given; upload-store.sh gives
app.json's version), ITSAppUsesNonExemptEncryption false, the developer menu stamped off, ExactBuildKind archive, no ExactDevelopment* key, and an embedded provisioning profile
that is an unexpired iOS App Store profile for exactly that id (check-store-signing.py with no identity, so the same rules, expiry and prefix included).
It also judges what was BUILT, not only what was asked for: the executable inside the .ipa is searched for every name in store-dev-names.txt (the developer launch hooks and their
launch-argument keys), and the app is unpacked and read with codesign: the seal verifies, the authority is Apple Distribution, the entitlements carry the application-identifier the
embedded profile grants and no get-task-allow. Used by archive-store.sh after the build and by upload-store.sh before the upload, so an .ipa from any other source is held to the
same rules. Exit 0 and a summary on stdout, or exit 1 and the reason on
stderr; it prints no certificate, hash or key, and not the build machine's host name (Exact stamps it: it says the key is there)."""
import os, plistlib, subprocess, sys, tempfile, zipfile

def dev_names_in(data, here):
    """The names of store-dev-names.txt that `data` (an executable's bytes) holds."""
    try:
        listed = [l.split() for l in open(os.path.join(here, "store-dev-names.txt"), encoding="utf8").read().splitlines() if l.strip() and not l.lstrip().startswith("#")]
        names = [p[1] for p in listed if len(p) == 2]
    except OSError:
        names = []
    if not names:
        sys.stderr.write("scripts/store-dev-names.txt is missing or empty: an executable cannot be judged\n")
        sys.exit(1)
    return [n for n in names if n.encode("utf8") in data]

if len(sys.argv) == 3 and sys.argv[1] == "--executable":
    # check-store-ipa.py --executable <file>: only the name search, on one file (the proof the lead runs on a development app's executable: it must be refused)
    try:
        found = dev_names_in(open(sys.argv[2], "rb").read(), os.path.dirname(os.path.abspath(sys.argv[0])))
    except OSError:
        sys.stderr.write("check-store-ipa: cannot read " + sys.argv[2] + "\n"); sys.exit(1)
    if found:
        sys.stderr.write("check-store-ipa: " + sys.argv[2] + " holds developer-only names: " + ", ".join(found) + "\n"); sys.exit(1)
    print("check-store-ipa: " + sys.argv[2] + " holds none of the developer-only names")
    sys.exit(0)

who, ipa, bid = sys.argv[1:4]
want_ver = sys.argv[4] if len(sys.argv) >= 5 else None
want_build = sys.argv[5] if len(sys.argv) >= 6 else None
here = os.path.dirname(os.path.abspath(sys.argv[0]))

def no(m):
    sys.stderr.write(who + ": the .ipa is not what was asked for: " + m + "\n")
    sys.exit(1)

try:
    z = zipfile.ZipFile(ipa)
except Exception:
    no("the file is not an ipa (not a zip)")
names = z.namelist()
plists = [n for n in names if n.count("/") == 2 and n.startswith("Payload/") and n.endswith(".app/Info.plist")]
if len(plists) != 1:
    no("it has no single Payload/<app>.app/Info.plist")
try:
    info = plistlib.loads(z.read(plists[0]))
    if not isinstance(info, dict):
        raise ValueError
except Exception:
    no("its Info.plist could not be read")
app = plists[0][: -len("Info.plist")]
if info.get("CFBundleIdentifier") != bid:
    no("bundle id is %r, not the production app %s" % (info.get("CFBundleIdentifier"), bid))
ver, build = info.get("CFBundleShortVersionString"), info.get("CFBundleVersion")
if not ver or not isinstance(ver, str):
    no("it has no version (CFBundleShortVersionString)")
if not build or not isinstance(build, str):
    no("it has no build number (CFBundleVersion)")
if want_ver is not None and ver != want_ver:
    no("version is %r, wanted %r" % (ver, want_ver))
if want_build is not None and build != want_build:
    no("build number is %r, wanted %r" % (build, want_build))
if info.get("ITSAppUsesNonExemptEncryption") is not False:
    no("ITSAppUsesNonExemptEncryption is %r, wanted false" % (info.get("ITSAppUsesNonExemptEncryption"),))
if info.get("ExactDevMenu") is not False:
    no("the developer menu is not stamped off (ExactDevMenu is %r)" % (info.get("ExactDevMenu"),))
if info.get("ExactBuildKind") != "archive":
    no("ExactBuildKind is %r, wanted 'archive'" % (info.get("ExactBuildKind"),))
dev = sorted(k for k in info if k.startswith("ExactDevelopment"))
if dev:
    no("it carries a development client's keys: " + ", ".join(dev))
prof = app + "embedded.mobileprovision"
if prof not in names:
    no("it has no embedded provisioning profile (the .ipa is not signed for the store)")
with tempfile.TemporaryDirectory() as d:
    f = os.path.join(d, "p.mobileprovision")
    open(f, "wb").write(z.read(prof))
    r = subprocess.run(["/usr/bin/python3", "-I", os.path.join(here, "check-store-signing.py"), bid, "-", f], capture_output=True)
if r.returncode != 0:
    msg = r.stderr.decode("utf8", "replace").strip().replace("archive-store: ", "").replace("EXACT_PROFILE", "its embedded provisioning profile")
    no(msg or "its embedded provisioning profile is not an App Store profile for " + bid)
appid = r.stdout.decode("utf8", "replace").strip()     # the profile's application-identifier (check-store-signing.py prints it when it is given no identity)

# The executable: whatever Exact compiled, searched for the names a store binary must not hold.
exe_name = info.get("CFBundleExecutable")
if not isinstance(exe_name, str) or not exe_name or "/" in exe_name or exe_name in (".", ".."):
    no("its Info.plist names no executable (CFBundleExecutable)")
if app + exe_name not in names:
    no("it has no executable " + exe_name + " inside the app")
exe = z.read(app + exe_name)
if not exe:
    no("its executable is empty")
found = dev_names_in(exe, here)
if found:
    no("its executable holds developer-only names (the launch hooks of a development build): " + ", ".join(found))

# The signature: what codesign reads from the signed app, not only the profile that was embedded.
with tempfile.TemporaryDirectory() as d:
    def tool(args):
        try:
            return subprocess.run(args, capture_output=True)
        except OSError:
            no("codesign (or ditto) could not be run, so the signature cannot be read")
    if tool(["ditto", "-x", "-k", ipa, d]).returncode != 0:
        no("it could not be unpacked to read its signature")
    appdir = os.path.join(d, app.rstrip("/"))
    if tool(["codesign", "--verify", "--strict", appdir]).returncode != 0:
        no("its signature does not verify (codesign --verify)")
    e = tool(["codesign", "-d", "--entitlements", ":-", appdir])
    try:
        out = e.stdout
        ents = plistlib.loads(out[out.index(b"<?xml"):]) if e.returncode == 0 else None
        if not isinstance(ents, dict):
            raise ValueError
    except Exception:
        no("codesign could not read the entitlements of the signed app")
    v = tool(["codesign", "-dvv", appdir])
    auth = [l for l in v.stderr.decode("utf8", "replace").splitlines() if l.startswith("Authority=")]
    if v.returncode != 0 or not auth or not auth[0].startswith("Authority=Apple Distribution"):
        no("it is not signed by an Apple Distribution identity (the first authority of its signature is not one)")
signed_id = ents.get("application-identifier")
if ents.get("get-task-allow") is True:
    no("its signature carries get-task-allow (a development signature)")
if not isinstance(signed_id, str) or not signed_id.endswith("." + bid) or signed_id == "." + bid:
    no("its signed application-identifier is not <team>." + bid)
if signed_id != appid:
    no("its signed application-identifier is not the one its embedded profile grants")
print("ipa: " + ipa)
print("bundle id: " + bid)
print("version: " + ver)
print("build number: " + build)
print("ITSAppUsesNonExemptEncryption: false")
if "ExactBuildHost" in info:
    print("ExactBuildHost: present (Exact stamps the build machine's host name into every build, archives included, and offers no way to leave it out; the value is not printed)")
commit = info.get("ExactAppCommit")
if isinstance(commit, str) and commit:
    print("ExactAppCommit: %s (%s)" % (commit[:12], "dirty" if info.get("ExactAppCommitDirty") else "clean"))
