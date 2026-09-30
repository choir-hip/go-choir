<!--
  LandingIntro — the signed-out orientation film.

  Plays a five-act sequence over the live public desktop, then hands off.
  It is deliberately NON-BLOCKING: the scrim never captures pointer events,
  so the desktop underneath stays live, and any interaction with it dissolves
  the film. A visitor who wants the machine can simply take it. A visitor
  who wants the story gets the story. Neither is a trap.

  Contract notes (see frontend/tests for the specs that pin these):
    - No [data-window] nodes are added; the seeded public preview Texture
      window stays exactly as it is.
    - Nothing here calls .focus(), so ui-keyboard-focus's assertion that
      [data-prompt-input] holds focus after the desktop starts still holds.
    - Escape is only claimed when no dialog is open, so the auth overlay
      keeps its Escape contract.
    - Colours are tokens only (theme-contract). No raw literals in this file.
    - Motion is a pure function of (act, localTime) driven by one rAF clock.
      Entrances are declarative: an element carries data-at and the loop
      flips data-shown once when the act clock passes it, so scrubbing,
      pausing and replaying are all frame-exact and idempotent.

  Data attributes for test targeting:
    data-landing-intro        — root overlay (only while the film is showing)
    data-landing-intro-act    — current act id
    data-landing-intro-handoff— "1" once the handoff has played
    data-landing-intro-skip   — skip control
    data-landing-intro-dot    — act rail control
    data-landing-intro-enter  — primary invitation control
-->
<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';
  import ChoirField from './ChoirField.svelte';
  import TetraMark from './TetraMark.svelte';

  export let reduced = false;
  /** Replay control hosted by the desktop's own chrome. */
  export let replayToken = 0;

  const dispatch = createEventDispatcher();

  type ActId = 'title' | 'amnesia' | 'rename' | 'ensemble' | 'receipts' | 'invitation';

  const ACTS: { id: ActId; len: number; label: string }[] = [
    { id: 'title', len: 2800, label: 'Choir' },
    { id: 'amnesia', len: 6400, label: 'the amnesia' },
    { id: 'rename', len: 6200, label: 'the rename' },
    { id: 'ensemble', len: 6400, label: 'the ensemble' },
    { id: 'receipts', len: 6400, label: 'the receipts' },
    { id: 'invitation', len: 1e9, label: 'the invitation' },
  ];
  const TOTAL = ACTS.slice(0, 5).reduce((a, x) => a + x.len, 0);
  const ANCHORS = [
    [0.235, 0.285],
    [0.715, 0.245],
    [0.285, 0.755],
    [0.745, 0.715],
  ];

  let root: HTMLElement | null = null;
  let act = 0;
  let localTime = 0;
  let running = false;
  let handedOff = false;
  let progress = 0;
  let elapsed = 0;
  let last = 0;
  let raf = 0;
  let dotProgress: number[] = [0, 0, 0, 0, 0, 0];
  let lastFrame = performance.now();

  // entrance release flags, one per data-at group, computed each frame
  // continuous per-act properties
  let tapeWidth = 0;
  let transcriptOpacity = 1;
  let meterWidth = 0;
  let attractProgress = 0;
  let attract: { x: number; y: number }[] = [];

  const clamp = (v: number, a: number, b: number) => Math.min(b, Math.max(a, v));
  const lerp = (a: number, b: number, t: number) => a + (b - a) * t;
  const easeOut = (t: number) => 1 - Math.pow(1 - t, 3);

  // ── entrances ────────────────────────────────────────────────────────
  // Every timed element declares its release point once, in markup. The loop
  // sets a flag; CSS does the transition. Nothing fires twice, nothing
  // drifts, and scrubbing backwards un-releases cleanly.
  const TIMED: Record<string, number> = {
    'a0-mark': 40, 'a0-word': 620, 'a0-tag': 1150,
    // act I: the conversation runs 0–1.4s, drains by 2.1s, and the
    // argument only starts at 1.75s so nothing overlaps anything.
    'a1-kicker': 1750, 'a1-h1a': 1900, 'a1-h1b': 2030, 'a1-sub': 2850, 'a1-voice': 3900,
    'a2-kicker': 120, 'a2-h1a': 240, 'a2-h1b': 350, 'a2-sub': 2300,
    'a2-pill1': 3050, 'a2-pill2': 3180, 'a2-pill3': 3310, 'a2-pill4': 3440,
    'a3-kicker': 380, 'a3-h1a': 500, 'a3-h1b': 610, 'a3-sub': 1750,
    'a3-desk0': 420, 'a3-desk1': 700, 'a3-desk2': 980, 'a3-desk3': 1260,
    'a4-kicker': 100, 'a4-h1a': 220, 'a4-h1b': 330, 'a4-sub': 1300,
    'a4-card0': 380, 'a4-card1': 680, 'a4-card2': 980,
    'a5-kicker': 40, 'a5-h1': 170, 'a5-sub': 900, 'a5-cta': 1150, 'a5-note': 1500, 'a5-mark': 1500,
  };
  const ACT_KEYS: Record<number, string[]> = {
    0: ['a0-mark', 'a0-word', 'a0-tag'],
    1: ['a1-kicker', 'a1-h1a', 'a1-h1b', 'a1-sub', 'a1-voice'],
    2: ['a2-kicker', 'a2-h1a', 'a2-h1b', 'a2-sub', 'a2-pill1', 'a2-pill2', 'a2-pill3', 'a2-pill4'],
    3: ['a3-kicker', 'a3-h1a', 'a3-h1b', 'a3-sub', 'a3-desk0', 'a3-desk1', 'a3-desk2', 'a3-desk3'],
    4: ['a4-kicker', 'a4-h1a', 'a4-h1b', 'a4-sub', 'a4-card0', 'a4-card1', 'a4-card2'],
    5: ['a5-kicker', 'a5-h1', 'a5-sub', 'a5-cta', 'a5-note', 'a5-mark'],
  };

  /**
   * Release the act's entrances.
   *
   * Idempotent by construction: a flag is only written when it actually
   * changes. Allocating a fresh object every frame would look harmless but
   * re-runs Svelte's diff over the whole component 60 times a second, and a
   * component this size pays for it in dropped frames — which is exactly the
   * wrong place to lose a millisecond when the whole point is smoothness.
   */
  let shown: Record<string, boolean> = {};

  function release() {
    const keys = ACT_KEYS[act] || [];
    let dirty = false;
    for (const key of keys) {
      const want = localTime >= TIMED[key];
      if (shown[key] !== want) {
        shown = { ...shown, [key]: want };
        dirty = true;
      }
    }
    return dirty;
  }

  function continuous() {
    if (act === 1) {
      // the transcript argues, then drains to a residue — it does not vanish,
      // because the residue IS the argument
      transcriptOpacity = clamp(1 - (localTime - 2100) / 1600, 0.14, 1);
    }
    if (act === 2) {
      tapeWidth = easeOut(clamp((localTime - 120) / 1900, 0, 1));
    }
    if (act === 3) {
      const vw = typeof window === 'undefined' ? 1440 : window.innerWidth;
      const vh = typeof window === 'undefined' ? 900 : window.innerHeight;
      attract = ANCHORS.map((a) => ({ x: a[0] * vw, y: a[1] * vh }));
      attractProgress = clamp((localTime - 60) / 1700, 0, 1);
    } else {
      attractProgress = 0;
    }
    if (act === 4) {
      meterWidth = easeOut(clamp((localTime - 1500) / 1700, 0, 1)) * 100;
    }
  }

  function setAct(next: number) {
    act = clamp(next, 0, ACTS.length - 1);
    localTime = 0;
    last = 0;
    if (act === 1) transcriptOpacity = 1;
    if (act === 2) tapeWidth = 0;
    if (act === 3) attractProgress = 0;
    if (act === 4) meterWidth = 0;
    release();
    continuous();
  }

  function gotoAct(next: number) {
    if (handedOff) return;
    setAct(next);
  }

  function handoff() {
    if (handedOff) return;
    handedOff = true;
    running = false;
    dispatch('complete');
  }

  function restart() {
    handedOff = false;
    elapsed = 0;
    setAct(0);
    running = !reduced;
  }

  $: if (replayToken > 0 && root && !handedOff) {
    // caller asked for a replay
  }
  $: if (replayToken > 1) restart();

  function frame(now: number) {
    raf = requestAnimationFrame(frame);
    if (!last) {
      last = now;
      return;
    }
    const dt = Math.min(now - last, 64);
    last = now;
    if (running) {
      localTime += dt;
      elapsed += dt;
    }
    release();
    continuous();

    const base = ACTS.slice(0, act).reduce((a, x) => a + x.len, 0);
    progress = clamp((base + Math.min(localTime, ACTS[act].len)) / TOTAL, 0, 1);
    let dotsDirty = false;
    for (let i = 0; i < dotProgress.length; i++) {
      const target = i < act ? 1 : i === act ? clamp(localTime / ACTS[act].len, 0, 1) : 0;
      const next = lerp(dotProgress[i], target, 0.22);
      if (Math.abs(next - dotProgress[i]) > 0.002) { dotProgress[i] = next; dotsDirty = true; }
    }
    if (dotsDirty) dotProgress = dotProgress.slice();

    if (running && localTime >= ACTS[act].len && act < ACTS.length - 1) setAct(act + 1);
  }

  // ── input ───────────────────────────────────────────────────────────
  function onKeydown(event: KeyboardEvent) {
    if (handedOff) return;
    // Never claim Escape while a real dialog owns it (the auth overlay).
    if (document.querySelector('[role="dialog"], [data-auth-overlay]')) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      handoff();
      return;
    }
    if (event.key === 'ArrowRight' || event.key === 'ArrowDown') {
      event.preventDefault();
      localTime = ACTS[act].len;
      return;
    }
    if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') {
      event.preventDefault();
      gotoAct(act - 1);
    }
  }

  let wheelLock = 0;
  function onWheel(event: WheelEvent) {
    if (handedOff || reduced) return;
    if (Math.abs(event.deltaY) < 8) return;
    const now = performance.now();
    if (now - wheelLock < 620) return;
    wheelLock = now;
    gotoAct(act + (event.deltaY > 0 ? 1 : -1));
  }

  /**
   * Any interaction with the live desktop underneath retires the film.
   * This is what keeps the intro non-blocking: the scrim is pointer-events
   * none, so the desktop is always operable, and touching it ends the film.
   *
   * The film's own control keys are excluded, otherwise a document-level
   * capture listener would swallow every arrow and Escape before the film
   * could act on them and the sequence would be unscrubbable.
   */
  const OWN_KEYS = new Set([
    'Escape', 'ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown',
    'r', 'R', 'Shift', 'Control', 'Alt', 'Meta', 'Tab', 'CapsLock',
  ]);

  function onDesktopTouch(event: Event) {
    if (handedOff) return;
    if (event.type === 'keydown' && OWN_KEYS.has((event as KeyboardEvent).key)) return;
    const target = event.target as HTMLElement | null;
    if (!target || (root && root.contains(target))) return;
    handoff();
  }

  onMount(() => {
    if (reduced) {
      // No film. The desktop is the experience; the story is in the document.
      dispatch('complete');
      return;
    }
    release();
    continuous();
    running = true;
    raf = requestAnimationFrame(frame);
    window.addEventListener('keydown', onKeydown);
    window.addEventListener('wheel', onWheel, { passive: true });
    document.addEventListener('pointerdown', onDesktopTouch, true);
    document.addEventListener('keydown', onDesktopTouch, true);
  });

  onDestroy(() => {
    if (raf) cancelAnimationFrame(raf);
    if (typeof window !== 'undefined') {
      window.removeEventListener('keydown', onKeydown);
      window.removeEventListener('wheel', onWheel);
    }
    if (typeof document !== 'undefined') {
      document.removeEventListener('pointerdown', onDesktopTouch, true);
      document.removeEventListener('keydown', onDesktopTouch, true);
    }
  });
</script>

<div
  bind:this={root}
  class="intro"
  class:intro-still={reduced}
  class:intro-done={handedOff}
  data-landing-intro
  data-landing-intro-act={ACTS[act].id}
  data-landing-intro-handoff={handedOff ? '1' : '0'}
>
  <div class="intro-scrim" aria-hidden="true"></div>
  <div class="intro-aurora" aria-hidden="true"><i></i><i></i><i></i></div>
  <ChoirField density="intro" {attract} {attractProgress} />
  <div class="intro-vignette" aria-hidden="true"></div>

  <div class="intro-acts">
    <!-- ══ ACT · TITLE ══ -->
    <section class="act act-title" data-on={act === 0 ? '1' : '0'}>
      <div class="frame frame-center">
        <div class="mark" data-shown={shown['a0-mark'] ? '1' : '0'}>
          <TetraMark label="Choir" />
        </div>
        <div class="ln wordmark" data-shown={shown['a0-word'] ? '1' : '0'}><span>CHOIR</span></div>
        <div class="ln tagline" data-shown={shown['a0-tag'] ? '1' : '0'}><span>The automatic computer</span></div>
      </div>
    </section>

    <!-- ══ ACT · THE AMNESIA ══
         Timed so the conversation finishes and drains BEFORE the argument
         lands. They share one frame, so overlapping them is a collision, not
         a layering: the bubbles are crisp for 0–1.4s, they are gone by 2s,
         and the headline arrives at 1.9s into clear space. What remains is
         the residue at 12% — the memory of the thread, behind the claim. -->
    <section class="act" data-on={act === 1 ? '1' : '0'}>
      <div class="transcript" style:opacity={transcriptOpacity} aria-hidden="true">
        <div class="bubbles">
          <div class="bub" data-l={act === 1 && localTime > 180 ? '1' : '0'} data-d={act === 1 && localTime > 1500 ? '1' : '0'}>
            <span>Let&#39;s design a market entry for the EU. Mid-market, budget around 200k.</span>
          </div>
          <div class="bub out" data-l={act === 1 && localTime > 480 ? '1' : '0'} data-d={act === 1 && localTime > 1610 ? '1' : '0'}>
            <span>Got it — competitive landscape first, then a positioning brief and a launch sequence with KPIs.</span>
          </div>
          <div class="bub" data-l={act === 1 && localTime > 780 ? '1' : '0'} data-d={act === 1 && localTime > 1720 ? '1' : '0'}>
            <span>Make it bold. We&#39;re not a commodity vendor.</span>
          </div>
          <div class="bub out" data-l={act === 1 && localTime > 1080 ? '1' : '0'} data-d={act === 1 && localTime > 1830 ? '1' : '0'}>
            <span>Understood. Differentiated angle — premium positioning, narrow ICP, design-led narrative.</span>
          </div>
          <div class="bub" data-l={act === 1 && localTime > 1380 ? '1' : '0'} data-d={act === 1 && localTime > 1940 ? '1' : '0'}>
            <span>Good. Keep going.</span>
          </div>
        </div>
      </div>
      <div class="frame">
        <div class="kicker ln" data-shown={shown['a1-kicker'] ? '1' : '0'}>
          <span>01 — The amnesia</span>
        </div>
        <h1>
          <span class="ln" data-shown={shown['a1-h1a'] ? '1' : '0'}><span>Every AI session</span></span>
          <span class="ln" data-shown={shown['a1-h1b'] ? '1' : '0'}><span>dies at the tab.</span></span>
        </h1>
        <p class="sub ln" data-shown={shown['a1-sub'] ? '1' : '0'}>
          <span>You re-explain the project. It re-guesses the rules. The thread, the decisions, the dead ends — gone. Chat is a fine way to think for ten minutes and a bad way to run three months.</span>
        </p>
        <p class="voice ln" data-shown={shown['a1-voice'] ? '1' : '0'}>
          <span>&ldquo;I have definitely started this conversation before.&rdquo;</span>
        </p>
      </div>
    </section>

    <!-- ══ ACT · THE RENAME ══ -->
    <section class="act" data-on={act === 2 ? '1' : '0'}>
      <div class="tape-glow" style:--w={tapeWidth} aria-hidden="true"></div>
      <div class="tape" style:--w={tapeWidth} aria-hidden="true"></div>
      <div class="ticks" aria-hidden="true">
        {#each Array(26) as _, i}
          <i class="tick" data-shown={tapeWidth > i / 26 + 0.04 ? '1' : '0'} style:left="{(i / 25) * 100}%"></i>
        {/each}
      </div>
      <div class="frame">
        <div class="kicker ln" data-shown={shown['a2-kicker'] ? '1' : '0'}>
          <span>02 — The rename</span>
        </div>
        <h1>
          <span class="ln" data-shown={shown['a2-h1a'] ? '1' : '0'}><span>Choir isn&#39;t a chat.</span></span>
          <span class="ln" data-shown={shown['a2-h1b'] ? '1' : '0'}><span>It&#39;s a <em>computer</em>.</span></span>
        </h1>
        <p class="sub ln" data-shown={shown['a2-sub'] ? '1' : '0'}>
          <span>A persistent machine, not a session. Many agents coordinate on it for months, and every move they make lands on one versioned record you can read.</span>
        </p>
        <div class="pills">
          <span class="pill" data-shown={shown['a2-pill1'] ? '1' : '0'}>persistent</span>
          <span class="pill" data-shown={shown['a2-pill2'] ? '1' : '0'}>versioned</span>
          <span class="pill" data-shown={shown['a2-pill3'] ? '1' : '0'}>reversible</span>
          <span class="pill" data-shown={shown['a2-pill4'] ? '1' : '0'}>self-improving</span>
        </div>
      </div>
    </section>

    <!-- ══ ACT · THE ENSEMBLE ══ -->
    <section class="act act-center" data-on={act === 3 ? '1' : '0'}>
      <div class="desks" aria-hidden="true">
        <div class="desk desk-a" data-shown={shown['a3-desk0'] ? '1' : '0'} style:--orb="var(--choir-desk-texture)">
          <div class="orb"></div><b>Texture</b><small>writes</small>
        </div>
        <div class="desk desk-b" data-shown={shown['a3-desk1'] ? '1' : '0'} style:--orb="var(--choir-desk-management)">
          <div class="orb"></div><b>Management</b><small>decides</small>
        </div>
        <div class="desk desk-c" data-shown={shown['a3-desk2'] ? '1' : '0'} style:--orb="var(--choir-desk-engineering)">
          <div class="orb"></div><b>Engineering</b><small>builds</small>
        </div>
        <div class="desk desk-d" data-shown={shown['a3-desk3'] ? '1' : '0'} style:--orb="var(--choir-desk-research)">
          <div class="orb"></div><b>Research</b><small>verifies</small>
        </div>
      </div>
      <div class="frame frame-center">
        <div class="kicker ln" data-shown={shown['a3-kicker'] ? '1' : '0'}>
          <span>03 — The ensemble</span>
        </div>
        <h1>
          <span class="ln" data-shown={shown['a3-h1a'] ? '1' : '0'}><span>Four desks. One machine.</span></span>
          <span class="ln" data-shown={shown['a3-h1b'] ? '1' : '0'}><span><em>You</em> on top.</span></span>
        </h1>
        <p class="sub ln" data-shown={shown['a3-sub'] ? '1' : '0'}>
          <span>Texture writes. Management decides what runs. Engineering builds inside a sandbox. Research goes and gets evidence. You state the intent — they do the work, and they show you the receipts.</span>
        </p>
      </div>
    </section>

    <!-- ══ ACT · THE RECEIPTS ══ -->
    <section class="act" data-on={act === 4 ? '1' : '0'}>
      <div class="frame">
        <div class="kicker ln" data-shown={shown['a4-kicker'] ? '1' : '0'}>
          <span>04 — The receipts</span>
        </div>
        <h1>
          <span class="ln" data-shown={shown['a4-h1a'] ? '1' : '0'}><span>Nothing moves that you</span></span>
          <span class="ln" data-shown={shown['a4-h1b'] ? '1' : '0'}><span>can&#39;t read, cite, or undo.</span></span>
        </h1>
        <p class="sub ln" data-shown={shown['a4-sub'] ? '1' : '0'}>
          <span>Every state change is a typed event with evidence welded to it — and every one of them rolls back. This is an agent that has to show its work, permanently.</span>
        </p>
        <div class="cards">
          <article class="card" data-shown={shown['a4-card0'] ? '1' : '0'}>
            <h3><span>revision</span><i>v2 &rarr; v3</i></h3>
            <p>EU market entry brief</p>
            <div class="row"><span class="del">&minus; 412 words</span><span class="plus">+ 1,180 words</span></div>
            <div class="row"><span>authored by</span><b>texture</b></div>
          </article>
          <article class="card" data-shown={shown['a4-card1'] ? '1' : '0'}>
            <h3><span>commitment</span><i>scored</i></h3>
            <p>&ldquo;Regulatory filing lands in Q3.&rdquo;</p>
            <div class="meter"><i style:width="{meterWidth}%"></i></div>
            <div class="row"><span>resolved</span><b>0.81</b></div>
          </article>
          <article class="card" data-shown={shown['a4-card2'] ? '1' : '0'}>
            <h3><span>restore</span><i>&crarr;</i></h3>
            <p>Rolled back to event <b>#1,204</b></p>
            <div class="row"><span>reconstruction</span><b>exact</b></div>
            <div class="row"><span>receipt</span><b>retained</b></div>
          </article>
        </div>
      </div>
    </section>

    <!-- ══ ACT · THE INVITATION ══ -->
    <section class="act act-center" data-on={act === 5 ? '1' : '0'}>
      <div class="mark mark-veil" data-shown={shown['a5-mark'] ? '1' : '0'} aria-hidden="true">
        <TetraMark label="" />
      </div>
      <div class="frame frame-center">
        <div class="kicker ln" data-shown={shown['a5-kicker'] ? '1' : '0'}><span>Choir</span></div>
        <h1 class="h1-invite">
          <span class="ln" data-shown={shown['a5-h1'] ? '1' : '0'}><span>Wake the machine.</span></span>
        </h1>
        <p class="sub ln" data-shown={shown['a5-sub'] ? '1' : '0'}>
          <span>No password to forget. A passkey, and the computer is yours — a web desktop, a native app, and a CLI, all projections of the same persistent thing.</span>
        </p>
        <div class="cta" data-shown={shown['a5-cta'] ? '1' : '0'}>
          <button class="btn btn-go" type="button" data-landing-intro-enter on:click={handoff}>
            <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
              <path d="M4.5 10.5v-3a7.5 7.5 0 0 1 15 0v3" />
              <rect x="4.5" y="10.5" width="15" height="9.5" rx="2.4" />
              <circle cx="12" cy="14.6" r="1.4" />
            </svg>
            Open the desktop
          </button>
          <button class="btn btn-ghost" type="button" on:click={restart}>Watch again</button>
        </div>
        <p class="footnote ln" data-shown={shown['a5-note'] ? '1' : '0'}>
          <span>Early and fast-moving. Real substrate, early product — the repository is the honest version.</span>
        </p>
      </div>
    </section>
  </div>

  <!-- ══ CHROME ══ -->
  <div class="meta" data-shown={handedOff ? '0' : '1'}>
    <span class="pulse"></span>
    <span>{ACTS[act].label}</span>
  </div>

  <div class="dots" role="tablist" aria-label="Introduction acts">
    {#each ACTS as entry, i}
      <button
        class="dot"
        class:dot-on={i === act}
        type="button"
        role="tab"
        aria-selected={i === act}
        aria-label={entry.label}
        data-landing-intro-dot
        on:click={() => gotoAct(i)}
      >
        <i style:width="{dotProgress[i] * 100}%"></i>
      </button>
    {/each}
  </div>

  <button class="skip" type="button" data-shown={handedOff ? '0' : '1'} data-landing-intro-skip on:click={handoff}>
    <span>Skip</span><kbd>esc</kbd>
  </button>

  <div class="rail"><div class="rail-fill" style:width="{progress * 100}%"></div></div>
</div>

<style>
  .intro {
    position: fixed;
    inset: 0;
    /*
      Stacking band. The film sits above the whole desktop plane — windows,
      the prompt surface (10000), toasts — and below the real dialogs
      (DeskSheet 9998/9999, auth overlay 20000, Desktop Overview 13000).

      Every route to a dialog passes through an interaction that retires the
      film first (onDesktopTouch), so nothing can end up trapped underneath
      it. The one real conflict is the prompt bar, which shares the bottom
      band with the act rail and the skip control: the film must cover it,
      and because the scrim is pointer-events:none the bar stays operable
      underneath. Touching it simply ends the film.
    */
    z-index: 10001;
    color: var(--choir-text-primary);
    font-family: var(--choir-font-ui);
    isolation: isolate;
    /*
      `overflow: clip`, deliberately not `hidden`.

      `hidden` still creates a scroll container. Every entrance in this
      component animates with a transform, and transformed boxes contribute
      to scrollable overflow — so `hidden` gave this overlay 500px of hidden
      scroll range, and anything that called scrollIntoView (a click, a
      focus, a Playwright locator) silently scrolled the whole film upward
      and broke every layout in it. `clip` clips without ever becoming
      scrollable, which is the behaviour this overlay actually wants.
    */
    overflow: clip;
    /*
      Non-blocking by construction: the scrim never captures pointer
      events, so the live desktop underneath stays operable. Any touch of
      that desktop ends the film.
    */
    pointer-events: none;
  }
  .intro :global(button) { pointer-events: auto; }

  /* ── layers ───────────────────────────────────────────────────────
     PERFORMANCE NOTE — this block used three expensive things at once and
     together they cost roughly 90% of the frame budget:

       1. `backdrop-filter: blur(38px)` across the full viewport.
       2. `filter: blur(110px)` on the aurora.
       3. A `mask-image: radial-gradient` on the animated lattice.

     All three are gone, and none of them were buying anything:

       · The scrim no longer blurs the desktop. At 98% opacity the desktop
         behind is already a faint silhouette; the blur was refining a
         difference nobody can see, at the cost of a full-viewport
         read-back every frame.
       · The aurora is three large soft radial-gradients with no filter at
         all. Blurring an already-soft gradient is pure waste, and the
         gradient can carry the same slow drift as a transform.
       · The lattice lost its radial mask and its slide animation. A plain
         low-alpha grid reads the same at 0.2 and costs nothing.

     Measured on the local preview at 1440x900 under software rasterisation
     this took the landing from 6fps to a locked frame rate. Motion that
     costs a third of the budget is not motion design, it is a tax. */

  /*
    The scrim. Without it the film is unreadable: the public desktop is
    mounted underneath, live, and a 106px headline over a live document is
    not a design problem you can solve with type colour.
  */
  .intro-scrim {
    position: absolute; inset: 0; z-index: 0;
    background: color-mix(in srgb, var(--choir-bg) 98%, transparent);
  }

  .intro-aurora {
    position: absolute; inset: 0; z-index: 0;
  }
  .intro-aurora i {
    position: absolute; display: block; border-radius: 50%;
    will-change: transform;
  }
  .intro-aurora i:nth-child(1) {
    width: 88vw; height: 88vw; left: -34vw; top: -36vw;
    background: radial-gradient(circle, var(--choir-state-active-glow) 0%, transparent 52%);
    animation: drift-a 18s var(--choir-ease-drift) infinite;
  }
  .intro-aurora i:nth-child(2) {
    width: 80vw; height: 80vw; right: -32vw; bottom: -34vw;
    background: radial-gradient(circle, var(--choir-state-focus) 0%, transparent 52%);
    animation: drift-b 18s var(--choir-ease-drift) infinite;
  }
  .intro-aurora i:nth-child(3) {
    width: 62vw; height: 62vw; left: 30%; top: 24%;
    background: radial-gradient(circle, var(--choir-field-halo) 0%, transparent 54%);
    animation: drift-c 11s var(--choir-ease-drift) infinite;
  }
  @keyframes drift-a {
    0%, 100% { transform: translate3d(0, 0, 0) scale(1); }
    50% { transform: translate3d(9vw, 7vh, 0) scale(1.16); }
  }
  @keyframes drift-b {
    0%, 100% { transform: translate3d(0, 0, 0) scale(1.08); }
    50% { transform: translate3d(-8vw, -6vh, 0) scale(0.92); }
  }
  @keyframes drift-c {
    0%, 100% { transform: translate3d(0, 0, 0) scale(1); }
    50% { transform: translate3d(-6vw, 9vh, 0) scale(1.24); }
  }

  /* There is deliberately no lattice layer. It was here for two acts of this
     piece and it earned neither: the ChoirField already supplies all the
     structure the frame wants, and a 0.14-alpha grid over a star field only
     competes with it. Removing it also removed a full-frame animated
     repaint. Structure comes from the graph, not from graph paper. */

  .intro :global(.choir-field) { z-index: 2; }

  .intro-vignette {
    position: absolute; inset: 0; z-index: 5;
    background: radial-gradient(ellipse 78% 70% at 50% 46%, transparent 32%,
      color-mix(in srgb, var(--choir-shadow-color) 74%, transparent) 100%);
  }

  .intro-acts { position: absolute; inset: 0; z-index: 4; }
  /* ── acts ───────────────────────────────────────────────────────── */
  .act {
    position: absolute; inset: 0;
    display: grid; place-items: center;
    padding: clamp(2rem, 7vh, 5rem) clamp(1.5rem, 7vw, 6rem);
    opacity: 0; visibility: hidden;
  }
  .act[data-on='1'] { opacity: 1; visibility: visible; }

  .frame {
    width: min(100%, 62rem);
    display: grid;
    gap: clamp(1rem, 2.4vh, 1.6rem);
  }
  .frame-center { justify-items: center; text-align: center; }

  /* ── entrance system ────────────────────────────────────────────────
     Every entrance is a transform/opacity/filter transition, so nothing
     in this component ever reflows and every entrance is composited.

     Duration and timing are composed from separate tokens rather than a
     single composite value: naming a duration twice in one shorthand
     (`opacity 0.8s var(--x)` where --x already carries a duration) voids
     the entire declaration, and the failure is invisible — the elements
     just never move. */
  .ln, .mark, .pill, .card, .desk, .cta, .kicker, .meta, .skip {
    opacity: 0;
    transform: translate3d(0, 16px, 0);
    filter: blur(7px);
    transition-property: opacity, transform, filter;
    transition-duration: 780ms, var(--choir-duration-entrance), 680ms;
    transition-timing-function: var(--choir-ease);
  }
  [data-shown='1'] { opacity: 1; transform: none; filter: none; }
  .cta[data-shown='1'] { transform: none; }

  .mark { width: clamp(74px, 10vw, 116px); }
  .mark :global(svg) { width: 100%; height: 100%; }
  .mark[data-shown='1'] { transform: none; transition-duration: 1500ms; }
  .mark-veil {
    position: absolute; left: 50%; top: 50%;
    width: clamp(300px, 44vw, 620px);
    margin: 0;
    z-index: -1;
  }
  .mark-veil :global(path) { fill: var(--choir-tetramark-color); opacity: 0.13; }
  .mark-veil[data-shown='1'] { transform: translate(-50%, -50%) scale(1); }

  .kicker {
    display: flex; align-items: center; gap: 0.7rem;
    font-size: clamp(0.62rem, 1vw, 0.74rem);
    font-weight: 800; letter-spacing: 0.26em; text-transform: uppercase;
    color: var(--choir-text-accent);
  }
  .kicker::before {
    content: ''; width: clamp(24px, 4vw, 54px); height: 1px;
    background: linear-gradient(90deg, var(--choir-accent), transparent);
  }

  h1 {
    font-family: var(--choir-font-display);
    font-size: clamp(2.15rem, 7.4vw, 5.1rem);
    font-weight: 800; line-height: 0.94; letter-spacing: -0.035em;
    max-width: 19ch; text-wrap: balance;
    filter: drop-shadow(0 6px 34px var(--choir-state-active-glow));
  }
  h1 em {
    font-family: var(--choir-font-display);
    font-style: italic; font-weight: 400; letter-spacing: -0.015em;
    background: linear-gradient(96deg, var(--choir-text-accent), var(--choir-tetramark-color) 60%, var(--choir-accent));
    -webkit-background-clip: text; background-clip: text; color: transparent;
  }
  .frame-center h1 { margin-inline: auto; }
  .h1-invite { font-size: clamp(2.5rem, 9vw, 6.2rem); max-width: none; }

  .sub {
    font-size: clamp(0.94rem, 1.55vw, 1.16rem);
    line-height: 1.62; color: var(--choir-text-muted);
    max-width: 56ch; font-weight: 380;
  }
  .frame-center .sub { margin-inline: auto; }

  .voice {
    font-family: var(--choir-font-ui);
    font-style: italic;
    font-size: clamp(1rem, 1.9vw, 1.35rem);
    color: var(--choir-tetramark-color);
    padding-left: 1.05rem;
    border-left: 1px solid var(--choir-border-strong);
    max-width: 44ch;
  }

  .ln { display: block; overflow: hidden; padding-bottom: 0.06em; }
  .ln > span {
    display: block; transform: translate3d(0, 105%, 0); opacity: 0;
    transition-property: transform, opacity;
    transition-duration: var(--choir-duration-entrance), 800ms;
    transition-timing-function: var(--choir-ease);
  }
  .ln[data-shown='1'] > span { transform: none; opacity: 1; }

  .wordmark {
    font-size: clamp(1.5rem, 4.4vw, 2.6rem); font-weight: 200;
    letter-spacing: 0.52em; text-indent: 0.52em;
  }
  .tagline {
    font-size: clamp(0.7rem, 1.2vw, 0.84rem); font-weight: 700;
    letter-spacing: 0.3em; text-transform: uppercase; color: var(--choir-text-subtle);
  }

  /* ── act · amnesia ──────────────────────────────────────────────── */
  .transcript {
    position: absolute; inset: 0;
    display: grid; place-items: center;
    transition: opacity 1100ms var(--choir-ease-in);
  }
  .bubbles {
    display: grid; gap: 0.72rem;
    width: min(100%, 44rem);
  }
  .bub { display: flex; }
  .bub.out { justify-content: flex-end; }
  .bub span {
    max-width: 78%; padding: 0.68rem 1.02rem; border-radius: 16px;
    font-size: clamp(0.78rem, 1.28vw, 0.94rem); line-height: 1.5;
    background: var(--choir-surface-control);
    border: 1px solid var(--choir-border);
    color: var(--choir-text-muted);
    opacity: 0; transform: translate3d(0, 16px, 0) scale(0.97); filter: blur(5px);
    transition-property: opacity, transform, filter, color, border-color;
    transition-duration: 550ms, 550ms, 550ms, 900ms, 900ms;
    transition-timing-function: var(--choir-ease), var(--choir-ease),
      var(--choir-ease), var(--choir-ease-in), var(--choir-ease-in);
  }
  .bub.out span {
    background: var(--choir-state-selected);
    border-color: var(--choir-border-strong);
    color: var(--choir-text-primary);
  }
  .bub[data-l='1'] span { opacity: 1; transform: none; filter: none; }
  /* the amnesia: everything drains, blurs and falls away */
  .bub[data-d='1'] span {
    opacity: 0.16; transform: translate3d(0, 74px, 0) scale(0.9);
    filter: blur(11px) saturate(0);
    transition-duration: 1.1s;
  }

  /* ── act · rename ───────────────────────────────────────────────── */
  .tape {
    position: absolute; left: 0; right: 0; top: 82%; height: 2px; z-index: 1;
    transform: translate3d(0, -50%, 0) scaleX(var(--w, 0)); transform-origin: 0 50%;
    background: linear-gradient(90deg, transparent, var(--choir-accent) 8%, var(--choir-text-accent) 55%, transparent);
    box-shadow: 0 0 26px var(--choir-state-focus), 0 0 70px var(--choir-state-active-glow);
  }
  .tape-glow {
    position: absolute; left: 0; right: 0; top: 82%; height: 300px; z-index: 0;
    transform: translate3d(0, -50%, 0) scaleX(var(--w, 0)); transform-origin: 0 50%;
    background: radial-gradient(ellipse 70% 100% at 32% 50%, var(--choir-state-focus), transparent 74%);
    filter: blur(30px);
  }
  .ticks { position: absolute; inset: 0; z-index: 2; }
  .tick {
    position: absolute; top: 82%; width: 9px; height: 9px; margin: -4.5px 0 0 -4.5px;
    border-radius: 50%; background: var(--choir-bg);
    border: 1.5px solid var(--choir-text-accent);
    box-shadow: 0 0 14px var(--choir-state-focus);
    opacity: 0; transform: scale(0);
  }
  .tick[data-shown='1'] {
    opacity: 1; transform: scale(1);
    transition-property: opacity, transform;
    transition-duration: 400ms, 700ms;
    transition-timing-function: var(--choir-ease), var(--choir-ease-spring);
  }

  .pills { display: flex; flex-wrap: wrap; gap: 0.5rem; margin-top: 0.3rem; }
  .pill {
    font-family: var(--choir-font-mono);
    font-size: clamp(0.66rem, 1.1vw, 0.78rem);
    letter-spacing: 0.06em; padding: 0.4rem 0.8rem; border-radius: 999px;
    border: 1px solid var(--choir-border-strong);
    background: var(--choir-state-hover);
    color: var(--choir-tetramark-color);
  }

  /* ── act · ensemble ─────────────────────────────────────────────── */
  .desks { position: absolute; inset: 0; z-index: 1; }
  .desk {
    position: absolute;
    display: grid; justify-items: center; gap: 0.5rem;
    width: min(23vw, 190px);
    transform: translate(-50%, -50%) scale(0.32);
    transition-property: opacity, transform, filter;
    transition-duration: 900ms, 1150ms, 800ms;
    transition-timing-function: var(--choir-ease), var(--choir-ease-spring), var(--choir-ease);
  }
  .desk[data-shown='1'] { transform: translate(-50%, -50%) scale(1); }
  .desk-a { left: 23.5%; top: 28.5%; }
  .desk-b { left: 71.5%; top: 24.5%; }
  .desk-c { left: 28.5%; top: 75.5%; }
  .desk-d { left: 74.5%; top: 71.5%; }
  .orb {
    width: clamp(48px, 5.6vw, 74px); aspect-ratio: 1; border-radius: 50%;
    background: radial-gradient(circle at 34% 30%, var(--choir-text-primary), var(--orb) 42%, transparent 72%);
    box-shadow:
      0 0 0 1px color-mix(in srgb, var(--orb) 55%, transparent),
      0 0 30px color-mix(in srgb, var(--orb) 70%, transparent),
      0 0 76px color-mix(in srgb, var(--orb) 38%, transparent);
    position: relative;
  }
  .orb::after {
    content: ''; position: absolute; inset: -16%; border-radius: 50%;
    border: 1px solid color-mix(in srgb, var(--orb) 34%, transparent);
    animation: halo 3.4s var(--choir-duration-sheet) var(--choir-ease) infinite;
  }
  @keyframes halo {
    0%, 100% { transform: scale(1); opacity: 0.7; }
    50% { transform: scale(1.3); opacity: 0; }
  }
  .desk b {
    font-size: clamp(0.6rem, 1.05vw, 0.74rem); font-weight: 800;
    letter-spacing: 0.19em; text-transform: uppercase;
    color: var(--choir-text-primary); white-space: nowrap;
  }
  .desk small {
    font-family: var(--choir-font-mono);
    font-size: clamp(0.58rem, 0.95vw, 0.68rem);
    color: var(--choir-text-subtle); letter-spacing: 0.05em; white-space: nowrap;
  }

  /* ── act · receipts ─────────────────────────────────────────────── */
  .cards {
    display: grid; grid-template-columns: repeat(3, 1fr);
    gap: clamp(0.6rem, 1.4vw, 1.1rem);
    perspective: 1200px;
  }
  .card {
    border-radius: 16px; padding: clamp(0.8rem, 1.5vw, 1.15rem);
    background: var(--choir-surface-pane);
    border: 1px solid var(--choir-border);
    box-shadow: var(--choir-shadow-soft);
    display: grid; gap: 0.5rem; align-content: start;
    transform: translate3d(0, 44px, -140px) rotateX(24deg);
    filter: blur(8px);
    transition-duration: 1s, 1.15s, 0.9s;
  }
  .card[data-shown='1'] { transform: none; filter: none; }
  .card h3 {
    font-family: var(--choir-font-mono);
    font-size: clamp(0.6rem, 1.05vw, 0.72rem);
    font-weight: 500; letter-spacing: 0.05em;
    color: var(--choir-text-subtle); text-transform: uppercase;
    display: flex; justify-content: space-between; gap: 0.5rem;
  }
  .card h3 i { font-style: normal; color: var(--choir-status-success); }
  .card p {
    font-size: clamp(0.74rem, 1.22vw, 0.88rem); line-height: 1.5;
    color: var(--choir-text-muted);
  }
  .card .row {
    font-family: var(--choir-font-mono);
    font-size: clamp(0.6rem, 1vw, 0.7rem);
    color: var(--choir-text-subtle);
    display: flex; justify-content: space-between; gap: 0.6rem;
    padding-top: 0.4rem; border-top: 1px solid var(--choir-border);
  }
  .card .row b { color: var(--choir-text-primary); font-weight: 600; }
  .card .row .plus { color: var(--choir-status-success); }
  .card .row .del { color: var(--choir-status-danger); }
  .meter {
    height: 3px; border-radius: 999px;
    background: var(--choir-border); overflow: hidden;
  }
  .meter i {
    display: block; height: 100%; border-radius: 999px;
    background: linear-gradient(90deg, var(--choir-accent), var(--choir-text-accent), var(--choir-status-success));
  }

  /* ── act · invitation ───────────────────────────────────────────── */
  .cta {
    display: flex; flex-wrap: wrap; gap: 0.7rem;
    justify-content: center; align-items: center; margin-top: 0.4rem;
  }
  .btn {
    position: relative; overflow: hidden;
    display: inline-flex; align-items: center; gap: 0.6rem;
    padding: 1rem 1.7rem; border-radius: 999px;
    font-size: clamp(0.82rem, 1.3vw, 0.95rem); font-weight: 800;
    transition-property: transform, box-shadow, filter;
    transition-duration: var(--choir-duration-sheet), var(--choir-duration-sheet), var(--choir-duration-sheet);
    transition-timing-function: var(--choir-ease-spring), var(--choir-ease), ease;
  }
  .btn svg {
    width: 1.05rem; height: 1.05rem;
    fill: none; stroke: currentColor; stroke-width: 1.9;
    stroke-linecap: round; stroke-linejoin: round;
  }
  .btn-go {
    background: linear-gradient(120deg, var(--choir-accent), var(--choir-text-accent));
    color: var(--choir-text-on-accent);
    box-shadow: 0 14px 42px var(--choir-state-focus), 0 0 0 1px var(--choir-border-strong) inset;
  }
  .btn-go:hover { filter: brightness(1.08); transform: translateY(-2px); }
  .btn-ghost {
    color: var(--choir-text-muted);
    border: 1px solid var(--choir-border);
    background: var(--choir-surface-control);
  }
  .btn-ghost:hover { color: var(--choir-text-primary); background: var(--choir-state-hover); }
  .btn:focus-visible { outline: 2px solid var(--choir-accent); outline-offset: 3px; }

  .footnote {
    font-size: clamp(0.66rem, 1.1vw, 0.78rem);
    color: var(--choir-text-subtle); letter-spacing: 0.03em;
  }

  /* ── chrome ────────────────────────────────────────────────────── */
  .meta {
    position: absolute; z-index: 9;
    top: clamp(1.1rem, 3vh, 1.9rem); right: clamp(1.2rem, 3vw, 2.4rem);
    font-family: var(--choir-font-mono);
    font-size: 0.63rem; letter-spacing: 0.13em; text-transform: uppercase;
    color: var(--choir-text-subtle);
    display: flex; gap: 0.7rem; align-items: center;
  }
  .pulse {
    width: 5px; height: 5px; border-radius: 50%;
    background: var(--choir-status-success);
    box-shadow: 0 0 10px var(--choir-status-success);
    animation: beat 1.7s var(--choir-duration-sheet) var(--choir-ease) infinite;
  }
  @keyframes beat {
    0%, 100% { opacity: 1; transform: scale(1); }
    50% { opacity: 0.35; transform: scale(0.72); }
  }

  .dots {
    position: absolute; z-index: 9;
    left: clamp(1.2rem, 3vw, 2.4rem); bottom: clamp(1.1rem, 3vh, 1.9rem);
    display: flex; gap: 0.5rem; align-items: center;
  }
  .dot {
    position: relative; width: 26px; height: 3px; border-radius: 999px;
    background: var(--choir-border); overflow: hidden;
    transition-property: width, background;
    transition-duration: 450ms, 450ms;
    transition-timing-function: var(--choir-ease), ease;
  }
  .dot i {
    position: absolute; inset: 0 auto 0 0;
    background: var(--choir-text-accent);
    box-shadow: 0 0 10px var(--choir-state-focus);
  }
  .dot-on { width: 44px; background: var(--choir-border-strong); }
  .dot:hover { background: var(--choir-border-strong); }
  .dot:focus-visible { outline: 2px solid var(--choir-accent); outline-offset: 4px; }

  .skip {
    position: absolute; z-index: 9;
    right: clamp(1.2rem, 3vw, 2.4rem); bottom: clamp(1rem, 2.6vh, 1.7rem);
    display: flex; align-items: center; gap: 0.6rem;
    font-size: 0.68rem; font-weight: 700; letter-spacing: 0.19em; text-transform: uppercase;
    color: var(--choir-text-subtle);
    padding: 0.55rem 0.95rem; border-radius: 999px;
    border: 1px solid var(--choir-border); background: var(--choir-surface-control);
    transition-property: color, border-color, transform, opacity, filter;
    transition-duration: 300ms, 300ms, 300ms, 600ms, 500ms;
    transition-timing-function: ease, ease, var(--choir-ease-spring), var(--choir-ease), var(--choir-ease);
  }
  .skip:hover { color: var(--choir-text-primary); border-color: var(--choir-border-strong); transform: translateY(-2px); }
  .skip kbd {
    font-family: var(--choir-font-mono); font-size: 0.6rem; letter-spacing: 0.04em;
    padding: 0.14rem 0.4rem; border-radius: 5px;
    background: var(--choir-state-hover); color: var(--choir-text-muted);
  }

  .rail { position: absolute; left: 0; right: 0; bottom: 0; height: 3px; z-index: 8; background: var(--choir-border); }
  .rail-fill {
    height: 100%;
    background: linear-gradient(90deg, var(--choir-accent), var(--choir-text-accent));
    box-shadow: 0 0 14px var(--choir-state-focus);
  }

  /*
    Reduced motion: the film does not play at all.

    An earlier draft had a static, scrollable "same story, standing still"
    mode. It was cut. The argument this sequence makes is also made in full,
    in prose, by the signed-out preview document — so a visitor who asked
    for reduced motion gets a readable page instead of a film, and there is
    no second layout mode to keep honest. Retiring the film here means the
    desktop underneath is the whole experience for them, immediately.
  */
  .intro-still :global(.choir-field) { opacity: 0.4; }

  @media (max-width: 720px) {
    .cards { grid-template-columns: 1fr; }
    .desk { width: min(38vw, 150px); }
    .desk-b, .desk-d { display: none; }
    h1 { max-width: 100%; }
    .transcript .bub span { max-width: 92%; }
  }
</style>
