# Do the paper's existing subjects find its duplicate stories? A read of 50 pairs (2026-11-22)

Station F1 of [the first design](../definitions/choir-first-design-2026-11-22.md), read-only; ledger C190.

## What was done

- Source: the paper's public API (`/api/platform/publications/front` and
  `/resolve`), the route `scripts/status-entities` uses. Stories updated in
  the 14 days to 2026-11-22T05:00Z: 6,406. Only each story's central entities
  are served (at most 12), as id and label.
- Score for a pair: the sum over shared subjects of ln(N/df), divided by the
  smaller story's total weight.
- The 50 pairs read are the highest by that score, ties broken by raw shared
  weight. A Sonnet agent that did not compute the score, and was told to
  ignore the score and shared-label columns, read each pair's two texts and
  gave one verdict: SAME, PART (one is a part or direct development of the
  other), THEME (same actors or topic, different happenings), UNRELATED.

## Result

SAME 20, PART 12, THEME 18, UNRELATED 0. SAME or PART: 32 of 50.

## What the numbers under it say

- 4,162 distinct subjects; 2,907 (69.8%) are in exactly one story; 1,791
  stories (28%) carry no subject and cannot be scored.
- One thing under several rows: "Trump" and "Donald Trump"; "U.S." and
  "United States"; "Russia" and "Russian"; "EU" and "Union". Hubs that are
  not subjects: "Report", "Data", "Plan", "Customer".
- The score saturates: 35,177 pairs score 1.0 of 204,047 that share a
  subject. The 50 read are the top of a tie by raw weight (48 share three or
  four subjects), so 32 of 50 describes pairs sharing several rare subjects,
  not pairs at score 1.0 in general.
- The 18 THEME pairs are the instructive failures: different CVEs from one
  vendor's batch, two bills in one legislature, two publications by one
  government. They share every named subject and differ in the happening,
  which no proper-name row carries.

## Limits

- No vector control: the public API serves no story vectors, so this does
  not show what subjects add over vector nearness.
- The reader read each story's opening paragraphs, not always the whole text.
- One reader, one pass.

## Reading (the lead's)

Shared rare subjects find duplicates and developments well above chance, and
never joined unrelated stories in this sample. They cannot tell two
happenings among the same actors apart. That gap is the subjects that are not
proper names (the design's open question 6), and naming one thing once is the
first work, before the tables.

## The pairs

| # | verdict | shared subjects | headlines |
|---|---|---|---|
| 1 | SAME | Ethiopia; Eritrea; Ethiopian; Eritrean; Horn of Africa | Ethiopia and Eritrea trade artillery near Badme as G7 warns of regiona / Fresh clashes at the Eritrean border raise fears of a new regional war |
| 2 | SAME | Somalia; UAE; United Arab Emirates; Emirati; Somali | Somalia orders UAE ambassador to leave Mogadishu within 72 hours and r / Somalia orders UAE ambassador out within 72 hours, calling it an 'open |
| 3 | THEME | U.S.; India; Indian; International; USCIRF; U.S. Commission | U.S. religious freedom panel accuses India of targeting Muslims in vot / India’s ambassador to the US calls USCIRF “no locus standi” over Sharj |
| 4 | PART | Senate; Mississippi; U.S. Senate; Democrat Scott Colom; Mississippi Senate | Mississippi Senate race: Democrat Scott Colom wins as Associated Press / Colom leads Mississippi Senate race by 2,574 votes, but it remains unc |
| 5 | SAME | Senate; Mississippi; U.S. Senate; Democrat Scott Colom; Mississippi Senate | Scott Colom wins Mississippi Senate race, becomes first Black senator  / Mississippi Senate race: Democrat Scott Colom wins as Associated Press |
| 6 | THEME | Wales; Welsh Government; Cymru; Welsh | Plaid Cymru unveils first Welsh budget, promising £500m NHS boost / Welsh Government unveils new National Strategy for Unpaid Carers |
| 7 | THEME | Saudi Arabia; Saudi; Houthi; Houthis; Yemen | Saudi strikes on Sanaa province kill at least 44, Houthis say; toll un / Houthis claim strikes on Saudi East-West pipeline pumping station, Yan |
| 8 | THEME | House; Democrats; Democratic; Hakeem Jeffries; House Democrats | House Democrats nominate Hakeem Jeffries for speaker, 251-11 / Peltola flips Alaska Senate seat as Democrats head for sweeping midter |
| 9 | THEME | RedPacketSecurity; Linux; Red Hat; Red Hat Enterprise Linux | CVE-2026-119079 is a Kerberos denial-of-service flaw Red Hat rates Imp / Red Hat rates CVE-2026-112323 Important: a Kerberos kadmind denial-of- |
| 10 | THEME | RedPacketSecurity; Linux; Red Hat; Red Hat Enterprise Linux | CVE-2026-112320 is a Kerberos denial-of-service flaw, rated Important  / Red Hat rates CVE-2026-112323 Important: a Kerberos kadmind denial-of- |
| 11 | THEME | RedPacketSecurity; Linux; Red Hat; Red Hat Enterprise Linux | CVE-2026-112320 is a Kerberos denial-of-service flaw, rated Important  / CVE-2026-119079 is a Kerberos denial-of-service flaw Red Hat rates Imp |
| 12 | PART | U.S.; India; Indian; Gurpatwant Singh Pannun; Nikhil Gupta | US court gives Indian national 235 months for plot to kill Sikh separa / US seeks 24-year prison term for Nikhil Gupta ahead of November 20 sen |
| 13 | SAME | OpenAI; ChatGPT; ChatGPT Wallet; Wallet | OpenAI launches ChatGPT Wallet, letting its dots agents make purchases / OpenAI launches ChatGPT Wallet, letting its agents shop and pay with u |
| 14 | PART | Nayib Bukele; Haiti; El Salvador; Salvadoran | Bukele's 2,500 troops have turned a Haiti rescue mission into a gang w / Thousands of Haitians march in support of Salvadoran troops fighting g |
| 15 | PART | Senate; U.S. Senate; Dan Sullivan; Mary Peltola | Peltola defeats Sullivan in Alaska Senate race, giving Democrats a 56- / Peltola leads but still short of majority as Alaska Senate count nears |
| 16 | SAME | Support; Nayib Bukele; Haiti; Salvadoran | Thousands of Haitians march in support of Salvadoran troops fighting g / Haitians cheer Bukele and Salvadoran troops in Port-au-Prince as analy |
| 17 | SAME | UN; Myanmar; International; Genocide Convention | ICJ to deliver verdict on Thursday in historic genocide case against M / ICJ to rule Thursday on whether Myanmar committed genocide against Roh |
| 18 | PART | Nvidia; Jensen Huang; Huang; Nvidia CEO | Trump says he wants Tim Cook to lead Nvidia after Jensen Huang's resig / Jensen Huang Resigns as Nvidia CEO Amid Spy Scandal; Agrees to Testify |
| 19 | SAME | House; Republicans; Jordan; House Republican | Jim Jordan elected House Republican leader, defeating Steve Scalise 10 / Republicans elect Jim Jordan as new House GOP minority leader |
| 20 | THEME | Bill; Ohio Senate; Senate; Ohio | Ohio Senate president says child-marriage ban bill likely to fail / Ohio Senate passes bill banning DEI in public schools |
| 21 | SAME | Qatar; Defence; Türkiye; Turkiye | Qatar and Turkiye sign mutual defence pact in Istanbul / Türkiye and Qatar sign mutual-defence agreement; terms remain undisclo |
| 22 | SAME | Qatar; Defence; Türkiye; Turkiye | Turkiye and Qatar sign mutual defence agreement in Istanbul / Türkiye and Qatar sign mutual-defence agreement; terms remain undisclo |
| 23 | SAME | Qatar; Defence; Türkiye; Turkiye | Turkiye and Qatar sign mutual defence agreement in Istanbul / Qatar and Turkiye sign mutual defence pact in Istanbul |
| 24 | SAME | France; Data; La Poste; Maileva | La Poste subsidiary Maileva hit by cyberattack exposing data of 22 mil / La Poste breach at Maileva exposed data of 22 million people, one of F |
| 25 | SAME | Trump; Donald Trump; China; Vladimir Putin; Xi Jinping | Trump, Xi and Putin to hold first trilateral summit in Shenzhen on Thu / China confirms first Xi-Trump-Putin summit on Nov 19 in Shenzhen, on A |
| 26 | PART | Palestinian; Spain; Chambers; Spanish | Spanish court frees pro-Palestinian activist James Fergie Chambers on  / Pro-Palestinian donor Chambers, heir to the Cox family, tells Spanish  |
| 27 | SAME | Barcelona; Fermin Lopez; La Liga | Barcelona recover against Atlético Madrid to defend La Liga lead / Lopez double sends Barcelona three points clear with comeback win at A |
| 28 | SAME | Suno; Eminem; Elvis | Elvis Presley Enterprises and Eminem's publisher sue Suno over AI voic / Presley and Eminem rights holders sue Suno over AI voice and song repl |
| 29 | SAME | Suno; Eminem; Elvis | Elvis and Eminem copyright owners sue Suno over AI training / Presley and Eminem rights holders sue Suno over AI voice and song repl |
| 30 | SAME | Suno; Eminem; Elvis | Elvis and Eminem copyright owners sue Suno over AI training / Elvis Presley Enterprises and Eminem's publisher sue Suno over AI voic |
| 31 | SAME | Moldova; Transnistria; Vadim Krasnoselsky | Moldova bars Transnistria leader Krasnoselsky from leaving the country / Moldova bans Transnistria's Kremlin-backed leader from leaving for fiv |
| 32 | THEME | Democrats; Republicans; GOP; Republican | Republican redistricting push 'failed epically' as Democrats sweep mid / Republicans’ midterm losses put the cost of living and Iran war at the |
| 33 | PART | Egypt; Lina Attalah; Mada Masr | Egyptian court resumes retrial of Mada Masr editor Lina Attalah, riski / Egypt postpones Mada Masr editor's retrial as UN warns of widening att |
| 34 | PART | Democrat Scott Colom; Cindy Hyde Smith; Mississippi Senate | Scott Colom ousts Cindy Hyde-Smith in historic Mississippi Senate flip / Colom leads Mississippi Senate race by 2,574 votes, but it remains unc |
| 35 | PART | FAA; Sean Duffy; Federal Aviation Administration | FAA pauses installation of new Juniper routers after nationwide ground / Transportation Secretary Duffy renews push for $50 billion FAA moderni |
| 36 | SAME | UK; Meta; Data; WhatsApp | Ofcom fines Meta £450,000 over inaccurate WhatsApp Business data / Ofcom fines Meta £450,000 over repeated inaccuracies in WhatsApp Busin |
| 37 | PART | Nvidia; Jensen Huang; China; Nvidia CEO Jensen Huang | Nvidia's Jensen Huang is at the centre of a reported US-China espionag / What China could get from Jensen Huang |
| 38 | PART | Nvidia; Jensen Huang; China; Nvidia CEO Jensen Huang | Former Nvidia CEO Jensen Huang agrees to testify before Congress over  / What China could get from Jensen Huang |
| 39 | THEME | Nvidia; Jensen Huang; China; Nvidia CEO Jensen Huang | Senate Banking Committee asks Jensen Huang and Commerce Secretary Lutn / What China could get from Jensen Huang |
| 40 | SAME | United; Manchester United; Mauricio Pochettino | Pochettino bids tearful farewell to USMNT after Manchester United appo / Pochettino's move from USA to Manchester United is confirmed |
| 41 | THEME | ICE; U.S. Immigration and Customs Enforcement; Customs Enforcement | ICE detains Princeton neuroscience PhD after SfN conference; judge bar / ICE set record arrest and deportation totals in FY2026, DHS says |
| 42 | SAME | Fuel; Chile; José Antonio Kast | Chile strike deal holds as truckers clear most routes under police ult / Kast reaches deal with Chilean truckers, freezing fuel prices into 202 |
| 43 | THEME | Philippines; Manila; ASEAN | Trump lands in Manila for the end of the ASEAN summit, Quad talks and  / Hundreds arrested in Manila protests ahead of Trump visit as ASEAN sum |
| 44 | THEME | Linux; Red Hat; Red Hat Enterprise Linux | GStreamer flaw CVE-2026-119610 has a preliminary score of 8.2; Red Hat / Red Hat rates CVE-2026-112323 Important: a Kerberos kadmind denial-of- |
| 45 | THEME | Linux; Red Hat; Red Hat Enterprise Linux | CVE-2026-119079 is a Kerberos denial-of-service flaw Red Hat rates Imp / GStreamer flaw CVE-2026-119610 has a preliminary score of 8.2; Red Hat |
| 46 | THEME | Linux; Red Hat; Red Hat Enterprise Linux | CVE-2026-112320 is a Kerberos denial-of-service flaw, rated Important  / GStreamer flaw CVE-2026-119610 has a preliminary score of 8.2; Red Hat |
| 47 | THEME | Qatar; LNG; QatarEnergy | QatarEnergy's LNG expansion may leave it bigger but not less exposed,  / Orlen ends one of two long-term LNG contracts with QatarEnergy over fo |
| 48 | THEME | Grand Theft Auto; GTA VI; Rockstar Games | Rockstar says GTA VI made $3.79 billion in its first day of sales / Rockstar brings GTA VI to Xbox Cloud Gaming at launch |
| 49 | THEME | EU; Union; Ukraine; EU Council | EU Approves Another €1 Billion for Ukrainian Military Equipment / EU ministers approve largest-ever Ukraine Facility payout of nearly 8  |
| 50 | PART | Congress; Government Accountability Office; GAO | Tillis holds up Trump's nominee to lead the GAO over independence conc / Trump set to nominate Eric Ueland for GAO chief as Democrats cry foul  |
