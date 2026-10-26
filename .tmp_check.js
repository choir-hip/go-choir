const { spawn } = require('node:child_process');
const CHROME = '/nix/store/ini1ifgvcsvw48x4mxawrv9jvbdwn70p-chromium-155.0.8059.79/bin/chromium';
const PORT = 9341;
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const fails = [];
const ok = (cond, label, detail) => { console.log((cond ? 'PASS ' : 'FAIL ') + label + (detail !== undefined ? '  ' + JSON.stringify(detail) : '')); if (!cond) fails.push(label); };
(async () => {
  const chrome = spawn(CHROME, ['--headless=new','--no-sandbox','--disable-gpu','--hide-scrollbars','--remote-debugging-port=' + PORT,'--user-data-dir=/tmp/cdp-check-' + Date.now(),'about:blank'], { stdio: 'ignore' });
  let version = null;
  for (let i=0;i<80;i++){ try { version = await (await fetch('http://127.0.0.1:' + PORT + '/json/version')).json(); break; } catch(e){ await sleep(250); } }
  const ws = new WebSocket(version.webSocketDebuggerUrl);
  await new Promise((res, rej) => { ws.onopen = res; ws.onerror = rej; });
  let id = 0; const pending = new Map();
  ws.onmessage = (m) => { const msg = JSON.parse(m.data); if (msg.id && pending.has(msg.id)) { pending.get(msg.id)(msg); pending.delete(msg.id); } };
  const send = (method, params, sessionId) => new Promise((res) => { const mid = ++id; pending.set(mid, res); ws.send(JSON.stringify({ id: mid, method, params: params || {}, sessionId })); });
  const t = await send('Target.createTarget', { url: 'about:blank' });
  const a = await send('Target.attachToTarget', { targetId: t.result.targetId, flatten: true });
  const sid = a.result.sessionId;
  await send('Page.enable', {}, sid);
  await send('Runtime.enable', {}, sid);
  const ev = async (expression) => { const r = await send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }, sid); if (r.result.exceptionDetails) throw new Error(JSON.stringify(r.result.exceptionDetails).slice(0,400)); return r.result.result.value; };
  const go = async (u) => { await send('Page.navigate', { url: u }, sid); await sleep(1800); };
  const measure = () => ev('(() => { const q = (s) => document.querySelector(s); const box = (el) => { if (!el) return null; const b = el.getBoundingClientRect(); return { x: b.x, y: b.y, w: b.width, h: b.height }; }; const t = q("[data-reader-toolbar]"); const body = q(".reader-body"); const p = q("[data-reader-sheet] p"); return { toolbar: box(t), body: box(body), p: p ? { color: getComputedStyle(p).color, weight: getComputedStyle(p).fontWeight, style: getComputedStyle(p).fontStyle } : null, glyphs: Array.from(document.querySelectorAll(".reader-glyph")).length, unicodeGlyphButtons: Array.from(document.querySelectorAll("button")).filter((b) => /[\\u2325-\\u2BFF\\u2600-\\u27BF]/.test(b.textContent || "")).length, strayHint: !!q("[data-reader-drop-hint]"), toolbars: document.querySelectorAll("[data-reader-toolbar]").length, footer: !!q("[data-reader-footer]"), htmlText: document.querySelector("[data-reader-app]").textContent.slice(0, 300) }; })()');
  const setVp = (w, h) => send('Emulation.setDeviceMetricsOverride', { width: w, height: h, deviceScaleFactor: 1, mobile: false }, sid);

  let ready = false;
  for (let i = 0; i < 40 && !ready; i++) { try { const r = await fetch('http://127.0.0.1:8517/read.html'); ready = r.ok; } catch (e) { ready = false; } if (!ready) await sleep(250); }
  ok(ready, 'the built page answers');

  await setVp(1280, 800);
  await go('http://127.0.0.1:8517/read.html');
  const d = await measure();
  ok(!!d.toolbar, 'the Reader rendered');
  ok(d.toolbars === 1, 'exactly one toolbar', d.toolbars);
  ok(d.toolbar && d.toolbar.h >= 40, 'toolbar height >= 40px', d.toolbar && d.toolbar.h);
  ok(d.toolbar && d.body && d.toolbar.y + d.toolbar.h <= d.body.y + 0.5, 'toolbar does not intersect .reader-body', { toolbarBottom: d.toolbar.y + d.toolbar.h, bodyTop: d.body.y });
  ok(d.p && d.p.color === 'rgb(58, 21, 23)', 'body text is the theme ink, not grey', d.p && d.p.color);
  ok(d.p && d.p.weight === '400' && d.p.style === 'normal', 'prose is 400 upright', d.p && [d.p.weight, d.p.style]);
  ok(!d.strayHint, 'no permanent drop hint line');
  ok(!d.footer, 'no footer strip');
  ok(d.unicodeGlyphButtons === 0, 'no unicode glyph buttons', d.unicodeGlyphButtons);
  ok(d.glyphs > 0, 'inline SVG icons drawn', d.glyphs);
  ok(!/&lt;|&gt;/.test(d.htmlText), 'no escaped markup text');

  await setVp(390, 844);
  await go('http://127.0.0.1:8517/read.html');
  const p = await measure();
  ok(!!p.toolbar, 'the Reader rendered on phone');
  ok(p.toolbar && p.toolbar.h === 44, 'phone toolbar stays one 44px row', p.toolbar && p.toolbar.h);
  ok(p.toolbar.y + p.toolbar.h <= 112, 'phone chrome above the text <= 112px', p.toolbar.y + p.toolbar.h);
  ok(p.body && p.body.h / 844 >= 0.6, 'phone content >= 60% of height', p.body && p.body.h / 844);
  ok(p.unicodeGlyphButtons === 0, 'phone: no unicode glyph buttons');
  ok(await ev('(() => !!document.querySelector("[data-reader-overflow-toggle]") && !document.querySelector(".reader-toolbar [data-reader-search-toggle]"))()'), 'phone: secondary controls are in the overflow menu');

  console.log(fails.length ? 'CHECK FAILED: ' + fails.join(' | ') : 'CHECK OK');
  ws.close(); chrome.kill();
  process.exit(fails.length ? 1 : 0);
})().catch((e) => { console.error('CHECK CRASHED', (e && e.stack) || e); process.exit(2); });
