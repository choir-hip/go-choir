# Inbox

Standing: governing | kind: happened | settled-by: the owner's words, dated, as said (line 5); named an authority source in docs/definitions/choir-docs-transform-2026-11-27.md

The owner's words, dated, as said. Nothing here is a commitment or a design.
An entry leaves the inbox when it becomes a line in the plan
([big-picture.md](big-picture.md) while that is a draft) or is dropped.
Adding an entry costs one paragraph: quote, do not rewrite.

## 2026-11-26 and 27, a planning session after Autoradio 1 reached the App Store

**Order of work.** "before we do more code, fixes and features, we need a
big clean up. So, Big docs transform coming up." And: "after the docs
transform I will look to complete the cutover from swift to exact. It has
been lingering, with a long tail of residuals. We have bug fixes to do in
the app but I don't want to do any fixes or features until we complete the
exact cutover. And no need to worry about migrating user state."

**What "voice" means.** "By 'voice' I mean what must actually be swift
because exact can't do it." The Exact pin "will be updat[ed] to latest ~
weekly".

**Android.** "I'm fine with the android agent writing kotlin that will get
cutover. Better to have it at parity with swift than incomplete."

**Docs.** "The problem record is somewhat useful if verbose, and probably
could be improved upon rather than scrapped. While the key issue is I think
that we have too many, and conflicting, theory, architecture, and planning
docs, and should consolidate those. And also streamline the sdlc
significantly."

**Review.** "I like having one or more different providers agents reviewing
code, with at least 2 for security relevant stuff." "DeepSeek via Opencode
in omp is the other review seat."

**Landing.** "there's a growing queue to land code lanes that is reviewed
and approved. Should be a better way." "I don't want pull requests and to
be more dependent on GitHub. I know that's standard but I see it as a weird
antipattern." Then: "How can we use GitHub actions more tho? Maybe we do
want prs after all?" And: "while it's awesome as free compute (up to its
limits) it's only like 98% available so we do need backup system".

**Node A and hosts.** "One of the things I want to do soon is actually
repurpose node a. We need our podcast clips and transcription service to
run on a server... And we will soon need more choir user vm capacity, and
redundancy." "we should upgrade the platform so that in addition to running
choir VMs, we can run arbitrary VMs inside the choir platform, including
builder and lander, or utilities like podcast transcription or whatever.
And this can extend later to users using selfdev to build freestanding apps
(that they pay to deploy and maintain)". "we should loo[k] to make node a a
choir host very soon, post docs transform. So much so that I want to
integrate these desiderata as codesign for docs transform".

**Zero downtime and scale.** "We have to figure out zero downtime deploys
architecturally. It may need not just 2 but 3 machines... We will scale
everything. User VMs, paper data sources, agents running in paper, desks
(appagents) running in user VMs, etc"

**The desktop app.** "the plan is for it to enable running user VMs on your
own desktop., with transparent replication between cloud and local. And it
will be able to be a node for computer use, enabling controlling a desktop
machine from cloud or autoradio app client. And we could also make
autoradio a desktop app." "a 64gb Mac or windows machine with 3090+ is very
useful but a 3 year old $300 laptop doesn't offer much...except computer
use. That could be valuable regardless."

**No cloud voice.** "One commitment is to not use cloud based voice models.
No eleven labs or OpenAI/gemini live voice. It's just not economical."

**Free Autoradio.** "we can provide autoradio app to everyone for free
since its basic version (no user vm) just consumes autopaper as text and
uses on device models for voice, ai dj, rag informed answers, etc"

**Mike and the vault.** "Mike has one platform service choir desperately
needs: a vault." "to use a choir vm as a generic persistent computer, one
that just has a web ui...one which can be driven by Mike (a
ChatGPT/claude cowork/harvey chatbot clone), by autoradio, or by the web
desktop on choir.news...and many other apps and endpoints too. One critical
one is the choir cli, and MCP, which let choir computers be persistent
computers for external agents like Claude code and codex". "porting vault
to go as new nix systemd host service, making a Mike-host vm to run Mike's
platform code which for all users enables them to leverage a choir computer
as their Mike vm. Then builders and landers and MCP servers and other
systems are just natural things, as is choir-in-choir... We will have to
rearchitect vmctl as well for this, I bet." Then: "Or we could reengineer
so that VMs ask for permissions directly? Mike could instead be an app
inside choir?"

**Mike's place.** "Mike shouldn't be banned. Indeed, what it needs is for
us to investigate if main upstream has valuable additions for us to merge
in...I wanted to commit code back upstream too but really I need to drive
revenue before I get to the solution". "Mike started out as an idea to
start a consulting business for a private ai stack. Funny enough, that
business seems not to be working, but I still like it as an asset, a chat
agent, as it functions as a generic interface like Claude or any agent
using choir over mcp." "the right approach isn't to put a chatbot inside
choir.news or autoradio 1. But rather, a chatbot is a great debug interface
into a choir computer, and it's also great at using a choir computer as a
tool. Like, the point is that the chatbot is a great technology, not the
only way to use agentics"

**Onboarding.** "one key thing to do soon is to optimize autoradio
onboarding, so we do a nice radio and voice intro, where autoradio
introduces itself to users and interviews them for immediate customization,
while it downloads and configures all the compute, all the models --
correctly optimized for their device and account resources -- this is after
all the key point to provide value with free service while introducing
reasons to pay (persistent 24/7 agents accessible anywhere)"

**The paid product, generalised.** "Autopaper is one such agent. The
generalization is processing a business or team's relevant data and turning
that into a menagerie of live up to day reports, analysis, signals, and
consulting solutions"

**Flaws seen when showing Autoradio 1 to someone.** "autopaper has no
picture yet but needs them. I need a whole service, like the podcast
clipping and transcription service, a photojournalism service which has
pictures for everything and is rights respecting and on the path to
self-improving." "currently public autopaper articles aren't permalinks in
the url for them; autoradio 1 doesn't have good search over autopaper".
"there isn't a good enough autoradio explore feed." "Need a better podcast
explore and recommendations as well. New episodes from many shows."

**Compute.** "I need more compute to do all this! Ok, I trust that it's
coming, and that my current constraints are both liability and asset. A
strong asset if used wisely to drive efficiency"

**Visualising anything.** "After transcribing podcasts, do deep research on
them. And generate a presentation -- 'text to video generation', but
starting with a podcast and producing a series of still images to visualize
it. Well, not just podcasts but anything... it's compute intensive so we
just always do the practical approximation. That's semantic economics. We
can put the transcript, or any autoradio text, on top of the image"

**Amortised research.** "We amortize deep research! Since much of the
research on a podcast is in the corpus. And new information that we
research then goes into the corpus."

**Choir as a tool.** "It bundles the 'sandbox' with agentic research and
agent builders (make your own desk in your choir, now you have your own
multiagent system which could be a persistent concurrent service, or could
just be session based, or could be an open endpoint that other agents use).
This will be an incredible organization tool." "it's important that we
explain our value creation. What are we arbitraging?"

**Google.** "importing data from Google takeout, using googles official
connectors (they just cracked down on unauthorized tools)
https://developers.google.com/workspace/guides/configure-mcp-servers"

**Harnesses as desks.** "we need to get better at using Claude sdk inside
choir. We've experimented with Claude -p as an engineering desk delegate.
But we somewhat had an idea to use MCP for message passing with Claude, to
make it function as a desk. In this sense, we could abstract harnesses into
desks, and extend users accounts with model provider's subscriptions."

**Integrations.** "I keep flip flopping from improving the product to the
platform to the integrations. But I told myself earlier that it's strategic
to hold off on integrations. Everyone builds integrations. I should build
the rarefied core components."

**The mode.** "part of this vision quest is determining the product
roadmap. I've been for the last while in a flurry of implementation but the
key here is to step back into the mindset of creation"

## 2026-11-27, later the same session

**Two vaults, and staging.** "we need two vaults. I mean, choir needs better
secrets management at the platform level. I never meant to ship product
like this. Like, I see choir in 'staging' not yet 'production' even though
the app is live on the App Store."

**A year, not a week.** "On one hand the pace of development is fast. On
the other hand it is 1 year of automatic computer. The idea of the web
desktop came to me last thanksgiving. And it took tons of work to make it
work. Now to make it nice. It's important that I envision where I'll be
next year, not just next week."

**Next year's clients.** "More apps in media production and live streaming.
Hardware. An e-ink tablet that runs autoputer os, with on-device models"

**Takes, in a month.** "probably the most important thing is the key
feature of autoradio that needs focus on in the 1-month timeframe:
recording and distributing takes."

**Social.** "Making autoradio social takes things to a whole nother level.
That's the key unlock. It first requires making autoradio work, but
honestly it's already close. We need to go deeper on on-device models, the
autoradio desk for paid users, ui / ux improvements, photojournalism and
podcast clipping and analysis, and then we really do social justice. (We do
social, justice; we do justice to 'social'; 'social' has become the
category name for social media networks, graph networked news feeds with
algorithmic recommendations, often ad supported)"

**Branching and the recommender.** "stories should branch off. Each story,
including podcasts and user published stories (we half built the op ed
feature...needs revisiting) branches off into ugc responses, that's one key
facet of recommendations. But basically we have to build a whole agentic
recsys. I mean, that's what the ai dj is, and we need the autoradio desk
version in the choir computer, and also scaling down to on device
phone-local version"

**Who may publish.** "My business model is only offering voice take
publishing to paid users. Since voice means some moderation must be done,
and that's not 0MC"

**Everyone who talks has a computer.** "I just like the community culture
if everyone who can talk has at least one choir computer. Like, if you buy
an account just to talk, you're forced to get a choir computer, so
naturally you'll find a use for that compute"

**Talking is free; publishing a voice is paid.** "All users can talk to
their phone. Obviously. You can do dictation, or talk to an on-device
model. But only paid users can publish their voice."

**Signed out.** "we should expand what can be done signed out. All the
local on device stuff doesn't care about you having an account. Like, the
account is for the choir computer, which free users don't have."

**A local model exhibition.** "only after making autoradio did I realize
that it was viable as a local model exhibition"

**The Exact move is a day.** "Finishing the exact cutover is 1 day at
agentic speed. 8 hours really. Faster if I'm focused but I don't need to
allocate much human attention to it. Just decision and instruction framing.
Hence why it's docs transform then resume exact cutover. But the fact that
such cutovers can get needlessly drawn out, as this one already has, is
actually strong signal for the need for docspace refactoring, and realizing
the magnitude of the simplification process"

**Update Exact first.** "we should plan to update to upstream before the
cutover finishes."

**Submissions.** "With the App Store, I want to be not wasting their time
with bug fix updates. Rather, ship significant feature upgrades each
update. That's a sign of vitality. We can think in units of our next app
submission. That's the weekly cadence."

**Version 1.1.** "Making the app nice with its current feature set and
going deep on local models and honing the onboarding voice interview.
That's... autoradio 1 1.1"

**Branding.** "I haven't dropped the choir branding, not at all, it's just
that the app strategy is to seemingly niche down. Like, Apple approved it!
They wouldn't have approved an autoputer app, since no OS or App Store or
app generator is allowed technically in the App Store. An automated news
app is"

**Version 1.2.** "Takes focus is 1.2. That's all takes and social. Note
that docs transform is also ofc 1.1... like, we don't ship docs in the
autoradio app, but docs transform is the prerequisite."

**Sequencing.** "Building choir this past year has shown me a lot about
sequencing. Perhaps sequencing is the essence of engineering as a
discipline. Orchestration is the next step. Gotta get better at doing more
at once."

**Building on our own hosts.** "There's no real reason why choir hosts
can't also host building and merging choir platform code in our vacant
compute"

**Pull requests.** "My thoughts on prs are underdeveloped. I think they
work pretty well as a collaboration tool, almost like 'prompt requests',
but it's hard to just merge them in without doing a whole suite of tests on
them and rebasing onto the new version... it is somewhat of a solved
problem. Big tech has been doing massive merging and building at scale for
years... But I'm scaling myself, my own focus, not max agents or thousands
of engineers that need to coordinate"

**Monitoring.** "We also need better monitoring. I want that to be an app
in choir. It already is, actually. It's just shit. And the monitoring app
and data -- it should become a desk, an appagent -- should be accessible in
autoradio 1. Needs some decisions about what data is mine, and what is for
public. I want to publicize as much data as is possible securely. All the
usage data and analytics and performance metrics. Ofc gotta think of threat
model here...I don't see competitive threat from showing our true usage
metrics but maybe some analytics data is sensitive and I don't even
understand why."

**When the desktop apps return.** "one key question for me is when do the
Mac/windows/linux apps come back into focus. When computer use is the most
demanded feature. But it is related to the vm work here, as it enables
running VMs on desktop, and basically we can give users credits for that.
Not a discount on their subscription, but credits they can apply. We do
have to have some token system for rate limits. Though I'm uncertain of
what exactly that is."

**Free accounts today.** "Currently all free accounts get their own vm.
Because I don't even have my first users yet. Product isn't yet
exceptionally impeccable, just impressive. The learning system will get
much better, and that becomes incredible as you add more sources, and
attracts users to add their own sources; for that they'll gladly pay. Soon
I'll hit caps of what I can serve but I'll have more resources at that
point."

**The sequence of versions.** "docs/roadmap-2026-10-31.md was basically a
month ago. i didnt follow it exactly, but i did launch. agentic red-teaming
comes later. after takes 1.2 comes transcription and pictures service 1.4.
then desktop app 1.5 then vms replicated/running on desktop 1.6. then solid
economics 1.7. then red teaming 1.8. then the integrations work 1.9. any
steps im forgetting?"

**1.3 and the method.** "yes, 1.3 is discovery. and the point is to
discover how all the other requirements order and sequence the roadmap of
features"

**Adaptive planning.** "the point is that the planning and scheduling is
adaptive. the 10th week is four stroke learning engine imbuing everything.
the 11th week is a week like this, where we actually work to forget
everything. deletion. and yes, at some level the concept of weeks breaks
down, because we are always working on everything all at once. thats why we
do big picture planning"

**The learning engine on documents.** "we need to apply the learning engine
to documents. literally. the learning engine is a function we approximate.
thats the whole point, actually. thats what makes it amenable to in context
meta learning"

**Accumulate, and filter.** "thinking functionally, it should be fine that
we accumulate docs, if we have the right way to filter them"

**Forgetting and trust.** "forecast act score forget is essentially the
same. maybe its even tighter. i say trust because thats the positive
version, that it implies forgetting as loss of trust often leads to
traumatic memories. a lesson is data. its adding on. even if it is a
'deletion', its just an instruction for a deletion at some level. all the
past instructions necessarily exist no matter what. indeed, the separation
of lesson and trust was the whole idea i was ruminating on. its very deep.
but trust gets miscalibrated and it stops learning. as trust governs your
ways of perception"

**Actually existing agents.** "'always read set' is implying more control
over the agent and the harness than we can safely. need to recalibrate to
actually existing agents, external agents like claude code, and the yaegi
rlm repl design of choir desks"

**Autopaper updates documents, not its model or its trust.** "with
autopaper today we have updated versions of documents. but we dont have
much updated model of the world (lesson) and trust-updating which shifts
your attention map routing that weights source inclusions"

**Musings find their place; the irrelevant fades.** "the idea being that
all of my musings get updated into the right docs which naturally get
hooked into their correct place in the campaign while plans, sources,
topics, ideas, speakers no longer relevant fade out of attention"

**The campaign is the world model, and it is a programmable REPL.** "the
campaign is our world model. and the point isnt to mutate it with
deterministic code, but to have the world model be a repl that is highly
programmable, which enables arbitrary concurrency patterns and message
passing. all sorts of deterministic rules and tools and algorithms and
data structures then get used by agents over data. the programmability of
it is essential. now, claude is getting code mode."

**A mission is a forecast; execution scores it.** "the missions get
predicted as goal docs, and execution scores the prediction. analysis of
this yields a lesson. this is our world model update."

**Scoring needs its own desk.** "we had an overloaded management desk that
was doing all this scoring. but really that needs its own desk." (The
design of 2026-11-24 says management "manages all scoring"; this is newer
and differs.)

**The rearchitecture the owner does not remember testing.** "now i
remember that we recently did a big rearchitecture and i dont even
remember testing it... i didnt even notice it, so maybe it wasnt landed
meaningfully?!?!" (Answer on 2026-11-27, from
docs/design/desks-and-engine-2026-11-24.md: it was designed and two of its
six steps landed; the capture desk, the creation verb, the scoring move
and the root as designed are not built.)

**Half-started work, and completing missions.** "theres a bunch of half
started work. thats key to the docs transform. we should look into all of
that and use it to refine the roadmap and scope of next missions. one key
factor is we need to do better at completing missions. its easy for me to
request something, much harder to land. so we make landing easier, and we
make follow through and expectations setting easier and more accurate."
