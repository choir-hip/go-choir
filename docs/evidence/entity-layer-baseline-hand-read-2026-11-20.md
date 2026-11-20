# The story entity layer: a hand-read baseline (2026-11-20)

Station E0 of [the entity layer design](../design/entity-layer-2026-11-20.md).
Read by a builder who did not write the design, on 2026-11-20 between 05:36Z
and 05:40Z. This is the number later stations (E3 to E6) are scored against.
Class: green (a record). Evidence tags as in the design: **M** measured,
**C** read from code at `origin/main` (checked in the lane `entity-e0`),
**J** a judgment of mine, **U** not verified.

## 1. What was read, and how to repeat it

- **Source.** The public front list, `GET /api/platform/publications/front?limit=100`
  with its cursor, 300 stories, fetched 2026-11-20T05:36Z (newest story updated
  05:33Z, oldest 2026-11-19T07:15Z), and `GET /api/platform/entities/growth`.
  No key, no ssh, no write (**M**). The front read serves each story's central
  entities as `{id, label}` only: no kind, aliases, Wikidata id or p.
- **Sample.** All central attachments of those 300 stories (682, from 231
  stories that have any; 444 distinct entity ids), sorted by (publication id,
  entity id), then `random.Random(20261120).sample(rows, 200)`:
  `scripts/status-entities --newest 300 --sample 200 --seed 20261120`.
  The front list moves every few minutes, so a later fetch draws a different
  200. **The table below is the record**; the rows are also reproducible from
  any saved copy of the two reads with `--front-fixture` and `--growth-fixture`.
- **Rubric, fixed before reading** (design section 2, applied as written). An
  entity is a named, persistent person, organisation, place, or named product,
  work or law. Not an entity: a topic, an event, a demonym or adjective, a
  common noun, a role without its holder, a fragment of a longer name or of a
  headline.
  (a) **Entity?** E or N. (b) **Right entity for this story?** R or W, judged
  only on E rows, from the title and the first 170 characters of the summary:
  W is a same-name confusion or a label that is not what the story names.
  (c) **Duplicate?** D when another id in the 444 distinct ids of the window
  (or another row of the sample) is the same real thing, judged on E rows
  only. "-" is none found, not none exists.
- **Reader.** One reader, not blind to the code (I read the code paths below
  before the table, and the design's own tally of 240 front attachments, which
  I did not use). No second reader, so no agreement figure.

## 2. The three rates

| Question | Count | Rate |
|---|---|---|
| (a) not an entity | **58 of 200** | **29.0%** (95% interval about 23% to 36%) |
| (b) an entity, but the wrong one for the story | **2 of 142** entity rows | 1.4% (1.0% of all 200) |
| (c) an entity that has a second id in the window | **34 of 142** entity rows | **23.9%** (17.0% of all 200) |

Claims in the design: C-E1 (at least 12% of 200 are not entities, p 0.80)
is **true**, 29.0%. C-E2 (at least 10 with a second id, p 0.65) is **true**,
34. The design's own quick tally (17% not entities) was low by 12 points; the
likely reasons are my stricter rule on fragments ("Americas", "Communications",
"Spice") and that the sample is by attachment, so a story-popular id such as
"U.S." (26 stories) fills the sample.

What the 58 non-entities are (**J**): 25 common nouns, words or roles (Health,
IPO, Driver, Wife, Season, Match, Nothing, Lawyer ...); 16 demonyms or adjectives
(Russian 3, American 2, Democratic 2, Chinese, Indian, Korean, Emirati ...); 14
fragments of a longer name or a headline (National, Communications, Channel,
Americas, Opens, Spice, "Mississippi Elects Scott Colom", "Holiday Special
Returns" ...); 3 events (Iran war 2, Diwali).

(b) is low because an id is a name and the sample's names are mostly
unambiguous. It undercounts: the failure is visible in ids that carry
**different things across stories**, which a 200-row sample meets only
by chance (section 4, pattern 2).

The deterministic rule in `scripts/status-entities` against this read: it
flags 25 of the 58 non-entities and none of the 142 entities (precision 100%,
recall 43%). The 33 it misses are fragments, descriptive phrases and common
nouns outside its short word lists. Its lists were written after I had seen
these rows, so its precision is flattering and the recall is the number to
believe.

## 3. The table

Columns: row, story title (cut at 78), entity label, (a) E or N, (b) R or W,
(c) D or -, note. The ids are in the saved read, not here.

| # | Story | Entity | a | b | c | Note |
|---|---|---|---|---|---|---|
| 1 | Anthropic raised a record $81.3bn in the largest stock-market debut on reco... | Anthropic | E | R | - |  |
| 2 | 'PI' more than doubled output at its new Massachusetts plant — not U.S. man... | U.S. | E | R | D | U.S. / United States |
| 3 | HHS issues final HIPAA Privacy Rule changes on care coordination and patien... | Health | N | - | - | fragment of Health and Human Services |
| 4 | Zelensky says Ukraine would welcome US military bases as part of security g... | Nothing | N | - | - | a common word |
| 5 | Jamaica’s opposition seeks explanation from Holness over Melissa relief eff... | ODPEM | E | R | - |  |
| 6 | Fake AI crawlers are hunting exposed API keys, Tin.computer reports | Gecko | E | R | - |  |
| 7 | Former Ontario minister Jill Dunlop refuses questions on growing OSAP scandal | Ontario | E | R | - |  |
| 8 | Democrats push to defund the Iran war before Dec. 11 funding deadline, rais... | Iran | E | R | - |  |
| 9 | UnitedHealth shares plunge as CMS proposes Medicare Advantage crackdown wor... | U.S. | E | R | D | U.S. / United States |
| 10 | Ford tells EU that 'Made in Europe' rules excluding Britain, Turkey and Mor... | Ford Motor | E | R | - |  |
| 11 | Russia launches massive missile and drone attack on Ukraine overnight | Russia | E | R | - |  |
| 12 | Seoul subway suspends Seongsu branch after signal-violating train damages t... | Seoul | E | R | - |  |
| 13 | OpenAI builds a "Performance Factory" to keep ChatGPT fast as agentic codin... | ChatGPT | E | R | - |  |
| 14 | Hormuz tanker attacks: Iran claims of mine strikes go unverified as casualt... | Strait of Hormuz | E | R | - |  |
| 15 | Obsbot launches Tail 2 ADV PTZR camera pairing 21x optical zoom with AI tra... | ADV PTZR | N | - | - | fragment of a product name |
| 16 | National Audit Office warns next BBC savings round will be more noticeable ... | National Audit Office | E | R | - |  |
| 17 | Brazil takes step to recognize ancestral territory of Kayapó chief Raoni | Brazil | E | R | - |  |
| 18 | Moonshot AI scrapped $150M Nvidia B300 deal routed through Indonesia | Moonshot AI | E | R | - |  |
| 19 | SIPTU recommends members accept new Irish public-sector pay deal | SIPTU | E | R | - |  |
| 20 | NWSL goalkeeper Ann-Katrin Berger named 2026 Goalkeeper of the Year – Europ... | NWSL | E | R | - |  |
| 21 | Saarland parliament declares extraordinary state of emergency for 2027 | Saarland | E | R | - |  |
| 22 | Federal judge: police can't delegate probable cause to AI, letting wrongful... | Fourth Amendment | E | R | - |  |
| 23 | Philippine police charge Meghan Markle's father with three child abuse mate... | Markle | E | W | D | Markle is a surname; the story names Thomas Markle and Meghan Markle; Markle / Thomas Markle |
| 24 | Xi, Trump and Putin hold historic trilateral at APEC, issue 'peace declarat... | China | E | R | - |  |
| 25 | Paris launches 'MascuRadar', a card game to counter sexist discourse | Mascuradar | E | R | - |  |
| 26 | Merz warns Ukraine peace talks without Europe are doomed to fail | Europe | E | R | D | Europe / Europa |
| 27 | Family of Robb Elementary victim urges hospitals to prepare for mass-casual... | Uvalde | E | R | - |  |
| 28 | Kremlin's Kiriyenko Steps Into the Spotlight, Pushing Troops' Return as His... | Kremlin | E | R | - |  |
| 29 | National Audit Office warns next BBC savings round will be more noticeable ... | BBC | E | R | - |  |
| 30 | Former Howard-era minister Sharman Stone dies by voluntary assisted dying, ... | Australian | N | - | - | demonym |
| 31 | Mongolia's parliament approves President Ukhnaa's resignation; election due... | Parliament | N | - | - | common noun, no holder |
| 32 | National Audit Office warns next BBC savings round will be more noticeable ... | National | N | - | - | fragment of National Audit Office |
| 33 | Virginia district closes after ~500 staff missed child-abuse screening | Albemarle County Public Schools | E | R | - |  |
| 34 | Abby Meyers on returning to Israel and the women's EuroBasket qualifying fight | Maccabi Ashdod | E | W | - | story says Maccabi Bnot Ashdod (women's club); label is a different club's name |
| 35 | Philippine police charge Meghan Markle's father with three child abuse mate... | Thomas Markle | E | R | D | Thomas Markle / Markle |
| 36 | Kushner says Trump open to Article 5-style security guarantee for Ukraine | Jared Kushner | E | R | - |  |
| 37 | Mississippi Elects Scott Colom, Its First Black U.S. Senator in Nearly 150 ... | Mississippi | E | R | - |  |
| 38 | Indonesia parliament passes asset forfeiture bill into law | Bill | N | - | - | common noun |
| 39 | Trump calls nuclear disarmament with China and Russia "very difficult" afte... | Shenzhen | E | R | - |  |
| 40 | Google unveils Gemini 4 Neon, a faster Gemini 4 model pitched as near-flags... | Gemini | E | R | - |  |
| 41 | Houthi Strike Damages Saudi Sovereign Wealth Fund Headquarters in Riyadh | Riyadh | E | R | - |  |
| 42 | Xi, Trump and Putin hold historic trilateral at APEC, issue 'peace declarat... | APEC | E | R | - |  |
| 43 | Weil's wind-down confirmed: hundreds of partners race to rivals as century-... | Weil | E | R | - |  |
| 44 | UK equality chief urges 'take the temperature down' over single-sex space g... | UK | E | R | D | UK / United Kingdom |
| 45 | AbbVie pays up to $2.5 billion to tap German biotech Disco's bispecific can... | Disco | E | R | - |  |
| 46 | Cyberattack takes European Commission personnel systems offline for days, a... | AI API | N | - | - | fragment (Artex is the tool) |
| 47 | HHS issues final HIPAA Privacy Rule changes on care coordination and patien... | HHS | E | R | - |  |
| 48 | France's transport regulator approves CDG-Orly expansion deal, with five re... | CDG | E | R | - |  |
| 49 | Leaders of over 30 states to gather in Berlin for 'Coalition of the Willing... | Coalition of the Willing | E | R | - |  |
| 50 | Hegseth signs US-Venezuela military cooperation deal in Caracas | US Defence | N | - | - | role fragment (US Defence Secretary) |
| 51 | BMO returning to U.S. oil and gas investment banking, six years after its exit | BMO | E | R | D | BMO / Bank of Montreal |
| 52 | Hegseth signs US-Venezuela military cooperation deal in Caracas | Defence | N | - | - | common noun |
| 53 | Helsing scales back its £350m Plymouth factory after losing Royal Navy cont... | Downsizing | N | - | - | topic word |
| 54 | Netflix green-lights adult animated 'Grendel' series | Grendel | E | R | - |  |
| 55 | Kremlin's Kiriyenko Steps Into the Spotlight, Pushing Troops' Return as His... | Russia | E | R | - |  |
| 56 | Everful raises $48.5M Series A to connect retailers with Chinese factories | Chinese | N | - | - | demonym |
| 57 | Trump says he and Xi want moderate AI safety rules | Xi Jinping | E | R | - |  |
| 58 | Family of Robb Elementary victim urges hospitals to prepare for mass-casual... | Robb Elementary | E | R | - |  |
| 59 | Somalia orders UAE ambassador out within 72 hours, calling it an 'open inva... | Emirati | N | - | - | demonym |
| 60 | Judge finds Thomas Markle mentally fit to stand trial on child sexual abuse... | Thomas Markle | E | R | D | Thomas Markle / Markle |
| 61 | Denmark ratifies US–Greenland Arctic pact, expanding American military role | America | E | R | D | America / U.S. |
| 62 | FCC lets state prisons jam inmate cellphones for first time ever | Communications | N | - | - | fragment of Federal Communications Commission |
| 63 | U.S. religious freedom panel accuses India of targeting Muslims in voter ro... | Indian | N | - | - | demonym |
| 64 | Pentagon has no overarching plan to fix munitions shortfall worsened by Ira... | Department | N | - | - | role word |
| 65 | French prosecutors seek up to two years in prison over 'Femmeblancherie' Te... | France | E | R | - |  |
| 66 | Pedestrian struck at Nellis Blvd and Stewart Ave in Las Vegas | Las Vegas | E | R | - |  |
| 67 | UK sanctions eight Israeli firms building West Bank settlements; Sa'ar vows... | British | N | - | - | demonym |
| 68 | Moonshot AI scrapped $150M Nvidia B300 deal routed through Indonesia | Chinese AI | N | - | - | descriptive phrase |
| 69 | Nvidia says it is investigating a possible partner breach after ransomware ... | Gentlemen | E | R | - |  |
| 70 | Violent thefts jump 75% in Lyon as Rhône ranks second in France for insecur... | Rhône | E | R | - |  |
| 71 | Saudi crown prince meets US CENTCOM chief in Riyadh as Houthi strikes hit t... | Houthi | E | R | - |  |
| 72 | Ford tells EU that 'Made in Europe' rules excluding Britain, Turkey and Mor... | European Commission | E | R | - |  |
| 73 | Korean opera 'Hwajeonga' gets its French premiere in Montpellier this weekend | Korean | N | - | - | demonym |
| 74 | Army postpones Friday detonation of nearly 10,000 pounds of lead azide at I... | Health | N | - | - | fragment of Health and Human Services |
| 75 | Trump administration moves to block ABA questioning of White House officials | ABA | E | R | D | ABA / American Bar Association |
| 76 | European Commission deployed Matrix-based chat to more than 40,000 users, E... | EU | E | R | - |  |
| 77 | Iraq reaches compromise with Kataib Hezbollah over Jurf al-Sakher stronghold | Jurf al Sakhr | E | R | D | 3 ids: Sakhar, Sakher, Sakhr |
| 78 | Abby Meyers on returning to Israel and the women's EuroBasket qualifying fight | National | N | - | - | fragment of National Audit Office |
| 79 | Indian truck driver wins ₹12 crore (~$1.3m) in Punjab Diwali lottery | Driver | N | - | - | common noun |
| 80 | Fos Provence Basket loses 113–105 at Rouen in Élite 2 | Fos Provence Basket | E | R | - |  |
| 81 | 'Rivals' No More: Research Reveals Rationale Behind Disney-Channel 4 Deal | Channel | N | - | - | fragment of Channel 4 |
| 82 | Gulf stocks retreat as Iran conflict worries and Fed signals loom | Gulf | N | - | - | bare region word |
| 83 | Cyberattack disrupted European Commission systems for over a week, Politico... | European Commission | E | R | - |  |
| 84 | Dane County prosecutor declines charges in fatal Madison police shooting of... | Wisconsin | E | R | - |  |
| 85 | Indian truck driver wins ₹12 crore (~$1.3m) in Punjab Diwali lottery | Diwali | N | - | - | an event or festival |
| 86 | Anthropic completes record $81.3bn IPO at $1.5 trillion valuation | IPO | N | - | - | common noun |
| 87 | Mississippi Elects Scott Colom, Its First Black U.S. Senator in Nearly 150 ... | Mississippi Elects Scott Colom | N | - | - | headline fragment |
| 88 | OpenAI builds a "Performance Factory" to keep ChatGPT fast as agentic codin... | Codex | E | R | - |  |
| 89 | Bukele due to register for unprecedented third El Salvador presidential term | Nayib Bukele | E | R | - |  |
| 90 | Tórshavn council left without quorum over conflict-of-interest dispute | Faroese | N | - | - | demonym |
| 91 | World champion Peres Jepchirchir banned three years for EPO use; Sydney win... | World | N | - | - | fragment (World champion) |
| 92 | Edinburgh festival to mark 80-year anniversary with longest ever season | Season | N | - | - | common noun |
| 93 | IG Metall calls 24-hour warning strikes across Germany's metal and electric... | IG Metall | E | R | - |  |
| 94 | Four men convicted in England after husband drugged wife and arranged her a... | Wife | N | - | - | common noun |
| 95 | Grenoble prosecutor contests racist aggravating circumstance in Crépol kill... | Crépol | E | R | - |  |
| 96 | B. Riley upgrades AST SpaceMobile to Buy, calling its 60% selloff excessive | AST SpaceMobile | E | R | - |  |
| 97 | Russian national indicted for acting as foreign agent of Russia to target S... | San Francisco | E | R | - |  |
| 98 | Hormuz tanker attacks: Iran claims of mine strikes go unverified as casualt... | Iran | E | R | - |  |
| 99 | Ravens deactivate star safety Marlon Humphrey for Sunday after podcast joke... | Marlon Humphrey | E | R | - |  |
| 100 | House Democrats nominate Hakeem Jeffries for speaker, 251-11 | Democratic | N | - | - | adjective |
| 101 | Kushner says Trump open to Article 5-style security guarantee for Ukraine | Donald Trump | E | R | D | Donald Trump / Trump |
| 102 | Somalia orders UAE ambassador out within 72 hours, calling it an 'open inva... | United Arab Emirates | E | R | D | UAE / United Arab Emirates |
| 103 | House Democrats nominate Hakeem Jeffries for speaker, 251-11 | Democrats | E | R | - |  |
| 104 | Zelensky says Ukraine would welcome US military bases as part of security g... | U.S. | E | R | D | U.S. |
| 105 | Philippine police charge Meghan Markle's father with three child abuse mate... | Philippine | N | - | - | adjective |
| 106 | Former RCMP assistant commissioner David Teboul sentenced to conditional di... | David Teboul | E | R | - |  |
| 107 | US jails Pakistani man for 27½ years over Iran-backed plot to kill Trump, B... | Donald Trump | E | R | D | Donald Trump / Trump |
| 108 | France and Malaysia agree rare-earth deal as Macron secures supplies beyond... | Malaysia | E | R | - |  |
| 109 | EU economy chief tells France and Belgium to put deficits and debt on a dow... | European Commission | E | R | - |  |
| 110 | Gig-work staffing apps are filling shifts in US nursing homes with little o... | American | N | - | - | demonym |
| 111 | Regavim denounces Supreme Court intervention in Judean Desert nature reserv... | Judean Desert | E | R | - |  |
| 112 | LIV Golf shutting down after Saudi funding ends and rescue deal collapses | LIV Golf | E | R | - |  |
| 113 | Iran war cancels Middle East sporting events as two season-ending F1 races ... | Iran war | N | - | - | an event |
| 114 | Italy’s parliament gives final approval to antisemitism legislation | Italy | E | R | - |  |
| 115 | US intelligence reportedly indicates Iran is developing plans for a 'dirty ... | Iran | E | R | - |  |
| 116 | Fulshear City Council approves helmet rule for young riders, delays broader... | Safety | N | - | - | topic word |
| 117 | Musk calls Zelenskyy a liar at DealBook as Starlink feud escalates into Con... | Starshield | E | R | - |  |
| 118 | Latvia's new cabinet pledges a 'de-Russification declaration' to curb Russi... | Latvia | E | R | - |  |
| 119 | Meta's AI agent Muse accessed encrypted Signal messages and sent fabricated... | Agent | N | - | - | common noun |
| 120 | Hegseth hails deepest U.S.-Colombia security ties on Bogotá visit as Colomb... | Americas | N | - | - | fragment of Americas Counter Cartel Coalition |
| 121 | Mississippi Elects Scott Colom, Its First Black U.S. Senator in Nearly 150 ... | U.S. | E | R | D | U.S. |
| 122 | Crypto industry braces for skeptical Congress after Capitol Crypto Forum | Democrats | E | R | - |  |
| 123 | Eight Senate Republicans depart in January, some openly defying Trump | Trump | E | R | D | Trump / Donald Trump |
| 124 | Xi, Trump and Putin hold historic trilateral at APEC, issue 'peace declarat... | Russian | N | - | - | demonym |
| 125 | Northern Ireland resident doctors to strike for 24 hours from Friday over pay | Doctors | N | - | - | common noun |
| 126 | Houthi Strike Damages Saudi Sovereign Wealth Fund Headquarters in Riyadh | Houthi | E | R | - |  |
| 127 | Democrats push to defund the Iran war before Dec. 11 funding deadline, rais... | Senate | E | R | - |  |
| 128 | Canada courts AI data centre builders as US projects face local resistance | U.S. | E | R | D | U.S. |
| 129 | Taiz, Yemen's besieged city, faces a deepening water crisis | City | N | - | - | common noun |
| 130 | "Spice and Wolf" gets a second season, broadcast from January 2027 | Spice | N | - | - | title fragment (Spice and Wolf) |
| 131 | EU economy chief tells France and Belgium to put deficits and debt on a dow... | EU | E | R | - |  |
| 132 | Four men convicted of raping a woman her husband drugged and sold to strangers | Woman | N | - | - | common noun |
| 133 | "Ragazzi al fiume" has its world premiere at IDFA, following a day in "para... | IDFA | E | R | D | IDFA / International Documentary Festival Amsterdam |
| 134 | Nvidia investigates possible breach claim by The Gentlemen ransomware group | Nvidia | E | R | - |  |
| 135 | Saarland parliament declares extraordinary state of emergency for 2027 | Parliament | N | - | - | common noun, no holder |
| 136 | JPMorgan Chase says cyberattack on third-party vendors may have exposed cus... | JPMorgan Chase | E | R | - |  |
| 137 | Judge lets Trump's $15bn libel suit against the New York Times proceed | NYT | E | R | D | NYT / York Times |
| 138 | ICJ finds Myanmar committed genocide against Rohingya | Myanmar | E | R | D | Myanmar / Republic of the Union of Myanmar |
| 139 | OpenAI builds a "Performance Factory" to keep ChatGPT fast as agentic codin... | OpenAI | E | R | - |  |
| 140 | Iran war cancels Middle East sporting events as two season-ending F1 races ... | F1 | E | R | D | F1 / Formula |
| 141 | Google activates earthquake alerts on Android in Italy | Italy | E | R | - |  |
| 142 | Pedestrian struck at Nellis Blvd and Stewart Ave in Las Vegas | VegasAlerts | E | R | - |  |
| 143 | Kushner says Trump open to Article 5-style security guarantee for Ukraine | Ukraine | E | R | - |  |
| 144 | Slotkin and Kelly mark one year since 'illegal orders' video drew Trump's s... | Elissa Slotkin | E | R | - |  |
| 145 | RKI estimates 6 million acute respiratory illnesses in Germany in mid-Novem... | Germany | E | R | - |  |
| 146 | Mosque set on fire in suspected arson in northern Israeli village | Israel | E | R | - |  |
| 147 | Bradford councillor Ibrar Ali stabbed while door-knocking; 32-year-old man ... | Ibrar Ali | E | R | - |  |
| 148 | First Native American justice on Washington state Supreme Court steps down | American | N | - | - | demonym |
| 149 | France's transport regulator approves CDG-Orly expansion deal, with five re... | France | E | R | - |  |
| 150 | Rakuten Securities says data breach may have exposed up to 15.72 million ac... | Rakuten Securities | E | R | - |  |
| 151 | Russia begins buying gasoline from Egypt for the first time as domestic fue... | Russian | N | - | - | demonym |
| 152 | Democrats push to defund the Iran war before Dec. 11 funding deadline, rais... | Democratic | N | - | - | adjective |
| 153 | Army postpones Friday detonation of nearly 10,000 pounds of lead azide at I... | Middletown | E | R | - |  |
| 154 | Houthi Strike Damages Saudi Sovereign Wealth Fund Headquarters in Riyadh | Public Investment Fund | E | R | D | Public Investment Fund / PIF |
| 155 | Six killed in suspected gas explosion at Georgia apartment building | Georgia | E | R | - |  |
| 156 | Trump calls nuclear disarmament with China and Russia "very difficult" afte... | Russia | E | R | - |  |
| 157 | House committee releases data showing Trump administration approved nearly ... | China | E | R | - |  |
| 158 | Democrats sweep both chambers of Congress; Trump prepares to wield the veto | Congress | E | R | - |  |
| 159 | Four men convicted in England after husband drugged wife and arranged her a... | UK | E | R | D | UK |
| 160 | EU programme guide bars Russia residents from Erasmus+ and Erasmus Mundus | Russia | E | R | - |  |
| 161 | Leaders of over 30 states to gather in Berlin for 'Coalition of the Willing... | Ukraine | E | R | - |  |
| 162 | EU agrees to delay methane import rules for fossil fuel suppliers by a year | EU | E | R | - |  |
| 163 | Denmark ratifies US–Greenland Arctic pact, expanding American military role | Arctic | E | R | - |  |
| 164 | Violent thefts jump 75% in Lyon as Rhône ranks second in France for insecur... | France | E | R | - |  |
| 165 | Onlook relaunches as open-source "agentic design" tool, lets designers work... | Onlook | E | R | - |  |
| 166 | Russia begins buying gasoline from Egypt for the first time as domestic fue... | Egypt | E | R | - |  |
| 167 | Grenoble prosecutor contests racist aggravating circumstance in Crépol kill... | Opens | N | - | - | verb |
| 168 | Judge lets Trump's $15bn libel suit against the New York Times proceed | Donald Trump | E | R | D | Donald Trump |
| 169 | Cyberattack takes European Commission personnel systems offline for days, a... | European Commission | E | R | - |  |
| 170 | PostgreSQL 19 is released; Neon and Databricks Lakebase offer it day one | Databricks | E | R | - |  |
| 171 | Houthi Strike Damages Saudi Sovereign Wealth Fund Headquarters in Riyadh | PIF | E | R | D | PIF / Public Investment Fund |
| 172 | UK sanctions eight Israeli firms building West Bank settlements; Sa'ar vows... | UK | E | R | D | UK |
| 173 | Zverev faces fresh lawsuit from ex-partner over comments about abuse trial | Alexander Zverev | E | R | - |  |
| 174 | Bukele due to register for unprecedented third El Salvador presidential term | El Salvador | E | R | - |  |
| 175 | Damian Lillard returns to Portland Trail Blazers on three-year, $42 million... | Damian Lillard | E | R | - |  |
| 176 | Damian Lillard returns to Portland Trail Blazers on three-year, $42 million... | Portland Trail Blazers | E | R | D | Portland Trail Blazers / Blazers |
| 177 | South Korea's Supreme Court overturns former PM Han's 15-year term in marti... | South Korea | E | R | - |  |
| 178 | Regavim denounces Supreme Court intervention in Judean Desert nature reserv... | Israel | E | R | - |  |
| 179 | Gig-work staffing apps are filling shifts in US nursing homes with little o... | U.S. | E | R | D | U.S. |
| 180 | Five aid groups grade Trump's Gaza peace plan 4 out of 26 and call it 'disp... | Trump | E | R | D | Trump |
| 181 | Brazil takes step to recognize ancestral territory of Kayapó chief Raoni | Raoni | E | R | - |  |
| 182 | Democrats push to defund the Iran war before Dec. 11 funding deadline, rais... | Iran war | N | - | - | an event |
| 183 | Game4Ricky: star-studded charity match honours boxing legend Ricky Hatton, 3-2 | Match | N | - | - | common noun |
| 184 | Senate rejects war powers resolution to end Iran hostilities | Senate Republicans | E | R | - |  |
| 185 | EU governments approve trade deal with Indonesia to offset Trump tariffs | EU | E | R | - |  |
| 186 | FCC lets state prisons jam inmate cellphones for first time ever | FCC | E | R | - |  |
| 187 | Disney's 'Holiday Spectacular' Returns for 11th Year With Star Lineup, Airs... | Holiday Special Returns | N | - | - | headline fragment |
| 188 | Cyberattack disrupted European Commission systems for over a week, Politico... | EU | E | R | - |  |
| 189 | ECHR bars Romanian lawyer for two years over fabricated AI case-law citations | Lawyer | N | - | - | role word |
| 190 | Kushner says Trump open to Article 5-style security guarantee for Ukraine | NATO | E | R | - |  |
| 191 | Hegseth signs US-Venezuela military cooperation deal in Caracas | Venezuelan | N | - | - | demonym |
| 192 | FDA stands up CBER Safety Council to oversee vaccines, gene therapies after... | U.S. | E | R | D | U.S. |
| 193 | Grenoble prosecutor contests racist aggravating circumstance in Crépol kill... | Prosecution | N | - | - | role word |
| 194 | JPMorgan Chase says cyberattack on third-party vendors may have exposed cus... | Customer | N | - | - | common noun |
| 195 | Fos Provence Basket loses 113–105 at Rouen in Élite 2 | Game | N | - | - | common noun |
| 196 | Iran demands compensation from Persian Gulf states over US-Israeli attacks | U.S. | E | R | D | U.S. |
| 197 | Zelensky says Ukraine would welcome US military bases as part of security g... | Volodymyr Zelensky | E | R | D | Volodymyr Zelensky / Zelenskyy |
| 198 | Kremlin's Kiriyenko Steps Into the Spotlight, Pushing Troops' Return as His... | Kiriyenko | E | R | D | Kiriyenko / Sergei Kiriyenko |
| 199 | Latvia's new cabinet pledges a 'de-Russification declaration' to curb Russi... | Russian | N | - | - | demonym |
| 200 | House Democrats nominate Hakeem Jeffries for speaker, 251-11 | House | N | - | - | common word, no holder |

## 4. Error patterns, tied to code

1. **A junk entity, once minted, is permanent and re-attached by lookup.**
   The default records a lexicon hit by its role answer alone; an existing
   entity id skips the type check (`internal/agentcore/story_entities_default.go:57-58`,
   the `c.EntityID != ""` branch; the type question applies only to a new
   phrase, lines 59-66). Candidates for existing entities come from
   gazetteer matching of the story text (`internal/platform/publication_entity_candidates.go:302`,
   `addHit` at 307). And nothing can stop one: "an entity's name or alias
   is never stopped" (`internal/platform/publication_entity_learning.go:12-13`),
   so a stopword (p at least 0.8 in three stories, same file lines 9-11)
   cannot retire a name that is already an entity. Explains: the 16 demonyms,
   most common nouns (Health 2, Parliament 2, National 2), which recur across
   stories (**C**; which of the 58 were minted by a new-phrase answer and which
   were lexicon hits cannot be read from the public rows, **U**).
2. **Identity is the normalised name, so one id spans different things.**
   `MintedEntityID` hashes the normalised name (`internal/types/story_entity.go:109-116`);
   a second node of the same normalised name is refused (`internal/platform/publication_story_entities.go:274-285`;
   within one request, line 271). Seen in the window, outside the sampled rows
   (**M**, from the same 300 stories): **Parliament** is one id on Italy's,
   Saarland's, Mongolia's and Indonesia's parliaments; **Supreme Court**
   on Israel's, Washington state's and South Korea's; **Washington** on the
   state and on the capital; **Paris** on the city and on the story about
   Paris Jackson at the Bataclan. Sample rows 31 and 135 are the first two
   (marked N, so not in (b)); a reader who counted a bare "Parliament" as an
   entity would score them W.
3. **Two ids for one thing, forever.** The same hash rule gives "U.S." and
   "United States" different ids, and the alias path refuses to join them:
   `learnAlias` returns without learning when the alias already matches
   another entity's name or alias (`internal/platform/publication_entity_learning.go:172-174`).
   Nothing merges ids. Explains: 8 of the 34 duplicate rows are "U.S." alone,
   Trump / Donald Trump (5 rows), UK / United Kingdom, UAE, BMO, PIF, Blazers,
   IDFA, NYT / York Times, three spellings of Jurf al Sakhr (**C** and **M**).
   Same id-by-spelling reason: "Mississippi Elects Scott Colom" and "Democrat
   Scott Colom" exist beside "Scott Colom".
4. **A phrase of the headline becomes an entity when the kind question answers
   with a real type.** The default attaches a new phrase when the kind answer
   ranks one of the six types first (`story_entities_default.go:60-66`,
   `types.EntityTypes`, `internal/types/story_entity.go:48`); "Holiday Special
   Returns", "Mississippi Elects Scott Colom", "Chinese AI", "AI API" are
   headline or sentence fragments that passed it. Whether the kind question
   offered "not an entity" for them, and what p it gave, is in the tape and
   not in the public read (**U**; the design's section 14 lists the same gap).
5. **Role is not a test of being an entity.** The role answer (central or
   incidental) decides attachment; a demonym or common noun that the story
   uses centrally ("Chinese factories", "Russian language") gets role central
   (**J**, consistent with pattern 1's branch at `story_entities_default.go:53-55`).
6. **No Wikidata linking exists in this path.** Zero of 444 ids start with
   `wd:` (**M**); `WikidataEntityID` (`internal/types/story_entity.go:127`) is
   called only where a Wikidata id arrives, and nothing in the story path
   supplies one (design section 7, not re-read).

## 5. What the public reads showed besides the sample (the script, 300 stories, 05:36Z)

231 of 300 stories (77.0%) carry at least one central entity; 69 carry none.
Attachments per story over those 231: median 3, p90 5, max 8 (the read serves at
most 12, `publication_story_entities.go:42`). 347 of 444 distinct entities
(78.2%) sit on one story only. The growth read gives 6,690 entities, 9,120
lexicon keys and 34 stopwords. The script's duplicate rule finds 70 candidate
pairs (containment or initials); its top pairs include true duplicates
(U.S. / United States, Trump / Donald Trump, UK / United Kingdom) and many
false ones, since containment pairs "U.S." with "U.S. House", "Iran" with
"Iran war", "American" with "American Bar Association". The hand read does not
score the pair rule's precision; that needs a pair sample (the design's C-E8).

## 6. What I could not judge

- **Kind.** Not served publicly; I judged "is it a person, organisation, place,
  product, work or law" from the label and the story, not from the row's type.
  An entity typed wrongly would pass (a).
- **Centrality and role.** Only central attachments are served; I did not judge
  whether each is central. Several are thin ("Arctic" modifying a pact, "U.S."
  in a story that says the claim is not about US output, "Gecko", "VegasAlerts").
- **Duplicates beyond the window.** (c) searched only the 444 ids of these 300
  stories and the other sample rows; the lexicon has 6,690. The 34 is a floor.
- **The reader.** One reader, no blind second. A reader who counts
  "Democrats", "Senate" or "Kremlin" as non-entities (institution words
  without a holder) would raise (a); one who counts "Chinese" as a mention of
  China would lower it. Borderline rows I called E: Democrats (party), Senate
  (the US Senate in all three stories), Congress, Gecko, VegasAlerts, Arctic.
  Borderline N: "Gulf", "Parliament", "Diwali", "Iran war".
- **Whether a row is a lexicon hit or a new mint**, and p of each answer: tape
  only.
- **Same-name confusion at the row level.** Only two W rows; the id-level
  confusion in section 4 pattern 2 is real but is observed on other rows.
- The 300 stories are the newest of the paper only; older stories (since
  2026-10-26) were not read.
