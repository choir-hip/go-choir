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
import { describe, expect, setDefaultTimeout, test } from 'bun:test';
import { createHash } from 'node:crypto';
import { cpSync, existsSync, realpathSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync, chmodSync } from 'node:fs';
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
    // The Go list (internal/auth/device_login.go) is read, not changed here: this test says what it must hold once the production id is added to it.
    const go = readFileSync(join(MOBILE, '..', 'internal', 'auth', 'device_login.go'), 'utf8');
    const listed = /var DeviceReturnSchemes = \[\]string\{([^}]*)\}/.exec(go)![1].match(/"[^"]+"/g)!.map((s) => s.slice(1, -1));
    for (const s of [LAB, LABONE]) expect(listed).toContain(s);
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
function writeSigning(tr: Tree, over: { id?: string; teamPrefix?: string; taskAllow?: boolean; devices?: boolean; all?: boolean; expires?: Date; cert?: Buffer; identityLine?: string } = {}) {
  const team = over.teamPrefix ?? 'ABCDE12345';
  const plist = (v: string) => v;
  const xml = `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict>
<key>Name</key><string>test store profile</string><key>TeamIdentifier</key><array><string>${team}</string></array>
<key>ExpirationDate</key><date>${(over.expires ?? new Date(Date.now() + 86400e3 * 300)).toISOString().replace(/\.\d+Z$/, 'Z')}</date>
${over.devices ? '<key>ProvisionedDevices</key><array><string>0000</string></array>' : ''}${over.all ? '<key>ProvisionsAllDevices</key><true/>' : ''}
<key>DeveloperCertificates</key><array><data>${(over.cert ?? CERT).toString('base64')}</data></array>
<key>Entitlements</key><dict><key>application-identifier</key><string>${team}.${over.id ?? PROD}</string><key>get-task-allow</key>${over.taskAllow ? '<true/>' : '<false/>'}
<key>com.apple.developer.team-identifier</key><string>${team}</string></dict></dict></plist>`;
  writeFileSync(join(tr.bin, 'profile.plist'), plist(xml));
  writeFileSync(join(tr.bin, 'identities'), over.identityLine ?? `  1) ${SHA} "Apple Distribution: Someone (${team})"\n     1 valid identities found\n`);
  const profile = join(tr.t, 'store.mobileprovision');
  writeFileSync(profile, 'cms bytes the stub ignores');
  return profile;
}
const archive = (tr: Tree, extra: Record<string, string> = {}) => run(['/bin/sh', join(tr.m, 'scripts', 'archive-store.sh')], tr.env(extra));

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
      expect(seen).toContain(`EXACT_PROFILE=[${profile}]`);
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
  test('S3 the build number rises with time and a number can be given; two archives of one commit are two numbers', () => {
    const tr = tree();
    try {
      const profile = writeSigning(tr);
      const num = (extra: Record<string, string> = {}) => {
        const r = archive(tr, { EXACT_IDENTITY: SHA, EXACT_PROFILE: profile, ...extra });
        expect(r.code, r.err).toBe(0);
        return Number(/build number: (\d+)/.exec(r.out)![1]);
      };
      const a = num();
      expect(num({ AR_BUILD_NUMBER: String(a + 5) })).toBe(a + 5);
      const nowMinutes = Math.floor((Date.now() - Date.UTC(2026, 0, 1)) / 60000);
      expect(a).toBeGreaterThan(nowMinutes - 3); expect(a).toBeLessThan(nowMinutes + 3);   // minutes since 2026-01-01 UTC: a later run is a higher number
      expect(String(a).length).toBeLessThanOrEqual(9);
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
  const ipaFor = (tr: Tree, id = PROD) => {
    const p = join(tr.t, 'x.ipa');
    const r = run(['/usr/bin/python3', '-I', '-c', 'import plistlib,sys,zipfile\nz=zipfile.ZipFile(sys.argv[1],"w")\nz.writestr("Payload/A.app/Info.plist",plistlib.dumps({"CFBundleIdentifier":sys.argv[2],"CFBundleShortVersionString":"1.0","CFBundleVersion":"7"}))\nz.writestr("Payload/A.app/embedded.mobileprovision",b"x")\nz.close()', p, id], BASE);
    expect(r.code, r.err).toBe(0);
    return p;
  };
  const upload = (tr: Tree, args: string[], extra: Record<string, string> = {}) => run(['/bin/sh', join(tr.m, 'scripts', 'upload-store.sh'), ...args], tr.env(extra));
  const KEY = 'ZZKEYIDSECRET', ISS = '11111111-2222-3333-4444-555555555555';
  test('S6 it needs an ipa that is the production app, a key id, an issuer and a key file, and never echoes them', () => {
    const tr = tree();
    try {
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
});

describe('the store kind of the staged Swift (sync-exact-package.sh)', () => {
  // A tree with the real model sources and a stub kit; the xcframework directory exists so the models are staged.
  function staged(kind: string | null) {
    const t = mkdtempSync(join(tmpdir(), 'store-kind-'));
    const m = join(t, 'mobile');
    mkdirSync(join(m, 'AutoradioKit', 'Sources', 'AutoradioKit'), { recursive: true });
    mkdirSync(join(m, 'AutoradioLab', 'modules', 'apple'), { recursive: true });
    mkdirSync(join(m, 'exact-package', 'apple'), { recursive: true });
    mkdirSync(join(m, '.deps', 'xcframeworks', 'FluidAudio.xcframework'), { recursive: true });
    cpSync(join(MOBILE, 'scripts'), join(m, 'scripts'), { recursive: true });
    cpSync(join(MOBILE, 'AutoradioKit', 'Sources', 'AutoradioModels'), join(m, 'AutoradioKit', 'Sources', 'AutoradioModels'), { recursive: true });
    writeFileSync(join(m, 'AutoradioKit', 'Sources', 'AutoradioKit', 'Kit.swift'), 'public struct Kit {}\n');
    writeFileSync(join(m, 'scripts', 'gen-build-info.sh'), '#!/bin/sh\nexit 0\n');
    const r = run(['/bin/sh', join(m, 'scripts', 'sync-exact-package.sh')], { ...BASE, ...(kind ? { AR_BUILD_KIND: kind } : {}) });
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
    const bad = staged('weird');
    try { expect(bad.r.code).not.toBe(0); expect(bad.r.err).toContain('AR_BUILD_KIND'); } finally { bad.done(); }
  });
});

describe('exact.sh hands Exact what env -i would drop', () => {
  test('E1 signing, version, build number, export answer and developer menu reach Exact; nothing else of the caller\'s does', () => {
    const tr = tree();
    try {
      const r = run(['/bin/sh', join(tr.m, 'scripts', 'exact.sh'), 'ios', '--device', '--archive', join(tr.t, 'o.ipa')], tr.env({ EXACT_IDENTITY: SHA, EXACT_PROFILE: '/p', EXACT_VERSION: '1.0', EXACT_BUILD_NUMBER: '42',
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
