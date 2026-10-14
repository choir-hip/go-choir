# Rights by Jurisdiction: Reporting vs Copying, Learning vs Working From

Date: 2026-10-14. Supporting file for [README.md](README.md). Not legal
advice; see "Questions for a lawyer" in the README. Every legal claim
carries a source URL. Where the only source is a law-firm or blog summary
(not the opinion or statute), that is said.

## 0. Two distinctions that organise everything

**Reporting vs copying.** The law treats taking someone's facts, quoting a
few of their words to report on them, and pointing readers to the original
(reporting) very differently from republishing their expression so readers
no longer need the original (copying). US fair use names "news reporting"
as a favoured purpose ([17 USC 107](https://www.law.cornell.edu/uscode/text/17/107));
EU law has a separate exception for reporting current events and one for
quotation ([InfoSoc Directive 2001/29/EC Art. 5(3)(c),(d)](https://eur-lex.europa.eu/eli/dir/2001/29/oj)).
Facts themselves are not copyrightable anywhere that matters here.

**Learning from vs working from.** Copying a source in order to analyse it
(text and data mining, training, indexing, reading a page into a model's
context) is a reproduction and needs a defence, but most systems give one,
sometimes with an opt-out. Publishing output built from the source (quotes,
summaries, images, transcripts) is judged separately, mainly by how much
expression it carries and whether it substitutes for the original. Courts
in the AI cases have kept these two stages apart (see 1.3).

## 1. United States

### 1.1 Fair use and news reporting: the cases that frame Autopaper

| case | what happened | what it means for Autopaper | source |
|---|---|---|---|
| *Harper & Row v. Nation Enterprises*, 471 U.S. 539 (1985) | The Nation printed ~300 verbatim words of Ford's unpublished memoir, the "heart" of it, scooping a licensed excerpt. Not fair use. | "News reporting" is not a blanket pass. Limits: unpublished works, taking the heart, and pre-empting the author's own market. | [Justia](https://supreme.justia.com/cases/federal/us/471/539/) |
| *Swatch Group v. Bloomberg*, 756 F.3d 73 (2d Cir. 2014) | Bloomberg published the full recording and transcript of an invitation-only earnings call, without commentary. Fair use. | Strongest support for transcribing a newsworthy recording and publishing it as news, even verbatim and non-transformative; copyright in the call was "thin" and Swatch had no market for it. | [Stanford Fair Use summary](https://fairuse.stanford.edu/case/swatch-group-v-bloomberg) |
| *Núñez v. Caribbean Int'l News Corp.*, 235 F.3d 18 (1st Cir. 2000) | A newspaper printed modelling photos that were themselves the news. Fair use; attribution counted in its favour. | The First Circuit (which covers Massachusetts) accepts using a photo when the photo is the story. | [Copyright Office summary](https://www.copyright.gov/fair-use/summaries/nunez-caribbean-1stcir2000.pdf) |
| *Associated Press v. Meltwater*, 931 F. Supp. 2d 537 (S.D.N.Y. 2013) | A paid clipping service sent subscribers the lede (~300 characters) and the matching sentence of AP stories. Not fair use. | The edge: automated excerpting of news *as the product* competes with the source's licensing market. Autopaper must be reporting with quotes, not a clipping feed. | [Copyright Office summary](https://www.copyright.gov/fair-use/summaries/ap-meltwater-sdny2013.pdf) |
| *Authors Guild v. Google*, 804 F.3d 202 (2d Cir. 2015) | Scanning whole books to build search, with only snippets shown. Fair use. | Copying everything in order to index and analyse, while showing little, is the classic "learning from" defence. | [Copyright Office summary](https://www.copyright.gov/fair-use/summaries/authorsguild-google-2dcir2015.pdf) |
| *Andy Warhol Foundation v. Goldsmith*, 598 U.S. 508 (2023) | A licensed magazine use of a Warhol print based on a photo was not fair use: same purpose (magazine illustration of Prince) as the photo's own market. | Using a photo for the same purpose it is licensed for (illustrating a news story) weighs against fair use. This is why "grab the article's photo" is risky. | [Justia](https://supreme.justia.com/cases/federal/us/598/21-869/) |

**Hot news.** *INS v. AP* (1918) created a misappropriation tort for
copying time-sensitive news; *NBA v. Motorola*, 105 F.3d 841 (2d Cir.
1997), narrowed it, and *Barclays Capital v. Theflyonthewall.com*, 650
F.3d 876 (2d Cir. 2011), held a claim pre-empted where the defendant
reported the plaintiff's news rather than free-riding on its gathering
([NBA v. Motorola](https://en.wikipedia.org/wiki/National_Basketball_Ass%27n_v._Motorola,_Inc.);
[DMLP on Barclays](https://www.dmlp.org/blog/2011/second-circuit-rules-hot-news-claims-preempted)).
Low risk for Autopaper as long as stories add their own reporting and
attribution rather than relaying a single source's scoops wholesale.

### 1.2 First Amendment

The Supreme Court treats fair use and the idea/expression line as copyright's
"built-in First Amendment accommodations", which is why a free-speech
argument rarely beats a copyright claim on its own
([Eldred v. Ashcroft, 537 U.S. 186 (2003)](https://supreme.justia.com/cases/federal/us/537/186/);
[Golan v. Holder, 565 U.S. 302 (2012)](https://supreme.justia.com/cases/federal/us/565/302/)).
*Harper & Row* rejected a separate "public figure" First Amendment
exception ([Justia](https://supreme.justia.com/cases/federal/us/471/539/)).
Practical reading: the First Amendment protects Autopaper's right to report
what someone said in a podcast and quote it; it does so *through* fair use,
not around it. No court has given AI-assisted journalism a different First
Amendment status; the AI cases below are decided under ordinary fair use.

### 1.3 The AI cases: training vs output

| case | stage | holding / status | source |
|---|---|---|---|
| *Bartz v. Anthropic* (N.D. Cal., Alsup J., June 2025) | training | Training on lawfully acquired books "exceedingly transformative", fair use; building a library from pirate sites not. Settled Sept 2025 (≥ $1.5B, ~$3,000/work). | [Kilpatrick Townsend](https://ktslaw.com/insights/alert/2025/9/parties-reach-a-landmark-settlement-in-the-bartz-v-anthropic-litigation) |
| *Kadrey v. Meta* (N.D. Cal., Chhabria J., June 2025) | training | Fair use on that record, but dicta that "market dilution" by competing outputs could defeat fair use in most cases. | [Authors Alliance](https://www.authorsalliance.org/2025/06/26/meta-wins-on-fair-use-for-now-but-court-leaves-door-open-for-market-dilution/) |
| *Thomson Reuters v. Ross* (D. Del. 2025; 3d Cir. affirmed Sept 2026) | training (non-generative) | Copying Westlaw headnotes to build a competing legal search tool: not fair use. First appellate AI-training ruling; footnote sets generative AI aside. | [LawNext](https://www.lawnext.com/2026/09/3rd-circuit-rules-for-thomson-reuters-in-its-copyright-fight-against-legal-research-startup-ross.html) |
| *NYT v. Microsoft & OpenAI* (S.D.N.Y., MDL 3143) | training and output | Most claims survived dismissal (April 2025); summary-judgment briefing runs to Nov 2026, no ruling yet. Central theory: outputs substitute for Times journalism. | [Axios, Sept 2026](https://axios.com/2026/09/08/nyt-openai-microsoft-copyright-lawsuit) |
| *Advance Local Media v. Cohere* (S.D.N.Y., Nov 2025) | output (RAG, summaries) | Motion to dismiss denied: verbatim and close-paraphrase "summaries" of news articles plausibly infringe; also Lanham Act claims for misattributed text. SJ motions due Dec 2026. | [ChatGPT Is Eating the World (blog)](https://chatgptiseatingtheworld.com/2025/11/14/cohere-loses-its-partial-motion-to-dismiss-advance-local-medias-direct-copyright-infringement-claim/) |
| *Intercept v. OpenAI* (S.D.N.Y., Feb 2025) | training | DMCA 1202(b)(1) claim for stripping author/title/copyright notice from articles survived. | [Opinion PDF](https://ip.osbar.org/files/2025/04/Judge-Rakoff-decision-mtd-The-Intercept-Feb-20-2025.pdf) |

The US Copyright Office's Part 3 report (pre-publication, May 2025) says
training copies are prima facie reproductions, places uses on a spectrum,
and treats outputs that compete with the originals, especially where
licensing markets exist, as weighing against fair use
([report PDF](https://www.copyright.gov/ai/Copyright-and-Artificial-Intelligence-Part-3-Generative-AI-Training-Report-Pre-Publication-Version.pdf);
[Mishcon summary](https://www.mishcon.com/news/us-copyright-office-report-part-3-generative-ai-training)).

What the cases say, in one line: **learning is usually defensible if the
copy was lawfully obtained; output that substitutes for the source is
where liability lives.** That matches the reporting/copying line exactly.

### 1.4 DMCA and attribution

- **17 USC 1202** forbids knowingly removing or altering copyright
  management information (author, title, licence terms)
  ([Cornell LII](https://www.law.cornell.edu/uscode/text/17/1202)). For
  Autopaper this is a reason to *keep* bylines, photographer credits and
  licence fields on captures and images, never strip them.
- **17 USC 512** safe harbours cover hosting user material, not a
  publisher's own editorial output; Autopaper stories are Choir's own
  speech, so a DMCA agent helps only for user-posted content
  ([Cornell LII](https://www.law.cornell.edu/uscode/text/17/512)).

### 1.5 Embedding and linking images (US)

- *Kelly v. Arriba Soft* (9th Cir. 2003): search-engine thumbnails were
  fair use ([summary](https://www.copyright.gov/fair-use/summaries/kelly-arriba-9thcir2003.pdf)).
- *Perfect 10 v. Amazon* (9th Cir. 2007): the "server test", inline
  linking to an image on someone else's server is not a "display" by the
  linker ([summary](https://www.copyright.gov/fair-use/summaries/perfect10-amazon-9thcir2007.pdf));
  reaffirmed in *Hunley v. Instagram* (9th Cir. 2023)
  ([Davis Wright Tremaine](https://dwt.com/insights/2023/08/server-test-instagram-case-ninth-circuit)).
- *Goldman v. Breitbart* (S.D.N.Y. 2018) rejected the server test for
  embedded tweets of a photo, and later S.D.N.Y. cases followed; the
  Supreme Court declined to resolve the split in 2025
  ([Wilson Sonsini](https://wsgr.com/en/insights/thinking-of-framing-or-embedding-content-new-york-federal-courts-question-the-copyright-server-test.html);
  [Nixon Peabody](https://www.nixonpeabody.com/insights/alerts/2025/07/09/supreme-court-declines-to-resolve-looming-split-on-embedded-content-rights)).
- Practical reading: hot-linking a source's og:image is not safe outside
  the Ninth Circuit, and Massachusetts is in the First.

### 1.6 Massachusetts and Delaware

- **Copyright is federal**, so state of residence or incorporation does
  not change the rules; it changes *where* Choir can be sued and which
  circuit's precedent applies. Delaware incorporation means Choir can be
  sued in the District of Delaware (Third Circuit, which just affirmed
  *Ross*); operating from Massachusetts brings the First Circuit
  (*Núñez*). A plaintiff can also sue where harm is felt,
  so the Second (New York) and Ninth (California) Circuits remain in
  play. (General venue rule for copyright: [28 USC 1400(a)](https://www.law.cornell.edu/uscode/text/28/1400).)
- **Massachusetts right of publicity**, G.L. c. 214 § 3A, bars using a
  person's name, portrait or picture "for advertising or trade purposes"
  without written consent, with treble damages for knowing use; news
  content is generally outside it, but promotional uses (ads, social
  promos for Autopaper) are not
  ([statute](https://malegislature.gov/Laws/GeneralLaws/PartIII/TitleI/Chapter214/Section3A);
  [DMLP guide](https://www.dmlp.org/node/11498)). Pending bills would add
  "likeness" and "voice", including digital replicas (same DMLP/legislature
  sources) — relevant to any AI illustration of a real person.

### 1.7 AI-generated images: copyright status

No copyright without a human author: *Thaler v. Perlmutter* (D.C. Cir.
March 2025), cert. denied March 2, 2026
([Baker Donelson](https://www.bakerdonelson.com/supreme-court-denies-certiorari-in-thaler-v-perlmutter-ai-cannot-be-an-author-under-the-copyright-act)).
The Copyright Office Part 2 report (Jan 2025): prompts alone are not
enough; human selection, arrangement or modification can be protected
([report PDF](https://www.copyright.gov/ai/Copyright-and-Artificial-Intelligence-Part-2-Copyrightability-Report.pdf)).
Consequence: Choir's AI illustrations would be largely public domain in
the US; anyone could reuse them. That is fine for a newspaper.

## 2. European Union

| instrument | rule | relevance | source |
|---|---|---|---|
| DSM Directive Art. 3 | TDM by research organisations and cultural heritage institutions for scientific research; no opt-out. | Not Choir (commercial). | [EUR-Lex 2019/790](https://eur-lex.europa.eu/eli/dir/2019/790/oj) |
| DSM Directive Art. 4 | TDM by anyone, for any purpose, of lawfully accessible works, *unless the rightholder reserved it "in an appropriate manner, such as machine-readable means"* for online content; copies kept only as long as needed for the TDM. | Covers Choir's "learning from" (fetch, analyse, extract) if we honour reservations. Retention of full capture bodies "for as long as necessary" is the open point. | [EUR-Lex 2019/790](https://eur-lex.europa.eu/eli/dir/2019/790/oj) |
| DSM Directive Art. 15 | Press publishers' right over online use of their press publications by information society services, for 2 years; hyperlinks and "individual words or very short extracts" excluded. | Directly applies to Autopaper quoting EU news sites. Short quotes with links are the intended carve-out; long quotes and reproduced photos are not. Member states vary (Romania: 120 characters). | [EUR-Lex 2019/790](https://eur-lex.europa.eu/eli/dir/2019/790/oj); [Article 15 commentary](https://www.ippt.eu/legal-texts/dsmc/article-15) |
| InfoSoc Art. 5(3)(c),(d) | Exceptions for reporting current events and for quotation (work lawfully public, source and author named, extent justified). | The EU home of "reporting vs copying". *Spiegel Online* (C-516/17, 2019): reporting current events needs no prior authorisation; quotation needs a lawfully published work. | [EUR-Lex 2001/29](https://eur-lex.europa.eu/eli/dir/2001/29/oj); [CURIA C-516/17 press release](https://curia.europa.eu/jcms/jcms/p1_2278295/cs) |
| Database Directive 96/9 | Sui generis right against extracting a substantial part of a database. | Bulk-ingesting a structured dataset (not articles) can trigger it. | [EUR-Lex 96/9](https://eur-lex.europa.eu/eli/dir/1996/9/oj) |
| AI Act Art. 53 | *Providers* of general-purpose AI models: copyright policy honouring Art. 4 opt-outs, public training-data summary (template July 2025), Code of Practice. | Choir is a *deployer* of others' models, not a GPAI provider, unless it trains or substantially modifies a general-purpose model. | [EUR-Lex 2024/1689](https://eur-lex.europa.eu/eli/reg/2024/1689/oj); [Clifford Chance](https://www.cliffordchance.com/insights/resources/blogs/ip-insights/2025/10/copyright-compliance-under-the-eu-ai-act-for-gpai-model-providers.html) |
| AI Act Art. 50(4) | *Deployers* publishing AI-generated text "to inform the public on matters of public interest" must disclose it, **unless it has undergone human review or editorial control and someone holds editorial responsibility**; deepfake images/audio/video must always be disclosed. Applies from 2 Aug 2026. | The one AI Act duty that lands on Autopaper today. Either label stories as AI-written or have a named editorial owner and a review step; label any AI illustration. | [Art. 50 text](https://artificialintelligenceact.eu/article/50/); [Ropes & Gray, Aug 2026](https://www.ropesgray.com/en/insights/viewpoints/2026/08/102nfqm/you-talkin-to-me-operationalising-the-eu-ai-acts-transparency-obligations) |

**EU cases to watch**
- *Kneschke v. LAION* (OLG Hamburg, 10 Dec 2025): TDM for a training
  dataset covered by the German Art. 4 implementation; a natural-language
  ban in site terms was *not* a valid machine-readable reservation on those
  facts ([Bird & Bird](https://cm.twobirds.com/en/insights/2025/germany/higher-regional-court-hamburg-confirms-ai-training-was-permitted-(kneschke-v,-d-,-laion))).
  Other national courts have read "machine-readable" more loosely, so
  treat ToS opt-outs as binding in practice.
- *GEMA v. OpenAI* (LG München I, 11 Nov 2025): memorised lyrics in model
  weights are reproductions outside the TDM exception; output reproduction
  infringes ([Taylor Wessing](https://www.taylorwessing.com/de/insights-and-events/insights/2025/11/gema-v-openai)).
- *Like Company v. Google* (CJEU C-250/25): first CJEU AI-copyright
  reference, on whether chatbot summaries of press articles engage Art. 15
  and reproduction rights; hearing March 2026, judgment pending
  ([Stephenson Harwood](https://www.stephensonharwood.com/insights/cjeu-to-rule-on-ai-and-copyright-in-a-landmark-case-against-google/)).
  **This is the single most relevant pending ruling for Autopaper.**
- Linking and framing: *Svensson* (C-466/12) free linking to freely
  available works is not a new communication; *GS Media* (C-160/15) linking
  to infringing copies for profit presumes knowledge; *VG Bild-Kunst*
  (C-392/19) framing that circumvents the rightholder's anti-framing
  measures is a new communication
  ([C-466/12](https://curia.europa.eu/juris/liste.jsf?num=C-466/12);
  [C-160/15](https://curia.europa.eu/juris/liste.jsf?num=C-160/15);
  [C-392/19](https://curia.europa.eu/juris/liste.jsf?num=C-392/19)).

## 3. United Kingdom

- **s29A CDPA**: TDM copies only for *non-commercial* research
  ([legislation.gov.uk](https://www.legislation.gov.uk/ukpga/1988/48/section/29A)).
  No commercial TDM exception, so Choir's analysis copies of UK sources
  rest on other defences or licence.
- **s30 CDPA**: fair dealing for criticism, review, quotation, and
  reporting current events (the last excludes photographs)
  ([legislation.gov.uk](https://www.legislation.gov.uk/ukpga/1988/48/section/30)).
  Quoting UK sources in news is covered; republishing UK photos for news
  is specifically not.
- **Policy**: the March 2026 government report under the Data (Use and
  Access) Act 2025 abandoned the preferred "TDM exception with opt-out"
  and proposes no change yet ([DLA Piper](https://www.dlapiper.com/insights/publications/2026/03/tdm-and-training-ai-models-uk-government-publishes-report-and-impact-assessment)).
- ***Getty v. Stability AI*** [2025] EWHC 2863 (Ch): model weights that
  store no copies are not "infringing copies"; primary training claims were
  dropped; appeal permitted Dec 2025
  ([BAILII](https://www.bailii.org/ew/cases/EWHC/Ch/2025/2863.html)).

## 4. Japan

- **Art. 30-4** permits exploiting works for purposes that are not
  "enjoying" their expression (including data analysis), unless it
  unreasonably prejudices the owner
  ([CRIC translation](https://www.cric.or.jp/english/clj/cl2.html)).
  The Agency for Cultural Affairs' 2024 "General Understanding" says the
  exception does not apply where an enjoyment purpose coexists, and flags
  retrieval systems that output parts of the source
  ([Ueno, WIPO deck](https://www.wipo.int/documents/d/office-japan/docs-en-tatsuhiro-ueno_general-understanding-on-ai-and-copyright-in-japan_set.pdf)).
- **Art. 32** quotation (fair practice, justified purpose) and **Art. 41**
  reporting current events cover news quotes (same CRIC translation).
- Nikkei, Asahi and Yomiuri sued Perplexity (2025) over answer-engine
  outputs and ignored robots.txt
  ([The Register](https://www.theregister.com/2025/08/26/perplexity_asahi_nikkei_lawsuits/)).
  Japan's permissive training rule does not extend to output.

## 5. Others, briefly

| jurisdiction | learning (TDM/training) | working from (quotes/news) | source |
|---|---|---|---|
| Canada | No TDM exception; fair dealing for research is arguable. | Fair dealing for news reporting (s.29.2) requires naming source and author. | [Copyright Act s.29-29.2](https://laws-lois.justice.gc.ca/eng/acts/c-42/page-8.html) |
| Australia | Government ruled out a TDM exception (Oct 2025). | Fair dealing for reporting news (s.42-43). | [Mallesons](https://pulse.mallesons.com/ip-whiteboard/creators-first-no-text-and-data-mining-for-australia/); [Copyright Act 1968](https://www.legislation.gov.au/C1968A00063/latest/text) |
| Singapore | s.244 computational data analysis, any purpose, needs lawful access, cannot be contracted out of. | Fair use. | [NUS TRAIL](https://law.nus.edu.sg/trail/?p=7038) |
| India | *ANI v. OpenAI* (Delhi HC, July 2026, interim): training prima facie fair dealing; jurisdiction over a foreign provider accepted. | Fair dealing s.52. | [WTR](https://www.worldtrademarkreview.com/article/delhi-high-court-addresses-copyright-and-llm-training-in-ani-v-openai) |
| China | No TDM exception; courts found infringement in LoRA training on a character (2025). | Mandatory explicit and implicit labels on AI-generated content since 1 Sept 2025. | [Loeb & Loeb](https://www.loeb.com/en/insights/publications/2025/03/chinas-ai-labeling-measures-and-mandatory-national-standards-take-effect-september-1) |

## 6. Machine-readable signals

| signal | status | what it says | source |
|---|---|---|---|
| robots.txt | IETF RFC 9309 (2022). Crawl control, not a licence. | Per-user-agent allow/deny of paths. Many publishers now deny AI agents by name. | [RFC 9309](https://www.rfc-editor.org/rfc/rfc9309) |
| TDMRep | W3C Community Group final report (2024). | `tdm-reservation: 1` plus optional policy URL, via /.well-known/tdmrep.json, HTTP header or HTML meta; built for DSM Art. 4. | [W3C TDMRep](https://www.w3.org/community/reports/tdmrep/CG-FINAL-tdmrep-20240510/) |
| IETF AI Preferences (aipref) | Working-group draft (vocab-08, Sept 2026), not an RFC. | Vocabulary for training / search / etc. preferences carried in robots.txt and HTTP headers. | [draft-ietf-aipref-vocab-08](https://www.ietf.org/ietf-ftp/internet-drafts/draft-ietf-aipref-vocab-08.html) |
| ai.txt | Spawning's convention; separate individual IETF draft. | Per-media-type permissions for AI training. | [Spawning](https://spawning.ai/ai-txt) |
| Site terms of service | Contract, not copyright; enforceability varies; German court held natural-language ToS not machine-readable for Art. 4. | Often "no scraping / no AI". | see *Kneschke* above |

**Contract is separate from copyright.** A use that is fair use can still
breach a site's terms (YouTube's terms forbid downloading and automated
access, see [media-sources-and-terms.md](media-sources-and-terms.md)). The
remedy is different (breach of contract, account termination, possibly
anti-circumvention or computer-misuse claims) but real.
