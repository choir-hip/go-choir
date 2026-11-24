// The build identity and the store build (scripts/app-identity.sh, archive-store.sh, upload-store.sh, check-store-signing.py, the store kind of
// sync-exact-package.sh). Shell scripts run over stubs in a temporary copy of the tree: no Exact build, no Xcode, no signing key, no Apple server.
//
// Failure list (written before the code, CLAUDE.md "Proof"; each line is a test below):
//  I1 building with no variant is not the production app: the id is not news.choir.autoradio, the name is not "Autoradio 1", or app.json has no version.
//  I2 the two development apps already on the owner's phone change id: lab must stay news.choir.autoradio.lab "Autoradio", lab-one news.choir.autoradio.lab.one.
//  I3 a choice app.json does not declare is accepted: an unknown variant, the retired name `one`, an id nobody has, a name that is not the chosen one, two choices that disagree.
//  I4 a sign-in callback scheme is not the app id for every one of the three apps (grants auth.callback), or the server's allowlist and app.json drift.
//  I5 two apps would install under one name (Exact refuses a variant whose name is another's, whether or not it is selected).
//  S1 a build for a phone with no variant (the production id would reach a phone as a development build); an archive that is not the store kind; the store kind
//     for anything but an archive.
//  S2 the store archive is attempted with a variant, outside the build tree, with no or a malformed EXACT_IDENTITY, with no profile, or with a profile that is not
//     an App Store profile for exactly news.choir.autoradio signed by that identity and unexpired: each must stop with a plain message and run nothing.
//  S3 the archive does not carry what the store needs to Exact: EXACT_VERSION 1.0, a build number that rises, EXACT_NON_EXEMPT_ENCRYPTION=false,
//     EXACT_DEV_MENU=0 (Exact's iOS developer menu is on in a release build unless the build says off), and the signing pair.
//  S4 the .ipa is not read back: a wrong id, version, build number, a missing export-compliance answer, a developer menu left on, a non-archive kind must fail the script.
//  S5 key material or an App Store Connect credential is printed (the identity's name, the profile's certificate, the key id, the issuer).
//  S6 the upload runs without an ipa, a key, an issuer or a key file, or for an ipa that is not the production app, or echoes the credentials.
//  K1 the store kind ships the developer launch hooks (ModelProof and EmbedDeviceTrial launch arguments, the embedder fixture); the development kind loses them.
//  E1 exact.sh drops the signing, version and build-kind variables that `env -i` removes.
//  Review round 2 (the lane was sent back; each line is a test below):
//  B1 exact.sh, the entry the README and the Exact reference name for a phone build, lets a development install of the production id through (device with no variant), an archive that
//     is not the store kind, the store kind that is not an archive, an archive of a development app, and a phone build outside the build tree: the guard lived in build-lab.sh only.
//  R1 a variant given as `--variant` or EXACT_VARIANT is dropped or half-applied (the sign-in scheme stays the production id).
//  R2 a relative EXACT_PROFILE or AR_STORE_OUT_DIR is read from one directory by the checks and from another by Exact.
//  R3 upload-store.sh's readback takes an ipa with the developer menu on, a non-archive kind, no export answer, development keys, or a development profile.
//  R4 ALLOW_NO_MODELS makes a store archive with no voice models.
//  R5 the server's list is not asserted to hold the production id here.
//  R6 the build number does not rise past an override or inside a minute.
//  R7 the key-file warning misses a group-only or other-only readable key.
//  R8 make-profile.sh with no choice makes a development profile of the production id (see test-app-identity.sh).
//  R9 docs still tell a builder to run a phone build with no variant, count four app names, or cite the retired variant `one`. (NO TEST: documentation, read by hand; named so in docs/changes.md.)
//  R10 a store build keeps -AutoradioProofSilent (a developer default) because it is not behind the development condition. (Covered since round 5 by D4: every use of a "compiled" name sits inside a build condition.)
//  Review round 3 (the second reviewer's review, read after the first fixes):
//  T1 the microphone purpose string says the app listens only when Voice is on, but Take records the voice and a published take is uploaded.
//  T2 build-lab.sh takes AR_BUILD_KIND=store with a development variant (it must refuse itself, not only through exact.sh at the end).
//  T3 upload-store.sh checks the identity before it refuses a variant (the wrong message), and its key-file check does not follow a symlink.
//  T4 a multi-line EXACT_IDENTITY or AR_BUILD_NUMBER passes a per-line grep; archive-store.sh's comment says the build lock is taken by the script.
//  Round 3 (the guard is inverted: every round of review found another bad invocation, so exact.sh now runs only what is on a list):
//  A1 with no development app chosen, exact.sh runs only the verbs and targets that cannot put a signed production-id app on a phone or publish it (one test per verb of
//     Exact's CLI at the pin); anything else, a verb Exact adds later included, is refused. The two reviewers' commands (`share`, `--archive=path`, a store-kind archive called
//     by hand with a development identity) are cases.
//  A2 the store archive passes only with the marker archive-store.sh sets, and then exact.sh itself requires the store settings and the distribution signing.
//  A3 with a development app chosen, an archive and `share` are refused. Flags are read in both spellings by one function (exact-args.sh).
//  A4 EXACT_DEV_MENU=0 is set for every build by exact.sh; a caller's other value is refused.
//  B1 check-store-ipa.py and check-store-signing.py: expiry, team and prefix, Platform iOS, a corrupt plist is a refusal; the ipa's version is app.json's.
//  B2 EXACT_DEVELOPMENT for the Mac package in release; the fixture turn only in development builds; no background microphone grant; proof-real.sh's process name;
//     a leading-zero record, a non-object JSON line; the docs the reviews name.
//  Round 4 (three rounds without convergence: the guard judged the command line, not the tree the build reads or the artifact it made; docs/problems/store-build-guard-checks-the-route-not-the-artifact-2026-11-24.md):
//  G1 exact.sh and build-lab.sh run an archive (--archive in any spelling, any identity, any environment, the old marker included): only archive-store.sh makes one.
//  G2 exact.sh runs with an empty first argument and further arguments, or help with further arguments.
//  D1 archive-store.sh builds from a tree staged for development: it restages the store kind itself, every time, immediately before Exact runs (the staged hooks are gone when Exact reads them).
//  D2 check-store-ipa.py passes an executable that holds a developer-only name (scripts/store-dev-names.txt); an ipa with no executable; upload-store.sh takes such an ipa from any source.
//  D3 check-store-ipa.py reads only the embedded profile: a signature with get-task-allow, another application-identifier than the profile's, a development authority, or a broken seal passes.
//  D4 the list of names rots: a name is no longer in the developer staging (a rename), or is in the store staging.
//  D5 AR_APP_JSON (another identity file) reaches archive-store.sh or upload-store.sh and ships another version.
//  D6 two archive runs (or a run after the record was lost) are given one build number; an override already given is given again.
//  R11 check-store-signing.py refuses a valid App Store profile that omits get-task-allow, or one whose App ID prefix is not its team id (legacy prefix).
import { describe, expect, setDefaultTimeout, test } from 'bun:test';
// The store scripts run only on macOS: they call codesign, ditto, security and plutil at their system paths (F3), and the store archive is made only on the
// lead's Mac. The groups that execute those tools run there and are skipped on the Linux CI runner, where the tools do not exist; every other group runs everywhere.
const MAC = process.platform === 'darwin';
import { createHash } from 'node:crypto';
import { cpSync, existsSync, renameSync, symlinkSync, realpathSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync, chmodSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';

setDefaultTimeout(120000);          // each test runs shell scripts over a copy of the tree
const MOBILE = join(dirname(new URL(import.meta.url).pathname), '..');
const APP_JSON = JSON.parse(readFileSync(join(MOBILE, 'AutoradioLab', 'app.json'), 'utf8'));
const PROD = 'news.choir.autoradio', LAB = 'news.choir.autoradio.lab', LABONE = 'news.choir.autoradio.lab.one';

const CHILD_MS = 30000;
type R = { code: number; out: string; err: string };
const run = (cmd: string[], env: Record<string, string>, cwd?: string): R => {
  // Every child has a deadline: a script that spins fails the test (and is killed) instead of holding the machine's test lock.
  const p = Bun.spawnSync(cmd, { env, cwd, stdout: 'pipe', stderr: 'pipe', timeout: CHILD_MS, killSignal: 'SIGKILL' });
  if (p.exitCode === null || p.signalCode) throw new Error(`a child ran longer than ${CHILD_MS / 1000}s and was killed: ${cmd.join(' ')}`);
  return { code: p.exitCode ?? -1, out: p.stdout.toString(), err: p.stderr.toString() };
};
const BASE = { PATH: '/usr/bin:/bin:/usr/sbin:/sbin', HOME: process.env.HOME ?? '/tmp' };

describe('the build identity (app-identity.sh over the real app.json)', () => {
  const id = (what: string, env: Record<string, string> = {}) => run(['/bin/sh', join(MOBILE, 'scripts', 'app-identity.sh'), what], { ...BASE, ...env });
  test('I1 no variant is the production app: news.choir.autoradio, "Autoradio 1", version 1.0', () => {
    expect(id('id').out.trim()).toBe(PROD);
    expect(id('name').out.trim()).toBe('Autoradio 1');
    expect(id('variant').out.trim()).toBe('');
    expect(id('version').out.trim()).toBe('1.0');
    expect(APP_JSON.app.id).toBe(PROD);
    expect(APP_JSON.id).toBe(PROD);
    expect(APP_JSON.app.name).toBe('Autoradio 1');
    expect(APP_JSON.version).toMatch(/^[0-9]+(\.[0-9]+){0,2}$/);
  });
  test('I2 the lab apps keep the ids on the owner\'s phone', () => {
    expect(id('id', { AR_VARIANT: 'lab' }).out.trim()).toBe(LAB);
    expect(id('name', { AR_VARIANT: 'lab' }).out.trim()).toBe('Autoradio');
    expect(id('id', { AR_VARIANT: 'lab-one' }).out.trim()).toBe(LABONE);
    expect(id('variant', { AR_APP_ID: LABONE }).out.trim()).toBe('lab-one');
    expect(id('variant', { AR_APP_ID: LAB }).out.trim()).toBe('lab');
    expect(id('variant', { AR_APP_ID: PROD }).out.trim()).toBe('');
    expect(id('id', { AR_APP_ID: LAB, AR_APP_NAME: 'Autoradio' }).out.trim()).toBe(LAB);
  });
  test('I3 a choice app.json does not declare is refused, naming what it declares', () => {
    for (const env of [{ AR_VARIANT: 'one' }, { AR_VARIANT: 'nope' }, { AR_APP_ID: 'news.choir.autoradio.one' }, { AR_APP_NAME: 'Other' }, { AR_VARIANT: 'lab', AR_APP_NAME: 'Autoradio 1' },
      { AR_VARIANT: 'lab', AR_APP_ID: LABONE }, { AR_VARIANT: 'lab', AR_APP_ID: PROD }, { AR_APP_ID: PROD, AR_APP_NAME: 'Autoradio' }]) {
      const r = id('id', env);
      expect(r.code, JSON.stringify(env)).not.toBe(0);
      expect(r.out).toBe('');
      expect(r.err).toContain('app-identity:');
    }
    expect(id('id', { AR_VARIANT: 'one' }).err).toContain(`lab-one ${LABONE}`);   // the retired name says what replaced it
    expect(id('bogus').code).not.toBe(0);
  });
  test('I4 the sign-in scheme is the app id for all three apps (the server\'s list is the Go test\'s: TestDeviceReturnSchemesAreTheAppsInAppJSON)', () => {
    const app = readFileSync(join(MOBILE, 'AutoradioLab', 'app-id.ts'), 'utf8');      // the literal lives in app-id.ts (app.ts imports it)
    expect(/APP_ID[^\n]*\?\? '([^']+)'/.exec(app)![1]).toBe(PROD);
    const schemes = [PROD];
    for (const v of Object.values(APP_JSON.variants) as { idSuffix: string; env: { APP_ID: string } }[]) {
      expect(v.env.APP_ID).toBe(PROD + v.idSuffix);
      schemes.push(v.env.APP_ID);
    }
    expect(schemes.sort()).toEqual([LAB, LABONE, PROD].sort());
  });
  test('I5 no two apps share an installed name (Exact refuses a variant named as another is, selected or not)', () => {
    const files = [APP_JSON.app.name, ...Object.values(APP_JSON.variants).map((v: any) => v.name)].map((n: string) => n.toLowerCase());
    expect(new Set(files).size).toBe(files.length);
    const suffixes = Object.values(APP_JSON.variants).map((v: any) => v.idSuffix);
    expect(new Set(suffixes).size).toBe(suffixes.length);
    for (const name of Object.keys(APP_JSON.variants)) expect(name).toMatch(/^[a-z][a-z0-9-]*$/);
    expect(APP_JSON.deploy.store.ios).toBe('0');             // no update store: the archive needs no receipt (build-and-ship.md)
  });
});

let pin: { dir: string; head: string } | null = null;
function pinnedExact() {                                                                  // an empty git repository with one commit, made once per test run
  if (!pin) {
    const dir = mkdtempSync(join(tmpdir(), 'store-build-pin-'));
    run(['git', 'init', '-q', dir], BASE);
    run(['git', '-C', dir, '-c', 'user.name=t', '-c', 'user.email=t@t', 'commit', '-q', '--allow-empty', '-m', 'pin'], BASE);
    pin = { dir, head: run(['git', '-C', dir, 'rev-parse', 'HEAD'], BASE).out };
  }
  return pin;
}
// ---- a copy of the tree over stubs ----------------------------------------------------------------------------------------------------------------------
type Tree = { t: string; m: string; bin: string; env: (extra?: Record<string, string>) => Record<string, string>; cleanup: () => void };
type TreeOpts = { realStage?: boolean; systemTools?: boolean };   // systemTools: the two checkers are the real ones with no tools argument (the system's own tools, not this tree's stubs)
function tree(opts: TreeOpts = {}): Tree {
  const t = mkdtempSync(join(tmpdir(), 'store-build-'));
  const m = join(t, 'mobile'), bin = join(t, 'bin'), home = join(t, 'home');
  for (const d of [join(m, 'AutoradioLab'), bin, join(home, 'Library')]) mkdirSync(d, { recursive: true });
  cpSync(join(MOBILE, 'scripts'), join(m, 'scripts'), { recursive: true });
  if (!opts.systemTools) {
    // The checkers find codesign, ditto and security in /usr/bin and nowhere else; a test's stubs reach them through their own `--tools <dir>` argument, which archive-store.sh and
    // upload-store.sh never pass. In THIS copy of the tree only, each checker is a two-line shim that hands its real self that argument; the real files are what runs.
    for (const c of ['check-store-ipa', 'check-store-signing']) {
      renameSync(join(m, 'scripts', c + '.py'), join(m, 'scripts', c + '.real.py'));
      writeFileSync(join(m, 'scripts', c + '.py'), `#!/usr/bin/python3 -I\nimport os, sys\nreal = os.path.join(os.path.dirname(os.path.abspath(__file__)), "${c}.real.py")\nextra = [] if "--tools" in sys.argv[1:3] else ["--tools", "${bin}"]\nos.execv("/usr/bin/python3", ["python3", "-I", real] + extra + sys.argv[1:])\n`);
    }
  }
  cpSync(join(MOBILE, 'AutoradioLab', 'app.json'), join(m, 'AutoradioLab', 'app.json'));
  cpSync(pinnedExact().dir, join(m, '.exact-src'), { recursive: true });                 // one made repository, copied: not two git runs per tree
  writeFileSync(join(m, 'exact.pin'), pinnedExact().head);
  writeFileSync(join(m, 'scripts', 'sync-exact-package.sh'), `#!/bin/sh\nprintf 'sync kind=%s\\n' "\${AR_BUILD_KIND-unset}" >> "${t}/order.log"\nexit 0\n`);
  if (opts.realStage) {                                                                 // the real staging over the real model sources and a stub kit, as the K1 test does
    cpSync(join(MOBILE, 'scripts', 'sync-exact-package.sh'), join(m, 'scripts', 'sync-real.sh'));          // the real staging, behind the same logging wrapper
    writeFileSync(join(m, 'scripts', 'sync-exact-package.sh'), `#!/bin/sh\nprintf 'sync kind=%s\\n' "\${AR_BUILD_KIND-unset}" >> "${t}/order.log"\nexec /bin/sh "$(dirname "$0")/sync-real.sh"\n`);
    mkdirSync(join(m, 'AutoradioKit', 'Sources', 'AutoradioKit'), { recursive: true });
    mkdirSync(join(m, 'AutoradioLab', 'modules', 'apple'), { recursive: true });
    mkdirSync(join(m, 'exact-package', 'apple'), { recursive: true });
    mkdirSync(join(m, '.deps', 'xcframeworks', 'FluidAudio.xcframework'), { recursive: true });
    cpSync(join(MOBILE, 'AutoradioKit', 'Sources', 'AutoradioModels'), join(m, 'AutoradioKit', 'Sources', 'AutoradioModels'), { recursive: true });
    writeFileSync(join(m, 'AutoradioKit', 'Sources', 'AutoradioKit', 'Kit.swift'), 'public struct Kit {}\n');
  }
  writeFileSync(join(m, 'scripts', 'gen-exact-interface.py'), '');
  writeFileSync(join(m, 'scripts', 'gen-build-info.sh'), '#!/bin/sh\nexit 0\n');
  writeFileSync(join(m, 'scripts', 'ensure-deps.sh'), '#!/bin/sh\nexit 0\n');
  // xe.sh: records what it would run (the upload's arguments), runs nothing.
  writeFileSync(join(m, 'scripts', 'xe.sh'), `#!/bin/sh\nprintf '%s\\n' "$*" >> "${t}/xe.log"\nexit \${XE_EXIT:-0}\n`);
  // a Bun that stands for Exact: records the variables it was given and writes an .ipa as Exact's archive does (its Info.plist from those variables).
  writeFileSync(join(bin, 'bun'), `#!/bin/sh
D="${bin}"
[ "\${1:-}" = install ] && exit 0
printf 'exact\\n' >> "${t}/order.log"
{ printf 'staged=[%s]\\n' "$(ls modules/apple 2>/dev/null | tr '\\n' ' ')"; printf 'args=[%s]\\n' "$*"; for v in EXACT_VARIANT APP_ID EXACT_VERSION EXACT_BUILD_NUMBER EXACT_NON_EXEMPT_ENCRYPTION EXACT_DEV_MENU EXACT_IDENTITY EXACT_PROFILE EXACT_FEEDBACK EXACT_SIM; do eval "x=\\\${$v-unset}"; printf '%s=[%s]\\n' "$v" "$x"; done; } > "$D/bun.last"
out=""; prev=""; for a in "$@"; do [ "$prev" = --archive ] && out="$a"; prev="$a"; done
[ -n "$out" ] || exit 0
mode="$(cat "$D/mode" 2>/dev/null || echo ok)"
/usr/bin/python3 -I - "$out" "$mode" "$EXACT_VARIANT" "$D" <<'PY'
import plistlib, sys, os, zipfile, json
out, mode, variant = sys.argv[1], sys.argv[2], sys.argv[3]
base = "news.choir.autoradio"
ident = {"": base, "lab": base + ".lab", "lab-one": base + ".lab.one"}[variant]
info = {"CFBundleExecutable": "Autoradio1", "CFBundleIdentifier": ident, "CFBundleDisplayName": "Autoradio 1", "CFBundleShortVersionString": os.environ.get("EXACT_VERSION", "0.1.0"),
        "CFBundleVersion": os.environ.get("EXACT_BUILD_NUMBER", "1"), "ExactBuildKind": "archive"}
if os.environ.get("EXACT_NON_EXEMPT_ENCRYPTION") == "false": info["ITSAppUsesNonExemptEncryption"] = False
if os.environ.get("EXACT_DEV_MENU") == "0": info["ExactDevMenu"] = False
if mode == "wrong-id": info["CFBundleIdentifier"] = base + ".lab"
if mode == "wrong-build": info["CFBundleVersion"] = "1"
if mode == "wrong-version": info["CFBundleShortVersionString"] = "0.1.0"
if mode == "no-encryption": info.pop("ITSAppUsesNonExemptEncryption", None)
if mode == "encryption-true": info["ITSAppUsesNonExemptEncryption"] = True
if mode == "devmenu-on": info.pop("ExactDevMenu", None)
if mode == "debug-kind": info["ExactBuildKind"] = "debug"
if mode == "dev-scheme": info["ExactDevelopmentURLScheme"] = "exact-dev"
if mode == "with-host": info.update({"ExactBuildHost": "secret-host", "ExactAppCommit": "0123456789abcdef0123456789abcdef01234567", "ExactAppCommitDirty": False})
os.makedirs(os.path.dirname(out), exist_ok=True)
with zipfile.ZipFile(out, "w") as z:
    z.writestr("Payload/Autoradio 1.app/Info.plist", plistlib.dumps(info))
    exe = b"\\xcf\\xfa\\xed\\xfe fixture executable"                       # the "compiled" app: what Exact would compile, here the staged sources it reads and any extra bytes the test leaves
    import glob
    for f in sorted(glob.glob("modules/apple/*.swift")): exe += open(f, "rb").read()
    if os.path.exists(sys.argv[4] + "/exe-extra"): exe += open(sys.argv[4] + "/exe-extra", "rb").read()
    z.writestr("Payload/Autoradio 1.app/Autoradio1", exe)
    if os.path.exists(sys.argv[4] + "/framework-extra"): z.writestr("Payload/Autoradio 1.app/Frameworks/ExactModules.framework/ExactModules", b"\\xcf\\xfa\\xed\\xfe\\x0c\\x00\\x00\\x01" + open(sys.argv[4] + "/framework-extra", "rb").read())
    if mode != "no-profile": z.writestr("Payload/Autoradio 1.app/embedded.mobileprovision", b"x")
print("null" if mode == "json-null" else json.dumps({"path": out, "target": "ios", "trust": "production", "signing": os.environ.get("EXACT_IDENTITY", "ad-hoc")}))
PY
`);
  chmodSync(join(bin, 'bun'), 0o755);
  // security: the identity list and the decoded profile are files the test writes.
  writeFileSync(join(bin, 'security'), `#!/bin/sh
case "$1" in
  find-identity) cat "${bin}/identities" 2>/dev/null; exit 0 ;;
  cms) cat "${bin}/profile.plist"; exit 0 ;;
esac
exit 1
`);
  chmodSync(join(bin, 'security'), 0o755);
  // codesign: the signature's entitlements, its authority lines and whether the seal verifies are files the test writes (writeCodesign); the default is a store signature.
  writeFileSync(join(bin, 'codesign'), `#!/bin/sh
case "$*" in
  *--entitlements*) cat "${bin}/ents.plist"; exit 0 ;;
  *-dvv*) cat "${bin}/dv.txt" >&2; exit 0 ;;
  *--verify*) exit \$(cat "${bin}/verify" 2>/dev/null || echo 0) ;;
esac
exit 1
`);
  chmodSync(join(bin, 'codesign'), 0o755);
  writeCodesign({ bin } as Tree);
  writeFileSync(join(t, 'tree'), `${realpathSync(m)}\n`);
  const env = (extra: Record<string, string> = {}) => ({ ...BASE, PATH: `${bin}:${BASE.PATH}`, HOME: home, BUN: join(bin, 'bun'), ALLOW_NO_MODELS: '1',
    AUTORADIO_BUILD_TREE_FILE: join(t, 'tree'), AR_STORE_OUT_DIR: join(t, 'out'), TMPDIR: t, ...extra });
  return { t, m, bin, env, cleanup: () => rmSync(t, { recursive: true, force: true }) };
}

// A profile as `security cms -D` prints it, and the identity list line, for a made-up certificate.
const CERT = Buffer.from('not a real certificate: the test only needs its hash');
const SHA = createHash('sha1').update(CERT).digest('hex').toUpperCase();
const OTHER_SHA = createHash('sha1').update('another').digest('hex').toUpperCase();
function writeSigning(tr: Tree, over: { platform?: string[] | null; raw?: string; id?: string; teamPrefix?: string; prefix?: string; idPrefix?: string; noPrefixKey?: boolean; noTaskAllow?: boolean; taskAllow?: boolean; devices?: boolean; all?: boolean; expires?: Date; cert?: Buffer; identityLine?: string } = {}) {
  const team = over.teamPrefix ?? 'ABCDE12345';
  const plist = (v: string) => v;
  const prefix = over.prefix ?? team, idPrefix = over.idPrefix ?? prefix;            // ApplicationIdentifierPrefix is the team id except for a legacy App ID
  const taskAllow = over.noTaskAllow ? '' : `<key>get-task-allow</key>${over.taskAllow ? '<true/>' : '<false/>'}`;
  const xml = `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict>
<key>Name</key><string>test store profile</string>${over.platform === null ? '' : `<key>Platform</key><array>${(over.platform ?? ['iOS']).map((x) => `<string>${x}</string>`).join('')}</array>`}<key>TeamIdentifier</key><array><string>${team}</string></array>${over.noPrefixKey ? '' : `<key>ApplicationIdentifierPrefix</key><array><string>${prefix}</string></array>`}
<key>ExpirationDate</key><date>${(over.expires ?? new Date(Date.now() + 86400e3 * 300)).toISOString().replace(/\.\d+Z$/, 'Z')}</date>
${over.devices ? '<key>ProvisionedDevices</key><array><string>0000</string></array>' : ''}${over.all ? '<key>ProvisionsAllDevices</key><true/>' : ''}
<key>DeveloperCertificates</key><array><data>${(over.cert ?? CERT).toString('base64')}</data></array>
<key>Entitlements</key><dict><key>application-identifier</key><string>${idPrefix}.${over.id ?? PROD}</string>${taskAllow}
<key>com.apple.developer.team-identifier</key><string>${team}</string></dict></dict></plist>`;
  writeFileSync(join(tr.bin, 'profile.plist'), over.raw ?? plist(xml));
  writeFileSync(join(tr.bin, 'identities'), over.identityLine ?? `  1) ${SHA} "Apple Distribution: Someone (${team})"\n     1 valid identities found\n`);
  const profile = join(tr.t, 'store.mobileprovision');
  writeFileSync(profile, 'cms bytes the stub ignores');
  return profile;
}
// What `codesign -d` says of the signed app: the entitlements as a plist, the authority lines, and whether `codesign --verify` passes.
function writeCodesign(tr: Pick<Tree, 'bin'>, over: { id?: string; taskAllow?: boolean | null; authority?: string; verify?: number; team?: string } = {}) {
  const ta = over.taskAllow === null ? '' : `<key>get-task-allow</key>${over.taskAllow ? '<true/>' : '<false/>'}`;
  writeFileSync(join(tr.bin, 'ents.plist'), `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>application-identifier</key><string>${over.id ?? 'ABCDE12345.' + PROD}</string>${ta}</dict></plist>\n`);
  writeFileSync(join(tr.bin, 'dv.txt'), `Identifier=${PROD}\nAuthority=${over.authority ?? 'Apple Distribution: Someone (ABCDE12345)'}\nAuthority=Apple Worldwide Developer Relations Certification Authority\nTeamIdentifier=${over.team ?? 'ABCDE12345'}\n`);
  writeFileSync(join(tr.bin, 'verify'), String(over.verify ?? 0));
}
// An ipa as Exact writes one, for check-store-ipa.py: `info` overrides or (null) removes a key; `exe` is the executable's bytes (null: none in the archive); `profile` false leaves the profile out.
function makeIpa(tr: Tree, name: string, o: { id?: string; info?: Record<string, unknown>; exe?: string | null; profile?: boolean; corrupt?: boolean } = {}) {
  const p = join(tr.t, name);
  const base: Record<string, unknown> = { CFBundleExecutable: 'A', CFBundleIdentifier: o.id ?? PROD, CFBundleShortVersionString: '1.0', CFBundleVersion: '7', ExactBuildKind: 'archive', ExactDevMenu: false, ITSAppUsesNonExemptEncryption: false };
  const info: Record<string, unknown> = { ...base, ...(o.info ?? {}) };
  for (const k of Object.keys(info)) if (info[k] === null) delete info[k];
  const r = run(['/usr/bin/python3', '-I', '-c', 'import json,plistlib,sys,zipfile\nz=zipfile.ZipFile(sys.argv[1],"w")\nz.writestr("Payload/A.app/Info.plist",b"not a plist" if sys.argv[3]=="1" else plistlib.dumps(json.loads(sys.argv[2])))\nif sys.argv[5]=="1": z.writestr("Payload/A.app/embedded.mobileprovision",b"x")\nif sys.argv[4]!="-": z.writestr("Payload/A.app/A",sys.argv[4].encode())\nz.close()',
    p, JSON.stringify(info), o.corrupt ? '1' : '0', o.exe === null ? '-' : (o.exe ?? 'a fixture executable'), o.profile === false ? '0' : '1'], BASE);
  expect(r.code, r.err).toBe(0);
  return p;
}
const archive = (tr: Tree, extra: Record<string, string> = {}, cwd?: string) => {
  const e = tr.env(extra);
  if (!('ALLOW_NO_MODELS' in extra)) delete e.ALLOW_NO_MODELS;                       // a store archive never runs without the model libraries (R4); the other scripts' tests do
  return run(['/bin/sh', join(tr.m, 'scripts', 'archive-store.sh')], e, cwd);
};

describe('building for a phone and the store kind (build-lab.sh)', () => {
  test('S1 a phone build must name which development app; an archive is never build-lab.sh\'s, and a development build is always staged as one', () => {
    const tr = tree();
    try {
      const lab = (args: string[], extra: Record<string, string> = {}) => run(['/bin/sh', join(tr.m, 'scripts', 'build-lab.sh'), ...args], tr.env(extra));
      let r = lab(['ios', '--device', '--phone', 'X']);
      expect(r.code).not.toBe(0); expect(r.err).toContain('AR_VARIANT'); expect(existsSync(join(tr.bin, 'bun.last'))).toBe(false);
      r = lab(['ios', '--device', '--phone', 'X'], { AR_VARIANT: 'lab' });
      expect(r.code).toBe(0); expect(readFileSync(join(tr.bin, 'bun.last'), 'utf8')).toContain('EXACT_VARIANT=[lab]');
      r = lab(['ios']);                                                                  // the simulator: any identity, the production id by default
      expect(r.code).toBe(0); expect(readFileSync(join(tr.bin, 'bun.last'), 'utf8')).toContain('EXACT_VARIANT=[unset]');
      rmSync(join(tr.t, 'order.log'), { force: true });
      for (const extra of [{}, { AR_BUILD_KIND: 'store' }, { AR_BUILD_KIND: 'store', AR_STORE_ARCHIVE: '1' }, { AR_VARIANT: 'lab' }]) {   // an archive is not build-lab.sh's, whatever the environment says
        r = lab(['ios', '--device', '--archive', join(tr.t, 'x.ipa')], extra);
        expect(r.code).not.toBe(0); expect(r.err).toContain('archive-store.sh');
      }
      expect(existsSync(join(tr.t, 'order.log'))).toBe(false);                            // nothing was staged and nothing ran
      // AR_BUILD_KIND is not read: a development build is always staged as one, so a stale exported kind cannot strip the hooks from a development build or gate anything
      writeFileSync(join(tr.t, 'order.log'), ''); r = lab(['ios'], { AR_BUILD_KIND: 'store' });
      expect(r.code, r.err).toBe(0); expect(readFileSync(join(tr.t, 'order.log'), 'utf8')).toContain('sync kind=development');
    } finally { tr.cleanup(); }
  });
});

describe('archive-store.sh refuses plainly before it builds anything', () => {
  const expectRefused = (r: R, tr: Tree, words: string) => {
    expect(r.code, r.err).not.toBe(0);
    expect(r.err).toContain('archive-store:');
    expect(r.err).toContain(words);
    expect(existsSync(join(tr.bin, 'bun.last')), 'Exact must not have run').toBe(false);
    expect(r.out + r.err).not.toContain(SHA);                                           // S5: no hash echoed back with a refusal either
  };
  test('S2 a variant, the wrong tree', () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      expectRefused(archive(tr, { AR_VARIANT: 'lab', EXACT_IDENTITY: SHA, EXACT_PROFILE: profile }), tr, 'production');
      expectRefused(archive(tr, { AR_APP_ID: LAB, EXACT_IDENTITY: SHA, EXACT_PROFILE: profile }), tr, 'production');
      writeFileSync(join(tr.t, 'tree'), `${tr.t}\n`);
      expectRefused(archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile }), tr, 'build tree');
    } finally { tr.cleanup(); }
  });
  test('S2 the identity and the profile must be named, and be what the store takes', () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      expectRefused(archive(tr, { EXACT_PROFILE: profile }), tr, 'EXACT_IDENTITY');
      expectRefused(archive(tr, { EXACT_IDENTITY: SHA }), tr, 'EXACT_PROFILE');
      expectRefused(archive(tr, { EXACT_IDENTITY: 'Apple Distribution: Someone', EXACT_PROFILE: profile }), tr, '40');
      expectRefused(archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: join(tr.t, 'missing.mobileprovision') }), tr, 'EXACT_PROFILE');
      writeSigning(tr, { identityLine: `  1) ${SHA} "Apple Development: Someone (ABCDE12345)"\n` });
      expectRefused(archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile }), tr, 'Apple Distribution');
      writeSigning(tr, { identityLine: '     0 valid identities found\n' });
      expectRefused(archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile }), tr, 'Apple Distribution');
      writeSigning(tr, { id: LAB });
      expectRefused(archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile }), tr, PROD);
      writeSigning(tr, { id: '*' });
      expectRefused(archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile }), tr, PROD);
      writeSigning(tr, { taskAllow: true });
      expectRefused(archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile }), tr, 'App Store');
      writeSigning(tr, { devices: true });
      expectRefused(archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile }), tr, 'App Store');
      writeSigning(tr, { all: true });
      expectRefused(archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile }), tr, 'App Store');
      writeSigning(tr, { expires: new Date(Date.now() - 86400e3) });
      expectRefused(archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile }), tr, 'expired');
      writeSigning(tr, { cert: Buffer.from('some other certificate') });
      expectRefused(archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile }), tr, 'identity');
      writeSigning(tr);
      expectRefused(archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile, AR_BUILD_NUMBER: '12ab' }), tr, 'AR_BUILD_NUMBER');
      expectRefused(archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile, AR_BUILD_NUMBER: '0' }), tr, 'AR_BUILD_NUMBER');
      expectRefused(archive(tr, { EXACT_IDENTITY: OTHER_SHA, EXACT_PROFILE: profile }), tr, 'Apple Distribution');
    } finally { tr.cleanup(); }
  });
});

describe.skipIf(!MAC)('check-store-signing.py reads the profile as Apple writes it', () => {
  test('R11 a valid App Store profile that omits get-task-allow, or carries a legacy App ID prefix, is accepted; a prefix that is not the profile\'s own is refused', () => {
    const tr = tree();
    try {
      const ok = (over: Parameters<typeof writeSigning>[1], what: string) => {
        const profile = writeSigning(tr, over);
        writeCodesign(tr, { id: (over?.prefix ?? 'ABCDE12345') + '.' + PROD });                // the signed app carries the profile's own application-identifier
        const r = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });
        expect(r.code, what + ': ' + r.err).toBe(0);
      };
      ok({ noTaskAllow: true }, 'no get-task-allow');
      ok({ prefix: 'LEGACY9999' }, 'legacy prefix, the profile\'s own');
      ok({ noPrefixKey: true }, 'no ApplicationIdentifierPrefix key: the team id');
      const profile = writeSigning(tr, { prefix: 'LEGACY9999', idPrefix: 'ABCDE12345' });   // application-identifier does not start with the profile's own prefix
      const r = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });
      expect(r.code).not.toBe(0); expect(r.err).toContain(PROD);
      const t = writeSigning(tr, { taskAllow: true });                                       // true stays refused
      const rt = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: t });
      expect(rt.code).not.toBe(0); expect(rt.err).toContain('App Store');
    } finally { tr.cleanup(); }
  });
});

describe.skipIf(!MAC)('archive-store.sh builds and reads the ipa back', () => {
  test('S3 Exact is given the version, a rising build number, the export answer, the developer menu off, and the signing pair; the ipa is read back', () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      const r = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });
      expect(r.code, r.err + r.out).toBe(0);
      const seen = readFileSync(join(tr.bin, 'bun.last'), 'utf8');
      expect(seen).toMatch(/args=\[exact\.mjs ios --device --archive \S+\.ipa --json\]/);
      expect(seen).toContain('EXACT_VARIANT=[unset]');
      expect(seen).toContain(`APP_ID=[${PROD}]`);
      expect(seen).toContain('EXACT_VERSION=[1.0]');
      expect(seen).toMatch(/EXACT_BUILD_NUMBER=\[[1-9][0-9]{0,8}\]/);
      expect(seen).toContain('EXACT_NON_EXEMPT_ENCRYPTION=[false]');
      expect(seen).toContain('EXACT_DEV_MENU=[0]');
      expect(seen).toContain(`EXACT_IDENTITY=[${SHA}]`);
      expect(seen).toContain(`EXACT_PROFILE=[${realpathSync(profile)}]`);          // R2: absolute and canonical
      const build = /EXACT_BUILD_NUMBER=\[(\d+)\]/.exec(seen)![1];
      expect(r.out).toContain('Autoradio-1-1.0-' + build + '.ipa');
      expect(r.out).toContain(`bundle id: ${PROD}`);
      expect(r.out).toContain('version: 1.0');
      expect(r.out).toContain(`build number: ${build}`);
      expect(r.out).toContain('ITSAppUsesNonExemptEncryption: false');
      expect(r.out + r.err).not.toContain(SHA);                                         // S5: the identity is named by Exact's JSON `signing` and by the environment, never by this script
      expect(r.out + r.err).not.toContain('Someone');
      expect(readdirSync(join(tr.t, 'out')).some((f) => f.endsWith('.ipa'))).toBe(true);
    } finally { tr.cleanup(); }
  });
  test('R6 the build number starts at the minute count and rises with every run, also inside one minute and after an override', () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      const num = (extra: Record<string, string> = {}) => {
        const r = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile, ...extra });
        expect(r.code, r.err).toBe(0);
        return Number(/build number: (\d+)/.exec(r.out)![1]);
      };
      const a = num();
      const nowMinutes = Math.floor((Date.now() - Date.UTC(2026, 0, 1)) / 60000);
      expect(a).toBeGreaterThan(nowMinutes - 3); expect(a).toBeLessThan(nowMinutes + 3);   // minutes since 2026-01-01 UTC
      expect(String(a).length).toBeLessThanOrEqual(9);
      const b = num();                                                                      // a second default run, almost certainly in the same minute
      expect(b).toBeGreaterThan(a);
      const given = b + 5_000_000;                                                          // an override far above the clock (what App Store Connect might hold)
      expect(num({ AR_BUILD_NUMBER: String(given) })).toBe(given);
      expect(num()).toBeGreaterThan(given);                                                 // the next default run does not fall below the uploaded one
      expect(num({ AR_BUILD_NUMBER: '7' })).toBe(7);                                        // a low override is the caller's own word ...
      expect(num()).toBeGreaterThan(given + 1);                                             // ... and does not lower the record
      writeFileSync(join(tr.t, 'out', 'last-build'), 'garbage\n');
      const bad = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });
      expect(bad.code).not.toBe(0); expect(bad.err).toContain('last-build');
    } finally { tr.cleanup(); }
  });
  test('R2 a relative EXACT_PROFILE and a relative AR_STORE_OUT_DIR mean the directory the script was started in, for the checks and for Exact alike', () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      const here = realpathSync(tr.t);
      const r = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: 'store.mobileprovision', AR_STORE_OUT_DIR: 'relout' }, tr.t);
      expect(r.code, r.err + r.out).toBe(0);
      expect(readFileSync(join(tr.bin, 'bun.last'), 'utf8')).toContain(`EXACT_PROFILE=[${here}/store.mobileprovision]`);
      expect(r.out).toContain(`ipa: ${here}/relout/`);
      expect(readdirSync(join(tr.t, 'relout')).some((f) => f.endsWith('.ipa'))).toBe(true);
      expect(profile).toContain('store.mobileprovision');
    } finally { tr.cleanup(); }
  });
  test('R4 ALLOW_NO_MODELS makes no store archive', () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      const r = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile, ALLOW_NO_MODELS: '1' });
      expect(r.code).not.toBe(0); expect(r.err).toContain('archive-store:'); expect(r.err).toContain('ALLOW_NO_MODELS');
      expect(existsSync(join(tr.bin, 'bun.last'))).toBe(false);
    } finally { tr.cleanup(); }
  });
  test('S4 an ipa that is not what was asked for fails the script', () => {
    for (const [mode, words] of [['wrong-id', 'bundle id'], ['wrong-build', 'build number'], ['wrong-version', 'version'], ['no-encryption', 'ITSAppUsesNonExemptEncryption'],
      ['encryption-true', 'ITSAppUsesNonExemptEncryption'], ['devmenu-on', 'developer menu'], ['debug-kind', 'ExactBuildKind'], ['dev-scheme', 'ExactDevelopment'], ['no-profile', 'provisioning profile']] as const) {
      const tr = tree();
      try {
        const profile = writeSigning(tr);
        writeFileSync(join(tr.bin, 'mode'), mode);
        const r = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });
        expect(r.code, mode).not.toBe(0);
        expect(r.err, mode).toContain(words);
      } finally { tr.cleanup(); }
    }
  });
});

describe.skipIf(!MAC)('upload-store.sh', () => {
  // An ipa as archive-store.sh leaves it (makeIpa): archive kind, developer menu off, export answer false, a profile embedded, an executable. `info` overrides or (null) removes a key.
  const ipaFor = (tr: Tree, id = PROD, info: Record<string, unknown> = {}) => makeIpa(tr, 'x.ipa', { id, info });
  const upload = (tr: Tree, args: string[], extra: Record<string, string> = {}) => run(['/bin/sh', join(tr.m, 'scripts', 'upload-store.sh'), ...args], tr.env(extra));
  const KEY = 'ZZKEYIDSECRET', ISS = '11111111-2222-3333-4444-555555555555';
  test('S6 it needs an ipa that is the production app, a key id, an issuer and a key file, and never echoes them', () => {
    const tr = tree();
    try {
      writeSigning(tr);                                                                  // the profile the stub `security cms -D` decodes out of the ipa
      const keyfile = join(tr.t, 'AuthKey.p8'); writeFileSync(keyfile, 'private key bytes'); chmodSync(keyfile, 0o600);
      const good = { ASC_KEY_ID: KEY, ASC_ISSUER_ID: ISS, ASC_KEY_FILE: keyfile };
      const ipa = ipaFor(tr);
      for (const [args, env, words] of [[[], good, 'ipa'], [[join(tr.t, 'no.ipa')], good, 'ipa'], [[ipa], { ...good, ASC_KEY_ID: '' }, 'ASC_KEY_ID'], [[ipa], { ...good, ASC_ISSUER_ID: '' }, 'ASC_ISSUER_ID'],
        [[ipa], { ...good, ASC_KEY_FILE: '' }, 'ASC_KEY_FILE'], [[ipa], { ...good, ASC_KEY_FILE: join(tr.t, 'nokey.p8') }, 'ASC_KEY_FILE'],
        [[ipaFor(tr, LAB)], good, PROD], [[ipaFor(tr, LABONE)], good, PROD]] as [string[], Record<string, string>, string][]) {
        const r = upload(tr, args, env);
        expect(r.code, JSON.stringify(args)).not.toBe(0);
        expect(r.err).toContain('upload-store:'); expect(r.err).toContain(words);
        expect(existsSync(join(tr.t, 'xe.log'))).toBe(false);
        for (const secret of [KEY, ISS]) expect(r.out + r.err).not.toContain(secret);
      }
      const r = upload(tr, [ipaFor(tr)], good);
      expect(r.code, r.err).toBe(0);
      const log = readFileSync(join(tr.t, 'xe.log'), 'utf8');
      expect(log).toContain('altool --upload-app -f');
      expect(log).toContain('--p8-file-path ' + keyfile);
      expect(log).not.toContain('--password'); expect(log).not.toContain(' -p ');
      for (const secret of [KEY, ISS]) expect(r.out + r.err, 'credentials are not echoed').not.toContain(secret);
      expect(r.out).toContain('uploaded');
      const f = run(['/bin/sh', join(tr.m, 'scripts', 'upload-store.sh'), ipaFor(tr)], tr.env({ ...good, XE_EXIT: '1' }));
      expect(f.code).not.toBe(0); expect(f.err).toContain('altool'); for (const secret of [KEY, ISS]) expect(f.out + f.err).not.toContain(secret);
    } finally { tr.cleanup(); }
  });
  test('R3 the readback refuses an ipa that is not what archive-store.sh makes, whatever made it', () => {
    const tr = tree();
    try {
      const keyfile = join(tr.t, 'AuthKey.p8'); writeFileSync(keyfile, 'private key bytes'); chmodSync(keyfile, 0o600);
      const good = { ASC_KEY_ID: KEY, ASC_ISSUER_ID: ISS, ASC_KEY_FILE: keyfile };
      writeSigning(tr);
      const refused = (ipa: string, words: string, what: string) => {
        const r = upload(tr, [ipa], good);
        expect(r.code, what).not.toBe(0); expect(r.err, what).toContain('upload-store:'); expect(r.err, what).toContain(words);
        expect(existsSync(join(tr.t, 'xe.log')), what + ': altool must not run').toBe(false);
      };
      refused(ipaFor(tr, PROD, { ExactDevMenu: null }), 'developer menu', 'menu not stamped');
      refused(ipaFor(tr, PROD, { ExactDevMenu: true }), 'developer menu', 'menu on');
      refused(ipaFor(tr, PROD, { ExactBuildKind: 'debug' }), 'ExactBuildKind', 'debug kind');
      refused(ipaFor(tr, PROD, { ITSAppUsesNonExemptEncryption: null }), 'ITSAppUsesNonExemptEncryption', 'no export answer');
      refused(ipaFor(tr, PROD, { ITSAppUsesNonExemptEncryption: true }), 'ITSAppUsesNonExemptEncryption', 'export answer true');
      refused(ipaFor(tr, PROD, { ExactDevelopmentURLScheme: 'x' }), 'ExactDevelopment', 'development key');
      refused(ipaFor(tr, PROD, { CFBundleShortVersionString: '' }), 'version', 'no version');
      refused(ipaFor(tr, PROD, { CFBundleVersion: null }), 'build number', 'no build number');
      writeSigning(tr, { taskAllow: true });
      refused(ipaFor(tr), 'App Store', 'a development profile embedded');
      writeSigning(tr, { devices: true });
      refused(ipaFor(tr), 'App Store', 'an ad hoc profile embedded');
      writeSigning(tr, { id: LAB });
      refused(ipaFor(tr), PROD, 'a profile for another id');
      writeSigning(tr, { noTaskAllow: true });                                         // Apple's own App Store profiles may omit get-task-allow
      const r = upload(tr, [ipaFor(tr)], good);
      expect(r.code, r.err).toBe(0);
    } finally { tr.cleanup(); }
  });
  test('R7 a key file readable by group or others is warned about, whichever of the two it is; mode 600 and 400 are not', () => {
    const tr = tree();
    try {
      writeSigning(tr);
      const ipa = ipaFor(tr);
      for (const [mode, warned] of [[0o600, false], [0o400, false], [0o640, true], [0o604, true], [0o644, true], [0o660, true]] as [number, boolean][]) {
        const keyfile = join(tr.t, 'AuthKey.p8'); rmSync(keyfile, { force: true }); writeFileSync(keyfile, 'private key bytes'); chmodSync(keyfile, mode);
        const r = upload(tr, [ipa], { ASC_KEY_ID: KEY, ASC_ISSUER_ID: ISS, ASC_KEY_FILE: keyfile });
        expect(r.code, r.err).toBe(0);
        expect(r.err.includes('readable by others'), mode.toString(8)).toBe(warned);
      }
    } finally { tr.cleanup(); }
  });
});

describe('the store kind of the staged Swift (sync-exact-package.sh)', () => {
  // A tree with the real model sources and a stub kit; the xcframework directory exists so the models are staged.
  function staged(kind: string | null, extra: Record<string, string> = {}, models = true) {
    const t = mkdtempSync(join(tmpdir(), 'store-kind-'));
    const m = join(t, 'mobile');
    mkdirSync(join(m, 'AutoradioKit', 'Sources', 'AutoradioKit'), { recursive: true });
    mkdirSync(join(m, 'AutoradioLab', 'modules', 'apple'), { recursive: true });
    mkdirSync(join(m, 'exact-package', 'apple'), { recursive: true });
    if (models) mkdirSync(join(m, '.deps', 'xcframeworks', 'FluidAudio.xcframework'), { recursive: true });
    cpSync(join(MOBILE, 'scripts'), join(m, 'scripts'), { recursive: true });
    cpSync(join(MOBILE, 'AutoradioKit', 'Sources', 'AutoradioModels'), join(m, 'AutoradioKit', 'Sources', 'AutoradioModels'), { recursive: true });
    writeFileSync(join(m, 'AutoradioKit', 'Sources', 'AutoradioKit', 'Kit.swift'), 'public struct Kit {}\n');
    writeFileSync(join(m, 'scripts', 'gen-build-info.sh'), '#!/bin/sh\nexit 0\n');
    const r = run(['/bin/sh', join(m, 'scripts', 'sync-exact-package.sh')], { ...BASE, ...(kind ? { AR_BUILD_KIND: kind } : {}), ...extra });
    const files = readdirSync(join(m, 'AutoradioLab', 'modules', 'apple')).filter((f) => f.endsWith('.swift'));
    const text = files.map((f) => readFileSync(join(m, 'AutoradioLab', 'modules', 'apple', f), 'utf8')).join('\n');
    return { r, files, text, done: () => rmSync(t, { recursive: true, force: true }) };
  }
  test('K1 development stages the launch hooks, the store kind does not, and a kind that is neither is refused', () => {
    const dev = staged(null);
    try {
      expect(dev.r.code, dev.r.err).toBe(0);
      expect(dev.files).toContain('Models_EmbedDeviceTrial.swift'); expect(dev.files).toContain('Models_EmbedTrialFixture.swift');
      expect(dev.text).toContain('EmbedDeviceTrial.runFromLaunchArguments()'); expect(dev.text).toContain('ModelProof.runFromLaunchArguments()');
    } finally { dev.done(); }
    const store = staged('store');
    try {
      expect(store.r.code, store.r.err).toBe(0);
      expect(store.files).toContain('Models_ModelHub.swift'); expect(store.files).toContain('Models_ModelProof.swift');   // the product's own model code stays
      expect(store.files).not.toContain('Models_EmbedDeviceTrial.swift'); expect(store.files).not.toContain('Models_EmbedTrialFixture.swift');
      expect(store.text).not.toMatch(/\.runFromLaunchArguments\(\)/); expect(store.text).not.toContain('EmbedDeviceTrial');   // no call; ModelProof's own (uncalled) definition stays, with the product's ModelProof.frames beside it
      expect(store.text).not.toContain('EmbedTrialFixture');
      expect(store.text).toContain('ModelHub.register(into: r)');
    } finally { store.done(); }
    for (const withModels of [true, false]) {                                           // R4: the store kind never takes ALLOW_NO_MODELS, with or without the libraries
      const nm = staged('store', { ALLOW_NO_MODELS: '1' }, withModels);
      try { expect(nm.r.code).not.toBe(0); expect(nm.r.err).toContain('ALLOW_NO_MODELS'); } finally { nm.done(); }
    }
    const bad = staged('weird');
    try { expect(bad.r.code).not.toBe(0); expect(bad.r.err).toContain('AR_BUILD_KIND'); } finally { bad.done(); }
  });
});

describe('exact.sh runs only what is on its list (A1, A3, A4, G1, G2)', () => {
  const ex = (tr: Tree, args: string[], extra: Record<string, string> = {}) => run(['/bin/sh', join(tr.m, 'scripts', 'exact.sh'), ...args], tr.env(extra));
  const ran = (tr: Tree) => existsSync(join(tr.bin, 'bun.last'));
  const seen = (tr: Tree) => readFileSync(join(tr.bin, 'bun.last'), 'utf8');
  const refused = (tr: Tree, args: string[], extra: Record<string, string>, words = '') => {
    rmSync(join(tr.bin, 'bun.last'), { force: true });
    const r = ex(tr, args, extra);
    expect(r.code, args.join(' ') + ' ' + JSON.stringify(extra)).not.toBe(0);
    expect(r.err, args.join(' ')).toContain('exact.sh:'); expect(r.err).toContain(words);
    expect(ran(tr), args.join(' ') + ': Exact must not have run').toBe(false);
  };
  const allowed = (tr: Tree, args: string[], extra: Record<string, string> = {}) => {
    rmSync(join(tr.bin, 'bun.last'), { force: true });
    const r = ex(tr, args, extra);
    expect(r.code, args.join(' ') + ' ' + JSON.stringify(extra) + '\n' + r.err).toBe(0); expect(ran(tr), args.join(' ')).toBe(true);
  };
  // every verb of Exact's CLI at the pin (scripts/app-cli.mjs appVerbs), and what exact.sh does with it when no development app is chosen
  const NO_VARIANT_RUNS: string[][] = [[], ['help'], ['--help'], ['contract', 'build', 'app.contract'], ['forma', 'check'], ['docs'], ['web'], ['web-build'], ['serve'],
    ['test', 'web'], ['agent', 'web'], ['test', 'ios'], ['agent', 'ios', 'tree'], ['test', 'macos'], ['agent', 'mac'], ['verify', 'web'], ['ios'], ['ios', '--run'], ['ios', '--json'], ['mac'], ['mac', '--run']];
  const NO_VARIANT_REFUSED: string[][] = [['share'], ['share', '--app', 'x'], ['deploy'], ['push'], ['update'], ['lock'], ['add'], ['hatch'], ['snapback'], ['backend'], ['pattern'], ['feedback'], ['linux'], ['windows'], ['android'],
    ['status'], ['check'], ['game'], ['prove'], ['tamber'], ['frobnicate'], ['verify'], ['verify', 'ios'], ['test'], ['agent'], ['test', 'android'], ['agent', 'windows'], ['test', 'linux'],
    ['test', 'ios', '--device'], ['agent', 'ios', '--device', '--phone', 'X'], ['agent', 'ios', '--device=1'], ['agent', 'ios', '--phone=X'], ['ios', '--device'], ['ios', '--device', '--phone', 'X'],
    ['ios', '--device=1'], ['ios', '--archive', '/tmp/x.ipa'], ['ios', '--archive=/tmp/x.ipa'], ['ios', '--unsigned'], ['ios', '--release'], ['ios', '--sim', 'x'], ['mac', '--archive', '/tmp/x'], ['mac', '--device'],
    ['--variant', 'lab', 'ios'], ['ios', '--variant=lab'], ['ios', '--run', '--device']];
  test('A1 no development app chosen: each verb of Exact\'s CLI is run or refused as listed; what is not listed is refused', () => {
    const tr = tree();
    try {
      for (const args of NO_VARIANT_RUNS) allowed(tr, args);
      for (const args of NO_VARIANT_REFUSED) refused(tr, args, {});
    } finally { tr.cleanup(); }
  });
  test('A1 the reviewers\' own commands: share, --archive=path outside the build tree, a hand-run archive with a development identity or the old marker', () => {
    const tr = tree();
    try {
      refused(tr, ['share'], {}, 'share');
      refused(tr, ['ios', '--archive=/tmp/prod.ipa'], { AUTORADIO_BUILD_TREE_FILE: join(tr.t, 'nowhere') }, 'archive-store.sh');
      refused(tr, ['ios', '--device', '--archive=/tmp/dev.ipa'], { AR_VARIANT: 'lab' }, 'archive-store.sh');
      const profile = writeSigning(tr, { identityLine: `  1) ${SHA} "Apple Development: Someone (ABCDE12345)"\n`, taskAllow: true });
      refused(tr, ['ios', '--device', '--archive', '/tmp/p.ipa'], { AR_BUILD_KIND: 'store', EXACT_IDENTITY: SHA, EXACT_PROFILE: profile }, 'archive-store.sh');
      refused(tr, ['ios', '--device', '--archive', '/tmp/p.ipa'], { ...STORE_ENV(tr, profile) }, 'archive-store.sh');                   // the retired marker, the store settings: nothing an environment says makes an archive
    } finally { tr.cleanup(); }
  });
  const STORE_ENV = (tr: Tree, profile: string, over: Record<string, string> = {}): Record<string, string> => ({ AR_BUILD_KIND: 'store', AR_STORE_ARCHIVE: '1', EXACT_VERSION: '1.0', EXACT_BUILD_NUMBER: '42',
    EXACT_NON_EXEMPT_ENCRYPTION: 'false', EXACT_IDENTITY: SHA, EXACT_PROFILE: profile, ...over });
  test('G1 exact.sh runs no archive: --archive in any spelling, for any identity and any environment, even a perfect store one (one route: archive-store.sh)', () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      const out = join(tr.t, 'o.ipa');
      const spellings = [['ios', '--device', '--archive', out, '--json'], ['ios', '--device', '--archive', out], ['ios', '--device', `--archive=${out}`], ['ios', '--archive', out], ['mac', '--archive', out],
        ['--archive', out], ['test', 'ios', '--archive', out], ['ios', '--device', '--archive', ''], ['ios', '--archive']];
      for (const env of [{}, { AR_VARIANT: 'lab' }, { AR_VARIANT: 'lab-one' }, STORE_ENV(tr, profile), { ...STORE_ENV(tr, profile), AR_VARIANT: 'lab' }])
        for (const args of spellings) refused(tr, args, env, 'archive-store.sh');
      refused(tr, ['ios', '--device', '--archive', out], { ...STORE_ENV(tr, profile), AUTORADIO_BUILD_TREE_FILE: join(tr.t, 'nowhere') }, 'archive-store.sh');
    } finally { tr.cleanup(); }
  });
  test('G2 an empty first argument is refused (not a verb that runs with any arguments), and help runs only with no argument after it', () => {
    const tr = tree();
    try {
      refused(tr, ['', 'ios', '--device', '--phone', 'X'], {}, 'empty');
      refused(tr, [''], {}, 'empty');
      refused(tr, ['', 'ios'], { AR_VARIANT: 'lab' }, 'empty');
      for (const v of ['help', '--help', '-h']) { allowed(tr, [v]); refused(tr, [v, '--device', '--phone', 'X'], {}, v); refused(tr, [v, 'x'], {}); }
      allowed(tr, []);
    } finally { tr.cleanup(); }
  });
  test('A3 a development app chosen: a phone build runs in the build tree; an archive and share are refused, in either flag spelling', () => {
    const tr = tree();
    try {
      for (const v of ['lab', 'lab-one']) {
        for (const args of [['ios', '--device', '--phone', 'X'], ['ios', '--device', '--run', '--phone', 'X'], ['agent', 'ios', '--device'], ['ios'], ['test', 'web'], ['contract', 'build']]) allowed(tr, args, { AR_VARIANT: v });
        for (const args of [['share'], ['ios', '--device', '--archive', '/tmp/x.ipa'], ['ios', '--device', '--archive=/tmp/x.ipa'], ['ios', '--archive', '/tmp/x.ipa']]) {
          refused(tr, args, { AR_VARIANT: v }, args[0] === 'share' ? 'share' : 'archive-store.sh');
          refused(tr, args, { AR_VARIANT: v, AR_BUILD_KIND: 'store', AR_STORE_ARCHIVE: '1' }, args[0] === 'share' ? 'share' : 'archive-store.sh');
        }
        refused(tr, ['ios', '--device', '--phone', 'X'], { AR_VARIANT: v, AUTORADIO_BUILD_TREE_FILE: join(tr.t, 'nowhere') }, 'build tree');
        refused(tr, ['agent', 'ios', '--device=1'], { AR_VARIANT: v, AUTORADIO_BUILD_TREE_FILE: join(tr.t, 'nowhere') }, 'build tree');
      }
      writeFileSync(join(tr.t, 'elsewhere'), `${tr.t}\n`);
      expect(ex(tr, ['ios'], { AUTORADIO_BUILD_TREE_FILE: join(tr.t, 'elsewhere') }).code).toBe(0);   // a simulator build needs no build tree here
    } finally { tr.cleanup(); }
  });
  test('A3 a variant given as --variant or EXACT_VARIANT is refused, naming AR_VARIANT, instead of being half applied', () => {
    const tr = tree();
    try {
      refused(tr, ['--variant', 'lab-one', 'ios'], {}, 'AR_VARIANT');
      refused(tr, ['ios', '--variant=lab-one'], {}, 'AR_VARIANT');
      refused(tr, ['ios'], { EXACT_VARIANT: 'lab-one' }, 'AR_VARIANT');
      allowed(tr, ['ios'], { EXACT_VARIANT: '' });                                         // an empty value names nothing
      allowed(tr, ['ios'], { AR_VARIANT: 'lab-one' });
      expect(seen(tr)).toContain('EXACT_VARIANT=[lab-one]'); expect(seen(tr)).toContain(`APP_ID=[${LABONE}]`);
    } finally { tr.cleanup(); }
  });
  test('A4 EXACT_DEV_MENU=0 reaches Exact for every build; a caller\'s other value is refused; nothing else of the caller\'s environment does', () => {
    const tr = tree();
    try {
      allowed(tr, ['ios']); expect(seen(tr)).toContain('EXACT_DEV_MENU=[0]');
      allowed(tr, ['agent', 'ios', 'tree'], { AR_VARIANT: 'lab' }); expect(seen(tr)).toContain('EXACT_DEV_MENU=[0]');
      allowed(tr, ['mac', '--run']); expect(seen(tr)).toContain('EXACT_DEV_MENU=[0]');
      allowed(tr, ['ios'], { EXACT_DEV_MENU: '0', SOME_OTHER: 'x' }); expect(seen(tr)).toContain('EXACT_DEV_MENU=[0]');
      for (const v of ['1', 'true', '', 'on']) refused(tr, ['ios'], { EXACT_DEV_MENU: v }, 'EXACT_DEV_MENU');
      allowed(tr, ['ios']);
      for (const v of ['EXACT_IDENTITY', 'EXACT_PROFILE', 'EXACT_VERSION', 'EXACT_BUILD_NUMBER', 'EXACT_NON_EXEMPT_ENCRYPTION']) expect(seen(tr)).toContain(`${v}=[unset]`);   // unset stays unset: a development build is as it was
      allowed(tr, ['ios', '--device', '--phone', 'X'], { AR_VARIANT: 'lab', EXACT_IDENTITY: SHA, EXACT_PROFILE: '/p', EXACT_VERSION: '2.0', EXACT_BUILD_NUMBER: '9' });   // a development build keeps its signing pair
      for (const line of [`EXACT_IDENTITY=[${SHA}]`, 'EXACT_PROFILE=[/p]', 'EXACT_VERSION=[2.0]', 'EXACT_BUILD_NUMBER=[9]']) expect(seen(tr)).toContain(line);
    } finally { tr.cleanup(); }
  });
});

describe.skipIf(!MAC)('round 3: the second review', () => {
  test('T1 the microphone string tells the truth about Take: it records, and a published take is uploaded', () => {
    const en = JSON.parse(readFileSync(join(MOBILE, 'AutoradioLab', 'strings', 'en.json'), 'utf8'));
    const t: string = en['purpose.microphone'];
    expect(t).toContain('Voice');
    expect(t).toMatch(/Take/);
    expect(t).toMatch(/records/);
    expect(t).toMatch(/publish/);
    expect(t).toMatch(/uploaded/);
    expect(t).toContain('text on this phone');
  });
  test('T2 build-lab.sh refuses an archive in its own words, with or without a development variant, before anything is staged', () => {
    const tr = tree();
    try {
      for (const v of [{}, { AR_VARIANT: 'lab' }, { AR_VARIANT: 'lab-one' }, { AR_APP_ID: LAB }]) {
        const r = run(['/bin/sh', join(tr.m, 'scripts', 'build-lab.sh'), 'ios', '--device', '--archive', join(tr.t, 'x.ipa')], tr.env({ AR_BUILD_KIND: 'store', ...v }));
        expect(r.code).not.toBe(0); expect(r.err).toContain('build-lab:'); expect(r.err).toContain('archive-store.sh');
        expect(existsSync(join(tr.bin, 'bun.last'))).toBe(false); expect(existsSync(join(tr.t, 'order.log'))).toBe(false);
      }
    } finally { tr.cleanup(); }
  });
  test('T3 upload-store.sh refuses a variant in its own words, and warns about a key reached through a symlink', () => {
    const tr = tree();
    try {
      writeSigning(tr);
      const keyfile = join(tr.t, 'AuthKey.p8'); writeFileSync(keyfile, 'k'); chmodSync(keyfile, 0o640);
      const link = join(tr.t, 'link.p8'); symlinkSync(keyfile, link);
      const good = { ASC_KEY_ID: KEY2, ASC_ISSUER_ID: ISS2, ASC_KEY_FILE: link };
      const ipa = makeIpa(tr, 'y.ipa');
      const up = (extra: Record<string, string> = {}) => run(['/bin/sh', join(tr.m, 'scripts', 'upload-store.sh'), ipa], tr.env({ ...good, ...extra }));
      const w = up();
      expect(w.code, w.err).toBe(0); expect(w.err).toContain('readable by others');
      for (const v of [{ AR_VARIANT: 'one' }, { AR_VARIANT: 'lab' }]) {
        const r = up(v);
        expect(r.code).not.toBe(0); expect(r.err).toContain('upload-store:'); expect(r.err).not.toContain('app-identity:');
      }
    } finally { tr.cleanup(); }
  });
  test('T4 a multi-line EXACT_IDENTITY or AR_BUILD_NUMBER is refused', () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      const a = archive(tr, { EXACT_IDENTITY: SHA + '\nnot hex', EXACT_PROFILE: profile });
      expect(a.code).not.toBe(0); expect(a.err).toContain('40'); expect(existsSync(join(tr.bin, 'bun.last'))).toBe(false);
      const b = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile, AR_BUILD_NUMBER: '5\n6' });
      expect(b.code).not.toBe(0); expect(b.err).toContain('AR_BUILD_NUMBER'); expect(existsSync(join(tr.bin, 'bun.last'))).toBe(false);
    } finally { tr.cleanup(); }
  });
});
const KEY2 = 'ZZKEYIDTWO', ISS2 = '99999999-2222-3333-4444-555555555555';

describe.skipIf(!MAC)('round 3: the profile, the ipa and the rest', () => {
  const keyfile = (tr: Tree) => { const k = join(tr.t, 'AuthKey.p8'); writeFileSync(k, 'k'); chmodSync(k, 0o600); return k; };
  const ipa = (tr: Tree, info: Record<string, unknown> = {}, name = 'z.ipa') => makeIpa(tr, name, { info: { ...info, __corrupt: undefined }, corrupt: !!info.__corrupt });
  const up = (tr: Tree, p: string) => run(['/bin/sh', join(tr.m, 'scripts', 'upload-store.sh'), p], tr.env({ ASC_KEY_ID: 'ZZKEYIDTHREE', ASC_ISSUER_ID: '77777777-2222-3333-4444-555555555555', ASC_KEY_FILE: keyfile(tr) }));
  test('B1 the embedded profile is held to the same rules as the one named for the archive: expiry, team and prefix, iOS; the ipa is app.json\'s version; a corrupt plist is a refusal', () => {
    const tr = tree();
    try {
      const refusedBy = (over: Parameters<typeof writeSigning>[1], words: string, info: Record<string, unknown> = {}) => {
        writeSigning(tr, over);
        const r = up(tr, ipa(tr, info));
        expect(r.code, words).not.toBe(0); expect(r.err, words).toContain(words); expect(r.err).not.toContain('Traceback');
        expect(existsSync(join(tr.t, 'xe.log')), 'altool must not run').toBe(false);
      };
      refusedBy({ expires: new Date(Date.now() - 86400e3) }, 'expired');
      refusedBy({ prefix: 'LEGACY9999', idPrefix: 'ABCDE12345' }, PROD);
      refusedBy({ idPrefix: '' }, PROD);
      refusedBy({ platform: ['macOS'] }, 'iOS');
      refusedBy({ platform: null }, 'iOS');
      refusedBy({ raw: '<?xml version="1.0"?><plist version="1.0"><array/></plist>' }, 'profile');
      refusedBy({ raw: 'not xml at all' }, 'profile');
      refusedBy({}, 'version', { CFBundleShortVersionString: '0.1.0' });
      writeSigning(tr, { prefix: 'LEGACY9999' }); writeCodesign(tr, { id: 'LEGACY9999.' + PROD });
      const ok = up(tr, ipa(tr)); expect(ok.code, ok.err).toBe(0);
      const bad = up(tr, ipa(tr, { __corrupt: true }, 'c.ipa'));
      expect(bad.code).not.toBe(0); expect(bad.err).toContain('Info.plist'); expect(bad.err).not.toContain('Traceback');
    } finally { tr.cleanup(); }
  });
  test('B1 check-store-signing.py: Platform, a corrupt or odd profile, a profile of the wrong shape is a plain refusal', () => {
    const tr = tree();
    try {
      for (const [over, words] of [[{ platform: ['macOS'] }, 'iOS'], [{ platform: null }, 'iOS'], [{ raw: '<?xml version="1.0"?><plist version="1.0"><array/></plist>' }, 'profile'], [{ raw: 'garbage' }, 'profile'],
        [{ raw: '<?xml version="1.0"?><plist version="1.0"><dict><key>TeamIdentifier</key><string>x</string><key>Entitlements</key><string>y</string></dict></plist>' }, 'EXACT_PROFILE']] as [Parameters<typeof writeSigning>[1], string][]) {
        const profile = writeSigning(tr, over);
        const r = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });
        expect(r.code, JSON.stringify(over)).not.toBe(0); expect(r.err).toContain(words); expect(r.err).not.toContain('Traceback');
      }
    } finally { tr.cleanup(); }
  });
  test('B2 a leading-zero last-build and a non-object JSON result line are plain refusals', () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      mkdirSync(join(tr.t, 'out'), { recursive: true });
      writeFileSync(join(tr.t, 'out', 'last-build'), '08\n');
      const a = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });
      expect(a.code).not.toBe(0); expect(a.err).toContain('last-build'); expect(a.err).not.toContain('Traceback'); expect(existsSync(join(tr.bin, 'bun.last'))).toBe(false);
      rmSync(join(tr.t, 'out', 'last-build'));
      writeFileSync(join(tr.bin, 'mode'), 'json-null');
      const b = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });
      expect(b.code).not.toBe(0); expect(b.err).toContain('archive-store:'); expect(b.err).not.toContain('Traceback');
    } finally { tr.cleanup(); }
  });
  test('B2 the readback prints the build commit and says the build machine\'s host name is in the ipa (Exact offers no way to leave it out)', () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      writeFileSync(join(tr.bin, 'mode'), 'with-host');
      const r = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });
      expect(r.code, r.err).toBe(0);
      expect(r.out).toContain('ExactBuildHost: present'); expect(r.out).not.toContain('secret-host');       // the key is named, the value is not printed
      expect(r.out).toContain('ExactAppCommit: 0123456789ab (clean)');
    } finally { tr.cleanup(); }
  });
});

// ---- round 4: the artifact is judged, the route is one ----------------------------------------------------------------------------------------------------
// scripts/store-dev-names.txt: one name a store binary must not hold per line, "staged <name>" (the store kind of sync-exact-package.sh leaves it out of the staged sources) or
// "compiled <name>" (the source holds it behind a build condition Exact leaves undefined for an archive).
const NAMES = readFileSync(join(MOBILE, 'scripts', 'store-dev-names.txt'), 'utf8').split('\n').map((l) => l.trim()).filter((l) => l && !l.startsWith('#'))
  .map((l) => { const [kind, name] = l.split(/\s+/); return { kind, name }; });
const keyfileFor = (tr: Tree) => { const k = join(tr.t, 'AuthKey.p8'); writeFileSync(k, 'k'); chmodSync(k, 0o600); return k; };
const uploadOf = (tr: Tree, ipa: string, extra: Record<string, string> = {}) => run(['/bin/sh', join(tr.m, 'scripts', 'upload-store.sh'), ipa],
  tr.env({ ASC_KEY_ID: 'ZZKEYIDFOUR', ASC_ISSUER_ID: '66666666-2222-3333-4444-555555555555', ASC_KEY_FILE: keyfileFor(tr), ...extra }));

describe.skipIf(!MAC)('D1 archive-store.sh stages the store kind itself, every time, immediately before Exact reads the tree', () => {
  test('D1 a tree last staged for development (the launch hooks in it) is restaged: the "compiled" app holds none of them, and the order is sync(store) then Exact', () => {
    const tr = tree({ realStage: true });
    try {
      const profile = writeSigning(tr);
      // the development staging, as any development build leaves it
      const dev = run(['/bin/sh', join(tr.m, 'scripts', 'build-lab.sh'), 'ios'], tr.env({ ALLOW_NO_MODELS: '' }));
      expect(dev.code, dev.err).toBe(0);
      const staged = readdirSync(join(tr.m, 'AutoradioLab', 'modules', 'apple'));
      expect(staged).toContain('Models_EmbedDeviceTrial.swift'); expect(staged).toContain('Models_ModelProofLaunch.swift');
      rmSync(join(tr.t, 'order.log'), { force: true });
      const r = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });
      expect(r.code, r.err + r.out).toBe(0);                                              // the ipa passed check-store-ipa.py: no listed name in its executable
      expect(readFileSync(join(tr.t, 'order.log'), 'utf8').trim().split('\n')).toEqual(['sync kind=store', 'exact']);
      const seen = readFileSync(join(tr.bin, 'bun.last'), 'utf8');
      expect(seen).toMatch(/staged=\[[^\]]*Models_ModelHub\.swift/);
      expect(seen).not.toContain('EmbedDeviceTrial'); expect(seen).not.toContain('ModelProofLaunch');   // the staged tree Exact read held neither hook file
      // and the hooks come back for the next development build
      const again = run(['/bin/sh', join(tr.m, 'scripts', 'build-lab.sh'), 'ios'], tr.env({ ALLOW_NO_MODELS: '' }));
      expect(again.code, again.err).toBe(0);
      expect(readdirSync(join(tr.m, 'AutoradioLab', 'modules', 'apple'))).toContain('Models_EmbedDeviceTrial.swift');
    } finally { tr.cleanup(); }
  });
  test('D1 the same archive from a tree that Exact would compile with the hooks is refused after the build (the readback judges the executable, whatever staged it)', () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      writeFileSync(join(tr.bin, 'exe-extra'), 'x EmbedDeviceTrial x');
      const r = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });
      expect(r.code).not.toBe(0); expect(r.err).toContain('EmbedDeviceTrial'); expect(r.err).toContain('archive-store:');
    } finally { tr.cleanup(); }
  });
});

describe.skipIf(!MAC)('D2 check-store-ipa.py judges the executable inside the ipa', () => {
  test('D2 every name on the list refuses the ipa, in archive-store.sh and in upload-store.sh; a clean executable and the same names beside it (in Info.plist, in the file names) pass', () => {
    expect(NAMES.length).toBeGreaterThanOrEqual(5);
    const tr = tree();
    try {
      writeSigning(tr);
      // all the names at once, in three shapes (a refusal names every name it finds, so each name must be in the message: a name the checker did not search is missing from it)
      for (const shape of [(n: string) => `-${n}-`, (n: string) => `prefix-${n}`, (n: string) => `$s7Autoradio${n.length}${n}V`]) {
        const r = uploadOf(tr, makeIpa(tr, 'bad.ipa', { exe: 'MZ' + NAMES.map((n) => shape(n.name)).join(' ') + ' tail' }));
        expect(r.code).not.toBe(0); expect(r.err).toContain('upload-store:');
        for (const { name } of NAMES) expect(r.err, name).toContain(name);
        expect(existsSync(join(tr.t, 'xe.log')), 'altool must not run').toBe(false);
      }
      const profile = writeSigning(tr);
      writeFileSync(join(tr.bin, 'exe-extra'), 'compiled with ' + NAMES.map((n) => n.name).join(' '));
      const a = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });
      expect(a.code).not.toBe(0); expect(a.err).toContain('archive-store:'); for (const { name } of NAMES) expect(a.err, name).toContain(name);
      rmSync(join(tr.bin, 'exe-extra'));
      const clean = uploadOf(tr, makeIpa(tr, 'ok.ipa', { exe: 'MZ nothing of a developer in here' }));
      expect(clean.code, clean.err).toBe(0);
      const okWithNamesElsewhere = uploadOf(tr, makeIpa(tr, 'ok2.ipa', { exe: 'MZ clean', info: { SomeKey: NAMES.map((n) => n.name).join(' ') } }));
      expect(okWithNamesElsewhere.code, okWithNamesElsewhere.err).toBe(0);                // only the executable is searched
    } finally { tr.cleanup(); }
  });
  test('D2 an ipa with no executable (none named, none in the archive, an empty one) is refused', () => {
    const tr = tree();
    try {
      writeSigning(tr);
      for (const [o, words] of [[{ exe: null }, 'executable'], [{ info: { CFBundleExecutable: null } }, 'executable'], [{ exe: '' }, 'executable'], [{ info: { CFBundleExecutable: '../A' } }, 'executable']] as [Parameters<typeof makeIpa>[2], string][]) {
        const r = uploadOf(tr, makeIpa(tr, 'n.ipa', o));
        expect(r.code, JSON.stringify(o)).not.toBe(0); expect(r.err).toContain(words); expect(r.err).not.toContain('Traceback');
      }
    } finally { tr.cleanup(); }
  });
});

describe('D2b check-store-ipa.py --executable <file> (the lead\'s proof on a development app\'s executable)', () => {
  test('D2b a file holding a listed name is refused, naming it; a file without one is accepted; an unreadable file is refused', () => {
    const tr = tree();
    try {
      const run1 = (f: string) => run(['/usr/bin/python3', '-I', join(MOBILE, 'scripts', 'check-store-ipa.py'), '--executable', f], BASE);
      for (const { name } of NAMES) {
        const f = join(tr.t, 'exe-' + name); writeFileSync(f, Buffer.concat([Buffer.from([0xcf, 0xfa, 0xed, 0xfe, 0, 0]), Buffer.from('xx' + name + 'yy')]));
        const r = run1(f); expect(r.code, name).not.toBe(0); expect(r.err).toContain(name);
      }
      const clean = join(tr.t, 'exe-clean'); writeFileSync(clean, Buffer.from([0xcf, 0xfa, 0xed, 0xfe, 1, 2, 3]));
      expect(run1(clean).code).toBe(0);
      const r = run1(join(tr.t, 'missing')); expect(r.code).not.toBe(0); expect(r.err).not.toContain('Traceback');
    } finally { tr.cleanup(); }
  });
});

describe.skipIf(!MAC)('D3 check-store-ipa.py reads the signature, not only the embedded profile', () => {
  test('D3 get-task-allow in the signature, another application-identifier, a development authority, a broken seal, a codesign that cannot read the app are each a plain refusal; a store signature passes', () => {
    const tr = tree();
    try {
      writeSigning(tr);
      const ipa = makeIpa(tr, 'sig.ipa');
      const refusedBy = (over: Parameters<typeof writeCodesign>[1], words: string) => {
        writeCodesign(tr, over);
        const r = uploadOf(tr, ipa);
        expect(r.code, JSON.stringify(over)).not.toBe(0); expect(r.err).toContain(words); expect(r.err).not.toContain('Traceback');
        expect(existsSync(join(tr.t, 'xe.log'))).toBe(false);
      };
      refusedBy({ taskAllow: true }, 'get-task-allow');
      refusedBy({ id: 'ABCDE12345.' + LAB }, PROD);
      refusedBy({ id: 'OTHERTEAM1.' + PROD }, 'profile');                                  // not the application-identifier the embedded profile grants
      refusedBy({ id: PROD }, PROD);
      refusedBy({ authority: 'Apple Development: Someone (ABCDE12345)' }, 'Apple Distribution');
      refusedBy({ verify: 1 }, 'signature');
      writeCodesign(tr, { taskAllow: null });                                               // an App Store signature may leave get-task-allow out
      let r = uploadOf(tr, ipa); expect(r.code, r.err).toBe(0);
      writeCodesign(tr);
      writeFileSync(join(tr.bin, 'ents.plist'), 'not a plist');
      r = uploadOf(tr, ipa); expect(r.code).not.toBe(0); expect(r.err).toContain('entitlements'); expect(r.err).not.toContain('Traceback');
      writeCodesign(tr);
      writeFileSync(join(tr.bin, 'codesign'), '#!/bin/sh\nexit 1\n');                       // codesign cannot read the app
      r = uploadOf(tr, ipa); expect(r.code).not.toBe(0); expect(r.err).toContain('codesign'); expect(r.err).not.toContain('Traceback');
    } finally { tr.cleanup(); }
  });
});

describe('D4 the list of names cannot rot', () => {
  const sources = (dir: string) => readdirSync(dir, { recursive: true, withFileTypes: false }).map(String).filter((f) => f.endsWith('.swift')).map((f) => readFileSync(join(dir, f), 'utf8')).join('\n');
  test('D4 every "staged" name is in the development staging and in no file of the store staging; every "compiled" name is still in the sources', () => {
    const stage = (kind: string | null) => {
      const t = mkdtempSync(join(tmpdir(), 'dev-names-')); const m = join(t, 'mobile');
      mkdirSync(join(m, 'AutoradioKit', 'Sources', 'AutoradioKit'), { recursive: true }); mkdirSync(join(m, 'AutoradioLab', 'modules', 'apple'), { recursive: true });
      mkdirSync(join(m, 'exact-package', 'apple'), { recursive: true }); mkdirSync(join(m, '.deps', 'xcframeworks', 'FluidAudio.xcframework'), { recursive: true });
      cpSync(join(MOBILE, 'scripts'), join(m, 'scripts'), { recursive: true });
      cpSync(join(MOBILE, 'AutoradioKit', 'Sources', 'AutoradioModels'), join(m, 'AutoradioKit', 'Sources', 'AutoradioModels'), { recursive: true });
      writeFileSync(join(m, 'scripts', 'gen-build-info.sh'), '#!/bin/sh\nexit 0\n');
      const r = run(['/bin/sh', join(m, 'scripts', 'sync-exact-package.sh')], { ...BASE, ...(kind ? { AR_BUILD_KIND: kind } : {}) });
      expect(r.code, r.err).toBe(0);
      const dir = join(m, 'AutoradioLab', 'modules', 'apple');
      const text = readdirSync(dir).join('\n') + '\n' + sources(dir);
      rmSync(t, { recursive: true, force: true });
      return text;
    };
    const dev = stage(null), store = stage('store');
    const kit = sources(join(MOBILE, 'AutoradioKit', 'Sources', 'AutoradioKit'));            // staged whole, in both kinds
    for (const { kind, name } of NAMES) {
      expect(['staged', 'compiled'], name).toContain(kind);
      if (kind === 'staged') {
        expect(dev, name + ' is no longer in the development staging: renamed? update scripts/store-dev-names.txt').toContain(name);
        expect(store, name + ' is in the store staging: the store kind must leave it out').not.toContain(name);
        expect(kit, name + ' is in the Kit, which both kinds stage').not.toContain(name);
      } else {
        expect(kit + dev, name + ' is no longer in the sources: renamed? update scripts/store-dev-names.txt').toContain(name);
      }
    }
    expect(NAMES.map((n) => n.name)).toContain('EmbedDeviceTrial');
    expect(NAMES.map((n) => n.name)).toContain('ModelProofStages');
  });
});

describe('D5 the identity file is not replaceable for a store script', () => {
  test('D5 AR_APP_JSON is refused by archive-store.sh and upload-store.sh, in their own words, before anything runs', () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      const fake = join(tr.t, 'fake-app.json'); writeFileSync(fake, readFileSync(join(MOBILE, 'AutoradioLab', 'app.json'), 'utf8').replace('"version": "1.0"', '"version": "0.1.0"'));
      const a = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile, AR_APP_JSON: fake });
      expect(a.code).not.toBe(0); expect(a.err).toContain('archive-store:'); expect(a.err).toContain('AR_APP_JSON'); expect(existsSync(join(tr.bin, 'bun.last'))).toBe(false);
      const u = uploadOf(tr, makeIpa(tr, 'v.ipa', { info: { CFBundleShortVersionString: '0.1.0' } }), { AR_APP_JSON: fake });
      expect(u.code).not.toBe(0); expect(u.err).toContain('upload-store:'); expect(u.err).toContain('AR_APP_JSON'); expect(existsSync(join(tr.t, 'xe.log'))).toBe(false);
    } finally { tr.cleanup(); }
  });
});

describe.skipIf(!MAC)('D6 a build number is given once', () => {
  test('D6 two runs at the same moment get two numbers; a record that was lost does not give a number again; an override already given is refused', async () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      const e = tr.env({ EXACT_IDENTITY: SHA, EXACT_PROFILE: profile }); delete e.ALLOW_NO_MODELS;
      const go = () => Bun.spawn(['/bin/sh', join(tr.m, 'scripts', 'archive-store.sh')], { env: e, stdout: 'pipe', stderr: 'pipe', timeout: CHILD_MS, killSignal: 'SIGKILL' });
      const procs = [go(), go(), go()];
      const outs = await Promise.all(procs.map(async (p) => { const code = await p.exited; return { code, out: await new Response(p.stdout).text(), err: await new Response(p.stderr).text() }; }));
      for (const o of outs) expect(o.code, o.err).toBe(0);
      const nums = outs.map((o) => Number(/build number: (\d+)/.exec(o.out)![1]));
      expect(new Set(nums).size).toBe(3);
      rmSync(join(tr.t, 'out', 'last-build'));                                              // the record is lost: the claims still stand
      const again = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });
      expect(again.code, again.err).toBe(0);
      expect(nums).not.toContain(Number(/build number: (\d+)/.exec(again.out)![1]));
      const dup = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile, AR_BUILD_NUMBER: String(nums[0]) });
      expect(dup.code).not.toBe(0); expect(dup.err).toContain('already'); expect(dup.err).toContain('archive-store:');
    } finally { tr.cleanup(); }
  });
});

describe('D7 what the app grants is data the build reads', () => {
  test('D7 the purpose strings are the two the app uses (the sign-in strings are the app.signin ones), and the background mode is playback only', () => {
    const en = JSON.parse(readFileSync(join(MOBILE, 'AutoradioLab', 'strings', 'en.json'), 'utf8'));
    expect(Object.keys(en).filter((k) => k.startsWith('purpose.')).sort()).toEqual(['purpose.microphone', 'purpose.speech']);
    expect(APP_JSON.host.ios.backgroundModes).toEqual(['audio']);
  });
});

// ---- round 5: every Mach-O of the bundle, the system's own tools -------------------------------------------------------------------------------------------
// Failure list (docs/changes.md; each line is a test below):
//  F1 the hooks are searched for in CFBundleExecutable only: Exact links the app module into libexact_modules.dylib (on iOS production, Frameworks/ExactModules.framework/ExactModules) and package
//     Swift into frameworks, so a name that is only in one of those passes. Every Mach-O in the bundle is searched (the main executable, Frameworks/, PlugIns/, any .dylib).
//  F2 two .app directories in Payload, a symlink that leaves the bundle, a duplicate zip entry: what is searched is not what is signed.
//  F3 codesign, ditto and security are found on the caller's PATH: a stub there fools the signature and the profile checks.
//  F4 the codesign authority is only a prefix match (Apple Distribution Test); the signed TeamIdentifier is not the profile's team.
//  F5 a malformed line of store-dev-names.txt silently drops a name; a store-staged file nobody listed; a "compiled" name used outside a build condition; ProofKey unlisted.
//  F6 archive-store.sh spins on an unwritable claims directory, claims a number before the staging that can fail, lets last-build go backwards.
//  F7 exact.sh splits an EXACT_SIM with spaces; checks the pin of the caller's EXACT2 and not the Exact it runs; ea_only treats an argument as a glob; --phone alone is not a phone build.
const MACHO = Buffer.from([0xcf, 0xfa, 0xed, 0xfe, 0x0c, 0x00, 0x00, 0x01]);
// Entries added to an ipa (zip) after makeIpa: files (path -> bytes), symlinks (path -> target).
function addToIpa(ipa: string, files: Record<string, Buffer | string> = {}, links: Record<string, string> = {}) {
  const spec = JSON.stringify({ files: Object.fromEntries(Object.entries(files).map(([k, v]) => [k, Buffer.from(v).toString('base64')])), links });
  const r = run(['/usr/bin/python3', '-I', '-c', 'import json,sys,base64,zipfile\nd=json.loads(sys.argv[2])\nz=zipfile.ZipFile(sys.argv[1],"a")\nfor k,v in d["files"].items(): z.writestr(k,base64.b64decode(v))\nfor k,v in d["links"].items():\n    i=zipfile.ZipInfo(k); i.create_system=3; i.external_attr=(0o120755<<16); z.writestr(i,v)\nz.close()', ipa, spec], BASE);
  expect(r.code, r.err).toBe(0);
  return ipa;
}
const refuses = (r: R, words: string[], who = 'upload-store:') => { expect(r.code, r.out).not.toBe(0); expect(r.err).toContain(who); for (const w of words) expect(r.err).toContain(w); expect(r.err).not.toContain('Traceback'); };

describe.skipIf(!MAC)('F1 the names are searched in every Mach-O of the bundle', () => {
  test('F1 a listed name only in the module dylib, a framework binary, a plug-in or a loose dylib refuses the ipa, in upload-store.sh and in archive-store.sh', () => {
    const tr = tree();
    try {
      writeSigning(tr);
      const places = ['Frameworks/ExactModules.framework/ExactModules', 'libexact_modules.dylib', 'Frameworks/ExactPkg_autoradio.framework/ExactPkg_autoradio', 'PlugIns/Share.appex/Share', 'Frameworks/libextra.dylib'];
      for (const where of places) {                                                         // every name at once, in each place: the message must name them all
        const ipa = addToIpa(makeIpa(tr, 'f1.ipa', { exe: 'MZ clean' }), { ['Payload/A.app/' + where]: Buffer.concat([MACHO, Buffer.from(NAMES.map((n) => '..' + n.name + '..').join(' '))]) });
        const r = uploadOf(tr, ipa);
        refuses(r, NAMES.map((n) => n.name)); expect(r.err).toContain(where.split('/').pop()!);
        expect(existsSync(join(tr.t, 'xe.log')), 'altool must not run').toBe(false);
      }
      // a dylib that carries no Mach-O magic is still searched by its name; a plain resource file that holds a listed name (a string table, a script) is not a binary and passes
      refuses(uploadOf(tr, addToIpa(makeIpa(tr, 'f1b.ipa', { exe: 'MZ clean' }), { 'Payload/A.app/Frameworks/libodd.dylib': 'odd ' + NAMES[0].name })), [NAMES[0].name]);
      const ok = uploadOf(tr, addToIpa(makeIpa(tr, 'f1c.ipa', { exe: 'MZ clean' }), { 'Payload/A.app/strings.json': NAMES.map((n) => n.name).join(' '), 'Payload/A.app/Frameworks/ExactModules.framework/ExactModules': Buffer.concat([MACHO, Buffer.from('product code only')]) }));
      expect(ok.code, ok.err).toBe(0);
      // archive-store.sh: the .ipa Exact wrote holds the name in a framework only
      const profile = writeSigning(tr);
      writeFileSync(join(tr.bin, 'framework-extra'), 'compiled with ' + NAMES[1].name);
      const a = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });
      refuses(a, [NAMES[1].name], 'archive-store:');
    } finally { tr.cleanup(); }
  });
});

describe.skipIf(!MAC)('F2 what is searched is what is signed', () => {
  test('F2 two apps in Payload, a symlink out of the bundle, a duplicate entry are each refused', () => {
    const tr = tree();
    try {
      writeSigning(tr);
      refuses(uploadOf(tr, addToIpa(makeIpa(tr, 'two.ipa'), { 'Payload/B.app/Info.plist': '<plist/>' })), ['Payload']);
      refuses(uploadOf(tr, addToIpa(makeIpa(tr, 'link.ipa'), {}, { 'Payload/A.app/Frameworks/out': '../../../../../../etc/hosts' })), ['symlink']);
      refuses(uploadOf(tr, addToIpa(makeIpa(tr, 'abs.ipa'), {}, { 'Payload/A.app/Frameworks/out': '/usr/bin/true' })), ['symlink']);
      refuses(uploadOf(tr, addToIpa(makeIpa(tr, 'dup.ipa'), { 'Payload/A.app/A': 'a second executable of the same name' })), ['twice']);
      const inside = uploadOf(tr, addToIpa(makeIpa(tr, 'in.ipa'), { 'Payload/A.app/Frameworks/X.framework/X': Buffer.concat([MACHO, Buffer.from('x')]) }, { 'Payload/A.app/Frameworks/X.framework/Current': 'X' }));
      expect(inside.code, inside.err).toBe(0);                                              // a link that stays inside the bundle is not followed and is not a defect
    } finally { tr.cleanup(); }
  });
});

describe.skipIf(!MAC)('F3 the tools are the system\'s, not the caller\'s PATH', () => {
  test('F3 stubs of codesign, ditto and security on PATH are ignored by upload-store.sh and archive-store.sh (the system\'s security refuses the fixture\'s profile and identity)', () => {
    const tr = tree({ systemTools: true });
    try {
      const profile = writeSigning(tr);
      for (const t of ['ditto', 'unzip']) { writeFileSync(join(tr.bin, t), '#!/bin/sh\nexit 0\n'); chmodSync(join(tr.bin, t), 0o755); }
      const r = uploadOf(tr, makeIpa(tr, 'fixture.ipa'));
      expect(r.code, 'a fixture ipa is no signed app: the real tools refuse it').not.toBe(0); expect(r.err).toContain('upload-store:'); expect(r.err).not.toContain('Traceback');
      expect(existsSync(join(tr.t, 'xe.log'))).toBe(false);
      const a = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });          // the real `security` finds no such identity: the stub that says it does is not asked
      expect(a.code).not.toBe(0); expect(a.err).toContain('archive-store:'); expect(a.err).toContain('identity'); expect(existsSync(join(tr.bin, 'bun.last'))).toBe(false);
    } finally { tr.cleanup(); }
  });
  test('F3 with only a valid-profile security given, the REAL ditto and codesign judge the fixture, whatever stubs of them are on PATH', () => {
    const tr = tree();
    try {
      writeSigning(tr);
      const only = join(tr.t, 'only-security'); mkdirSync(only);
      cpSync(join(tr.bin, 'security'), join(only, 'security'));
      for (const t of ['codesign', 'ditto']) { const f = join(tr.bin, t); writeFileSync(f, '#!/bin/sh\ntouch "' + tr.t + '/' + t + '.used"\nexit 0\n'); chmodSync(f, 0o755); }
      const ipa = makeIpa(tr, 'real-tools.ipa');
      const r = run(['/usr/bin/python3', '-I', join(MOBILE, 'scripts', 'check-store-ipa.py'), '--tools', only, 'direct', ipa, PROD, '1.0'], tr.env());
      expect(r.code).not.toBe(0);
      expect(r.err, 'the real ditto unpacked it and the real codesign found it unsigned').toContain('signature does not verify');
      expect(existsSync(join(tr.t, 'codesign.used')) || existsSync(join(tr.t, 'ditto.used')), 'a stub on PATH was run').toBe(false);
    } finally { tr.cleanup(); }
  });
  test('F3 the first thing a store script does is not resolved from PATH either: a dirname on PATH changes nothing', () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      writeFileSync(join(tr.bin, 'dirname'), '#!/bin/sh\ntouch "' + tr.t + '/dirname.used"\necho /nonexistent\n'); chmodSync(join(tr.bin, 'dirname'), 0o755);
      const u = uploadOf(tr, makeIpa(tr, 'd.ipa')); expect(u.code, u.err).toBe(0);
      const a = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile }); expect(a.code, a.err).toBe(0);
      expect(existsSync(join(tr.t, 'dirname.used')), 'the caller\'s dirname ran').toBe(false);
    } finally { tr.cleanup(); }
  });
  test('F3 the checker takes a tools directory only as an explicit argument (a test\'s; the two store scripts never pass one): with it the stubs judge, without it the real tools do', () => {
    const tr = tree();
    try {
      writeSigning(tr);
      const ipa = makeIpa(tr, 'direct.ipa');
      const direct = (tools: string[]) => run(['/usr/bin/python3', '-I', join(MOBILE, 'scripts', 'check-store-ipa.py'), ...tools, 'direct', ipa, PROD, '1.0'], BASE);
      expect(direct([]).code).not.toBe(0);                                                  // no tools directory: the system's tools, which refuse a fixture
      const r = direct(['--tools', tr.bin]); expect(r.code, r.err).toBe(0);
    } finally { tr.cleanup(); }
  });
});

describe.skipIf(!MAC)('F4 the signature names Apple Distribution and the profile\'s team', () => {
  test('F4 "Apple Distribution Test" and a TeamIdentifier other than the profile\'s are refused; "Apple Distribution: Name (TEAM)" passes', () => {
    const tr = tree();
    try {
      writeSigning(tr);
      const ipa = makeIpa(tr, 'sig.ipa');
      writeCodesign(tr, { authority: 'Apple Distribution Test' });
      refuses(uploadOf(tr, ipa), ['Apple Distribution']);
      writeCodesign(tr, { team: 'OTHERTEAM1' });
      refuses(uploadOf(tr, ipa), ['team']);
      writeCodesign(tr);
      expect(uploadOf(tr, ipa).code).toBe(0);
    } finally { tr.cleanup(); }
  });
});

describe.skipIf(!MAC)('F5 the list of names is parsed strictly and covers what is staged and what is compiled out', () => {
  test('F5 a line that is not "<kind> <name>" makes the checker refuse everything, instead of dropping the name', () => {
    const tr = tree();
    try {
      writeSigning(tr);
      const f = join(tr.m, 'scripts', 'store-dev-names.txt');
      for (const bad of ['staged EmbedDeviceTrial   # the launch hook', 'staged', 'sometimes EmbedDeviceTrial', 'staged A B']) {
        writeFileSync(f, readFileSync(join(MOBILE, 'scripts', 'store-dev-names.txt'), 'utf8') + bad + '\n');
        const r = uploadOf(tr, makeIpa(tr, 'strict.ipa'));
        refuses(r, ['store-dev-names.txt'], '');
      }
    } finally { tr.cleanup(); }
  });
  test('F5 every line of the real list is "<staged|compiled> <name>", and ProofKey and its file name are on it', () => {
    const lines = readFileSync(join(MOBILE, 'scripts', 'store-dev-names.txt'), 'utf8').split('\n').filter((l) => l.trim() && !l.trim().startsWith('#'));
    for (const l of lines) expect(l, l).toMatch(/^(staged|compiled) [A-Za-z0-9_.-]+$/);
    expect(lines).toContain('compiled ProofKey'); expect(lines).toContain('compiled proof-key');
  });
  test('F5 every file the development staging has and the store staging lacks is named by the list; every use of a "compiled" name is inside a build condition', () => {
    const files = (kind: string | null) => {
      const t = mkdtempSync(join(tmpdir(), 'dev-files-')); const m = join(t, 'mobile');
      mkdirSync(join(m, 'AutoradioKit', 'Sources', 'AutoradioKit'), { recursive: true }); mkdirSync(join(m, 'AutoradioLab', 'modules', 'apple'), { recursive: true });
      mkdirSync(join(m, 'exact-package', 'apple'), { recursive: true }); mkdirSync(join(m, '.deps', 'xcframeworks', 'FluidAudio.xcframework'), { recursive: true });
      cpSync(join(MOBILE, 'scripts'), join(m, 'scripts'), { recursive: true });
      cpSync(join(MOBILE, 'AutoradioKit', 'Sources', 'AutoradioModels'), join(m, 'AutoradioKit', 'Sources', 'AutoradioModels'), { recursive: true });
      writeFileSync(join(m, 'scripts', 'gen-build-info.sh'), '#!/bin/sh\nexit 0\n');
      const r = run(['/bin/sh', join(m, 'scripts', 'sync-exact-package.sh')], { ...BASE, ...(kind ? { AR_BUILD_KIND: kind } : {}) });
      expect(r.code, r.err).toBe(0);
      const dir = join(m, 'AutoradioLab', 'modules', 'apple');
      const out: Record<string, string> = {};
      for (const f of readdirSync(dir)) if (f.endsWith('.swift')) out[f] = readFileSync(join(dir, f), 'utf8');
      rmSync(t, { recursive: true, force: true });
      return out;
    };
    const dev = files(null), store = files('store');
    for (const f of Object.keys(dev)) {
      if (f in store && store[f] === dev[f]) continue;
      const names = NAMES.filter((n) => n.kind === 'staged').map((n) => n.name);
      expect(names.some((n) => dev[f].includes(n) || f.includes(n)), f + ' differs between the kinds but holds no listed name: add the name that gives it away to scripts/store-dev-names.txt').toBe(true);
    }
    // every use of a compiled name is in a branch a store archive does not compile (ungatedUses, below)
    const roots = [join(MOBILE, 'AutoradioKit', 'Sources'), join(MOBILE, 'AutoradioLab', 'modules', 'apple'), join(MOBILE, 'exact-package', 'apple')];
    const swift = roots.flatMap((r) => readdirSync(r, { recursive: true }).map(String).filter((f) => f.endsWith('.swift')).map((f) => join(r, f)));
    for (const { name } of NAMES.filter((n) => n.kind === 'compiled')) {
      let seen = 0;
      for (const f of swift) {
        const text = readFileSync(f, 'utf8');
        seen += text.split('\n').filter((l) => !l.trim().startsWith('//') && l.includes(name)).length;
        expect(ungatedUses(text, name), `${f}: "${name}" is used where a store archive compiles it`).toEqual([]);
      }
      expect(seen, name + ' is no longer used anywhere: renamed? update scripts/store-dev-names.txt').toBeGreaterThan(0);
    }
  });
});

// The lines of a Swift file that mention `name` in a branch a store archive (an iOS device build with no EXACT_DEVELOPMENT) COMPILES. A branch is development-only when its condition is
// EXACT_DEVELOPMENT, targetEnvironment(simulator) or os(macOS), joined by || (every operand must be one) or && (one operand is enough); `!cond` makes the #if branch not development-only
// and its #else branch so; `#else` of a plain development condition is not; a use is safe when ANY enclosing branch is development-only.
function ungatedUses(text: string, name: string): string[] {
  const dev = (c: string) => c.split('||').every((op) => op.split('&&').some((a) => /^(EXACT_DEVELOPMENT|targetEnvironment\(simulator\)|os\(macOS\))$/.test(a.trim())));
  const frames: { now: boolean; elseDev: boolean }[] = [];
  const out: string[] = [];
  for (const raw of text.split('\n')) {
    const t = raw.trim();
    if (t.startsWith('#if ')) { const c = t.slice(4).replace(/\s*\/\/.*$/, '').trim(); const neg = c.startsWith('!'); const d = dev(neg ? c.slice(1) : c); frames.push({ now: !neg && d, elseDev: neg && d }); }
    else if (t.startsWith('#elseif ')) { const c = t.slice(8).replace(/\s*\/\/.*$/, '').trim(); if (frames.length) { frames[frames.length - 1].now = !c.startsWith('!') && dev(c); frames[frames.length - 1].elseDev = false; } }
    else if (t === '#else' || t.startsWith('#else ')) { if (frames.length) frames[frames.length - 1].now = frames[frames.length - 1].elseDev; }
    else if (t.startsWith('#endif')) frames.pop();
    else if (!t.startsWith('//') && raw.includes(name) && !frames.some((f) => f.now)) out.push(t);
  }
  return out;
}
describe('F5 the build-condition scan reads conditions, not just their presence', () => {
  test('F5 negations, #else and #elseif, nesting and || are read as a store archive compiles them', () => {
    const u = (src: string) => ungatedUses(src, 'N');
    expect(u('#if EXACT_DEVELOPMENT\nN\n#endif')).toEqual([]);
    expect(u('#if !EXACT_DEVELOPMENT\nN\n#endif')).toEqual(['N']);
    expect(u('#if EXACT_DEVELOPMENT\nx\n#else\nN\n#endif')).toEqual(['N']);
    expect(u('#if !EXACT_DEVELOPMENT\nx\n#else\nN\n#endif')).toEqual([]);
    expect(u('#if A\nx\n#elseif EXACT_DEVELOPMENT\nN\n#endif')).toEqual([]);
    expect(u('#if EXACT_DEVELOPMENT\nx\n#elseif B\nN\n#endif')).toEqual(['N']);
    expect(u('#if EXACT_DEVELOPMENT\n#if B\nN\n#endif\n#endif')).toEqual([]);
    expect(u('#if B\n#if EXACT_DEVELOPMENT\nx\n#endif\nN\n#endif')).toEqual(['N']);
    expect(u('#if targetEnvironment(simulator) || os(macOS)\nN\n#endif')).toEqual([]);
    expect(u('#if EXACT_DEVELOPMENT || FOO\nN\n#endif')).toEqual(['N']);
    expect(u('#if EXACT_DEVELOPMENT && FOO\nN\n#endif')).toEqual([]);
    expect(u('// N in a comment\nlet x = 1')).toEqual([]);
    expect(u('let y = "N"')).toEqual(['let y = "N"']);
  });
});

describe.skipIf(!MAC)('F6 the build number claim', () => {
  test('F6 an unwritable claims directory is a refusal, not a loop; a staging that fails burns no number; an override raises the record for the default runs after it', () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      const out = join(tr.t, 'out'); mkdirSync(join(out, 'claims'), { recursive: true }); chmodSync(join(out, 'claims'), 0o555);
      const t0 = Date.now();
      let r = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });
      expect(r.code).not.toBe(0); expect(r.err).toContain('claim'); expect(Date.now() - t0).toBeLessThan(20000);
      chmodSync(join(out, 'claims'), 0o755);
      writeFileSync(join(tr.m, 'scripts', 'sync-exact-package.sh'), '#!/bin/sh\nexit 1\n');
      r = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile, AR_BUILD_NUMBER: '4242' });
      expect(r.code).not.toBe(0); expect(r.err).toContain('staging');
      expect(existsSync(join(out, 'claims', '4242')), 'a number must not be claimed before the staging that can fail').toBe(false);
      expect(existsSync(join(out, 'last-build'))).toBe(false);
    } finally { tr.cleanup(); }
  });
  test('F6 the record never goes down: a run that holds a lower number does not overwrite a higher record', () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      const out = join(tr.t, 'out'); mkdirSync(out, { recursive: true });
      writeFileSync(join(out, 'last-build'), '900000000\n');
      const r = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile, AR_BUILD_NUMBER: '5' });
      expect(r.code, r.err).toBe(0);
      expect(readFileSync(join(out, 'last-build'), 'utf8').trim()).toBe('900000000');
      const d = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });
      expect(d.code, d.err).toBe(0); expect(Number(/build number: (\d+)/.exec(d.out)![1])).toBe(900000001);
    } finally { tr.cleanup(); }
  });
});

describe('F7 the wrapper around Exact', () => {
  test('F7 EXACT_SIM with spaces reaches Exact as one value', () => {
    const tr = tree();
    try {
      const r = run(['/bin/sh', join(tr.m, 'scripts', 'exact.sh'), 'ios'], tr.env({ EXACT_SIM: 'iPhone 18 Pro' }));
      expect(r.code, r.err).toBe(0); expect(readFileSync(join(tr.bin, 'bun.last'), 'utf8')).toContain('EXACT_SIM=[iPhone 18 Pro]');
    } finally { tr.cleanup(); }
  });
  test('F7 the pin that is checked is the Exact that runs (mobile/.exact-src), not the caller\'s EXACT2', () => {
    const tr = tree();
    try {
      const other = join(tr.t, 'other-exact');
      run(['git', 'clone', '-q', join(tr.m, '.exact-src'), other], BASE);
      run(['git', '-C', join(tr.m, '.exact-src'), '-c', 'user.name=t', '-c', 'user.email=t@t', 'commit', '-q', '--allow-empty', '-m', 'moved'], BASE);   // the tree's own Exact is no longer at the pin
      const r = run(['/bin/sh', join(tr.m, 'scripts', 'exact.sh'), 'ios'], tr.env({ EXACT2: other }));
      expect(r.code).not.toBe(0); expect(r.err).toContain('pin'); expect(existsSync(join(tr.bin, 'bun.last'))).toBe(false);
    } finally { tr.cleanup(); }
  });
  test('F7 an argument is a word, not a pattern: --ru? is not --run; --phone alone is a phone build for a development app and runs only in the build tree', () => {
    const tr = tree();
    try {
      const ex = (args: string[], extra: Record<string, string> = {}) => run(['/bin/sh', join(tr.m, 'scripts', 'exact.sh'), ...args], tr.env(extra));
      let r = ex(['ios', '--ru?']); expect(r.code).not.toBe(0); expect(existsSync(join(tr.bin, 'bun.last'))).toBe(false);
      r = ex(['ios', '--r*']); expect(r.code).not.toBe(0);
      r = ex(['ios', '--run']); expect(r.code, r.err).toBe(0);
      writeFileSync(join(tr.t, 'tree'), '/nowhere\n');                                      // not the build tree
      r = ex(['agent', 'ios', '--phone', 'X'], { AR_VARIANT: 'lab' }); expect(r.code).not.toBe(0); expect(r.err).toContain('build tree');
    } finally { tr.cleanup(); }
  });
});

// ---- round 6: the negative control is a script, not a recipe ----------------------------------------------------------------------------------------------
//  N1 the lead has to reproduce exact-env.sh's environment by hand to build the development app the control needs, and can leave the tree development-staged.
//  N2 the control passes when the check does NOT refuse the development app (it must fail, saying the check is inert), or when it refuses for another reason.
//  N3 the control can upload, or keeps the development ipa.
const control = (tr: Tree, extra: Record<string, string> = {}) => { const e = tr.env(extra); if (!('ALLOW_NO_MODELS' in extra)) delete e.ALLOW_NO_MODELS; return run(['/bin/sh', join(tr.m, 'scripts', 'store-negative-control.sh')], e); };
describe.skipIf(!MAC)('N store-negative-control.sh', () => {
  test('N1 a development app built as the store app is, refused for a hook naming the file, then the store kind staged again; nothing uploaded; the ipa deleted', () => {
    const tr = tree({ realStage: true });
    try {
      const profile = writeSigning(tr);
      const r = control(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile, ASC_KEY_ID: 'ZZ', ASC_ISSUER_ID: 'ZZ', ASC_KEY_FILE: join(tr.t, 'none.p8') });
      expect(r.code, r.err + r.out).toBe(0);
      expect(r.out).toContain('REFUSED'); expect(r.out).toContain('holds developer-only names'); expect(r.out).toContain('Autoradio1 holds developer-only names');                                        // the file, inside the app
      for (const n of ['EmbedDeviceTrial', 'ModelProofStages']) expect(r.out).toContain(n);
      const order = readFileSync(join(tr.t, 'order.log'), 'utf8').trim().split('\n');
      expect(order.filter((l) => l.startsWith('sync') || l === 'exact')).toEqual(['sync kind=development', 'exact', 'sync kind=store']);       // development, Exact, then the store kind again
      expect(readFileSync(join(tr.bin, 'bun.last'), 'utf8')).toContain('EXACT_BUILD_NUMBER=[1]');
      expect(existsSync(join(tr.t, 'xe.log')), 'nothing may be uploaded').toBe(false);
      expect(readdirSync(join(tr.t, 'out', 'negative-control')).filter((f) => f.endsWith('.ipa'))).toEqual([]);
    } finally { tr.cleanup(); }
  });
  test('N2 a check that does not refuse the development app makes the control fail, saying so; so does a refusal for another reason', () => {
    const tr = tree();                                                                          // the stub staging stages no hook: the "development" app holds none of the names
    try {
      const profile = writeSigning(tr);
      let r = control(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });
      expect(r.code).not.toBe(0); expect(r.err).toContain('NOT REFUSED'); expect(r.err).toContain('Xcode');
      writeFileSync(join(tr.bin, 'mode'), 'wrong-id');                                          // refused, but not for a hook
      r = control(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });
      expect(r.code).not.toBe(0); expect(r.err).toContain('ANOTHER reason');
      expect(existsSync(join(tr.t, 'xe.log'))).toBe(false);
    } finally { tr.cleanup(); }
  });
  test('N3 the control refuses a variant, a missing identity and a missing profile before it stages or builds anything', () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      for (const extra of [{ AR_VARIANT: 'lab' }, {}, { EXACT_IDENTITY: SHA }, { EXACT_IDENTITY: SHA, EXACT_PROFILE: join(tr.t, 'nope') }, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile, ALLOW_NO_MODELS: '1' }]) {
        const r = control(tr, extra); expect(r.code, JSON.stringify(extra)).not.toBe(0); expect(r.err).toContain('store-negative-control:');
      }
      expect(existsSync(join(tr.t, 'order.log'))).toBe(false);
    } finally { tr.cleanup(); }
  });
});

describe.skipIf(!MAC)('N4 the check says when it is looking at little', () => {
  test('N4 a store ipa with only its main executable, or with no product name in any binary, passes with a warning that says so; one with the module framework and a product name does not warn', () => {
    const tr = tree();
    try {
      writeSigning(tr);
      let r = uploadOf(tr, makeIpa(tr, 'one.ipa', { exe: 'MZ clean' })); expect(r.code, r.err).toBe(0);
      expect(r.out).toContain('WARNING: only one Mach-O'); expect(r.out).toContain('WARNING: no searched file holds a product name');
      const ok = addToIpa(makeIpa(tr, 'two.ipa', { exe: 'MZ clean' }), { 'Payload/A.app/Frameworks/ExactModules.framework/ExactModules': Buffer.concat([MACHO, Buffer.from('ModelProof AutoradioModule')]) });
      r = uploadOf(tr, ok); expect(r.code, r.err).toBe(0);
      expect(r.out).not.toContain('WARNING'); expect(r.out).toContain('product code found in: Frameworks/ExactModules.framework/ExactModules');
    } finally { tr.cleanup(); }
  });
});
