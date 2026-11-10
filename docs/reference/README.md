# Reference pages: dependencies newer than training data

Convention (owner, 2026-11-10): a dependency that is newer than a coding
agent's training data (a model, an API, a framework, a runtime) gets one
page here, named in capitals after the dependency (`SYSTEM1.md`, `EXACT.md`).

Each page states:

- what the dependency is, and the version or date observed;
- how we call it, with file:line;
- its limits and prices, each figure marked **verified** (how, when) or
  **relayed-unverified**;
- the rules we follow;
- the mistakes already made;
- sources, with fetch dates.

Upkeep: a page is updated in the same change that changes how we use the
dependency. An agent reads the page before touching the dependency and never
answers a question about it from memory. A figure that cannot be re-checked
stays marked as relayed, never promoted by repetition.

Pages:

- [SYSTEM1.md](SYSTEM1.md): System 1 decision models (Jev today, via OpenRouter): rules, wire shape, surprises, patterns, our measurements; eight evidence subpages in [system1/](system1/).
- [ON-DEVICE-MODELS.md](ON-DEVICE-MODELS.md): models and runtimes for the
  Autoradio phone app; all candidates untested as of 2026-11-10.
- [EXACT.md](EXACT.md): the Exact app-shell runtime (pinned commit, model, surprises, decision guide), with subpages in [exact/](exact/): ui-model, native-modules, audio, build-and-ship, testing-and-debugging, internals, gaps. The runnable example is [mobile/exact-spike/](../../mobile/exact-spike/README.md).
