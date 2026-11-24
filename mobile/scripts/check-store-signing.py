#!/usr/bin/python3 -I
"""check-store-signing.py <bundle id> <identity sha1 | -> <profile path>

Says whether the NAMED identity and the NAMED provisioning profile can sign an App Store build of <bundle id>. It looks at nothing else: it never searches for an
identity or a profile, and it prints no key material, no certificate and no hash (only what is wrong, in plain words). Exit 0: fine. Exit 1: the reason on stderr.
With `-` for the identity only the profile is judged (check-store-ipa.py holds the profile embedded in an .ipa to the same rules).
The tools are `security` (find-identity, cms -D) found on PATH, as archive-store.sh runs them."""
import datetime, hashlib, plistlib, subprocess, sys

bundle, sha, path = sys.argv[1:4]

def no(msg):
    sys.stderr.write("archive-store: " + msg + "\n")
    sys.exit(1)

def run(args):
    try:
        p = subprocess.run(["security"] + args, capture_output=True)
    except OSError:
        no("the `security` tool could not be run.")
    return p.returncode, p.stdout

# 1. EXACT_IDENTITY names an Apple Distribution identity this Mac holds (the list prints names and hashes, never keys).
if sha != "-":
    rc, out = run(["find-identity", "-v", "-p", "codesigning"])
    line = next((l for l in out.decode("utf8", "replace").splitlines() if sha.lower() in l.lower()), "")
    if rc != 0 or not line:
        no("EXACT_IDENTITY names no valid code-signing identity in this Mac's keychains. The store needs an Apple Distribution identity (Xcode, Settings, Accounts, Manage Certificates).")
    if "Apple Distribution" not in line:
        no("EXACT_IDENTITY is not an Apple Distribution identity (a development identity cannot sign for the store).")

# 2. EXACT_PROFILE is an iOS App Store profile for exactly this bundle id, unexpired, made for that identity.
rc, out = run(["cms", "-D", "-i", path])
if rc != 0:
    no("EXACT_PROFILE could not be decoded as a provisioning profile.")
try:
    p = plistlib.loads(out)
except Exception:
    no("EXACT_PROFILE decoded to something that is not a profile.")
if not isinstance(p, dict):
    no("EXACT_PROFILE decoded to something that is not a profile.")

def first(v):
    return v[0] if isinstance(v, list) and v and isinstance(v[0], str) else ""

team = first(p.get("TeamIdentifier"))
# The App ID prefix is the profile's own ApplicationIdentifierPrefix (the team id, except for a legacy App ID made before 2020); the team id stands in only where the key is absent.
prefix = first(p.get("ApplicationIdentifierPrefix")) or team
ents = p.get("Entitlements")
if not isinstance(ents, dict):
    no("EXACT_PROFILE has no entitlements: it is not a provisioning profile.")
if team == "" or prefix == "" or ents.get("application-identifier") != prefix + "." + bundle:
    no("EXACT_PROFILE is not for exactly " + bundle + " (a wildcard or another id cannot be uploaded; make an App Store profile for that explicit id).")
plat = p.get("Platform")
if not isinstance(plat, list) or "iOS" not in plat:
    no("EXACT_PROFILE is not an iOS profile (its platform is " + (", ".join(str(x) for x in plat) if isinstance(plat, list) else "not named") + "); make an iOS App Store profile.")
# An App Store profile may write get-task-allow false or leave the key out; true is a development profile.
if ents.get("get-task-allow") is True or "ProvisionedDevices" in p or p.get("ProvisionsAllDevices"):
    no("EXACT_PROFILE is not an App Store profile (a development, ad hoc or enterprise profile cannot be uploaded to App Store Connect).")
exp = p.get("ExpirationDate")
if not isinstance(exp, datetime.datetime) or exp.replace(tzinfo=datetime.timezone.utc) <= datetime.datetime.now(datetime.timezone.utc):
    no("EXACT_PROFILE has expired.")
if sha != "-":
    certs = p.get("DeveloperCertificates")
    if not isinstance(certs, list) or sha.lower() not in {hashlib.sha1(c).hexdigest() for c in certs if isinstance(c, bytes)}:
        no("EXACT_PROFILE was not made for the identity EXACT_IDENTITY names (make the profile with that Apple Distribution certificate).")
if sha == "-":
    print(ents.get("application-identifier"))   # the caller (check-store-ipa.py) compares it with what the signature carries
