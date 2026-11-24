#!/usr/bin/python3 -I
"""check-store-ipa.py <script name> <ipa> <bundle id> [<version> [<build number>]]

Reads a store .ipa back and says whether it is what archive-store.sh makes: the production bundle id, a version and a build number (the ones given; upload-store.sh gives
app.json's version), ITSAppUsesNonExemptEncryption false, the developer menu stamped off, ExactBuildKind archive, no ExactDevelopment* key, and an embedded provisioning profile
that is an unexpired iOS App Store profile for exactly that id (check-store-signing.py with no identity, so the same rules, expiry and prefix included). Used by archive-store.sh
after the build and by upload-store.sh before the upload, so an .ipa from any other source is held to the same rules. Exit 0 and a summary on stdout, or exit 1 and the reason on
stderr; it prints no certificate, hash or key, and not the build machine's host name (Exact stamps it: it says the key is there)."""
import os, plistlib, subprocess, sys, tempfile, zipfile

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
