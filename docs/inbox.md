# Inbox

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
