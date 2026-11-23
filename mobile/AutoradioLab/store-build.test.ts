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
//  R9 docs still tell a builder to run a phone build with no variant, count four app names, or cite the retired variant `one`.
//  R10 a store build keeps -AutoradioProofSilent (a developer default) because it is not behind the development condition.
//  Review round 3 (the second reviewer's review, read after the first fixes):
//  T1 the microphone purpose string says the app listens only when Voice is on, but Take records the voice and a published take is uploaded.
//  T2 build-lab.sh takes AR_BUILD_KIND=store with a development variant (it must refuse itself, not only through exact.sh at the end).
//  T3 upload-store.sh checks the identity before it refuses a variant (the wrong message), and its key-file check does not follow a symlink.
//  T4 a multi-line EXACT_IDENTITY or AR_BUILD_NUMBER passes a per-line grep; archive-store.sh's comment says the build lock is taken by the script.
//  R11 check-store-signing.py refuses a valid App Store profile that omits get-task-allow, or one whose App ID prefix is not its team id (legacy prefix).
import { describe, expect, setDefaultTimeout, test } from 'bun:test';
import { createHash } from 'node:crypto';
import { cpSync, existsSync, symlinkSync, realpathSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync, chmodSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';

setDefaultTimeout(120000);          // each test runs shell scripts over a copy of the tree
const MOBILE = join(dirname(new URL(import.meta.url).pathname), '..');
const APP_JSON = JSON.parse(readFileSync(join(MOBILE, 'AutoradioLab', 'app.json'), 'utf8'));
const PROD = 'news.choir.autoradio', LAB = 'news.choir.autoradio.lab', LABONE = 'news.choir.autoradio.lab.one';

type R = { code: number; out: string; err: string };
const run = (cmd: string[], env: Record<string, string>, cwd?: string): R => {
  const p = Bun.spawnSync(cmd, { env, cwd, stdout: 'pipe', stderr: 'pipe' });
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
  test('I4 the sign-in scheme is the app id for all three apps, and the server lists all three', () => {
    const app = readFileSync(join(MOBILE, 'AutoradioLab', 'app.ts'), 'utf8');
    expect(/SIGNIN_SCHEME \?\? '([^']+)'/.exec(app)![1]).toBe(PROD);
    const schemes = [PROD];
    for (const v of Object.values(APP_JSON.variants) as { idSuffix: string; env: { SIGNIN_SCHEME: string } }[]) {
      expect(v.env.SIGNIN_SCHEME).toBe(PROD + v.idSuffix);
      schemes.push(v.env.SIGNIN_SCHEME);
    }
    expect(schemes.sort()).toEqual([LAB, LABONE, PROD].sort());
    // The Go list (internal/auth/device_login.go) is read here; the Go test TestDeviceReturnSchemesAreTheAppsInAppJSON checks it from the other side.
    const go = readFileSync(join(MOBILE, '..', 'internal', 'auth', 'device_login.go'), 'utf8');
    const listed = /var DeviceReturnSchemes = \[\]string\{([^}]*)\}/.exec(go)![1].match(/"[^"]+"/g)!.map((s) => s.slice(1, -1));
    for (const s of [PROD, LAB, LABONE]) expect(listed).toContain(s);                 // R5: the production id too, or the production app's sign-in sheet is never sent back
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

// ---- a copy of the tree over stubs ----------------------------------------------------------------------------------------------------------------------
type Tree = { t: string; m: string; bin: string; env: (extra?: Record<string, string>) => Record<string, string>; cleanup: () => void };
function tree(): Tree {
  const t = mkdtempSync(join(tmpdir(), 'store-build-'));
  const m = join(t, 'mobile'), bin = join(t, 'bin'), home = join(t, 'home');
  for (const d of [join(m, 'AutoradioLab'), bin, join(home, 'Library')]) mkdirSync(d, { recursive: true });
  cpSync(join(MOBILE, 'scripts'), join(m, 'scripts'), { recursive: true });
  cpSync(join(MOBILE, 'AutoradioLab', 'app.json'), join(m, 'AutoradioLab', 'app.json'));
  run(['git', 'init', '-q', join(m, '.exact-src')], BASE);
  run(['git', '-C', join(m, '.exact-src'), '-c', 'user.name=t', '-c', 'user.email=t@t', 'commit', '-q', '--allow-empty', '-m', 'pin'], BASE);
  writeFileSync(join(m, 'exact.pin'), run(['git', '-C', join(m, '.exact-src'), 'rev-parse', 'HEAD'], BASE).out);
  writeFileSync(join(m, 'scripts', 'sync-exact-package.sh'), '#!/bin/sh\nexit 0\n');
  writeFileSync(join(m, 'scripts', 'gen-exact-interface.py'), '');
  writeFileSync(join(m, 'scripts', 'gen-build-info.sh'), '#!/bin/sh\nexit 0\n');
  writeFileSync(join(m, 'scripts', 'ensure-deps.sh'), '#!/bin/sh\nexit 0\n');
  // xe.sh: records what it would run (the upload's arguments), runs nothing.
  writeFileSync(join(m, 'scripts', 'xe.sh'), `#!/bin/sh\nprintf '%s\\n' "$*" >> "${t}/xe.log"\nexit \${XE_EXIT:-0}\n`);
  // a Bun that stands for Exact: records the variables it was given and writes an .ipa as Exact's archive does (its Info.plist from those variables).
  writeFileSync(join(bin, 'bun'), `#!/bin/sh
D="${bin}"
[ "\${1:-}" = install ] && exit 0
{ printf 'args=[%s]\\n' "$*"; for v in EXACT_VARIANT SIGNIN_SCHEME EXACT_VERSION EXACT_BUILD_NUMBER EXACT_NON_EXEMPT_ENCRYPTION EXACT_DEV_MENU EXACT_IDENTITY EXACT_PROFILE EXACT_FEEDBACK; do eval "x=\\\${$v-unset}"; printf '%s=[%s]\\n' "$v" "$x"; done; } > "$D/bun.last"
out=""; prev=""; for a in "$@"; do [ "$prev" = --archive ] && out="$a"; prev="$a"; done
[ -n "$out" ] || exit 0
mode="$(cat "$D/mode" 2>/dev/null || echo ok)"
/usr/bin/python3 -I - "$out" "$mode" "$EXACT_VARIANT" <<'PY'
import plistlib, sys, os, zipfile, json
out, mode, variant = sys.argv[1], sys.argv[2], sys.argv[3]
base = "news.choir.autoradio"
ident = {"": base, "lab": base + ".lab", "lab-one": base + ".lab.one"}[variant]
info = {"CFBundleIdentifier": ident, "CFBundleDisplayName": "Autoradio 1", "CFBundleShortVersionString": os.environ.get("EXACT_VERSION", "0.1.0"),
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
os.makedirs(os.path.dirname(out), exist_ok=True)
with zipfile.ZipFile(out, "w") as z:
    z.writestr("Payload/Autoradio 1.app/Info.plist", plistlib.dumps(info))
    if mode != "no-profile": z.writestr("Payload/Autoradio 1.app/embedded.mobileprovision", b"x")
print(json.dumps({"path": out, "target": "ios", "trust": "production", "signing": os.environ.get("EXACT_IDENTITY", "ad-hoc")}))
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
  writeFileSync(join(t, 'tree'), `${realpathSync(m)}\n`);
  const env = (extra: Record<string, string> = {}) => ({ ...BASE, PATH: `${bin}:${BASE.PATH}`, HOME: home, BUN: join(bin, 'bun'), ALLOW_NO_MODELS: '1',
    AUTORADIO_BUILD_TREE_FILE: join(t, 'tree'), AR_STORE_OUT_DIR: join(t, 'out'), TMPDIR: t, ...extra });
  return { t, m, bin, env, cleanup: () => rmSync(t, { recursive: true, force: true }) };
}

// A profile as `security cms -D` prints it, and the identity list line, for a made-up certificate.
const CERT = Buffer.from('not a real certificate: the test only needs its hash');
const SHA = createHash('sha1').update(CERT).digest('hex').toUpperCase();
const OTHER_SHA = createHash('sha1').update('another').digest('hex').toUpperCase();
function writeSigning(tr: Tree, over: { id?: string; teamPrefix?: string; prefix?: string; idPrefix?: string; noPrefixKey?: boolean; noTaskAllow?: boolean; taskAllow?: boolean; devices?: boolean; all?: boolean; expires?: Date; cert?: Buffer; identityLine?: string } = {}) {
  const team = over.teamPrefix ?? 'ABCDE12345';
  const plist = (v: string) => v;
  const prefix = over.prefix ?? team, idPrefix = over.idPrefix ?? prefix;            // ApplicationIdentifierPrefix is the team id except for a legacy App ID
  const taskAllow = over.noTaskAllow ? '' : `<key>get-task-allow</key>${over.taskAllow ? '<true/>' : '<false/>'}`;
  const xml = `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict>
<key>Name</key><string>test store profile</string><key>TeamIdentifier</key><array><string>${team}</string></array>${over.noPrefixKey ? '' : `<key>ApplicationIdentifierPrefix</key><array><string>${prefix}</string></array>`}
<key>ExpirationDate</key><date>${(over.expires ?? new Date(Date.now() + 86400e3 * 300)).toISOString().replace(/\.\d+Z$/, 'Z')}</date>
${over.devices ? '<key>ProvisionedDevices</key><array><string>0000</string></array>' : ''}${over.all ? '<key>ProvisionsAllDevices</key><true/>' : ''}
<key>DeveloperCertificates</key><array><data>${(over.cert ?? CERT).toString('base64')}</data></array>
<key>Entitlements</key><dict><key>application-identifier</key><string>${idPrefix}.${over.id ?? PROD}</string>${taskAllow}
<key>com.apple.developer.team-identifier</key><string>${team}</string></dict></dict></plist>`;
  writeFileSync(join(tr.bin, 'profile.plist'), plist(xml));
  writeFileSync(join(tr.bin, 'identities'), over.identityLine ?? `  1) ${SHA} "Apple Distribution: Someone (${team})"\n     1 valid identities found\n`);
  const profile = join(tr.t, 'store.mobileprovision');
  writeFileSync(profile, 'cms bytes the stub ignores');
  return profile;
}
const archive = (tr: Tree, extra: Record<string, string> = {}, cwd?: string) => {
  const e = tr.env(extra);
  if (!('ALLOW_NO_MODELS' in extra)) delete e.ALLOW_NO_MODELS;                       // a store archive never runs without the model libraries (R4); the other scripts' tests do
  return run(['/bin/sh', join(tr.m, 'scripts', 'archive-store.sh')], e, cwd);
};

describe('building for a phone and the store kind (build-lab.sh)', () => {
  test('S1 a phone build must name which development app; the store kind is for archives only, and an archive is the store kind', () => {
    const tr = tree();
    try {
      const lab = (args: string[], extra: Record<string, string> = {}) => run(['/bin/sh', join(tr.m, 'scripts', 'build-lab.sh'), ...args], tr.env(extra));
      let r = lab(['ios', '--device', '--phone', 'X']);
      expect(r.code).not.toBe(0); expect(r.err).toContain('AR_VARIANT'); expect(existsSync(join(tr.bin, 'bun.last'))).toBe(false);
      r = lab(['ios', '--device', '--phone', 'X'], { AR_VARIANT: 'lab' });
      expect(r.code).toBe(0); expect(readFileSync(join(tr.bin, 'bun.last'), 'utf8')).toContain('EXACT_VARIANT=[lab]');
      r = lab(['ios']);                                                                  // the simulator: any identity, the production id by default
      expect(r.code).toBe(0); expect(readFileSync(join(tr.bin, 'bun.last'), 'utf8')).toContain('EXACT_VARIANT=[unset]');
      r = lab(['ios', '--device', '--archive', join(tr.t, 'x.ipa')]);                    // an archive that is not the store kind
      expect(r.code).not.toBe(0); expect(r.err).toContain('AR_BUILD_KIND=store');
      r = lab(['ios'], { AR_BUILD_KIND: 'store' });                                     // the store kind for something that is not an archive
      expect(r.code).not.toBe(0); expect(r.err).toContain('--archive');
      r = lab(['ios'], { AR_BUILD_KIND: 'weird' });
      expect(r.code).not.toBe(0);
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

describe('check-store-signing.py reads the profile as Apple writes it', () => {
  test('R11 a valid App Store profile that omits get-task-allow, or carries a legacy App ID prefix, is accepted; a prefix that is not the profile\'s own is refused', () => {
    const tr = tree();
    try {
      const ok = (over: Parameters<typeof writeSigning>[1], what: string) => {
        const profile = writeSigning(tr, over);
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

describe('archive-store.sh builds and reads the ipa back', () => {
  test('S3 Exact is given the version, a rising build number, the export answer, the developer menu off, and the signing pair; the ipa is read back', () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      const r = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile });
      expect(r.code, r.err + r.out).toBe(0);
      const seen = readFileSync(join(tr.bin, 'bun.last'), 'utf8');
      expect(seen).toMatch(/args=\[exact\.mjs ios --device --archive \S+\.ipa --json\]/);
      expect(seen).toContain('EXACT_VARIANT=[unset]');
      expect(seen).toContain(`SIGNIN_SCHEME=[${PROD}]`);
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

describe('upload-store.sh', () => {
  // An ipa as archive-store.sh leaves it: archive kind, developer menu off, export answer false, a profile embedded. `info` overrides or (undefined) removes a key.
  const ipaFor = (tr: Tree, id = PROD, info: Record<string, unknown> = {}) => {
    const p = join(tr.t, 'x.ipa');
    const base: Record<string, unknown> = { CFBundleIdentifier: id, CFBundleShortVersionString: '1.0', CFBundleVersion: '7', ExactBuildKind: 'archive', ExactDevMenu: false, ITSAppUsesNonExemptEncryption: false };
    const merged = { ...base, ...info };
    for (const k of Object.keys(merged)) if (merged[k] === null) delete merged[k];
    const r = run(['/usr/bin/python3', '-I', '-c', 'import json,plistlib,sys,zipfile\nz=zipfile.ZipFile(sys.argv[1],"w")\nz.writestr("Payload/A.app/Info.plist",plistlib.dumps(json.loads(sys.argv[2])))\nz.writestr("Payload/A.app/embedded.mobileprovision",b"x")\nz.close()', p, JSON.stringify(merged)], BASE);
    expect(r.code, r.err).toBe(0);
    return p;
  };
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

describe('exact.sh hands Exact what env -i would drop', () => {
  test('E1 signing, version, build number, export answer and developer menu reach Exact; nothing else of the caller\'s does', () => {
    const tr = tree();
    try {
      const r = run(['/bin/sh', join(tr.m, 'scripts', 'exact.sh'), 'ios', '--device', '--archive', join(tr.t, 'o.ipa')], tr.env({ AR_BUILD_KIND: 'store', EXACT_IDENTITY: SHA, EXACT_PROFILE: '/p', EXACT_VERSION: '1.0', EXACT_BUILD_NUMBER: '42',
        EXACT_NON_EXEMPT_ENCRYPTION: 'false', EXACT_DEV_MENU: '0', SOME_OTHER: 'x' }));
      expect(r.code, r.err).toBe(0);
      const seen = readFileSync(join(tr.bin, 'bun.last'), 'utf8');
      for (const line of [`EXACT_IDENTITY=[${SHA}]`, 'EXACT_PROFILE=[/p]', 'EXACT_VERSION=[1.0]', 'EXACT_BUILD_NUMBER=[42]', 'EXACT_NON_EXEMPT_ENCRYPTION=[false]', 'EXACT_DEV_MENU=[0]', 'EXACT_FEEDBACK=[0]']) expect(seen).toContain(line);
      const none = run(['/bin/sh', join(tr.m, 'scripts', 'exact.sh'), 'ios'], tr.env());
      expect(none.code).toBe(0);
      const s2 = readFileSync(join(tr.bin, 'bun.last'), 'utf8');
      for (const v of ['EXACT_IDENTITY', 'EXACT_PROFILE', 'EXACT_VERSION', 'EXACT_BUILD_NUMBER', 'EXACT_NON_EXEMPT_ENCRYPTION', 'EXACT_DEV_MENU']) expect(s2).toContain(`${v}=[unset]`);   // unset stays unset: a development build is as it was
    } finally { tr.cleanup(); }
  });
});

describe('exact.sh is the one guard for a phone build (B1, R1)', () => {
  const ex = (tr: Tree, args: string[], extra: Record<string, string> = {}) => run(['/bin/sh', join(tr.m, 'scripts', 'exact.sh'), ...args], tr.env(extra));
  const ran = (tr: Tree) => existsSync(join(tr.bin, 'bun.last'));
  const refused = (tr: Tree, args: string[], extra: Record<string, string>, words: string) => {
    const r = ex(tr, args, extra);
    expect(r.code, args.join(' ') + ' ' + JSON.stringify(extra)).not.toBe(0);
    expect(r.err).toContain('exact.sh:'); expect(r.err).toContain(words);
    expect(ran(tr), 'Exact must not have run').toBe(false);
  };
  test('B1 a device build must name its development app; an archive is the store kind, the store kind is an archive, and an archive is the production app', () => {
    const tr = tree();
    try {
      refused(tr, ['ios', '--device', '--phone', 'X'], {}, 'AR_VARIANT');
      refused(tr, ['agent', 'ios', '--device'], {}, 'AR_VARIANT');
      refused(tr, ['ios', '--device', '--archive', join(tr.t, 'o.ipa')], {}, 'AR_BUILD_KIND=store');
      refused(tr, ['ios', '--device', '--archive', join(tr.t, 'o.ipa')], { AR_BUILD_KIND: 'development' }, 'AR_BUILD_KIND=store');
      refused(tr, ['ios'], { AR_BUILD_KIND: 'store' }, '--archive');
      refused(tr, ['ios', '--device', '--phone', 'X'], { AR_BUILD_KIND: 'store', AR_VARIANT: 'lab' }, '--archive');
      refused(tr, ['ios'], { AR_BUILD_KIND: 'weird' }, 'AR_BUILD_KIND');
      refused(tr, ['ios', '--device', '--archive', join(tr.t, 'o.ipa')], { AR_BUILD_KIND: 'store', AR_VARIANT: 'lab' }, 'production');
      for (const [args, extra] of [[['ios', '--device', '--phone', 'X'], { AR_VARIANT: 'lab' }], [['agent', 'ios', '--device'], { AR_VARIANT: 'lab-one' }], [['ios'], {}], [['contract', 'build'], {}],
        [['ios', '--device', '--archive', join(tr.t, 'o.ipa')], { AR_BUILD_KIND: 'store' }]] as [string[], Record<string, string>][]) {
        rmSync(join(tr.bin, 'bun.last'), { force: true });
        const r = ex(tr, args, extra);
        expect(r.code, args.join(' ') + r.err).toBe(0); expect(ran(tr)).toBe(true);
      }
    } finally { tr.cleanup(); }
  });
  test('B1 a phone build or an archive outside the build tree is refused; a simulator build is not', () => {
    const tr = tree();
    try {
      writeFileSync(join(tr.t, 'elsewhere'), `${tr.t}\n`);
      const away = { AUTORADIO_BUILD_TREE_FILE: join(tr.t, 'elsewhere') };
      refused(tr, ['ios', '--device', '--phone', 'X'], { ...away, AR_VARIANT: 'lab' }, 'build tree');
      refused(tr, ['ios', '--device', '--archive', join(tr.t, 'o.ipa')], { ...away, AR_BUILD_KIND: 'store' }, 'build tree');
      expect(ex(tr, ['ios'], away).code).toBe(0);
    } finally { tr.cleanup(); }
  });
  test('R1 a variant given as --variant or EXACT_VARIANT is refused, naming AR_VARIANT, instead of being half applied', () => {
    const tr = tree();
    try {
      refused(tr, ['--variant', 'lab-one', 'ios'], {}, 'AR_VARIANT');
      refused(tr, ['ios', '--variant=lab-one'], {}, 'AR_VARIANT');
      refused(tr, ['ios'], { EXACT_VARIANT: 'lab-one' }, 'AR_VARIANT');
      const r = ex(tr, ['ios'], { EXACT_VARIANT: '' });                                    // an empty value names nothing
      expect(r.code, r.err).toBe(0);
      const v = ex(tr, ['ios'], { AR_VARIANT: 'lab-one' });
      expect(v.code).toBe(0);
      const seen = readFileSync(join(tr.bin, 'bun.last'), 'utf8');
      expect(seen).toContain('EXACT_VARIANT=[lab-one]'); expect(seen).toContain(`SIGNIN_SCHEME=[${LABONE}]`);   // the variant and its scheme travel together
    } finally { tr.cleanup(); }
  });
});

describe('developer-only surfaces stay out of a store build by build kind (R10)', () => {
  test('R10 -AutoradioProofSilent is behind EXACT_DEVELOPMENT, which Exact does not define for an archive; the Mac package build keeps it in debug', () => {
    const src = readFileSync(join(MOBILE, 'AutoradioKit', 'Sources', 'AutoradioKit', 'LiveAudio.swift'), 'utf8');
    const lines = src.split('\n');
    const at = lines.findIndex((l) => l.includes('forKey: "AutoradioProofSilent"'));
    expect(at).toBeGreaterThan(0);
    let open = -1;
    for (let i = at; i >= 0; i--) { if (/^\s*#(if|endif)\b/.test(lines[i])) { open = i; break; } }
    expect(lines[open]?.trim()).toBe('#if EXACT_DEVELOPMENT');                           // the nearest conditional above is the development one ...
    let close = -1;
    for (let i = at; i < lines.length; i++) { if (/^\s*#(if|endif)\b/.test(lines[i])) { close = i; break; } }
    expect(lines[close]?.trim()).toBe('#endif');                                        // ... and it closes right after the line
    const pkg = readFileSync(join(MOBILE, 'AutoradioKit', 'Package.swift'), 'utf8');
    expect(pkg).toMatch(/\.define\("EXACT_DEVELOPMENT", \.when\(configuration: \.debug\)\)/);
  });
});

describe('the docs say what the scripts do (R9)', () => {
  const read = (...p: string[]) => readFileSync(join(MOBILE, '..', ...p), 'utf8');
  test('R9 no doc line runs a phone build through exact.sh or build-lab.sh with no variant, counts four names, or cites the retired variant', () => {
    for (const f of [['mobile', 'README.md'], ['docs', 'reference', 'exact', 'build-and-ship.md'], ['docs', 'reference', 'exact', 'auth-session.md']]) {
      if (f[3] === 'auth-session.md') expect(read(...f), 'a printed variant name `variants.one`').not.toContain('variants.one');
      for (const [n, line] of read(...f).split('\n').entries()) {
        const where = `${f.join('/')}:${n + 1}`;
        if (/(exact|build-lab)\.sh[^|\n]*--device(?![^\n]*--archive)/.test(line)) expect(line, where + ' names no variant').toMatch(/AR_VARIANT|AR_APP_ID/);
        expect(line, where).not.toContain('four different names');
        expect(line, where).not.toMatch(/--variant one\b/);
      }
    }
  });
});

describe('round 3: the second review', () => {
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
  test('T2 build-lab.sh refuses the store kind with a development variant, in its own words, before anything is staged', () => {
    const tr = tree();
    try {
      for (const v of [{ AR_VARIANT: 'lab' }, { AR_VARIANT: 'lab-one' }, { AR_APP_ID: LAB }]) {
        const r = run(['/bin/sh', join(tr.m, 'scripts', 'build-lab.sh'), 'ios', '--device', '--archive', join(tr.t, 'x.ipa')], tr.env({ AR_BUILD_KIND: 'store', ...v }));
        expect(r.code).not.toBe(0); expect(r.err).toContain('build-lab:'); expect(r.err).toContain('production');
        expect(existsSync(join(tr.bin, 'bun.last'))).toBe(false);
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
      const ipa = join(tr.t, 'y.ipa');
      const mk = run(['/usr/bin/python3', '-I', '-c', 'import plistlib,sys,zipfile\nz=zipfile.ZipFile(sys.argv[1],"w")\nz.writestr("Payload/A.app/Info.plist",plistlib.dumps({"CFBundleIdentifier":"news.choir.autoradio","CFBundleShortVersionString":"1.0","CFBundleVersion":"7","ExactBuildKind":"archive","ExactDevMenu":False,"ITSAppUsesNonExemptEncryption":False}))\nz.writestr("Payload/A.app/embedded.mobileprovision",b"x")\nz.close()', ipa], BASE);
      expect(mk.code, mk.err).toBe(0);
      const up = (extra: Record<string, string> = {}) => run(['/bin/sh', join(tr.m, 'scripts', 'upload-store.sh'), ipa], tr.env({ ...good, ...extra }));
      const w = up();
      expect(w.code, w.err).toBe(0); expect(w.err).toContain('readable by others');
      for (const v of [{ AR_VARIANT: 'one' }, { AR_VARIANT: 'lab' }]) {
        const r = up(v);
        expect(r.code).not.toBe(0); expect(r.err).toContain('upload-store:'); expect(r.err).not.toContain('app-identity:');
      }
    } finally { tr.cleanup(); }
  });
  test('T4 a multi-line EXACT_IDENTITY or AR_BUILD_NUMBER is refused, and the script does not claim a lock it does not take', () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      const a = archive(tr, { EXACT_IDENTITY: SHA + '\nnot hex', EXACT_PROFILE: profile });
      expect(a.code).not.toBe(0); expect(a.err).toContain('40'); expect(existsSync(join(tr.bin, 'bun.last'))).toBe(false);
      const b = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile, AR_BUILD_NUMBER: '5\n6' });
      expect(b.code).not.toBe(0); expect(b.err).toContain('AR_BUILD_NUMBER'); expect(existsSync(join(tr.bin, 'bun.last'))).toBe(false);
      const text = readFileSync(join(MOBILE, 'scripts', 'archive-store.sh'), 'utf8');
      expect(text).not.toContain('makes that impossible');
      expect(text).toContain('caller holds the build lock');
    } finally { tr.cleanup(); }
  });
});
const KEY2 = 'ZZKEYIDTWO', ISS2 = '99999999-2222-3333-4444-555555555555';
