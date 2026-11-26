// Failure list (written before the Android package):
//  P1 the interface generator drops the Android implementation export.
//  P2 the Android implementation does not declare the package class and web-page view Exact binds.
//  P3 the package has no Kotlin source for the declared class, or the source accepts non-http(s) in-app pages.
import { describe, expect, test } from 'bun:test';
import { cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';

const MOBILE = join(dirname(new URL(import.meta.url).pathname), '..');
const PACKAGE = join(MOBILE, 'exact-package');

describe('the Android Exact package', () => {
  test('P1 the generator preserves the Android export and the shared web-page view', () => {
    const root = mkdtempSync(join(tmpdir(), 'android-package-'));
    try {
      const mobile = join(root, 'mobile');
      mkdirSync(join(mobile, 'scripts'), { recursive: true });
      mkdirSync(join(mobile, 'exact-package', 'interface'), { recursive: true });
      mkdirSync(join(mobile, 'exact-package', 'android'), { recursive: true });
      cpSync(join(MOBILE, 'scripts', 'gen-exact-interface.py'), join(mobile, 'scripts', 'gen-exact-interface.py'));
      cpSync(join(PACKAGE, 'interface'), join(mobile, 'exact-package', 'interface'), { recursive: true });
      writeFileSync(join(mobile, 'exact-package', 'android', 'exact.json'), '{}\n');
      const r = Bun.spawnSync(['/usr/bin/python3', '-I', join(mobile, 'scripts', 'gen-exact-interface.py')], { stdout: 'pipe', stderr: 'pipe' });
      expect(r.exitCode, r.stderr.toString()).toBe(0);
      const pkg = JSON.parse(readFileSync(join(mobile, 'exact-package', 'package.json'), 'utf8'));
      expect(pkg.exports['./android']).toBe('./android/exact.json');
      const iface = JSON.parse(readFileSync(join(mobile, 'exact-package', 'exact.json'), 'utf8'));
      expect(iface.views['web-page'].props.page).toBe('string');
      rmSync(join(mobile, 'exact-package', 'android', 'exact.json'));
      const again = Bun.spawnSync(['/usr/bin/python3', '-I', join(mobile, 'scripts', 'gen-exact-interface.py')], { stdout: 'pipe', stderr: 'pipe' });
      expect(again.exitCode, again.stderr.toString()).toBe(0);
      const without = JSON.parse(readFileSync(join(mobile, 'exact-package', 'package.json'), 'utf8'));
      expect(without.exports['./android']).toBeUndefined();
    } finally { rmSync(root, { recursive: true, force: true }); }
  });

  test('P2 the Android manifest declares the package class and web-page view', () => {
    const path = join(PACKAGE, 'android', 'exact.json');
    expect(existsSync(path)).toBe(true);
    const manifest = JSON.parse(readFileSync(path, 'utf8'));
    expect(manifest.kind).toBe('implementation');
    expect(manifest.implements).toBe('@choir/autoradio@^1.0');
    expect(manifest.platform).toBe('android');
    expect(manifest.targets).toEqual(['android']);
    expect(manifest.class).toBe('choir.autoradio.AutoradioModule');
    expect(manifest.views).toEqual(['web-page']);
    expect(manifest.sources.kotlin.length).toBeGreaterThan(0);
  });

  test('P3 Kotlin serves the declared class and keeps in-app web pages on http(s)', () => {
    const source = readFileSync(join(PACKAGE, 'android', 'AutoradioModule.kt'), 'utf8');
    expect(source).toContain('package choir.autoradio');
    expect(source).toContain('class AutoradioModule(context: ExtensionContext) : ExactExtension(context)');
    expect(source).toContain('"web-page"');
    expect(source).toContain('"http"');
    expect(source).toContain('"https"');
    expect(source).toContain('"mailto"');
    expect(source).toContain('"tel"');
    expect(source).toContain('"sms"');
  });

  test('P5 the deck package pins Media3 and keeps tests outside APK sources', () => {
    const manifest = JSON.parse(readFileSync(join(PACKAGE, 'android', 'exact.json'), 'utf8'));
    expect(manifest.maven).toEqual(['com.microsoft.onnxruntime:onnxruntime-android:1.32.0', 'androidx.media3:media3-exoplayer:1.11.1', 'androidx.media3:media3-session:1.11.1']);
    expect(manifest.sources.kotlin).toEqual(['.']);
    const kotlin = readFileSync(join(PACKAGE, 'android', 'AutoradioModule.kt'), 'utf8');
    for (const name of ['deckState', 'deckPlay', 'deckEnqueue', 'deckAct', 'deckAck', 'deckWipe']) expect(kotlin).toContain(`"${name}"`);
    expect(kotlin).toContain('override fun release(name: String)');
    expect(kotlin).toContain('DeckAndroid.state(context)');
    expect(kotlin).toContain('DeckAndroid.play(context, args)');
    expect(readFileSync(join(PACKAGE, 'android', 'DeckContract.kt'), 'utf8')).toContain('"autoradio.deckState"');
    expect(existsSync(join(MOBILE, '..', 'scripts', 'android-deck-tests', 'kotlin', 'choir', 'autoradio', 'DeckControllerTest.kt'))).toBe(true);
  });

  test('P6 Media3 renders through analytics, keeps a paused claim and maps remote controls', () => {
    const source = readFileSync(join(PACKAGE, 'android', 'DeckMediaEngine.kt'), 'utf8');
    expect(source).toContain('onAudioPositionAdvancing');
    expect(source).toContain('ForwardingPlayer');
    expect(source).toContain('PlaybackService.claim');
    expect(source).toContain('context.audioFocus.claim');
    expect(source).toContain('setLanguage');
    expect(source).toContain('PlaybackService.claim(app, sessionPlayer)');
    expect(source).toContain('synthesizeToFile');
    expect(source).toContain('setWakeMode');
    expect(source).toContain('COMMAND_SEEK_TO_NEXT');
    expect(source).toContain('COMMAND_SEEK_TO_PREVIOUS');
    expect(source).not.toContain('stopForeground(STOP_FOREGROUND_REMOVE)');
  });

  test('P7 pinned Exact supplies the media service, focus and manifest assumptions', () => {
    const exact = process.env.EXACT2 ? resolve(process.env.EXACT2) : join(MOBILE, '.exact-src');
    if (!existsSync(exact)) return;                                   // a tree without the pinned checkout cannot check it (CI has it)
    const service = readFileSync(join(exact, 'host/android/app/src/playback/java/dev/exact/video/PlaybackService.kt'), 'utf8');
    expect(service).toContain('class PlaybackService : MediaSessionService()');
    expect(service).toContain('FOREGROUND_SERVICE_TYPE_MEDIA_PLAYBACK');
    expect(service).toContain('START_NOT_STICKY');
    const gradle = readFileSync(join(exact, 'host/android/app/build.gradle.kts'), 'utf8');
    expect(gradle).toContain('android.permission.FOREGROUND_SERVICE_MEDIA_PLAYBACK');
    expect(gradle).toContain('dev.exact.video.PlaybackService');
    const app = JSON.parse(readFileSync(join(MOBILE, 'AutoradioLab', 'app.json'), 'utf8'));
    expect(app.audio_session).toBe('playback');
  });

  test('P4 the seat-1 lifecycle and URL repairs are pinned', () => {
    const source = readFileSync(join(PACKAGE, 'android', 'AutoradioModule.kt'), 'utf8');
    expect(source).toContain('java.net.URI');
    expect(source).toContain('Character.isISOControl');
    expect(source).not.toContain('.also { it.show');
    expect(source).toContain('if (mainFrameFailed) return');
    expect(source).toContain('override fun onRenderProcessGone');
    expect(source).toContain('return true');
    expect(source).toContain('removeView(this)');
    expect(source).toContain('context.leave');
    expect(source).toContain('ViewLifecycle.RESUME');
  });
});
