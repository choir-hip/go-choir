# MISSION-AUTOMATIC-NEWSPAPER.md: The Choir Global Wire

**Status:** Active Development (V0)
**Owner:** Choir Core Team
**Last Updated:** 2026-05-30

## 1. Mission Statement: The Automatic Newspaper of Record for Planet Earth

Choir is building the **Automatic Newspaper of Record for Planet Earth**. This is not a mere aggregator or a passive feed reader. It is an active, agentic system designed to achieve comprehensive, real-time coverage of all noteworthy events globally, synthesized into a live-updating knowledge base. Our core deliverable is the **Choir Global Wire**: a continuous stream of high-fidelity, multilingual news issues, published every 15 minutes.

This mission demands a system that:

*   **Uncovers Hidden Signals**: Actively seeks out and prioritizes information from non-Western, non-English sources to provide unique perspectives and insights often missed by traditional media.
*   **Synthesizes at Scale**: Transforms raw, disparate signals into coherent, deeply contextualized narratives, delivering a high volume of structured stories.
*   **Maintains Epistemic Integrity**: Ensures every synthesized claim is traceable to its original source, regardless of language, upholding transparency and verifiability.
*   **Operates with Good Standing**: Respects the terms and conditions of all data providers, fostering a sustainable and ethical relationship with the global information ecosystem.

## 2. The Choir Global Wire: Output Specification (V0)

The immediate, tangible output of the source ingestion system is the **Choir Global Wire**. This is a programmatic publication designed for high-signal impact and rapid consumption.

### 2.1. Output Cadence & Volume

*   **Frequency**: Every 15 minutes.
*   **Content Volume**: Approximately 4,000 words per issue.
*   **Story Count**: 5-10 distinct, deeply contextualized stories per issue.

### 2.2. Multilingual Signal & Citation

Each story within the Choir Global Wire will be synthesized in English, but its core value will derive from its **multilingual source base**. The system will:

*   **Prioritize Non-English Sources**: Actively seek out and incorporate high-SNR sources in languages such as Arabic, Chinese, Swahili, Russian, Spanish, Portuguese, Hindi, etc., particularly for regions and topics relevant to Choir's key verticals.
*   **Inline Multilingual Citation**: Every significant claim or piece of information in a synthesized story **must** be cited with a reference to its original source, including the original language and a direct link. This demonstrates the 
system's ability to extract "extra signal" from diverse linguistic contexts.

### 2.3. Story Structure (Per-Story)

Each story in the Choir Global Wire will follow a structured format to maximize information density and clarity:

*   **Headline**: Concise, informative, and impactful.
*   **Summary**: A brief overview of the key developments.
*   **What Changed**: Objective reporting of new events or shifts.
*   **Why It Matters**: Analysis of the significance and implications.
*   **Who Disagrees / What is Contested**: Identification of differing perspectives or disputed facts, crucial for nuanced understanding.
*   **What Story is Being Pushed**: Analysis of framing, narratives, and potential agendas from various sources.
*   **Confidence and Gaps**: An assessment of the reliability of information and areas where more data is needed.
*   **What to Watch Next**: Forward-looking insights and potential future developments.
*   **Source Notes**: Detailed inline citations to all contributing sources, including original language and URL.

## 3. The 80/20 Source Stack: High-Impact, Multilingual Curation

To achieve the Choir Global Wire's ambitious goals, the ingestion system will focus on a highly curated, high-Signal-to-Noise Ratio (SNR) source stack. This selection is driven by the need for **event-driven, geopolitically dense information** that directly addresses Choir's key verticals: Asian supply chains, African and West Asia conflicts, global elections and democracy, AI, public health and wellness, and emerging cultural "scenius."

### 3.1. Tier-1 Foundational Sources (High Volume, Broad Coverage)

These sources are foundational due to their broad coverage, high signal quality, and programmatic accessibility. They form the backbone of Choir's continuous ingestion strategy.

*   **GDELT 15-Minute Update Stream**:
    *   **Description**: The Global Database of Events, Language, and Tone (GDELT) Project monitors global broadcast, print, and web news media in over 100 languages, identifying and extracting events, actors, locations, and sentiment [6]. It provides a 15-minute update stream of new events.
    *   **Relevance**: Unparalleled for its comprehensive global news monitoring across 100+ languages, directly covering all six verticals. Provides structured event data (CAMEO codes, actors, locations) and links to original source articles, making it ideal for multilingual signal extraction [6].
    *   **Access Pattern**: Poll-based ingestion of CSV files, with links to original source articles for on-demand retrieval and citation [6].

*   **Curated Telegram Channel List**:
    *   **Description**: Telegram channels are critical primary information sources for ground-truth reporting in many regions, particularly in conflict zones (West Asia, Africa) and for political movements. This involves a hand-curated list of ~50–100 high-signal public channels [7].
    *   **Relevance**: Provides immediate, unfiltered, and often multilingual information directly from regional journalists, OSINT researchers, military analysts, and government bodies, crucial for conflicts, elections, and popular movements [7].
    *   **Access Pattern**: Web preview scraping of `t.me/s/channelname` endpoints, which are unauthenticated and provide clean HTML for parsing [7].

*   **arXiv RSS Feeds (cs.AI, cs.LG, cs.CL)**:
    *   **Description**: arXiv is the leading open-access preprint server for scientific papers. The `cs.AI`, `cs.LG` (Machine Learning), and `cs.CL` (Computation and Language) categories are central to AI research [8].
    *   **Relevance**: Essential for the AI vertical, providing early access to cutting-edge research. New papers are announced via RSS feeds, allowing for rapid detection of emerging AI trends and innovations [8].
    *   **Access Pattern**: Poll-based ingestion of RSS feeds for metadata (titles, abstracts, authors, IDs), with full paper content retrieved on demand for deeper analysis [8].

*   **WHO Disease Outbreak News (DON) API + ProMED RSS**:
    *   **Description**: The World Health Organization (WHO) publishes official Disease Outbreak News (DON) reports, and ProMED (Program for Monitoring Emerging Diseases) provides an early warning system for infectious disease outbreaks [9] [10].
    *   **Relevance**: Critical for the public health and wellness trends vertical, offering authoritative and early signals of global health threats and innovations [9] [10].
    *   **Access Pattern**: WHO DON via structured JSON API; ProMED via RSS feed [9] [10].

*   **Prediction Markets (Polymarket + Kalshi) APIs**:
    *   **Description**: Prediction markets allow users to bet on the outcome of future events, often providing highly accurate forecasts for political events, elections, and technological adoption [11]. Polymarket and Kalshi are prominent platforms.
    *   **Relevance**: Provides a real-time, aggregated signal for elections, democracy, and popular movements globally, often outperforming traditional polling. Also relevant for AI adoption and other emerging trends [11].
    *   **Access Pattern**: Unauthenticated REST APIs for market listings and price changes [11].

*   **Bluesky Jetstream (Filtered)**:
    *   **Description**: Bluesky is a decentralized social network built on the AT Protocol, offering a public firehose of public posts [12].
    *   **Relevance**: Excellent for detecting early signals in AI and emerging cultural "scenius." Filtering the Jetstream for posts from `.edu` domains, known AI researchers, and cultural figures provides high-SNR insights from influential communities [12].
    *   **Access Pattern**: Stream-based ingestion via WebSocket, with client-side filtering for relevance [12].

*   **Curated RSS List (30–50 Feeds)**:
    *   **Description**: A tightly curated list of RSS feeds from specialized news outlets, blogs, and analytical publications focusing on the six verticals. This is a targeted approach, avoiding the noise of general news aggregators [5].
    *   **Relevance**: Provides in-depth, expert-level coverage for each vertical, complementing the broader GDELT stream. Examples include:
        *   *Supply Chains*: Nikkei Asia, Caixin Global, Supply Chain Dive, FreightWaves.
        *   *Conflicts*: Al-Monitor, Bellingcat, ACLED blog, The Africa Report, Al Jazeera Arabic.
        *   *Elections/Democracy*: IFES ElectionGuide, NDI, Freedom House, Varieties of Democracy (V-Dem).
        *   *AI*: Import AI (Jack Clark), The Batch (Andrew Ng), Stratechery, Ben's Bites.
        *   *Public Health*: STAT News, The Lancet RSS, BMJ, Devex Health.
        *   *Cultural Scenius*: Are.na, e-flux, Rhizome, specific Substack authors.
    *   **Access Pattern**: Poll-based ingestion of RSS/Atom feeds, utilizing HTTP Conditional Requests (ETag, Last-Modified) and adaptive polling schedules [5].

### 3.2. Tier-0 Query-on-Demand Sources (Never Ingested)

These sources are stable, well-indexed, and provide high-quality information that is best retrieved directly from the authoritative source when needed by an agent. There is no value in maintaining a local copy [2].

*   **Sacred Texts & Reference Corpora**: Quran API [13], Bible API [14], Project Gutenberg [15], Standard Ebooks [16], Wikidata [17], Europeana [18], DPLA [19], HathiTrust [20].
*   **Authoritative Statistical & Economic APIs**: FRED [21], BLS Public Data API [22], World Bank [23], OECD [24].
*   **Court Records & Legal Databases**: CourtListener API [25], Caselaw Access Project [26], EUR-Lex [27].
*   **Historical Property Data**: UK Land Registry Price Paid Dataset [28], Dubai Land Department (DLD) [29].

### 3.3. Tier-1 Metadata Ingest + Content-on-Demand Sources

For these sources, lightweight metadata and identifiers are ingested to track new items, but the full content is only retrieved on demand when a specific research task requires it. This keeps the local store lean while preserving deep access [2].

*   **Academic Preprints**: arXiv (metadata from RSS, full text on demand) [8], Semantic Scholar (metadata from API, full text on demand) [30].
*   **Event-Driven News**: GDELT (metadata from 15-minute update stream, full articles on demand) [6].
*   **Regulatory Filings**: SEC EDGAR (metadata from RSS/API, full documents on demand) [31].
*   **Wikipedia Edits**: Wikimedia EventStreams (metadata from SSE, full article content on demand) [32].

### 3.4. Tier-2 Selective Ingestion Sources (with Strict Rate Discipline)

These sources are ingested selectively and with careful adherence to rate limits, typically for time-series data or specific event types that are valuable to track locally [2].

*   **Energy Grids**: ENTSO-E Transparency Platform (Europe) [33], U.S. EIA Hourly Electric Grid Monitor [34].
*   **Air Quality**: OpenAQ [35].
*   **Maritime AIS**: AISHub [36], AISStream.io [37].

### 3.5. Tier-3 Stream Workers (Infrastructure-Dependent)

These sources provide real-time, continuous streams of data and require dedicated stream processing infrastructure. They are resource-intensive but offer the highest immediacy [2].

*   **Bluesky Jetstream** [12].
*   **Wikimedia EventStreams** [32].
*   **CertStream (Certificate Transparency Logs)** [38].

### 3.6. Excluded Sources (Low SNR or ToS Violation Risk)

Certain sources are explicitly excluded from ingestion due to low Signal-to-Noise Ratio, high risk of Terms of Service violations, or privacy concerns [3].

*   **Bulk X/Twitter Data** [3].
*   **Bulk LinkedIn Data** [3].
*   **General Facebook/TikTok/Instagram Feeds** [3].
*   **Bypassing Paywalls or Explicitly Prohibited Scraping** [2].

## 4. Technical Implementation Strategy: `sourcecycled` Daemon

The initial implementation of the source ingestion system will be a new background daemon, tentatively named `sourcecycled` or `ingestd`. This daemon will operate as a standalone executable within the `go-choir` repository, developed on a dedicated branch. This approach allows for independent development, testing, and validation of the ingestion cycle before deep integration with the broader Choir ecosystem [4].

### 4.1. Architecture and Components

The `sourcecycled` daemon will implement a continuous ingestion loop, performing the following steps every 15 minutes:

1.  **Source Polling**: Fetch new data from configured sources (RSS feeds, GDELT updates, Polymarket snapshots, WHO DON checks) based on their defined `pattern` and `poll_interval_secs` [4].
2.  **Deduplication**: Identify and filter out items that have already been processed, using a simple hash-based mechanism [4].
3.  **Vertical Scoring and Clustering**: Assign relevance scores to ingested items based on predefined keywords or basic LLM calls, and cluster related items that describe the same event from multiple sources [4].
4.  **LLM Synthesis**: Utilize the existing `internal/provider` infrastructure to call a large language model (e.g., DeepSeek v4-flash) to synthesize 5-10 distinct stories, totaling approximately 4,000 words, summarizing the most significant developments across the six key verticals. The prompt will instruct the LLM to prioritize multilingual sources, synthesize their content into English, and cite all original sources inline [4].
5.  **Artifact Storage**: Store the synthesized article, along with its metadata and raw source items, into a local SQLite database for initial evaluation and debugging [4].
6.  **Cycle Health Logging**: Record metrics such as the number of items processed, items used in synthesis, tokens spent, and latency for each cycle [4].

### 4.2. Leveraging Existing Choir Infrastructure

The `sourcecycled` daemon will strategically leverage existing `go-choir` infrastructure to accelerate development and ensure compatibility, while maintaining loose coupling:

*   **LLM Provider Integration**: The daemon will directly utilize the `internal/provider` package for making LLM calls to DeepSeek v4-flash. This ensures access to the configured LLM endpoints and authentication mechanisms without duplicating effort [4].
*   **SQLite Storage**: The `go.mod` file already includes `modernc.org/sqlite`, which will be used for local, temporary storage of ingested items and synthesized articles. This avoids introducing new database dependencies for the initial proof-of-concept [4].
*   **Event Bus (Optional)**: The `internal/events` bus can be optionally used to emit cycle-related events for observability, allowing for monitoring of the ingestion process [4].

### 4.3. Headless and Independent Operation

`sourcecycled` is designed to be headless and operate independently of other Choir components during its initial development phase. This means:

*   **No Frontend Dependency**: It will not have any user interface components [4].
*   **No VText or Dolt Integration (Initial)**: The daemon will not directly interact with the VText runtime or the Dolt database in its first iteration. Output will be to local files and SQLite [4].
*   **No Conductor or AppAgent Dependency**: It will not rely on the conductor or appagent system for its operation [4].
*   **Standalone Execution**: It can be run as a simple Go command (`go run ./cmd/sourcecycled`) or as a systemd service, facilitating easy deployment and testing [4].

### 4.4. Initial Data Schema (SQLite)

For the initial headless implementation, a simple SQLite schema will be used to store ingested data and synthesized articles:

```sql
CREATE TABLE sources (
    id          TEXT PRIMARY KEY,  -- e.g. "rss:nikkei-asia", "telegram:conflictmonitor"
    type        TEXT,              -- rss | telegram | gdelt | arxiv | polymarket | bluesky
    url         TEXT,
    name        TEXT,
    vertical    TEXT,              -- supply_chain | conflict | elections | ai | health | culture
    poll_interval_secs INTEGER,
    last_polled TEXT,
    last_etag   TEXT,
    last_modified TEXT,
    status      TEXT DEFAULT 'active'
);

CREATE TABLE items (
    id          TEXT PRIMARY KEY,  -- sha256 of (source_id + original_id)
    source_id   TEXT,
    original_id TEXT,              -- guid/url/message_id from source
    title       TEXT,
    body        TEXT,
    url         TEXT,
    published   TEXT,
    fetched_at  TEXT,
    vertical    TEXT,
    raw_json    TEXT               -- original payload
);

CREATE TABLE articles (
    id          TEXT PRIMARY KEY,  -- UUID for the synthesized article
    timestamp   TEXT,
    verticals   TEXT,              -- JSON array of verticals covered
    title       TEXT,
    content     TEXT,
    source_items TEXT,             -- JSON array of item IDs used
    llm_model   TEXT,
    tokens_used INTEGER
);
```

### 4.5. Future Integration Path

Once the `sourcecycled` daemon is proven to reliably produce high-quality synthesized articles, the integration path into the broader `go-choir` ecosystem will be straightforward:

1.  **Dolt Integration**: The `store.go` component will be modified to write directly to Dolt instead of SQLite, leveraging Dolt's versioning and data provenance capabilities [4].
2.  **VText Promotion**: Synthesized articles will be promoted into the VText system via existing promotion gates, making them discoverable by researchers and the `choir.news` aggregator [4].
3.  **Source Item Citation**: The raw source items used in article synthesis will be linked as citable evidence within the VText system, providing transparency and traceability [4].
4.  **Agent Tooling**: The source registry will be exposed as a tool for Choir agents, allowing them to query on-demand sources directly at research time [4].
5.  **Conductor Integration**: The `sourcecycled` daemon's MissionBag will connect to the conductor for broader task orchestration and resource management [4].

---

## References

[1] User Query, *Initial Project Brief*, May 30, 2026.
[2] Manus AI, *Good Standing First, Depth Over Time Principle*, May 30, 2026.
[3] Manus AI, *SNR as the Primary Filter Principle*, May 30, 2026.
[4] Manus AI, *Proposed `sourcecycled` Daemon*, May 30, 2026.
[5] Manus AI, *80/20 Source Selection for Your Six Verticals*, May 30, 2026.
[6] GDELT Project, *GDELT 15-Minute Global Event Data*, [https://www.gdeltproject.org/data.html](https://www.gdeltproject.org/data.html).
[7] Manus AI, *Curated Telegram Channel List for Geopolitical Signals*, May 30, 2026.
[8] arXiv, *arXiv.org e-Print archive*, [https://arxiv.org/](https://arxiv.org/).
[9] World Health Organization, *Disease Outbreak News (DONs)*, [https://www.who.int/emergencies/disease-outbreak-news](https://www.who.int/emergencies/disease-outbreak-news).
[10] ProMED-mail, *Program for Monitoring Emerging Diseases*, [https://promedmail.org/](https://promedmail.org/).
[11] Polymarket, *Polymarket API Documentation*, [https://docs.polymarket.com/](https://docs.polymarket.com/).
[12] Bluesky, *Bluesky Developer Documentation*, [https://docs.bsky.app/](https://docs.bsky.app/).
[13] Al Quran Cloud, *Quran API*, [https://alquran.cloud/api](https://alquran.cloud/api).
[14] Free Use Bible API, *Free Use Bible API*, [https://bible.helloao.org/](https://bible.helloao.org/).
[15] Project Gutenberg, *Project Gutenberg*, [https://www.gutenberg.org/](https://www.gutenberg.org/).
[16] Standard Ebooks, *Standard Ebooks*, [https://standardebooks.org/](https://standardebooks.org/).
[17] Wikidata, *Wikidata*, [https://www.wikidata.org/](https://www.wikidata.org/).
[18] Europeana, *Europeana APIs*, [https://api.europeana.eu/](https://api.europeana.eu/).
[19] Digital Public Library of America, *DPLA API Codex*, [https://pro.dp.la/developers/api-codex](https://pro.dp.la/developers/api-codex).
[20] HathiTrust, *HathiTrust Data API*, [https://old.www.hathitrust.org/data_api.html](https://old.www.hathitrust.org/data_api.html).
[21] Federal Reserve Economic Data, *FRED API*, [https://fred.stlouisfed.org/docs/api/fred/](https://fred.stlouisfed.org/docs/api/fred/).
[22] Bureau of Labor Statistics, *BLS Public Data API*, [https://www.bls.gov/developers/home.htm](https://www.bls.gov/developers/home.htm).
[23] World Bank, *World Bank Open Data API*, [https://data.worldbank.org/developers](https://data.worldbank.org/developers).
[24] OECD, *OECD.Stat API*, [https://stats.oecd.org/sdmx-json/](https://stats.oecd.org/sdmx-json/).
[25] CourtListener, *CourtListener API*, [https://www.courtlistener.com/api/v4/](https://www.courtlistener.com/api/v4/).
[26] Caselaw Access Project, *Caselaw Access Project API*, [https://case.law/](https://case.law/).
[27] EUR-Lex, *EUR-Lex Web Services*, [https://eur-lex.europa.eu/content/web-services/web-services.html](https://eur-lex.europa.eu/content/web-services/web-services.html).
[28] HM Land Registry, *Price Paid Data*, [https://www.gov.uk/government/statistical-data-sets/price-paid-data-downloads](https://www.gov.uk/government/statistical-data-sets/price-paid-data-downloads).
[29] Dubai Land Department, *Dubai Pulse API*, [https://dubai.dubai.ae/en/pages/dubai-pulse.aspx](https://dubai.dubai.ae/en/pages/dubai-pulse.aspx).
[30] Semantic Scholar, *Semantic Scholar API*, [https://api.semanticscholar.org/](https://api.semanticscholar.org/).
[31] SEC EDGAR, *EDGAR API*, [https://www.sec.gov/edgar/sec-api-documentation](https://www.sec.gov/edgar/sec-api-documentation).
[32] Wikimedia, *Wikimedia EventStreams*, [https://stream.wikimedia.org/](https://stream.wikimedia.org/).
[33] ENTSO-E, *ENTSO-E Transparency Platform API*, [https://transparency.entsoe.eu/](https://transparency.entsoe.eu/).
[34] U.S. Energy Information Administration, *EIA API*, [https://www.eia.gov/opendata/](https://www.eia.gov/opendata/).
[35] OpenAQ, *OpenAQ API*, [https://api.openaq.org/](https://api.openaq.org/).
[36] AISHub, *AISHub API*, [https://www.aishub.net/api](https://www.aishub.net/api).
[37] AISStream.io, *AISStream.io API*, [https://aisstream.io/](https://aisstream.io/).
[38] CertStream, *CertStream*, [https://certstream.calidog.io/](https://certstream.calidog.io/).
