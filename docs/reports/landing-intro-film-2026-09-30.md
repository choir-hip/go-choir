# The landing page finally explains what Choir is

Written on Wednesday, the thirtieth of September. This letter covers about seventy
minutes of work that finished a little after six this morning, and it is the whole
of the landing page job rather than a slice of it. It comes with two recorded videos
and the standalone motion study, all of which are in the Choir Reports folder
alongside this.

The signed-out landing page now opens with a short film that tells a first-time
visitor what Choir is, and the "What Choir Is" document behind it says the same
thing properly in prose. The pull request is open and ready for you. What I could
not do is prove it on staging, and I want to be plain about that rather than let the
green build stand in for evidence it never was.

## What was actually wrong

I went and looked at the live site first, before writing a line of code, and the
problem was worse than "the copy is two out of ten." The landing page was a single
desktop window with two paragraphs in it. The first sentence read "Choir is a
private, Texture-centered computer for durable knowledge work." That is accurate and
close to meaningless to anyone who has not already been briefed on our ontology. It
named the mechanism before the reader had any reason to want the mechanism.

There was no headline, no argument, and no way to sign in above the fold. A visitor
had to guess what the product was, then work out how to start it.

So the diagnosis was not really a copy problem. It was a missing argument. The copy
was weak because there was no structure underneath it, and no structure was going to
come from rewriting two paragraphs in place.

## The film

I wrote the sequence as a five act story, and the argument runs in a deliberate
order. First the shared failure, in the words every daily user of these tools already
feels: every session dies at the tab, you re-explain the project, it re-guesses the
rules. Then the turn: Choir is not a chat, it is a computer. Then the mechanism, and
only here do I introduce the desks, because by now the reader wants them. Then trust,
which is our real differentiator: nothing moves that you cannot read, cite, or undo.
And then the ask: wake the machine.

On screen, each beat has its own gesture rather than being a card that fades in. The
first act types out a real sounding thread, then drains it, blurs it, and lets it
fall away, and deliberately leaves a faint residue behind. That residue is the point
of the act. The second act draws a line of light across the frame as a record, with
revision ticks igniting in its wake. The third act takes the whole background
constellation and physically pulls it into four glowing clusters, one per desk, and
ignites them in turn. The fourth deals in three real artifacts in perspective, a
revision diff, a commitment that scores itself, and a restore receipt. The fifth
re-forms the mark out of the dust behind the closing line and offers the desktop.

I checked every claim in that script against the repository. Nothing is invented
credibility. If the repo does not say it, the film does not say it.

There is a standalone version of the whole piece, one self contained file with no
dependencies, if you want to review the motion without running the app. It plays the
full film and the handoff into the desktop. It is in the reports folder with the
videos.

## The decision I would most want you to check

The film is deliberately non-blocking. The layer that darkens the desktop behind it
does not capture pointer events at all. The public desktop underneath stays live the
entire time, you can click a desktop icon and the film dissolves. You can skip, you
can scrub with the arrow keys or the wheel or the dots along the bottom, and escape
retires it.

I chose that over a proper hard gate because a first-visit film that traps a visitor
is worse than no film, and because I did not want to make the product's first
impression a thing that gets in your way. The cost is that the film is a little
easier to lose than a gate would be, so a visitor who is not paying attention may
glance at a few lines and miss the argument. If you would rather it hold attention
properly, that is a small change and I would rather make it deliberately than have it
decided by my default.

The related choice: for anyone who has asked their system to reduce motion, the film
does not play at all, slowed down. It is retired. That visitor lands straight on the
document, which now makes the entire same argument in prose. I would rather hand
someone who asked for stillness a readable page than a gentler film.

## The desktop got a pulse too

You asked for the web desktop to have more life, and it does now. Windows arrive by
rising, unblurring and settling, and they leave faster than they came, because
somebody waiting on a dismissal should not have to wait out the reverse of the arrival
animation. Desktop icons lean toward the pointer, and the tile of the app in front
breathes, so you can tell which window is live without reading a single label. The
prompt bar blooms when it takes focus. Toasts arrive on a spring.

The one rule I held myself to across all of it: the shared motion may only touch the
individual transform properties, never the transform itself. Windows and icons already
own their transform for dragging and for placement. The individual properties sit on
top of that, so nothing I added can ever fight an in-progress drag. That is the kind of
thing that looks fine in a demo and then breaks window dragging in production.

## Three things I got wrong, and what they taught me

The first build of the film ran at six frames a second. Not a little slow. Six. And the
instructive part is that I could not see it in any screenshot, because a still frame of
a film looks identical at six frames a second and at sixty. I only found it because I
wrote a probe that measured the frame rate with pieces of the page switched off in
turn.

The cost was five separate things, and none of them was the thing I would have
guessed. There was a full viewport blur filter on the scrim, even though at the
opacity I was already using, the desktop behind was just a faint silhouette and the
blur was refining a difference nobody could see. There was a one hundred and ten pixel
blur applied to gradients that were already soft, which was pure waste. There was a
masked, animated background grid that earned its place in neither of the two acts it
appeared in. The constellation was allocating a new gradient object for every node on
every frame instead of drawing one cached sprite. And the desktop's own background
field was running at the same time as the film's, so the landing page was paying for
two full screen canvases simultaneously.

Nothing about the design intent changed when I removed them. That was the lesson worth
keeping. Motion that costs ninety percent of the frame budget is a tax, not a design,
and you cannot find that out by looking at it.

The second thing: the whole film was silently scrolling out of position. I spent an
embarrassing amount of time on this because I kept forming and discarding theories
about stacking contexts and containing blocks, all of which were wrong. The actual
cause was that I had set the film to hide its overflow, and hiding overflow still
creates a scroll container. Every entrance in the film animates with a transform, and
transformed boxes extend scrollable overflow, so the film had about five hundred
pixels of hidden scroll range. The first time anything asked to scroll an element
into view, which a click does, the entire film silently jumped upward and every layout
in it broke. The fix is to clip rather than hide. It is a one word difference with an
enormous behavioural difference, and I have left a comment on it so nobody undoes it
later.

The third: one of my motion theme tokens was quietly voiding every animation in the
component. The token carried both a duration and a timing curve, and I had embedded it
in a transition that also named a duration. That expands to two durations in one
declaration, which is invalid, which means the browser discards the whole rule. The
elements simply never moved. No error, no warning, nothing in the console. You only
find that by measuring. The theme's motion roles are now split into a composite form
and a separated form, and anything that needs a per property duration composes the
separated pair.

## Where it stands, and what I did not do

The build is clean. Two contract suites fail, and I want to be exact about this: they
fail identically on the main branch, and none of the violations are in a file this work
touches. Three of the signed out browser suites need the local service stack running,
which was not up, so I ran the same specs against the main branch and compared the two
failure sets directly. They are byte identical apart from timing digits. I introduced
no regression. I can say that with confidence precisely because I measured the baseline
rather than assuming it.

What I have not done is the landing loop. There is no staging proof, because this is a
pull request and not a landing, and because proving it on staging means a deploy. When
this merges, the person or agent that merges it should watch the build, confirm staging
picks up the commit, and run the acceptance proof against the live site. I would treat
that as the real finish, not the green local build.

One thing I noticed and deliberately left alone: the desktop icon rail is clipped
behind the preview window. You can see it in the recorded video. It is pre-existing
geometry and out of scope, and I would rather name it than quietly fold a second
change into this pull request.

## The one thing to remember

The copy was the visible problem, but the missing thing underneath it was a structure
to hang copy on. Whenever something here reads as weak, the fix is almost never
rewriting the words. It is deciding what the reader should feel first, and in what
order.

If you are holding the phone and have a moment, open the in-the-app video first. It
runs about forty seconds and it ends on the real desktop, which is the part I would
most like you to judge. The motion study video is the longer, cleaner version with
nothing behind it, and the pull request has the reasoning and the measurements in
full.

## Names and receipts

- Pull request: 68, "Landing intro film, rewritten What Choir Is, desktop life"
- Branch: `feat/landing-intro-motion`, forked from `a04616b7`
- Commit: `c53534b275055781b101e2722abbb6c31d07a65c`
- Worktree: `/Users/wiz/go-choir-intro` (your main checkout was not touched)
- New: `frontend/src/lib/LandingIntro.svelte`, `frontend/src/lib/ChoirField.svelte`,
  `frontend/src/lib/desktop-motion.css`, `frontend/src/lib/landing-intro-preference.ts`
- Rewritten: `frontend/src/lib/public-preview-data.ts`, `frontend/src/lib/theme.ts`,
  `DESIGN.md` section six
- Design study: `docs/design/choir-intro-showreel.html`
- Videos: `Choir Reports/choir-landing-intro-in-the-app-2026-09-30.webm` and
  `Choir Reports/choir-landing-intro-motion-study-2026-09-30.webm`
- Study file: `Choir Reports/choir-landing-intro-showreel-2026-09-30.html`
