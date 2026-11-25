#!/usr/bin/python3 -I
"""check-store-ipa.py [--tools <dir>] <script name> <ipa> <bundle id> [<version> [<build number>]]

Reads a store .ipa back and says whether it is what archive-store.sh makes: the production bundle id, a version and a build number (the ones given; upload-store.sh gives
app.json's version), ITSAppUsesNonExemptEncryption false, the developer menu stamped off, ExactBuildKind archive, no ExactDevelopment* key, and an embedded provisioning profile
that is an unexpired iOS App Store profile for exactly that id (check-store-signing.py with no identity, so the same rules, expiry and prefix included).
It also judges what was BUILT, not only what was asked for: the app is unpacked once (ditto) and EVERY Mach-O file of that copy (the main executable, Frameworks/, PlugIns/, any .dylib, any
file with Mach-O magic; no symlink followed) is searched for every name in store-dev-names.txt (the developer launch hooks, their launch-argument keys, ProofKey), a second .app, a
duplicate entry or a symlink out of the bundle is refused, and the same copy is read with codesign: the seal verifies (--deep), the authority is Apple Distribution, the team is the profile's,
the entitlements carry the application-identifier the embedded profile grants and no get-task-allow. Tools are /usr/bin's, never the caller's PATH (`--tools <dir>` is a test's argument). Used by archive-store.sh after the build and by upload-store.sh before the upload, so an .ipa from any other source is held to the
same rules. Exit 0 and a summary on stdout, or exit 1 and the reason on
stderr; it prints no certificate, hash or key, and not the build machine's host name (Exact stamps it: it says the key is there)."""
import os, plistlib, posixpath, re, stat, subprocess, sys, tempfile, zipfile

# Names the PRODUCT's own code holds (the voice model hub, the app module): if no Mach-O holds one the search may be blind (stripped, or code somewhere it did not look); printed as a warning, never a refusal.
PRODUCT = ("ModelProof", "AutoradioModule")
MACHO = (b"\xfe\xed\xfa\xce", b"\xfe\xed\xfa\xcf", b"\xce\xfa\xed\xfe", b"\xcf\xfa\xed\xfe", b"\xca\xfe\xba\xbe", b"\xbe\xba\xfe\xca", b"\xca\xfe\xba\xbf", b"\xbf\xba\xfe\xca")

def load_names(here):
    """The names of store-dev-names.txt. Every line that is not blank or a comment (starting with #) must be exactly `<staged|compiled> <name>`: a line that is anything else is a
    refusal of everything, never a name quietly dropped."""
    f = os.path.join(here, "store-dev-names.txt")
    try:
        lines = open(f, encoding="utf8").read().splitlines()
    except OSError:
        lines = []
    names = []
    for n, l in enumerate(lines, 1):
        if not l.strip() or l.lstrip().startswith("#"):
            continue
        m = re.fullmatch(r"(staged|compiled) ([A-Za-z0-9_.-]+)", l)
        if not m:
            sys.stderr.write("scripts/store-dev-names.txt line %d is not `<staged|compiled> <name>`: an executable cannot be judged by a list that is not read whole\n" % n)
            sys.exit(1)
        names.append(m.group(2))
    if not names:
        sys.stderr.write("scripts/store-dev-names.txt is missing or empty: an executable cannot be judged\n")
        sys.exit(1)
    return names

def dev_names_in(data, here):
    """The names of store-dev-names.txt that `data` (a binary's bytes) holds."""
    return [n for n in load_names(here) if n.encode("utf8") in data]

args = sys.argv[1:]
tools = None
if len(args) >= 2 and args[0] == "--tools":
    tools, args = args[1], args[2:]
def tool(name):
    """/usr/bin/<name>, never one found on the caller's PATH. `--tools <dir>` (a test's argument, never passed by archive-store.sh or upload-store.sh) names a directory whose tool wins when it has one."""
    if tools and os.path.exists(os.path.join(tools, name)):
        return os.path.join(tools, name)
    return "/usr/bin/" + name

if len(args) == 2 and args[0] == "--executable":
    # check-store-ipa.py --executable <file>: only the name search, on one file (the proof the lead runs on a development app's executable: it must be refused)
    try:
        found = dev_names_in(open(args[1], "rb").read(), os.path.dirname(os.path.abspath(sys.argv[0])))
    except OSError:
        sys.stderr.write("check-store-ipa: cannot read " + args[1] + "\n"); sys.exit(1)
    if found:
        sys.stderr.write("check-store-ipa: " + args[1] + " holds developer-only names: " + ", ".join(found) + "\n"); sys.exit(1)
    print("check-store-ipa: " + args[1] + " holds none of the developer-only names")
    sys.exit(0)

who, ipa, bid = args[0:3]
want_ver = args[3] if len(args) >= 4 else None
want_build = args[4] if len(args) >= 5 else None
here = os.path.dirname(os.path.abspath(sys.argv[0]))

def no(m):
    sys.stderr.write(who + ": the .ipa is not what was asked for: " + m + "\n")
    sys.exit(1)

try:
    z = zipfile.ZipFile(ipa)
except Exception:
    no("the file is not an ipa (not a zip)")
names = z.namelist()
if len(set(names)) != len(names):
    no("an entry of the archive is there twice (what is read is not what is signed)")
apps = {n.split("/")[1] for n in names if n.startswith("Payload/") and n.count("/") >= 2 and n.split("/")[1].endswith(".app")}
if len(apps) > 1:
    no("Payload holds more than one .app (" + str(len(apps)) + "): a store ipa is one app")
for zi in z.infolist():
    if stat.S_ISLNK(zi.external_attr >> 16):
        target = z.read(zi).decode("utf8", "replace")
        resolved = posixpath.normpath(posixpath.join(posixpath.dirname(zi.filename), target))
        top = "Payload/" + (next(iter(apps)) if apps else "") + "/"
        if posixpath.isabs(target) or not resolved.startswith(top):
            no("it holds a symlink that leaves the app bundle (" + zi.filename + ")")
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
    r = subprocess.run(["/usr/bin/python3", "-I", os.path.join(here, "check-store-signing.py")] + (["--tools", tools] if tools else []) + [bid, "-", f], capture_output=True)
if r.returncode != 0:
    msg = r.stderr.decode("utf8", "replace").strip().replace("archive-store: ", "").replace("EXACT_PROFILE", "its embedded provisioning profile")
    no(msg or "its embedded provisioning profile is not an App Store profile for " + bid)
out_lines = r.stdout.decode("utf8", "replace").split()     # check-store-signing.py prints the profile's application-identifier, then its team, when it is given no identity
if len(out_lines) != 2:
    no("its embedded provisioning profile could not be read for its identifiers")
appid, team = out_lines

# The executable and every other Mach-O of the bundle: whatever Exact compiled, searched for the names a store binary must not hold. Exact links the app module into
# libexact_modules.dylib (on iOS production, Frameworks/ExactModules.framework/ExactModules) and package Swift into frameworks, so the main executable alone is not the app.
exe_name = info.get("CFBundleExecutable")
if not isinstance(exe_name, str) or not exe_name or "/" in exe_name or exe_name in (".", ".."):
    no("its Info.plist names no executable (CFBundleExecutable)")
if app + exe_name not in names:
    no("it has no executable " + exe_name + " inside the app")

def run_tool(args):
    try:
        return subprocess.run(args, capture_output=True)
    except OSError:
        no(args[0] + " could not be run, so the signature cannot be read")

with tempfile.TemporaryDirectory() as d:
    # What is searched and what codesign reads is the one extracted copy, not the zip's entries.
    if run_tool([tool("ditto"), "-x", "-k", ipa, d]).returncode != 0:
        no("it could not be unpacked to read its signature")
    appdir = os.path.join(d, app.rstrip("/"))
    real_app = os.path.realpath(appdir)
    exe_path = os.path.join(appdir, exe_name)
    if not os.path.isfile(exe_path) or os.path.islink(exe_path):
        no("its executable " + exe_name + " is not a regular file")
    if os.path.getsize(exe_path) == 0:
        no("its executable is empty")
    searched = []
    markers = []
    for root, dirs, files in os.walk(appdir, followlinks=False):
        for entry in dirs + files:
            full = os.path.join(root, entry)
            if os.path.islink(full):
                if not os.path.realpath(full).startswith(real_app + os.sep):
                    no("it holds a symlink that leaves the app bundle (" + os.path.relpath(full, appdir) + ")")
        for f in files:
            full = os.path.join(root, f)
            if os.path.islink(full) or not os.path.isfile(full):
                continue
            with open(full, "rb") as fh:
                head = fh.read(4)
                binary = head in MACHO or f.endswith(".dylib") or full == exe_path
                if not binary:
                    continue
                data = head + fh.read()
            searched.append(os.path.relpath(full, appdir))
            if any(m.encode("utf8") in data for m in PRODUCT):
                markers.append(os.path.relpath(full, appdir))
            found = dev_names_in(data, here)
            if found:
                no(os.path.relpath(full, appdir) + " holds developer-only names (the launch hooks of a development build): " + ", ".join(found))
    # The signature: what codesign reads from the signed app, not only the profile that was embedded.
    if run_tool([tool("codesign"), "--verify", "--deep", "--strict", appdir]).returncode != 0:
        no("its signature does not verify (codesign --verify)")
    e = run_tool([tool("codesign"), "-d", "--entitlements", ":-", appdir])
    try:
        out = e.stdout
        ents = plistlib.loads(out[out.index(b"<?xml"):]) if e.returncode == 0 else None
        if not isinstance(ents, dict):
            raise ValueError
    except Exception:
        no("codesign could not read the entitlements of the signed app")
    v = run_tool([tool("codesign"), "-dvv", appdir])
    vlines = v.stderr.decode("utf8", "replace").splitlines()
    auth = [l for l in vlines if l.startswith("Authority=")]
    if v.returncode != 0 or not auth or not (auth[0] == "Authority=Apple Distribution" or auth[0].startswith("Authority=Apple Distribution:")):
        no("it is not signed by an Apple Distribution identity (the first authority of its signature is not one)")
    signed_team = [l[len("TeamIdentifier="):] for l in vlines if l.startswith("TeamIdentifier=")]
    if signed_team != [team]:
        no("its signature's team is not the team of its embedded profile")
    # Exact stages a declared resource tree under ExactResources/Resources; a flat bundle keeps it at the root.
    voice = next((d for d in (os.path.join(appdir, "ExactResources", "Resources", "TinyVoca"), os.path.join(appdir, "TinyVoca")) if os.path.isdir(d)), None)
    if voice is None:
        no("its signed app has no TinyVoca resource tree (ExactResources/Resources/TinyVoca)")
    check = run_tool(["/usr/bin/python3", "-I", os.path.join(here, "check-tiny-voca-assets.py"), voice, "ios"])
    if check.returncode != 0:
        no(check.stderr.decode("utf8", "replace").strip() or "its signed app has no valid TinyVoca resource tree")
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
print("Tiny Voca: compiled iOS models, the Dizione phonemizer data and the notices are verified in the signed app")
print("Mach-O files searched for developer-only names: %d (%s)" % (len(searched), ", ".join(sorted(searched))))
if len(searched) < 2:
    print("WARNING: only one Mach-O file was searched; Exact puts the app module in libexact_modules.dylib or Frameworks/ExactModules.framework, so a layout with none is new: read the unpacked app before trusting this")
if not markers:
    print("WARNING: no searched file holds a product name (%s): the search may be looking at the wrong files or at stripped code" % ", ".join(PRODUCT))
else:
    print("product code found in: " + ", ".join(sorted(markers)))
if "ExactBuildHost" in info:
    print("ExactBuildHost: present (Exact stamps the build machine's host name into every build, archives included, and offers no way to leave it out; the value is not printed)")
commit = info.get("ExactAppCommit")
if isinstance(commit, str) and commit:
    print("ExactAppCommit: %s (%s)" % (commit[:12], "dirty" if info.get("ExactAppCommitDirty") else "clean"))
