<!--
  ChoirField — the living constellation.

  A jittered-grid graph of drifting nodes and proximity edges, with light
  pulses travelling along the edges. It is the signature image of the
  product: many agents, connected, still moving after you look away.

  Two densities:
    density="intro"    full-frame backdrop for the landing sequence
    density="ambient"  a quieter, sparser field for the signed-out desktop

  Tokens only — theme-contract forbids raw colour literals outside theme.ts,
  so every fill is resolved from CSS custom properties at draw time.

  Performance: transform-free 2D drawing at a capped devicePixelRatio, one
  rAF loop shared by every instance, paused whenever the document is hidden
  and whenever prefers-reduced-motion asks for stillness.
-->
<script lang="ts">
  import { onDestroy, onMount } from 'svelte';

  export let density: 'intro' | 'ambient' = 'intro';
  /** Called every frame with the current attract progress, 0..1. */
  export let attract: { x: number; y: number }[] | null = null;
  export let attractProgress = 0;
  export let attractSlot: number[] = [];

  let canvas: HTMLCanvasElement | null = null;
  let ctx: CanvasRenderingContext2D | null = null;
  let raf = 0;
  let width = 0;
  let height = 0;
  let dpr = 1;
  let reduced = false;
  let visible = true;
  const t0 = performance.now();

  type Node = {
    x: number; y: number; slot: number; tw: number; depth: number;
    ax: number; ay: number; sx: number; sy: number; px: number; py: number;
    r: number; role: 'far' | 'mid' | 'near';
  };
  type Edge = { a: number; b: number; k: number };
  let nodes: Node[] = [];
  let edges: Edge[] = [];
  let pulses: { e: number; t: number; sp: number }[] = [];
  let palette: Record<string, string> = {};
  /*
    Halo sprite. Building a radial gradient per node per frame was the single
    most expensive thing in this component — CanvasGradient allocation is not
    cheap and a full-frame field has dozens of nodes. The halo is a fixed
    soft disc, so it is rasterised once and blitted thereafter.
  */
  let halo: HTMLCanvasElement | null = null;

  // mulberry32 — the composition must not reshuffle between resizes or
  // reduced-motion snapshots, or the piece stops being a piece.
  function rng(seed: number) {
    let s = seed >>> 0;
    return () => {
      s = (s + 0x6d2b79f5) >>> 0;
      let t = Math.imul(s ^ (s >>> 15), 1 | s);
      t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  }

  const clamp = (v: number, a: number, b: number) => Math.min(b, Math.max(a, v));
  const lerp = (a: number, b: number, t: number) => a + (b - a) * t;
  const easeInOut = (t: number) => (t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2);

  function readPalette(el: HTMLElement) {
    const cs = getComputedStyle(el);
    const read = (name: string, fallback: string) => {
      const v = cs.getPropertyValue(name).trim();
      return v || fallback;
    };
    palette = {
      edge: read('--choir-field-edge', 'currentColor'),
      far: read('--choir-field-node-far', 'currentColor'),
      mid: read('--choir-field-node-mid', 'currentColor'),
      near: read('--choir-field-node-near', 'currentColor'),
      halo: read('--choir-field-halo', 'currentColor'),
    };
  }

  /**
   * Jittered grid, not pure random. Random placement clumps — which reads as
   * noise and piles every bright node into one corner. The grid gives even
   * coverage so the constellation reads as deliberate structure; the jitter
   * keeps it from reading as a grid.
   */
  function build() {
    const r = rng(7331);
    const ambient = density === 'ambient';
    const cols = clamp(Math.round(width / (ambient ? 300 : 210)), 3, ambient ? 6 : 10);
    const rows = clamp(Math.round(height / (ambient ? 260 : 190)), 2, ambient ? 5 : 8);
    const cellW = width / cols;
    const cellH = height / rows;
    const cells: { x: number; y: number; depth: number; slot: number; tw: number }[] = [];
    for (let gy = 0; gy < rows; gy++) {
      for (let gx = 0; gx < cols; gx++) {
        cells.push({
          x: (gx + 0.14 + r() * 0.72) * cellW,
          y: (gy + 0.14 + r() * 0.72) * cellH,
          depth: r(),
          slot: (r() * 4) | 0,
          tw: r() * 6.283,
        });
      }
    }
    // depth is weighted toward the centre so the frame breathes outward
    const cx = width / 2;
    const cy = height / 2;
    const far = Math.hypot(cx, cy) || 1;
    nodes = cells.map((c) => {
      const depth = clamp(1 - Math.hypot(c.x - cx, c.y - cy) / far, 0, 1) * 0.75 + c.depth * 0.25;
      return {
        x: c.x,
        y: c.y,
        slot: c.slot,
        tw: c.tw,
        depth,
        ax: (ambient ? 4 : 7) + depth * (ambient ? 18 : 34),
        ay: (ambient ? 3 : 5) + depth * (ambient ? 13 : 24),
        sx: 0.00006 + depth * 0.00014,
        sy: 0.00005 + depth * 0.00013,
        px: c.tw,
        py: c.tw * 1.7,
        r: (ambient ? 0.7 : 0.9) + depth * (ambient ? 1.1 : 1.9),
        role: depth > 0.72 ? 'near' : depth > 0.42 ? 'mid' : 'far',
      } as Node;
    });

    edges = [];
    const reach = Math.min(width, height) * (ambient ? 0.2 : 0.26);
    for (let i = 0; i < nodes.length; i++) {
      for (let j = i + 1; j < nodes.length; j++) {
        const d = Math.hypot(nodes[i].x - nodes[j].x, nodes[i].y - nodes[j].y);
        if (d < reach) edges.push({ a: i, b: j, k: 1 - d / reach });
      }
    }
    const q = rng(9091);
    const count = ambient ? 2 : 5;
    pulses = Array.from({ length: count }, (_, i) => ({
      e: (q() * Math.max(1, edges.length)) | 0,
      t: -i * 0.55,
      sp: 0.0011 + q() * 0.0016,
    }));
  }

  /**
   * Rasterise the node halo once. A wide, very faint falloff — a tight
   * bright gradient on a small radius reads as a hard-edged disc the moment
   * the frame is scaled down, which is the difference between depth of field
   * and a smudge.
   */
  function buildHalo(color: string) {
    const size = 128;
    const c = document.createElement('canvas');
    c.width = size;
    c.height = size;
    const g = c.getContext('2d');
    if (!g) return null;
    const r = size / 2;
    const grad = g.createRadialGradient(r, r, 0, r, r, r);
    grad.addColorStop(0, color);
    grad.addColorStop(0.35, color);
    grad.addColorStop(1, 'transparent');
    g.fillStyle = grad;
    g.fillRect(0, 0, size, size);
    return c;
  }

  function resize() {
    if (!canvas || !ctx) return;
    // The field is a soft glow field; it gains nothing from a 2x backing
    // store and costs 4x the fill rate. 1.5 is the point of diminishing
    // returns for a radial gradient you cannot see the pixels of.
    dpr = Math.min(window.devicePixelRatio || 1, 1.5);
    width = canvas.clientWidth || window.innerWidth;
    height = canvas.clientHeight || window.innerHeight;
    canvas.width = Math.max(1, Math.round(width * dpr));
    canvas.height = Math.max(1, Math.round(height * dpr));
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    readPalette(canvas);
    halo = buildHalo(palette.halo);
    build();
  }

  function draw(now: number) {
    raf = requestAnimationFrame(draw);
    if (!ctx) return;
    const t = now - t0;
    ctx.clearRect(0, 0, width, height);

    const ambient = density === 'ambient';
    const e = easeInOut(clamp(attractProgress, 0, 1));
    const pos = nodes.map((n) => {
      let x = n.x + Math.sin(t * n.sx + n.px) * n.ax;
      let y = n.y + Math.cos(t * n.sy + n.py) * n.ay;
      if (attract && attract.length === 4 && e > 0) {
        const k = attract[attractSlot[n.slot] ?? n.slot] || attract[n.slot];
        if (k) {
          x = lerp(x, k.x, e * 0.82);
          y = lerp(y, k.y, e * 0.82);
        }
      }
      return { x, y };
    });

    for (const ed of edges) {
      const a = pos[ed.a];
      const b = pos[ed.b];
      ctx.globalAlpha = (ambient ? 0.5 : 1) * (0.28 + ed.k * 0.72);
      ctx.strokeStyle = palette.edge;
      ctx.lineWidth = 0.55 + ed.k * 0.7;
      ctx.beginPath();
      ctx.moveTo(a.x, a.y);
      ctx.lineTo(b.x, b.y);
      ctx.stroke();
    }
    ctx.globalAlpha = 1;

    for (const p of pulses) {
      p.t += p.sp * 16.6;
      if (p.t > 1) {
        p.t = -0.4;
        p.e = (p.e + 37) % Math.max(1, edges.length);
        continue;
      }
      const ed = edges[p.e];
      if (!ed) continue;
      const a = pos[ed.a];
      const b = pos[ed.b];
      const x = lerp(a.x, b.x, p.t);
      const y = lerp(a.y, b.y, p.t);
      ctx.fillStyle = palette.near;
      ctx.globalAlpha = 0.16;
      ctx.beginPath();
      ctx.arc(x, y, 20, 0, 6.2832);
      ctx.fill();
      ctx.globalAlpha = 0.85;
      ctx.beginPath();
      ctx.arc(x, y, 1.4, 0, 6.2832);
      ctx.fill();
      ctx.globalAlpha = 1;
    }

    for (let i = 0; i < nodes.length; i++) {
      const n = nodes[i];
      const p = pos[i];
      const tw = 0.72 + 0.28 * Math.sin(t * 0.0016 + n.tw);
      // Atmospheric halo, not a disc. Blitted from a cached sprite — see
      // buildHalo(). A per-frame createRadialGradient here costs more than
      // the entire rest of the frame.
      if (!ambient && halo && n.depth > 0.4) {
        const rad = 74 + n.r * 52;
        ctx.globalAlpha = tw * 0.9;
        ctx.drawImage(halo, p.x - rad, p.y - rad, rad * 2, rad * 2);
      }
      ctx.globalAlpha = tw * (ambient ? 0.5 : 1);
      ctx.fillStyle = palette[n.role];
      ctx.beginPath();
      ctx.arc(p.x, p.y, n.r, 0, 6.2832);
      ctx.fill();
      ctx.globalAlpha = 1;
    }
  }

  function start() {
    if (raf || reduced) return;
    raf = requestAnimationFrame(draw);
  }

  function stop() {
    if (!raf) return;
    cancelAnimationFrame(raf);
    raf = 0;
  }

  let resizeTimer: ReturnType<typeof setTimeout> | null = null;
  function onResize() {
    if (resizeTimer) clearTimeout(resizeTimer);
    resizeTimer = setTimeout(() => {
      resize();
      draw(performance.now());
    }, 160);
  }
  function onVisibility() {
    visible = document.visibilityState === 'visible';
    if (visible) start();
    else stop();
  }

  onMount(() => {
    if (!canvas) return;
    ctx = canvas.getContext('2d', { alpha: true });
    if (!ctx) return;
    const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
    reduced = mq.matches;
    mq.addEventListener?.('change', (ev) => {
      reduced = ev.matches;
      if (reduced) {
        stop();
        draw(performance.now());
      } else start();
    });
    resize();
    if (reduced) draw(performance.now());
    else start();
    window.addEventListener('resize', onResize, { passive: true });
    document.addEventListener('visibilitychange', onVisibility);
  });

  onDestroy(() => {
    stop();
    if (resizeTimer) clearTimeout(resizeTimer);
    if (typeof window !== 'undefined') {
      window.removeEventListener('resize', onResize);
      document.removeEventListener('visibilitychange', onVisibility);
    }
  });
</script>

<!-- A canvas is a replaced element: inset:0 alone will NOT stretch it, it
     falls back to the 300x150 intrinsic size. It needs explicit 100%/100%. -->
<canvas bind:this={canvas} class="choir-field" data-choir-field={density} aria-hidden="true"></canvas>

<style>
  .choir-field {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    display: block;
    pointer-events: none;
  }
</style>
