// One measurement scenario, run inside its own cgroup v2 leaf that mimics an
// engineering capsule (memory.max 2 GiB, cpu.max 1 CPU, pids.max 256).
// Prints one JSON line.
import { chromium } from 'playwright-core';
import fs from 'node:fs';
import path from 'node:path';

const [scenario, cg, outDir] = process.argv.slice(2);
const CG = `/sys/fs/cgroup/${cg}`;
const rd = (f) => fs.readFileSync(path.join(CG, f), 'utf8').trim();
const memStat = () => Object.fromEntries(rd('memory.stat').split('\n').map(l => l.split(' ')).map(([k, v]) => [k, Number(v)]));
const cpuUsec = () => Number(rd('cpu.stat').split('\n').find(l => l.startsWith('usage_usec')).split(' ')[1]);
const MiB = (b) => Math.round(b / 1048576);

function chromePss() {
  let pss = 0, rss = 0, n = 0, threads = 0;
  for (const pid of fs.readdirSync('/proc').filter(d => /^\d+$/.test(d))) {
    try {
      const cmd = fs.readFileSync(`/proc/${pid}/cmdline`, 'utf8');
      if (!cmd.includes('ms-playwright')) continue; // Chromium and Playwright's ffmpeg
      const roll = fs.readFileSync(`/proc/${pid}/smaps_rollup`, 'utf8');
      pss += Number(/^Pss:\s+(\d+)/m.exec(roll)[1]) * 1024;
      rss += Number(/^Rss:\s+(\d+)/m.exec(roll)[1]) * 1024;
      threads += Number(/^Threads:\s+(\d+)/m.exec(fs.readFileSync(`/proc/${pid}/status`, 'utf8'))[1]);
      n++;
    } catch { /* process gone */ }
  }
  return { pss, rss, n, threads };
}

const samples = { maxCur: 0, maxAnon: 0, maxFile: 0, maxShmem: 0, maxPss: 0, maxRss: 0, maxProcs: 0, maxThreads: 0, maxPids: 0 };
function sample() {
  const cur = Number(rd('memory.current'));
  const st = memStat();
  const c = chromePss();
  samples.maxCur = Math.max(samples.maxCur, cur);
  samples.maxAnon = Math.max(samples.maxAnon, st.anon);
  samples.maxFile = Math.max(samples.maxFile, st.file);
  samples.maxShmem = Math.max(samples.maxShmem, st.shmem);
  samples.maxPss = Math.max(samples.maxPss, c.pss);
  samples.maxRss = Math.max(samples.maxRss, c.rss);
  samples.maxProcs = Math.max(samples.maxProcs, c.n);
  samples.maxThreads = Math.max(samples.maxThreads, c.threads);
  samples.maxPids = Math.max(samples.maxPids, Number(rd('pids.current')));
}

const base = { cur: Number(rd('memory.current')), stat: memStat(), cpu: cpuUsec() };
const timer = setInterval(sample, 100);
const t0 = Date.now();
const timings = {};
const SITE = 'http://127.0.0.1:8080';
let videoBytes = 0, shotBytes = 0, shots = 0;

async function shot(page, name) {
  const buf = await page.screenshot({ fullPage: true, path: path.join(outDir, `${scenario}-${name}.png`) });
  shotBytes += buf.length; shots++;
}

const launchOpts = { headless: true };
if (scenario === 'new-headless') launchOpts.channel = 'chromium';
const browser = await chromium.launch(launchOpts);
timings.launch_ms = Date.now() - t0;
try {
  if (scenario === 'one-shot' || scenario === 'new-headless') {
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
    const t = Date.now();
    await page.goto(SITE + '/', { waitUntil: 'networkidle' });
    await shot(page, 'frontend');
    timings.capture_ms = Date.now() - t;
  } else if (scenario === 'heavy') {
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
    const t = Date.now();
    await page.goto(SITE + '/heavy.html', { waitUntil: 'load' });
    await page.waitForTimeout(1000);
    await shot(page, 'heavy');
    timings.capture_ms = Date.now() - t;
  } else if (scenario === 'five-pages') {
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
    const t = Date.now();
    for (const [i, u] of ['/', '/heavy.html', '/legal/', '/?a=1', '/heavy.html?b=2'].entries()) {
      await page.goto(SITE + u, { waitUntil: 'load' });
      await page.waitForTimeout(500);
      await shot(page, `p${i}`);
    }
    timings.capture_ms = Date.now() - t;
  } else if (scenario === 'four-parallel') {
    const t = Date.now();
    await Promise.all([0, 1, 2, 3].map(async (i) => {
      const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
      const page = await ctx.newPage();
      await page.goto(SITE + (i % 2 ? '/heavy.html' : '/'), { waitUntil: 'load' });
      await page.waitForTimeout(1000);
      await shot(page, `par${i}`);
    }));
    timings.capture_ms = Date.now() - t;
  } else if (scenario === 'heavy-viewport') {
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
    const t = Date.now();
    await page.goto(SITE + '/heavy.html', { waitUntil: 'load' });
    await page.waitForTimeout(1000);
    const buf = await page.screenshot({ path: path.join(outDir, `${scenario}.png`) });
    shotBytes += buf.length; shots++;
    timings.capture_ms = Date.now() - t;
  } else if (scenario === 'five-frontend-viewport') {
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
    const t = Date.now();
    for (const [i, u] of ['/', '/?a=1', '/legal/', '/?b=2', '/?c=3'].entries()) {
      await page.goto(SITE + u, { waitUntil: 'networkidle' });
      await page.waitForTimeout(1500);
      const buf = await page.screenshot({ path: path.join(outDir, `${scenario}-${i}.png`) });
      shotBytes += buf.length; shots++;
    }
    timings.capture_ms = Date.now() - t;
  } else if (scenario === 'four-parallel-viewport') {
    const t = Date.now();
    await Promise.all([0, 1, 2, 3].map(async (i) => {
      const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
      const page = await ctx.newPage();
      await page.goto(SITE + (i % 2 ? '/heavy.html' : '/'), { waitUntil: 'load' });
      await page.waitForTimeout(1000);
      const buf = await page.screenshot({ path: path.join(outDir, `${scenario}-${i}.png`) });
      shotBytes += buf.length; shots++;
    }));
    timings.capture_ms = Date.now() - t;
  } else if (scenario === 'video-30s-frontend') {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 720 }, recordVideo: { dir: outDir, size: { width: 1280, height: 720 } } });
    const page = await ctx.newPage();
    const t = Date.now();
    await page.goto(SITE + '/', { waitUntil: 'networkidle' });
    for (let i = 0; i < 28; i++) { await page.mouse.wheel(0, 300); await page.waitForTimeout(1000); }
    timings.capture_ms = Date.now() - t;
    const v = page.video();
    await ctx.close();
    videoBytes = fs.statSync(await v.path()).size;
  } else if (scenario === 'video-30s') {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 720 }, recordVideo: { dir: outDir, size: { width: 1280, height: 720 } } });
    const page = await ctx.newPage();
    const t = Date.now();
    await page.goto(SITE + '/', { waitUntil: 'networkidle' });
    await page.waitForTimeout(5000);
    await page.goto(SITE + '/heavy.html', { waitUntil: 'load' });
    for (let i = 0; i < 25; i++) { await page.mouse.wheel(0, 400); await page.waitForTimeout(1000); }
    timings.capture_ms = Date.now() - t;
    const v = page.video();
    await ctx.close();
    const vp = await v.path();
    videoBytes = fs.statSync(vp).size;
  }
} finally {
  sample();
  const tc = Date.now();
  await browser.close();
  timings.close_ms = Date.now() - tc;
}
clearInterval(timer);
const afterClose = { cur: Number(rd('memory.current')), stat: memStat() };
const peak = Number(rd('memory.peak'));
const cpu = (cpuUsec() - base.cpu) / 1e6;
const wall = (Date.now() - t0) / 1000;
// What the capsule teardown would do: reclaim the leaf before rmdir.
try { fs.writeFileSync(path.join(CG, 'memory.reclaim'), '4G'); } catch (e) { /* EAGAIN when it cannot reach the target; fine */ }
const afterReclaim = { cur: Number(rd('memory.current')), stat: memStat() };
const events = Object.fromEntries(rd('memory.events').split('\n').map(l => l.split(' ')).map(([k, v]) => [k, Number(v)]));

console.log(JSON.stringify({
  scenario,
  base_mib: MiB(base.cur),
  cgroup_peak_mib: MiB(peak),
  cgroup_peak_over_base_mib: MiB(peak - base.cur),
  sampled_max_anon_mib: MiB(samples.maxAnon), sampled_max_file_mib: MiB(samples.maxFile), sampled_max_shmem_mib: MiB(samples.maxShmem),
  chrome_max_pss_mib: MiB(samples.maxPss), chrome_max_rss_sum_mib: MiB(samples.maxRss),
  chrome_max_procs: samples.maxProcs, chrome_max_threads: samples.maxThreads, cgroup_max_pids: samples.maxPids,
  after_close_mib: MiB(afterClose.cur), after_close_anon_mib: MiB(afterClose.stat.anon), after_close_file_mib: MiB(afterClose.stat.file),
  after_reclaim_mib: MiB(afterReclaim.cur), after_reclaim_file_mib: MiB(afterReclaim.stat.file),
  cpu_s: Number(cpu.toFixed(2)), wall_s: Number(wall.toFixed(2)), cpu_cores_avg: Number((cpu / wall).toFixed(2)),
  ...timings, shots, shot_kib: Math.round(shotBytes / 1024), video_kib: Math.round(videoBytes / 1024),
  memory_events: events,
}));
