# Overnight Mission and Its Resource Use 2026-11-04

I wrote this on Wednesday, November fourth, a little after eleven in the morning UTC. It covers the whole overnight metamission, from Tuesday at a quarter past two UTC until now, about thirty-three hours, with most of the weight on memory, storage and model spending.

Node B's memory was tight on Tuesday evening and has been comfortable ever since, but the fastest-filling disk in this mission was not Node B at all: it was your laptop, where the Go build cache has grown to a hundred and thirty-two gigabytes.

## Where things stand

The mission had two stations. The first, progressive delivery, landed on Tuesday morning, with a ring rollback proved on staging. The second, self-development of apps on five test computers, is now wound down to crouton by your ruling. Two crouton rounds are still running, one with the Claude delegate and one without.

Node B never killed a process for lack of memory. Its lowest point was about four point eight gigabytes free on Tuesday evening. Since the build cap and the per-job memory leases went in, the floor has stayed between eight and fifteen gigabytes. One thing in that story is wrong, and I need to fix it: build jobs keep getting refused for memory at up to two hundred and forty-five an hour, while the machine has plenty free.

Node B's disk has a hundred and forty-seven gigabytes free, its lowest of the mission, but its use goes up and down rather than climbing. Your laptop is at eighty-eight percent, and the Go build cache alone grew by as much as fifty gigabytes on Tuesday. Nothing has been cleaned yet. Clearing that cache is the single most useful thing you could approve.

## What the mission did

In about thirty-three hours the work landed three hundred and eighty-one commits on main. A hundred and ninety-five of them touched code. Together they changed seven hundred and thirty-eight files, adding about seventy-five thousand lines and removing about eleven thousand. The mission also wrote forty-five problem records.

The self-development loop now publishes real apps. Slides, Files, several Library builds and a Calendar build came out of the test computers. This morning I landed six changes to that loop.

- An engineer whose build is rejected now keeps working in the same assignment for up to six attempts, instead of three.
- Time spent waiting for review no longer counts against the engineer's build budget.
- A rejection now grants at least twenty minutes to fix.
- The screenshot judge retries once when its answer comes back malformed.
- The judge now reads the notes you add after a request starts.
- The judge no longer fails a list row that is cut off only because it scrolls under the edge of its panel.

I also exported pool's Library build into the repository as a new first-party app, and built the command that does that export. Its deploy is in CI as I write. A separate builder is working on the install path, so you can install an app from the Library.

The crouton results are slow but real. Crouton's calendar showed events for the first time in eight rounds this morning, after I told its engineer to import the schedule through the app instead of writing the file directly. The judge still rejected that build, fairly: the cards overlap, the times ignore the time zone, and the phone screenshots were taken at desktop size. Its second build failed the same way a minute after it froze. The trace shows why the phone shots keep failing: the engineer names a shot "mobile", but the screenshot tool takes the phone size only from a flag. That is cheap to fix in the tool itself. Engineer reuse works mechanically on every test computer. The engineer resumes within seconds and fixes take between one and a half and nine minutes. None of the six reused builds has passed yet.

## Memory over the mission

On Tuesday morning the work was light, and free memory climbed from about fifteen gigabytes before three o'clock to twenty-three gigabytes by eleven. Through the afternoon it sat between twelve and nineteen.

The squeeze came on Tuesday evening, when several test computers built apps at the same time. By about seven, memory pressure spiked hard. Swap had already reached about three gigabytes at six. At nineteen minutes past nine, free memory reached its low of four point eight gigabytes, with eight virtual machines running. Even then the kernel killed nothing.

Two changes ended the squeeze. Between about a quarter to eleven and a quarter to midnight, I capped how many test computers could build at once, first at three, then at two. Around two on Wednesday morning, per-job memory leases replaced the cap. Since then the hourly minimum has stayed between eight and fifteen gigabytes. Swap has mostly stayed under a gigabyte, with brief rises to about one and a half gigabytes in the busiest hours. Each of the eight running machines holds between one point two and three point two gigabytes.

There are two loose ends. The first is the refusals. In the worst hour on Tuesday night, forty-nine build jobs were refused, and that was correct, because memory really was short. After the leases went in, refusals rose to between a hundred and forty and two hundred and forty-five an hour, while at least eight gigabytes stayed free. My check-in at a quarter to eight said there were no memory refusals in that hour. That was wrong. The log shows refusals in every hour since five. My working guess is that a lease reserves the full eight gigabytes of a guest's configured memory, while a guest actually holds about two. That is a hypothesis until I read the lease code. If it holds, the leases are throttling builds for no reason.

The second loose end is that the virtual machine controller restarted about forty-eight times in the metered window. I have not explained that number yet. It belongs in an investigation, not a guess.

## Storage on Node B

Node B's root disk holds four hundred and seventy-six gigabytes. Free space ran from a hundred and forty-six gigabytes early Tuesday to a high of a hundred and seventy-eight on Tuesday morning. It is back at about a hundred and forty-seven now, the low of the mission.

That number swings rather than climbs. Used space moved between about two hundred and forty-two and two hundred and eighty-four gigabytes, as guest release builds and the Nix store grew and were collected. The sixteen-hour metered window shows a net rise of about thirty-one gigabytes. Most of that is the sawtooth, not a trend.

Two things do grow steadily. The tape segments grow about a hundred and fifty megabytes an hour, roughly three and a half gigabytes a day. The Postgres control store grows about twenty-five megabytes an hour. At those rates, steady growth alone would take more than a month to matter. The real risk is a sawtooth peak landing on a low day.

The largest directories are platform build artifacts at a hundred and ten gigabytes, virtual machine state at a hundred and two, and guest releases at forty-nine. Inside the machine state, one long-lived computer holds twenty-eight gigabytes, and one candidate fleet holds eighteen. There are a hundred and sixty-eight hibernate snapshots, at about two-thirds of a gigabyte each in the median case.

## Model use and money

The provider meter saw about thirty-eight and a half thousand model requests in the sixteen-hour window from Tuesday evening, about two thousand two hundred an hour, rising to about two thousand eight hundred after five on Wednesday morning.

GPT-6 Luna through ChatGPT carried most of them, about twenty-six thousand, or seven in ten. DeepSeek through OpenRouter carried about five thousand. DeepSeek through Synthetic carried about four thousand. Crouton carried about three thousand two hundred. The Claude relay carried under nine hundred.

The OpenRouter drain was my mistake. On Wednesday morning, when the Synthetic weekly quota ran out, I moved four test computers to DeepSeek on OpenRouter. That spent your OpenRouter credit to zero. Requests began failing with payment errors at eighteen minutes past ten, and the failures lasted about ten minutes. I have moved those computers back to Synthetic. I removed the DeepSeek route from the OpenRouter catalog, and the gateway now admits only crouton there. Jev keeps its own separate route. Since then, the Synthetic quota has run out twice more: once at twenty past three, and again at nine minutes to eleven, after a brief refill. The test computers on Synthetic are short on quota for the rest of its week.

For dollars, I only have the figures recorded on Tuesday morning: about three dollars for the claude test computer, two for pool, five for deepseek and one for luna. The Claude relay reports a daily spend that rose about ninefold over the window, but I could not find its unit. I will not turn it into dollars by guessing.

## Your laptop's disk

Your laptop has a nine hundred and twenty-seven gigabyte disk, with about a hundred and twenty gigabytes free. A Sonnet agent measured it, read only, and changed nothing.

The Go build cache is the cause, by a wide margin. It holds a hundred and thirty-two gigabytes and has never been trimmed since it was created on October twenty-sixth. Its daily writes ran from thirteen to twenty-three gigabytes through last week, then about fifty gigabytes on Tuesday, and twenty-two so far today. Those counts include files the cache only touched, so they overstate new bytes somewhat. The size itself is real. The cache grows this fast because this work runs on hundreds of worktrees. Each one sits at a different path and builds with different flags, so the same package compiles again and again under new cache keys. The race-detector test binaries for the two big packages make it worse.

The next consumers are smaller.

- Docker's disk image holds fifty-nine gigabytes.
- This session's own scratch folder holds twenty-seven gigabytes. Twelve of that is three copies of a churn database from October thirty-first.
- About three hundred registered worktrees hold roughly forty gigabytes together.
- Xcode and its simulators hold forty-eight gigabytes. Model caches and downloaded media hold about twenty-five gigabytes each.

Clearing the Go build cache would give back about a hundred and thirty gigabytes at almost no risk. The only cost is one slow first build afterwards. It should run while no builds are running, and a weekly trim would keep it from coming back. My scratch folder is the second easy win. Five worktrees have uncommitted changes, including the install builder's, and none of them should be touched.

## What I would do next

Remember this: Node B came through without an out-of-memory kill, but its memory leases now refuse work the machine could carry. Meanwhile your laptop, not the server, is the disk that is running out.

If I were holding the phone, I would do four things in order. First, approve clearing the laptop's Go build cache and my scratch folder. Second, have me read the lease accounting and fix the refusals. Third, have me explain the forty-eight controller restarts. Fourth, let the Library install work land, so you can review the apps through it.

## Names and receipts

- Metamission goal: docs/definitions/choir-overnight-pd-then-selfdev-metamission-2026-11-03.md, started 2026-11-03 02:15Z.
- Mission scale: 381 commits on main since 1e836b78d; 195 touch code; 738 files changed; +75,268 and −11,439 lines; 45 problem docs dated 11-03 or 11-04.
- Commits landed this morning:
  - e2634dd4f: reuse cap 6.
  - f20398247: judge JSON retry.
  - afc343285: review wait suspends the deadline; 20-minute fix allowance.
  - 1c10e2971: judge reads owner notes.
  - 62b0d4174: OpenRouter allowlist is stealth/crouton-alpha only.
  - ff5377018: judge scroll-edge exception.
  - 12303db0c and 822af2ff3: docs.
  - ebd02b93d, 57abdf783, 4713b8150 and 8d78b7e78: package export and library-next. CI run 41816511175 was in flight at writing.
- Node B memory source: /var/lib/go-choir/growth-meter/mem-2026-11.jsonl, sampled each minute.
  - Window: 2026-11-03 18Z to 2026-11-04 about 11Z.
  - Low: 4.8 GiB at 21:19Z. PSI max 10.2 at 19:20Z. Swap max 3065 MiB at 18Z. Kernel OOM count 0.
- Refusals (refuse_job): 49 in the 22Z hour; after leases, 139 per hour at 05Z, 245 at 09Z, 199 at 10Z.
  - Admission floor: 4779 MiB plus 1024 MiB headroom. Hibernate guest memory: 8 GiB.
  - About 48 vmctl restarts.
  - The 07:45Z ledger line claiming 0 memory refusals is wrong.
- Node B disk:
  - Root /dev/md127, 476G. Used range 241.8 to 284.4 GiB. Lowest free 146.7G at 10Z on 11-04.
  - tape-seg grew from 52.9 to 55.4 GiB. Postgres control grew from 19.5 to 19.9 GiB.
  - platform-artifacts 110G, vm-state 102G, guest-releases 49G, 168 hibernate snapshots.
- Providers (window total 38,544 requests): chatgpt gpt-6-luna 26,434; openrouter deepseek 4,979; synthetic deepseek 3,928; openrouter stealth/crouton-alpha 3,203; anthropic relay claude-sonnet-5-5 881.
  - OpenRouter 402s from 10:18:26Z to 10:28:58Z.
  - Synthetic quota exhausted at 02:32Z, 03:20Z and 10:51Z on 11-04.
  - Relay day_spent went from 192,471 to 1,714,765, unit unknown.
- Crouton Calendar r8:
  - op selfdev-ecbc2c1b479a5117b1eedd164e15f5dc, assignment-a7772e7e, trajectory eb3a2cc5.
  - Frozen at 10:55:41Z and rejected at 10:56:18Z.
  - Crouton calibration: n=52, Brier 0.226.
- Laptop disk:
  - GOCACHE /Users/wiz/Dev/.caches/go-build, 132G.
  - Docker.raw 59G.
  - Session scratchpad /private/tmp/claude-501, 27G.
  - ~/Dev/lanes, 25G.
  - Dirty worktrees: /private/tmp/choir-review, /private/tmp/p28-review, /private/tmp/relshots-review-wt, /Users/wiz/Dev/go-choir-pkg-install and /Users/wiz/Dev/go-choir-switches-settings.
