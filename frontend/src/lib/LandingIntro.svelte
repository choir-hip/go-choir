<!--
  LandingIntro — the signed-out landing, as a three pane deck.

  Pane 1  the title card
  Pane 2  "Choir isn't a chat. It's a computer."
  Pane 3  the actual web desktop, live, with nothing to press

  ── Why it is a virtual scroll and not a native one ────────────────────
  The desktop is mounted underneath this overlay and must stay operable at
  every moment — that is what makes the landing non-blocking. A real scroll
  container covering the viewport would eat every click and drag aimed at
  the desktop, and would break every test that reaches for a desktop icon.

  So the track is a transform. A single `translate3d` moves three panes past
  a fixed viewport, wheel/touch/keys drive a float position, and the overlay
  never captures a pointer event. Everything the deck does is a pure
  function of that one float.

  ── Why that matters for the motion ────────────────────────────────────
  Because position is continuous, the choreography is scroll-linked rather
  than timed. The mark does not "play" then "leave" — it is mid-flight at
  any scroll offset, and it is exactly where the user's hand put it. The
  big move is the handoff on pane 2: the word *computer* scales up through
  the camera and dissolves, and the desktop is already running behind it.
  The word is the doorway, which is the whole argument for a landing page
  whose last pane is the product rather than a pitch for it.

  ── Contracts this must not break ──────────────────────────────────────
  · Adds no [data-window]; the seeded public preview window stays exactly one.
  · Never calls .focus(), so the prompt autofocus contract holds.
  · Never claims Escape, and never captures pointer events, so the auth
    overlay and the live desktop keep their own behaviour.
  · Tokens only — theme-contract forbids raw colour literals outside theme.ts.
  · reduced-motion is not a retirement here. Nothing autoplays, so a visitor
    who asked for stillness still gets the full narrative by scrolling — it
    is simply standing still while they do it.

  Data attributes for test targeting:
    data-landing-intro            — root overlay while the deck is showing
    data-landing-intro-pane       — nearest pane index, 0-based
    data-landing-intro-reveal     — 0..1 hand-off progress into the desktop
    data-landing-intro-desktop    — "1" once the desktop pane is reached
    data-landing-intro-continue   — the advance control
    data-landing-intro-rail       — the progress rail
-->
<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';
  import ChoirField from './ChoirField.svelte';
  import TetraMark from './TetraMark.svelte';

  export let reduced = false;

  const dispatch = createEventDispatcher();

  /** Total panes. The last one is the desktop, not a slide. */
  const PANES = 3;
  const LAST = PANES - 1;

  let root: HTMLElement | null = null;
  let pos = 0;          // current position, in pane units (0 .. LAST)
  let target = 0;       // where we are heading
  let velocity = 0;     // pane units per second, for snap projection
  let dragging = false;
  let dragFrom = 0;
  let dragStartPos = 0;
  let dragStartAt = 0;
  let lastInteractionAt = 0;
  let lastTouchY = 0;
  let lastTouchAt = 0;
  let hinted = false;
  let desktopAnnounced = false;

  let pane = 0;
  let arrive = 0;
  let reveal = 0;
  let trackY = 0;
  let railFill = 0;
  let railPips = [0, 0, 0];

  const clamp = (v: number, a: number, b: number) => Math.min(b, Math.max(a, v));
  const clamp01 = (v: number) => clamp(v, 0, 1);
  /** Frame-rate independent damping. Feels the same at 30fps and 144fps. */
  const damp = (a: number, b: number, rate: number, dt: number) =>
    a + (b - a) * (1 - Math.exp(-rate * dt));

  // ── input ───────────────────────────────────────────────────────────
  /**
   * Gesture state, explicit rather than inferred from a timer.
   *
   * The first version decided a gesture was over by asking whether 110ms
   * had passed since the last event. That is fine for a trackpad and broken
   * for a notched wheel, which delivers a notch roughly every 100ms — so
   * every notch looked like the end of a gesture, the deck snapped back
   * toward the pane it had left, and a wheel user could not cross a single
   * pane. The hint said "scroll", and the wheel did not work.
   *
   * So: a gesture stays alive until the input has genuinely paused, and
   * nothing touches the target position while it is.
   */
  const IDLE_MS = 300;
  /** Pane fraction per viewport-height of wheel delta. A notched wheel
   *  sends ~45px a notch, so this puts a pane about six notches away. */
  const WHEEL_GAIN = 0.34;

  let gestureActive = false;

  function beginGesture() {
    gestureActive = true;
    lastInteractionAt = performance.now();
    if (!hinted) hinted = true;
  }

  /**
   * Route a "go to pane n" request to whichever driver is live, so the
   * buttons, the rail and the keyboard all work identically on both.
   */
  function scrollToPane(index: number) {
    const n = clamp(index, 0, LAST);
    if (nativeScroll) {
      const h = scroller?.clientHeight || window.innerHeight;
      scroller?.scrollTo({ top: n * h, behavior: reduced ? 'auto' : 'smooth' });
    } else {
      target = n;
    }
    beginGesture();
  }

  function currentPane(): number {
    if (nativeScroll) {
      const h = scroller?.clientHeight || window.innerHeight;
      return h > 0 ? Math.round((scroller?.scrollTop || 0) / h) : 0;
    }
    return Math.round(target);
  }

  function advance(by: number) {
    scrollToPane(currentPane() + by);
  }

  function goTo(index: number) {
    scrollToPane(index);
  }

  function onWheel(event: WheelEvent) {
    if (event.ctrlKey) return;                       // pinch-zoom belongs to the browser
    // Never claim the wheel from a real surface that wants it.
    const el = event.target as HTMLElement | null;
    if (el?.closest?.('[data-auth-overlay], [data-desk-sheet], [data-desktop-overview]')) return;
    event.preventDefault();
    const unit = event.deltaMode === 1 ? 32 : event.deltaMode === 2 ? 800 : 1;
    const dy = event.deltaY * unit;
    // A trackpad gives many small deltas and a wheel gives a few large ones.
    // Both map to the same pane fraction, so the deck feels the same either way.
    target = clamp(target + dy / Math.max(240, window.innerHeight * WHEEL_GAIN), 0, LAST);
    beginGesture();
  }

  /**
   * Touch scrub, opted out when the gesture starts on something that owns
   * the gesture. Without this list a drag on a window would move the deck
   * and a drag on the deck would do nothing — the exact inverse of correct.
   */
  function ownsDrag(target_: EventTarget | null): boolean {
    const el = target_ as HTMLElement | null;
    if (!el || typeof el.closest !== 'function') return true;
    return !el.closest(
      '[data-window], [data-floating-window], [data-prompt-surface], [data-desktop-icon], ' +
        '[data-auth-overlay], [data-desk-sheet], [data-desktop-overview], button, a, input, textarea, ' +
        '[contenteditable="true"], [data-settings-app], [data-file-browser]',
    );
  }

  function onPointerDown(event: PointerEvent) {
    if (event.pointerType === 'mouse') return;
    if (!ownsDrag(event.target)) return;
    dragging = true;
    dragFrom = event.clientY;
    dragStartPos = target;
    lastTouchY = event.clientY;
    lastTouchAt = performance.now();
    beginGesture();
  }

  function onPointerMove(event: PointerEvent) {
    if (!dragging || event.pointerType === 'mouse') return;
    const now = performance.now();
    const dy = dragFrom - event.clientY;
    const dt = Math.max(1, now - lastTouchAt) / 1000;
    velocity = (lastTouchY - event.clientY) / dt / Math.max(360, window.innerHeight * 0.62);
    lastTouchY = event.clientY;
    lastTouchAt = now;
    target = clamp(dragStartPos + dy / Math.max(240, window.innerHeight * WHEEL_GAIN), 0, LAST);
    beginGesture();
  }

  function onPointerUp() {
    if (!dragging) return;
    dragging = false;
    // Project the throw, then land on the nearest pane. A flick goes one
    // pane; a slow drag lands where you left it.
    const thrown = target + velocity * 0.18;
    const from = dragStartPos;
    const moved = Math.abs(target - from);
    target = moved < 0.12 && Math.abs(velocity) < 0.6
      ? clamp(Math.round(from), 0, LAST)
      : clamp(Math.round(thrown), 0, LAST);
    velocity = 0;
    beginGesture();
  }

  function onKeydown(event: KeyboardEvent) {
    // A dialog owns the keyboard while it is open.
    if (document.querySelector('[role="dialog"], [data-auth-overlay]')) return;
    // Once the deck has arrived on a touch device it has stood down, and the
    // machine underneath owns arrow keys for its own navigation.
    if (nativeScroll && arrived) return;
    const k = event.key;
    if (k === 'ArrowDown' || k === 'PageDown' || k === ' ') { event.preventDefault(); advance(1); }
    else if (k === 'ArrowUp' || k === 'PageUp') { event.preventDefault(); advance(-1); }
    else if (k === 'Home') { event.preventDefault(); goTo(0); }
    else if (k === 'End') { event.preventDefault(); goTo(LAST); }
  }

  // ── two drivers, one choreography ───────────────────────────────────
  /**
   * Fine pointers scroll the deck by transform; coarse pointers scroll it
   * natively. Both produce the same `pos`, and everything downstream is
   * identical, so there is exactly one piece of choreography.
   *
   * Why two: the deck is deliberately `pointer-events: none` so the desktop
   * underneath stays live, and a native scroll container covering the
   * viewport cannot both scroll and let the desktop keep its clicks. But
   * that trick is only available to a pointer the deck is allowed to see.
   *
   * Touch is not that pointer. A phone has no wheel, so on a phone the deck
   * had exactly one input — the buttons — because the virtual scroll's
   * touch path was being refused. `ownsDrag` declines a drag that starts on
   * a window, which is correct for a laptop and useless on a phone, where
   * the preview window covers the entire viewport and so claims essentially
   * every touch on screen.
   *
   * On a coarse pointer the deck stops being transparent and starts being a
   * real scroller: `overflow-y: scroll`, proximity snap, momentum and snap
   * handled by the platform. That is what a phone expects,
   * and it is better than anything a virtual scroll would have given us —
   * rubber-banding, fling, and OS-level accessibility come free.
   *
   * The cost is honest: while the deck is up on a phone it owns the screen,
   * because two full-screen surfaces cannot both have the touches. So on
   * arrival it stands down — the root goes `pointer-events: none` and the
   * machine underneath becomes live and touchable, which is the whole point
   * of a third pane that is the desktop.
   */
  let scroller: HTMLElement | null = null;
  let nativeScroll = false;

  /** True once the deck has arrived, so the machine can take the touches. */
  $: arrived = reveal > 0.985;

  function backToStart() {
    if (!scroller) return;
    scroller.scrollTo({ top: 0, behavior: reduced ? 'auto' : 'smooth' });
    beginGesture();
  }

  // ── the clock ───────────────────────────────────────────────────────
  let last = 0;
  let raf = 0;

  function frame(now: number) {
    raf = requestAnimationFrame(frame);
    if (!last) { last = now; return; }
    const dt = Math.min((now - last) / 1000, 0.05);
    last = now;

    if (nativeScroll) {
      // The platform owns the rest position here, so the deck reads it
      // rather than deciding it. Momentum and snap therefore drive the
      // choreography directly, which is what makes it feel attached to the
      // finger instead of animated alongside it.
      const h = scroller?.clientHeight || window.innerHeight;
      pos = h > 0 ? (scroller?.scrollTop || 0) / h : 0;
    } else {
      // Settle onto a pane once the gesture is genuinely over. While a
      // gesture is alive the target is untouchable — that is the whole point
      // of the flag, and it is why a notched wheel can now cross a pane.
      if (!gestureActive && now - lastInteractionAt > IDLE_MS) {
        const settled = Math.round(target);
        if (Math.abs(target - settled) < 0.0015) target = settled;
        else target = damp(target, settled, 9, dt);
      }
      if (gestureActive && now - lastInteractionAt > IDLE_MS) gestureActive = false;

      pos = reduced ? target : damp(pos, target, dragging ? 26 : 7.5, dt);
      if (Math.abs(pos - target) < 0.0004) pos = target;
      trackY = pos;
    }
    // Pane 2 arriving, 0 .. 1 — drives its entrance AND pane 1's exit.
    arrive = clamp01(pos);
    // How far the desktop hand-off has progressed, 0 .. 1.
    reveal = clamp01(pos - 1);
    pane = Math.round(pos);
    railFill = clamp01(pos / LAST);
    railPips = [clamp01(1 - pos), clamp01(1 - Math.abs(pos - 1)), reveal];

    // The root property is how the deck drives the desktop plane's
    // reveal without either component knowing about the other.
    document.documentElement.style.setProperty('--choir-landing-reveal', reveal.toFixed(4));

    if (reveal > 0.02 && !desktopAnnounced) {
      desktopAnnounced = true;
      dispatch('enterdesktop');
    }
    if (reveal > 0.55) dispatch('complete');
  }

  onMount(() => {
    last = 0;
    nativeScroll = typeof window !== 'undefined'
      && typeof window.matchMedia === 'function'
      && window.matchMedia('(pointer: coarse)').matches;
    raf = requestAnimationFrame(frame);
    // Only the transform driver owns synthetic input. On touch the platform
    // is already doing the scrolling, and claiming the wheel or the drag as
    // well would be two implementations racing one gesture.
    if (!nativeScroll) {
      window.addEventListener('wheel', onWheel, { passive: false });
      window.addEventListener('pointerdown', onPointerDown, { passive: true });
      window.addEventListener('pointermove', onPointerMove, { passive: true });
      window.addEventListener('pointerup', onPointerUp, { passive: true });
      window.addEventListener('pointercancel', onPointerUp, { passive: true });
    }
    window.addEventListener('keydown', onKeydown);
    return () => {};
  });

  onDestroy(() => {
    if (raf) cancelAnimationFrame(raf);
    document.documentElement.style.removeProperty('--choir-landing-reveal');
    window.removeEventListener('wheel', onWheel);
    window.removeEventListener('keydown', onKeydown);
    window.removeEventListener('pointerdown', onPointerDown);
    window.removeEventListener('pointermove', onPointerMove);
    window.removeEventListener('pointerup', onPointerUp);
    window.removeEventListener('pointercancel', onPointerUp);
  });
</script>

<div
  bind:this={root}
  class="deck"
  class:deck-touch={nativeScroll}
  class:deck-arrived={arrived}
  style:--arrive={arrive.toFixed(4)}
  style:--reveal={reveal.toFixed(4)}
  data-landing-intro
  data-landing-intro-pane={pane}
  data-landing-intro-reveal={reveal.toFixed(3)}
  data-landing-intro-desktop={reveal > 0.55 ? '1' : '0'}
  data-landing-intro-driver={nativeScroll ? 'native' : 'transform'}
>
  <!-- the environment. constant across the deck, dissolves for the handoff -->
  <div class="scrim" aria-hidden="true"></div>
  <div class="aurora" aria-hidden="true"><i></i><i></i><i></i></div>
  <ChoirField density="intro" />
  <div class="vignette" aria-hidden="true"></div>

  <!-- the track -->
  <div class="scroller" bind:this={scroller} data-landing-intro-scroller>
    <div class="track" style:transform={nativeScroll ? undefined : `translate3d(0, ${-trackY * 100}dvh, 0)`}>
    <!-- ══ PANE 1 · the title card ══
         Everything here is a function of --arrive. There is no "playing"
         state: at any scroll offset the mark is exactly as far into its
         lunge as the hand has put it, and scrubbing back undoes it exactly. -->
    <section class="pane pane-title" aria-label="Choir">
      <div class="stack">
        <div class="mark"><TetraMark label="Choir" size="clamp(120px, 17vw, 210px)" /></div>
        <div class="wordmark"><span>CHOIR</span></div>
        <p class="tagline">The automatic computer</p>
      </div>
      <p class="hint" class:hidden={hinted || pane > 0}>scroll<span class="chev">&darr;</span></p>
    </section>

    <!-- ══ PANE 2 · not a chat ══ -->
    <section class="pane pane-rename" aria-label="Not a chat, a computer">
      <div class="tape-glow" aria-hidden="true"></div>
      <div class="tape" aria-hidden="true"></div>
      <div class="ticks" aria-hidden="true">
        {#each Array(24) as _, i}
          <i style:left="{(i / 23) * 86}%"></i>
        {/each}
      </div>

      <div class="stack">
        <p class="kicker">02 — The rename</p>
        <h1>
          <span class="line line-lead">Choir isn&rsquo;t a chat.</span>
          <span class="line"><span class="dim">It&rsquo;s a</span> <em>computer</em><span class="dim">.</span></span>
        </h1>
        <p class="sub">
          A persistent machine, not a session. Many agents coordinate on it for months,
          and every move they make lands on one versioned record you can read.
        </p>
        <ul class="pills">
          <li>persistent</li>
          <li>versioned</li>
          <li>reversible</li>
          <li>self-improving</li>
        </ul>
      </div>

      <p class="hint" class:hidden={hinted || pane > 1}>keep going<span class="chev">&darr;</span></p>
    </section>

    <!-- ══ PANE 3 · the desktop, itself ══
         Nothing renders here. The machine is already running underneath;
         this pane exists so the deck has somewhere to arrive. -->
    <section class="pane pane-desktop" aria-label="The Choir desktop"></section>
    </div>
  </div>

  <!-- ══ CHROME ══ -->
  <div class="chrome" aria-hidden="true">
    <div class="rail" data-landing-intro-rail>
      {#each [0, 1, 2] as i}
        <i style:transform="scaleX({railPips[i]})" class:lit={i === pane}></i>
      {/each}
    </div>
  </div>

  <button
    class="continue"
    type="button"
    data-landing-intro-continue
    style:pointer-events={reveal > 0.6 ? 'none' : 'auto'}
    aria-label="Continue to the desktop"
    on:click={() => advance(1)}
  >
    {#if pane < LAST}
      <!-- Deliberately never says "open". Nothing needs opening — the third
           pane IS the desktop. This is a shortcut along the scroll, not a
           gate, and the wording should not imply a gate. -->
      <span>Keep going</span>
      <span class="chev">&darr;</span>
    {/if}
  </button>

  <!--
    Once the deck has arrived on a touch device it has stood down, so the
    machine underneath gets the touches — otherwise a phone user would arrive
    at the desktop and be unable to touch it, which is a worse bug than the
    one this control exists to balance. The trade is that the scroll position
    is no longer theirs, so this puts it back. Touch-only: on a fine pointer
    the deck never stood down and the wheel still works.
  -->
  {#if nativeScroll && arrived}
    <button
      class="again"
      type="button"
      data-landing-intro-again
      on:click={backToStart}
    >
      <span class="chev">&uarr;</span>
      <span>read the intro again</span>
    </button>
  {/if}
</div>

<style>
  .deck {
    position: fixed;
    inset: 0;
    z-index: 10001;
    color: var(--choir-text-primary);
    font-family: var(--choir-font-ui);
    isolation: isolate;
    /*
      overflow: clip, not hidden. Every transform in here animates, and
      transformed boxes extend scrollable overflow — `hidden` would leave a
      few hundred px of hidden scroll range and any scrollIntoView would
      silently drag the whole deck out of frame.

      pointer-events: none throughout, at every pane. The desktop underneath
      is live from the first frame; a click on an icon or the prompt bar
      works while the deck is still on screen, and the deck is driven only
      by the wheel, the keyboard and touch-drags it opts into.
    */
    overflow: clip;
    pointer-events: none;
  }
  .deck button { pointer-events: auto; }

  /*
    The stand-down. On a touch device the deck has to own the screen while it
    is up — two full-screen surfaces cannot both have the touches — so on
    arrival it releases, and the machine underneath becomes live and
    touchable. That is not a concession: a third pane that *is* the desktop is
    the whole design, and a phone that arrives at an untouchable desktop has
    failed at the one job the third pane has.

    Fine pointers never needed this, and must not get it: on a laptop the
    deck is transparent throughout and the wheel keeps working.
  */
  .deck-touch.deck-arrived .scroller { pointer-events: none; }

  .again {
    position: absolute; left: 50%;
    bottom: calc(clamp(0.9rem, 2.4vh, 1.5rem) + 3.2rem);
    transform: translateX(-50%);
    z-index: 9;
    display: inline-flex; align-items: center; gap: 0.5rem;
    padding: 0.5rem 0.9rem; border-radius: 999px;
    border: 1px solid var(--choir-border);
    background: var(--choir-surface-control);
    color: var(--choir-text-muted);
    font-size: 0.68rem; font-weight: 740; letter-spacing: 0.05em;
  }
  .again:focus-visible { outline: 2px solid var(--choir-accent); outline-offset: 3px; }

  /* ── the environment ──────────────────────────────────────────────
     Deliberately cheap. This layer used a full-viewport backdrop blur, a
     blur on gradients that were already soft, and a masked animated grid.
     Together they cost about ninety percent of the frame budget and
     changed nothing a viewer could name. */
  .scrim {
    position: absolute; inset: 0; z-index: 0;
    background: color-mix(in srgb, var(--choir-bg) 97%, transparent);
  }
  /*
    Opaque on small viewports. The three percent of desktop showing through is
    a depth cue on a wide screen, where the window sits inside a frame and
    there is empty space to see around it. On a phone the window fills the
    whole viewport, so the same three percent is body copy sitting directly
    behind the hero mark, faintly legible. There is nothing to see around on
    a phone, so the ghost is not paying for itself either.
  */
  @media (max-width: 720px) {
    .scrim { background: var(--choir-bg); }
  }
  .aurora { position: absolute; inset: 0; z-index: 0; }
  .aurora i { position: absolute; display: block; border-radius: 50%; will-change: transform; }
  .aurora i:nth-child(1) {
    width: 88vw; height: 88vw; left: -34vw; top: -36vw;
    background: radial-gradient(circle, var(--choir-state-active-glow) 0%, transparent 52%);
    animation: drift-a 18s var(--choir-ease-drift) infinite;
  }
  .aurora i:nth-child(2) {
    width: 80vw; height: 80vw; right: -32vw; bottom: -34vw;
    background: radial-gradient(circle, var(--choir-state-focus) 0%, transparent 52%);
    animation: drift-b 18s var(--choir-ease-drift) infinite;
  }
  .aurora i:nth-child(3) {
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
  .vignette {
    position: absolute; inset: 0; z-index: 5;
    background: radial-gradient(ellipse 78% 70% at 50% 46%, transparent 32%,
      color-mix(in srgb, var(--choir-shadow-color) 74%, transparent) 100%);
  }

  /* ── the track ──────────────────────────────────────────────────────
     Three panes in normal flow, one viewport each, and the track slides
     them past the window. They must be in flow, not absolutely positioned
     at top:0 — three absolutely positioned panes share one slot, so the
     deck would silently have no second or third pane at all. It happened
     once and was invisible, because each pane's own arrival maths happened
     to hide the two that were not supposed to be showing. Layout bugs hide
     behind animation that happens to compensate for them.

     .scroller is inert for a fine pointer — it exists only so a touch
     device can scroll this natively. See the driver note in the script.
  */
  .scroller {
    position: absolute; inset: 0; z-index: 4;
  }
  .track {
    position: relative;
    will-change: transform;
  }
  .pane {
    position: relative;
    height: 100dvh;
    display: grid; place-items: center;
    padding: clamp(3.5rem, 9vh, 6rem) clamp(1.5rem, 7vw, 6rem);
  }

  /*
    Touch gets the platform's scroll, not ours.

    `proximity` rather than `mandatory`: the hand-off is built to be
    scrubbed, and a mandatory snap makes it impossible to stop halfway and
    look. Proximity gives real free scrolling and still tidies up when you
    let go near a pane. `mandatory` would have made the deck feel like it
    was refusing to scroll, which is the complaint this replaces.

    The root also stops clipping on touch. `overflow: clip` is right for the
    transform driver, but nesting a native scroller inside a clipped box is
    the classic way to lose momentum scrolling on iOS, and there is nothing
    here that needs clipping once the track is not transformed — the scroller
    clips its own content.
  */
  @media (pointer: coarse) {
    .deck { overflow: visible; }
    .scroller {
      overflow-y: scroll;
      overflow-x: clip;
      overscroll-behavior-y: contain;
      scroll-snap-type: y proximity;
      -webkit-overflow-scrolling: touch;
      pointer-events: auto;
      /* a visible scrollbar on a phone would just be furniture */
      scrollbar-width: none;
    }
    .scroller::-webkit-scrollbar { display: none; }
    .pane {
      scroll-snap-align: start;
    }
    .track { transform: none !important; will-change: auto; }
  }

  .stack {
    width: min(100%, 60rem);
    display: grid;
    gap: clamp(0.85rem, 2.2vh, 1.5rem);
  }

  /* ── scroll-linked choreography ──────────────────────────────────────
     Two numbers run the whole piece and both are the live scroll position,
     not a clock:

       --arrive   0 → 1   pane 2 coming up; drives its entrance and
                          pane 1's exit simultaneously
       --reveal   0 → 1   the hand-off into the desktop

     Every value below is a calc() on one of them, so the deck has no
     discrete states. Reverse the scroll and every element retraces its
     path exactly, because at any offset each transform is a pure function
     of the position rather than the residue of a transition that already
     started. That is the whole difference between scroll-linked motion and
     an animation you happen to be able to interrupt.
  */
  .scrim { opacity: calc(1 - var(--reveal, 0)); }
  .aurora { opacity: calc(1 - var(--reveal, 0)); }
  .vignette { opacity: calc((1 - var(--reveal, 0)) * 0.9); }
  .chrome, .continue { opacity: calc(1 - var(--reveal, 0)); }
  /*
    The constellation fades out the same way the rest of the environment
    does, by being faded directly. There was a "field fade" panel here that
    faded *up* with --reveal, on the theory that it would cross-dissolve the
    canvas into the background — which sounds reasonable and is exactly
    backwards: by the time it was fully opaque it was covering the desktop
    at the precise moment the desktop is the point.
  */
  .deck :global(.choir-field) { z-index: 1; opacity: calc(1 - var(--reveal, 0)); }

  /* ── pane 1 · leaving ─────────────────────────────────────────────────
     The mark lunges and blurs; the wordmark comes apart letter by letter
     as its tracking opens. A viewer scrolling slowly sees it stretch. */
  /*
    The mark is the hero of this pane, and it is sized through the component's
    own `size` prop rather than by a CSS rule written from out here. Svelte
    compiles a component's own class selector with the scoping class attached
    twice, so it outranks any `.wrapper :global(svg)` a parent can write — the
    rule lands, the mark stays 1.35rem, and the failure reads as a layout bug
    instead of a cascade bug. It was exactly that: 22px, left-aligned inside
    a 116px box, which is both too small and off-centre.

    4.8:1 mark-to-wordmark, the ratio the standalone study reads at.
  */
  .mark {
    display: flex;
    justify-content: center;
    transform: scale(calc(1 + var(--arrive, 0) * 0.75)) translateY(calc(var(--arrive, 0) * -6vh));
    filter: blur(calc(var(--arrive, 0) * 10px));
    opacity: calc(1 - var(--arrive, 0) * 1.15);
  }
  .wordmark {
    justify-self: center;
    font-size: clamp(1.5rem, 4.4vw, 2.6rem); font-weight: 200;
    letter-spacing: calc(0.52em + var(--arrive, 0) * 0.62em);
    text-indent: 0.52em;
    transform: scale(calc(1 + var(--arrive, 0) * 0.42));
    opacity: calc(1 - var(--arrive, 0) * 1.4);
  }
  .tagline {
    justify-self: center;
    font-size: clamp(0.7rem, 1.2vw, 0.84rem); font-weight: 700;
    letter-spacing: 0.3em; text-transform: uppercase; color: var(--choir-text-subtle);
    transform: translateY(calc(var(--arrive, 0) * 5vh)) scale(calc(1 - var(--arrive, 0) * 0.08));
    opacity: calc(1 - var(--arrive, 0) * 1.6);
  }

  /* ── pane 2 · arriving ────────────────────────────────────────────────
     It rises out of the lower edge and comes into focus, and the record
     draws itself beneath the claim as it settles. */
  .pane-rename .stack {
    /*
      The `+ var(--reveal) * 100dvh` term pins the pane's content to the
      centre of the viewport for the whole hand-off. Without it the content
      rides the track upward and is off the top of the frame long before
      the word has finished growing — which leaves a dead stretch of empty
      screen between the claim and the machine. The track still moves; the
      words just stop following it.
    */
    transform:
      translateY(calc((1 - var(--arrive, 0)) * 9vh + var(--reveal, 0) * 100dvh))
      scale(calc(0.97 + var(--arrive, 0) * 0.03));
    filter: blur(calc((1 - var(--arrive, 0)) * 9px));
    opacity: var(--arrive, 0);
  }

  /* ── pane 2 · the hand-off ────────────────────────────────────────────
     The move. Everything on the pane falls away and the word "computer"
     scales up through the camera instead, blurring as it goes, so the
     word is the last thing on screen and it is the doorway.

     Everything finishes by reveal 0.72, which leaves the last stretch of
     the scroll with nothing on it but the arriving machine. A handoff that
     fades out at the same moment the next thing fades in reads as a cut;
     finishing early reads as an arrival.
  */
  .pane-rename h1 .line-lead { opacity: calc(1 - var(--reveal, 0) * 5); }
  .pane-rename h1 em {
    display: inline-block;                 /* so it can be scaled in place */
    font-family: var(--choir-font-display);
    font-style: italic; font-weight: 400; letter-spacing: -0.015em;
    background: linear-gradient(96deg, var(--choir-text-accent), var(--choir-tetramark-color) 60%, var(--choir-accent));
    -webkit-background-clip: text; background-clip: text; color: transparent;
    transform: scale(calc(1 + var(--reveal, 0) * 3.1));
    filter: blur(calc(var(--reveal, 0) * 9px));
    opacity: calc(1 - var(--reveal, 0) * 1.4);
  }
  .pane-rename h1 .dim { opacity: calc(1 - var(--reveal, 0) * 5.5); }
  .pane-rename .sub,
  .pane-rename .pills,
  .pane-rename .kicker {
    opacity: calc(1 - var(--reveal, 0) * 4.2);
    transform: translateY(calc(var(--reveal, 0) * 4vh));
  }
  .pane-rename .tape,
  .pane-rename .tape-glow,
  .pane-rename .ticks { opacity: calc(1 - var(--reveal, 0) * 2.2); }

  .kicker {
    font-size: clamp(0.62rem, 1vw, 0.74rem);
    font-weight: 800; letter-spacing: 0.26em; text-transform: uppercase;
    color: var(--choir-text-accent);
  }
  h1 {
    font-family: var(--choir-font-display);
    font-size: clamp(2.15rem, 7.4vw, 5.1rem);
    font-weight: 800; line-height: 0.94; letter-spacing: -0.035em;
    max-width: 19ch; text-wrap: balance;
    filter: drop-shadow(0 6px 34px var(--choir-state-active-glow));
  }
  h1 .line { display: block; }
  .sub {
    font-size: clamp(0.94rem, 1.55vw, 1.16rem);
    line-height: 1.62; color: var(--choir-text-muted);
    max-width: 56ch; font-weight: 380;
  }
  .pills {
    display: flex; flex-wrap: wrap; gap: 0.5rem; margin: 0.3rem 0 0; padding: 0; list-style: none;
  }
  .pills li {
    font-family: var(--choir-font-mono);
    font-size: clamp(0.66rem, 1.1vw, 0.78rem);
    letter-spacing: 0.06em; padding: 0.4rem 0.8rem; border-radius: 999px;
    border: 1px solid var(--choir-border-strong);
    background: var(--choir-state-hover);
    color: var(--choir-tetramark-color);
  }

  /* the record, drawn under the claim */
  .tape {
    position: absolute; left: 0; right: 0; top: 84%; height: 2px; z-index: -1;
    transform: translate3d(0, -50%, 0) scaleX(calc(var(--arrive, 0) * 0.86));
    transform-origin: 0 50%;
    background: linear-gradient(90deg, transparent, var(--choir-accent) 8%, var(--choir-text-accent) 55%, transparent);
    box-shadow: 0 0 18px var(--choir-state-focus);
  }
  .tape-glow {
    position: absolute; left: 0; right: 0; top: 84%; height: 300px; z-index: -2;
    background: radial-gradient(ellipse 70% 100% at 32% 50%, var(--choir-state-focus), transparent 74%);
    filter: blur(40px);
  }
  .ticks { position: absolute; left: 0; right: 0; top: 84%; height: 0; z-index: -1; opacity: var(--arrive, 0); }
  .ticks i {
    position: absolute; top: 0; width: 9px; height: 9px; margin: -4.5px 0 0 -4.5px;
    border-radius: 50%; background: var(--choir-bg);
    border: 1.5px solid var(--choir-text-accent);
    box-shadow: 0 0 14px var(--choir-state-focus);
  }

  /* ── pane 3 ──────────────────────────────────────────────────────────
     Intentionally empty. The desktop is underneath; this pane is only the
     destination, so there is nothing to draw over it. */
  .pane-desktop { background: transparent; }

  /* ── chrome ──────────────────────────────────────────────────────── */
  .chrome {
    position: absolute; left: clamp(1.2rem, 3vw, 2.4rem);
    bottom: clamp(1.1rem, 3vh, 1.9rem); z-index: 8;
    transition: opacity 300ms var(--ease, ease);
  }
  .rail { display: flex; gap: 0.5rem; }
  .rail i {
    display: block; width: 30px; height: 3px; border-radius: 999px;
    background: var(--choir-border-strong);
    transform-origin: left center;
  }
  .rail i.lit { box-shadow: 0 0 10px var(--choir-state-focus); }

  .continue {
    position: absolute; right: clamp(1.2rem, 3vw, 2.4rem);
    bottom: clamp(0.9rem, 2.4vh, 1.5rem); z-index: 8;
    display: inline-flex; align-items: center; gap: 0.55rem;
    padding: 0.6rem 1rem; border-radius: 999px;
    border: 1px solid var(--choir-border);
    background: var(--choir-surface-control);
    color: var(--choir-text-muted);
    font-size: 0.72rem; font-weight: 760; letter-spacing: 0.06em;
    transition: opacity 300ms var(--ease, ease), color 0.2s, transform 0.3s var(--choir-ease-spring);
  }
  .continue:hover { color: var(--choir-text-primary); transform: translateY(-2px); }
  .continue:focus-visible { outline: 2px solid var(--choir-accent); outline-offset: 3px; }
  .chev { font-size: 0.95rem; line-height: 1; }

  .hint {
    position: absolute; bottom: clamp(1.1rem, 3vh, 1.9rem); left: 50%;
    transform: translateX(-50%);
    display: flex; align-items: center; gap: 0.5rem;
    font-family: var(--choir-font-mono);
    font-size: 0.62rem; letter-spacing: 0.24em; text-transform: uppercase;
    color: var(--choir-text-subtle);
    transition: opacity 400ms var(--ease, ease);
  }
  .hint.hidden { opacity: 0; }
  .hint .chev { animation: nudge 2.4s var(--choir-ease-drift) infinite; }
  @keyframes nudge {
    0%, 100% { transform: translateY(0); opacity: 0.5; }
    50% { transform: translateY(5px); opacity: 1; }
  }

  /* ── reduced motion ──────────────────────────────────────────────────
     Nothing here autoplays, so reduced motion does not retire the deck —
     it stops it moving. The narrative is still three scrolls away for
     anyone who wants it; it is simply standing still while they get there. */
  @media (prefers-reduced-motion: reduce) {
    .aurora i,
    .hint .chev { animation: none; }
    .mark, .wordmark, .tagline, h1 .dim, .pane-rename .stack, .chrome, .continue, .hint {
      transition-duration: 1ms !important;
    }
  }

  @media (max-width: 720px) {
    .pane { padding: 3.2rem 1.35rem 4.5rem; }
    h1 { max-width: 100%; }
    .stack { width: 100%; }
    .sub { max-width: 100%; }
    .hint { display: none; }
  }
</style>
