# Pitch Bones — Choir (Broadsheet redesign, September 25, 2026)

The deck is set as a newspaper, in two editions that are the two product themes.
Same type, same copy, different paper. Type: Didot masthead, Iowan Old Style for
headlines/deks/body, mono for rails, labels, and contacts.

Palettes and textures are taken verbatim from `frontend/src/lib/theme.ts`:

- **Carbon Fiber Kintsugi** (dark): bg `#0B0C0D`, fg `#F2EFE7`, muted `#B2AA98`,
  subtle `#766F62`, accent `#FFD86B`, accent2 `#FFF1BC`, border
  `rgba(255,216,107,0.18)`, borderStrong `rgba(255,241,188,0.48)`, tetramark
  `#FFE18A`.
- **London Salmon** (light): bg `#FDF1EE`, fg `#3A1517`, muted `#755B56`, subtle
  `#AD9088`, accent `#9C5852`, accent2 `#244F4A`, border `rgba(156,88,82,0.16)`,
  borderStrong `rgba(91,28,31,0.2)`, tetramark `#682A28`. Texture: ruled paper
  (1px lines every 28px) plus a 1px margin rule.

The tetramark (`TETRA_MARK_PATHS` in `frontend/src/lib/tetramark.ts`) appears on
the first and last slides of both editions. Slides use flat theme paper — the
product themes' background textures (carbon weave, ruled paper) are omitted.

Source of truth for this file: `docs/deck/choir-seed-deck-2026-09-25-carbon-kintsugi.html`
and `...-london-salmon.html`.

---

## 01 · Masthead (cover)

**Rail:** Vol. I · No. 1  |  September 2026 · open source
**Overline (gold, letterspaced):** A new species of computer
**Masthead:** CHOIR
**Dek (italic):** The automatic computer.
**Foot note:** choir.news · one machine that reads the world, keeps the record, and publishes

---

## 02 · The problem

**Section:** THE PROBLEM
**Headline:** Information outruns attention.
**Dek:** The world now produces more than any person — or any team — can read, verify, and answer.
**Columns (two, with rule):**
- Models work for hours across thousands of sources. What returns is a transcript, a feed, a folder of drafts — nothing a person can absorb, trust, or answer to.
- The scarce resource is no longer intelligence. It is attention, judgment, and voice. The bottleneck moved. The tools did not.
- Every team needs a voice and a record. Most rent their distribution, and keep their memory inside someone else's product.
**Pull quote (bottom-anchored):** A chat thread is not a workbench. *A feed is not a perspective.*

---

## 03 · The fix

**Section:** THE FIX
**Statement (centered, 66px):** The work itself is the interface.
**Dek:** Agents maintain a living document — the authoritative state of the work. You read it, you edit it, and the edit steers what happens next.
**Rule line (mono, gold):** v1 —— your edit ——▶ v2 ——▶ next actions
**Note (italic):** Precommitment records make every autonomous run auditable and self-improving. Models and agents are swappable. The state is yours.

---

## 04 · The product

**Section:** THE PRODUCT
**Headline:** The autonomous publishing computer.
**Lead photo:** Texture — a living research document. Every claim carries its source.
**Side columns:**
- READS THE WORLD — Continuous ingest, verification, and synthesis across global sources.
- KEEPS THE RECORD — Living documents you steer by editing — published as articles or spoken word.
**Pull quote (bottom-anchored):** Generate the visuals. *Preserve the voice.*

---

## 05 · The record

**Section:** THE RECORD
**Headline:** It runs.
**Dek:** Live on staging today — reading, drafting, and publishing on its own.
**Photo pair:**
- Mail — autonomous correspondence. Every send gated by owner approval.
- Automatic Newspaper — a provenance-linked wire, updated continuously.
**Bottom rail (mono):** Next — production launch, then evidence of repeat use

---

## 06 · The category

**Section:** THE CATEGORY
**Headline:** A new generation of the machine.
**Dek:** Every person, team, and organization gets an automatic computer.
**By the numbers (four columns):**
- 1975 — 10³ — mainframes
- 1995 — 2×10⁹ — personal computers
- 2007 — 4.5×10⁹ — smartphones
- Now — >10⁹ — automatic computers; 10¹⁰+ with agents
**Note:** Installed base by computing generation. Software-defined computers can outnumber their owners.
**Pull quote (bottom-anchored):** 50,000 publishers + 15,000 team seats = $54M ARR. *We don't need to own the category — only to be the default workbench.*

---

## 07 · The business

**Section:** THE BUSINESS
**Headline:** Give away the computer. / Sell the network.
**Dek:** Open source is how a sovereign machine earns trust. The network is how it earns revenue.
**Three columns (with rules):**
- THE CORE — Free and self-hosted. Auditable, portable, and yours.
- THE CONVENIENCE — $35 a month for publishers. $100 a seat for teams whose meetings steer agents.
- THE NETWORK — Subscriptions, citations, paywalls, and permissioned queries over the corpus.
**Pull quote (bottom-anchored):** We don't sell computers. *We build the network where perspectives compound.*

---

## 08 · The position

**Section:** THE POSITION
**Headline:** Personal agents need somewhere to work.
**Dek:** Choir is not another assistant. It is the computer underneath — reached by web, desktop, mobile, or any agent you choose.
**Four entries (columns with rules):**
- WEB — choir.news — Responsive, in any browser.
- DESKTOP — Mac · Windows · Linux — The app, wrapped native.
- MOBILE — Autoradio — Native app, planned.
- AGENT — Any chat agent — Claude, ChatGPT, Muse, Aeon — over CLI, API, and MCP.
**Base bar (2px rule):** Choir — one state for all four — self-hosted or managed, yours.
**Pull quote (bottom-anchored):** Agents come and go. *The workbench stays yours.*

---

## 09 · The founder

**Section:** THE FOUNDER
**Headline:** Built by the person who needed it.
**Byline:** By Yusef Mosiah Nathanson — founder
**Body:** I made a living turning conflicting signals into verified decisions — professional poker, then AI engineering since 2015. Choir is the computer I needed to supervise my agents and publish at machine speed. It is a technology-risk company, and its own newspaper is the marketing.
**Note:** Product-led growth. No golf. No martini lunches.
**Portrait cutline:** Yusef Mosiah Nathanson, founder.
**Contacts:** github.com/yusefmosiah · mosiah.org

---

## 10 · The ask

**Section:** THE ASK
**Headline:** Building the system that needs to exist.
**Dek:** We are raising a seed round.
**Three columns (with rules):**
- SHIP — Finish production launch of the computer and the newspaper.
- SEED THE COHORT — Bring on the first publishers, individual and organizational.
- OPEN THE NETWORK — Turn on subscriptions, citations, and the shared corpus.
**Boxed milestone:** Production launch, then evidence of repeat use.
**Closing note:** The work continues either way.
**Contacts:** choir.news · github.com/choir-hip/go-choir
