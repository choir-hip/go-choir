# The landing is three panes now, and the third one is the computer

Written on Thursday, the first of October. This replaces the letter I wrote yesterday
evening about the landing. That one described a five-act film that played on its own
on a timer. That film is gone. This letter covers what the landing is now, and it
covers the whole of the landing work rather than only the change, so you can read
this one and skip the other.

The signed-out landing is three panes and a scroll, and the third pane is the actual
desktop, already running, with the document already open. There is nothing to press.
The pull request is open and ready, and the videos and the standalone study are in the
Choir Reports folder next to this letter.

The short version of what changed and why: I had built something that played at you,
and you were right that it should be something you move through. That is a better idea
than mine, and it is not a small difference in feel — it is a different design.

## Why a film was the wrong shape

The old version was a timed sequence. It autoplayed, it could be skipped, and it moved
at a speed I had chosen. It was well made and it was the wrong instrument, for a reason
I did not see until I built it and then watched it go wrong.

Because the sequence ran on a clock rather than on the viewer, every entrance was a
transition that started whether you were ready or not. You could never go back and find
a moment halfway through, because there was no halfway to find. Scrubbing it would
have meant undoing a pile of transitions that had already begun, and it never looked
right, so I never let it.

The new one has no clock in it at all. There are two numbers, and they are your scroll
position. Every single movement in the piece is one of those two numbers fed into a
formula. So if you scroll back up, the mark un-lunges, the wordmark closes back up, the
record un-draws, and the word computer shrinks back into the sentence it came from.
Exactly. Not approximately. I could not have got that from transitions.

## The hand-off, which is the whole thing

Pane two says the sentence. It says that Choir is not a chat, it is a computer, and it
says it over a line of light that draws itself underneath like a record being written.

Then you keep scrolling, and that is the good part. Everything on the pane falls away
except the word computer, which grows straight through the camera, blurs, and dissolves
— and behind it the desktop is already there, already running, already open on the
document. The word does not announce the machine. The word becomes the way through to
it.

I did not plan that as a metaphor. I planned it because there was nothing else to put
in the space, and then it turned out to be the argument. A landing page whose last slide
is a pitch for the product is asking for a second click. One whose last slide is the
product has already got the answer.

## The document, and the title

You told me to drop the words "what choir is" and to call it "introducing choir, a new
species, of computer". That is not just a better title, it is a different speech act.
"What choir is" is a question that sets the reader up to grade an answer. "A new species
of computer" is a claim, and it asks them to look rather than to assess. I would not
have found that on my own and I would have argued for the old one.

The document in that window now carries the whole argument again, in prose, so that the
hand-off does not cost anyone the story. The three panes make it in fifteen seconds. The
machine you land on then says the same thing at whatever pace you want to read at. The
argument survives the transition, which is the thing a film cannot do.

## Four bugs, and one of them was embarrassing

A notched mouse wheel could not cross a pane. The code decided a gesture was finished
one hundred and ten milliseconds after the last input, and a mouse wheel sends a notch
roughly every hundred milliseconds. So every single notch looked like the end of a
gesture, the deck thought you had stopped, and it politely returned to the pane you had
just left. The hint on the screen said scroll, and the wheel did not scroll. That one I
found only because I wrote a probe that measured the deck's position after each notch
instead of assuming it worked, and it is now verified against three different input
patterns.

All three panes were absolutely positioned at the same place, so they were stacked on
top of each other and the deck only had one pane. This one is my favourite, because it
was invisible. Each pane's own arrival animation happened to hide the two that should
not have been showing, so it looked correct in every screenshot I took. Layout bugs hide
behind animation that accidentally compensates for them, and I only found it because the
hand-off looked empty and I refused to accept that the design was intentional.

The hand-off also left a dead stretch of screen where nothing happened, because the
words were riding the track upward and had left the frame before the word computer
finished growing. And a panel I had added to gently cross-fade the background
constellation was fading in at exactly the moment the desktop became the thing you were
looking at, so it was covering it. Both were mine and both were the same mistake: an
element with no job in the second half of its own life.

## What I got wrong about speed, twice

The first version of this ran at six frames a second, and I wrote about that yesterday
so I will not belabour it. What is new is that I then did a version of the same mistake
in the other direction, on purpose this time, and caught it with the same method.

I had put a fifteen pixel blur on the whole desktop plane, so that the machine would
look slightly out of focus behind the deck and sharpen as you arrived. It is a good
effect and it is completely invisible, because the deck's own dark layer already holds
the desktop down to about three percent visibility. So I was paying to blur something
nobody could see, and it cost me more than everything else on the page combined. Taking
it out took the landing from thirteen frames a second to twenty-two. The lesson is
identical both times and I apparently needed saying twice: if you cannot see what the
cost bought, it was not worth it.

## Where it stands

The build is clean. The two contract suites that fail, fail identically on the main
branch, and none of the failures are in anything I touched. The three signed-out browser
suites need the local service stack, which was not running, so I ran them against the
main branch and compared the two failure sets directly. They are the same set. I did not
break anything, and I can say that with confidence because I measured the baseline rather
than assuming it was fine.

Still no staging proof, and still the same reason: this is a pull request, not a landing.
When it merges, someone should watch the build, confirm staging picks up the commit, and
run the acceptance proof against the live site. I would treat that as the finish, not
the local build.

The one thing I would most like you to look at is the middle of the video, about eleven
seconds in, where the word computer fills the frame and the desktop is already visible
through it. That is the whole design in one moment, and it is the thing I would change
first if you did not like it.

## The one thing to remember

When something here is hard to get right, ask whether it is a timeline or a position. A
timeline fights the person using it. A position is already theirs.

## Names and receipts

- Pull request: 68, "Landing as a three-pane scroll deck; deck copy in the opened document"
- Branch: `feat/landing-intro-motion`, latest commit `e7df4f3d`
- Superseded: commit `c53534b2`, the five-act film, and this repository's
  `docs/reports/landing-intro-film-2026-09-30.md`
- Video: `Choir Reports/choir-landing-deck-in-the-app-2026-10-01.webm`, about seventeen seconds
- Study: `Choir Reports/choir-landing-deck-study-2026-10-01.html`, and
  `docs/design/choir-intro-showreel.html` in the branch
- Worktree: `/Users/wiz/go-choir-intro`; your main checkout was not touched
