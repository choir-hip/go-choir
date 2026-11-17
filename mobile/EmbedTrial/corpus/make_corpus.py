#!/usr/bin/env python3
"""Builds the synthetic news corpus and the 30 questions for the embedding trial (stories.json, questions.json).
Everything here is invented (names, places, numbers); the Alice questions point at Project Gutenberg #11 by phrase.
Run: python3 -I make_corpus.py   (deterministic: seed 17)."""
import json, random, datetime
R = random.Random(17)
D0 = datetime.date(2026, 11, 2)

# ---- the 24 target stories, each with the question that must find it -------------------------------------------
T = [
 dict(k="bridge-harlow", kind="para", h="Harlow Street Viaduct to shut for a rebuild",
  b="The span carrying Route 9 over the Calder River will be closed to all vehicles from December 1 while crews replace its deck and bearings. Engineers expect the work to take eleven months, and the state has not yet named a detour for trucks. Buses will cross by way of the Mill Road overpass.",
  q="Is the elevated roadway over the waterway going to be off limits to motorists for most of a year?"),
 dict(k="bridge-merrow", kind="exact", h="Merrow Pier footbridge reopens after cable repairs",
  b="The Merrow Pier footbridge reopened on Friday afternoon, nine days after inspectors found corroded suspension cables on its east side. Harbor officials said the replacement cables were rated for twice the load and that the lights along the deck had also been rewired.",
  q="What happened at the Merrow Pier footbridge?"),
 dict(k="senate-tidewater", kind="exact", h="Senate passes Tidewater Appropriations Act 61-38",
  b="The Senate approved the Tidewater Appropriations Act by 61 votes to 38 late Tuesday, sending the $14 billion measure for harbor dredging, levees and flood pumps to the House. Eight members of the minority party voted yes after an amendment protected inland counties from a new fee.",
  q="How did the Senate vote on the Tidewater Appropriations Act?"),
 dict(k="senate-solar", kind="para", h="Rooftop solar credit falls four votes short",
  b="The upper chamber rejected a proposal to extend tax credits for rooftop solar panels on Thursday, falling four votes short of the sixty needed to end debate. Backers said they would try again after the holiday recess, while opponents called the credits a giveaway to installers.",
  q="Did lawmakers in the upper house keep the incentives for home photovoltaic installations?"),
 dict(k="chip-aldersgate", kind="exact", h="Aldersgate Silicon pauses work on its Ohio fab",
  b="Aldersgate Silicon told investors it has halted construction of its planned factory near Dayton, Ohio, citing delays in delivery of lithography tools and a weaker outlook for memory prices. About 1,200 construction jobs are affected. The company kept its capital budget for Arizona unchanged.",
  q="Why did Aldersgate Silicon stop building its Ohio factory?"),
 dict(k="chip-veridian", kind="para", h="Veridian Micro cuts 900 jobs as demand cools",
  b="Veridian Micro, which designs accelerator processors for data centers, will lay off 900 workers, nearly one in eight of its employees, after orders from its largest cloud customers slowed. The company said it still expects to ship its next-generation design in the spring.",
  q="Which company that makes computing hardware for server farms is dismissing roughly an eighth of its staff?"),
 dict(k="weather-glaze", kind="para", h="Freezing drizzle glazes roads across the Upper Valley",
  b="A film of ice formed on roads, bridges and power lines across the Upper Valley overnight. Nineteen school districts closed or opened late, and two sections of the interstate were shut for several hours after a chain of collisions. Temperatures were forecast to rise above freezing by midday.",
  q="Why are children staying home in the northern river valley this morning?"),
 dict(k="health-mercy", kind="exact", h="Mercy General opens a 40-bed psychiatric wing",
  b="Mercy General Hospital opened a new 40-bed psychiatric wing on Monday, the first expansion of mental health beds in the county in two decades. The wing includes a youth unit and a crisis stabilization room, and it will be staffed by 62 new nurses and counselors.",
  q="When did Mercy General Hospital open the new wing?"),
 dict(k="health-measles", kind="para", h="Pell County outbreak reaches 27 cases",
  b="Public health officials in Pell County said 27 people have now been infected in an outbreak that began when an unvaccinated child attended a church summer camp. Seventeen of the patients are under ten. Free vaccination clinics will run at three fire stations this weekend.",
  q="How many people in the rural district have caught the contagious rash illness?"),
 dict(k="housing-cottage", kind="para", h="Council lets homeowners add backyard cottages",
  b="The city council voted 7 to 2 to allow homeowners in every residential zone to build a detached unit of up to 800 square feet behind their houses. The measure waives parking requirements near transit lines. Opponents worried about shade, noise and the loss of tree cover.",
  q="Can residents put a small second dwelling behind their houses now?"),
 dict(k="energy-gannet", kind="exact", h="Gannet Point wind farm starts delivering power",
  b="The Gannet Point wind farm began feeding 220 megawatts into the regional grid on Wednesday, enough for roughly 150,000 homes. The 62 turbines stand on a headland and offshore shoal, and the developer said a second phase of 90 megawatts would follow next summer.",
  q="How much power does the Gannet Point wind farm supply?"),
 dict(k="energy-bills", kind="para", h="Regulators approve a 6 percent electricity rate increase",
  b="State utility regulators approved a 6 percent increase in residential electricity rates that takes effect in January, adding about $9 to the typical monthly bill. The utility argued the money is needed to bury lines and replace aging substations. Consumer advocates had asked for 3 percent.",
  q="Will my monthly power charges go up after the new year?"),
 dict(k="court-lakemont", kind="exact", h="Lakemont water suit thrown out",
  b="Judge Ibarra-Whitfield dismissed the Lakemont water contamination lawsuit on Monday, ruling that residents had not shown the tannery's discharge was the cause of elevated lead in their taps. Lawyers for the plaintiffs said they would appeal.",
  q="What did Judge Ibarra-Whitfield rule?"),
 dict(k="court-pruitt", kind="para", h="Jury convicts former county treasurer",
  b="A jury found Dale Pruitt, the county's former treasurer, guilty of taking $412,000 from the fund set aside for road repairs and covering it with falsified ledgers. He faces up to fifteen years in prison. Sentencing is set for January.",
  q="Was the ex-official who took money meant for highways found guilty?"),
 dict(k="sport-gulls", kind="exact", h="Gulls edge Rangers in overtime",
  b="The Harbor City Gulls beat the Redfield Rangers 3-2 in overtime on Saturday night, with a wrist shot from the left circle by Tomas Kerr at 2:41 of the extra period. Goalie Nils Aaberg made 34 saves as Harbor City moved into second place in the division.",
  q="Who won when Harbor City Gulls played Redfield Rangers?"),
 dict(k="sport-marathon", kind="para", h="New world best in the 26.2 miles",
  b="A 24-year-old runner from Eldoret cut 41 seconds from the world best for the distance, finishing the course along the Tagus river in 2 hours, 0 minutes and 18 seconds. Pacemakers dropped out at the 30-kilometre mark and she ran the last stretch alone.",
  q="Did anyone set a new fastest time for the long road race in Portugal's capital?"),
 dict(k="science-exoplanet", kind="para", h="Rocky world found around a dim neighbor star",
  b="Astronomers reported an Earth-sized rocky planet circling a faint red dwarf only 14 light-years away. It sits in the zone where liquid water could exist on the surface, though the star's flares may have stripped its air. The team will look for an atmosphere with the next space telescope.",
  q="Has anyone found a possibly habitable world close to our solar system?"),
 dict(k="science-kestrel", kind="exact", h="Kestrel-4 touches down near lunar south pole",
  b="The Kestrel-4 lander touched down on a ridge near the Moon's south pole on Sunday, about 600 metres from its target. The craft is tilted by eight degrees but its solar array and radio are working. It carries a drill that will sample ice-bearing soil during the two-week lunar day.",
  q="Where did Kestrel-4 land?"),
 dict(k="edu-strike", kind="para", h="Teachers authorize a walkout over crowded rooms",
  b="Instructors in the Brannock school district voted 91 percent in favor of a strike if talks fail by December 5. They want caps on class sizes, which reach 38 in some middle school rooms, and a 7 percent raise. The district offered 3 percent and a hiring freeze review.",
  q="Are educators in the eastern-county district threatening to stop teaching over packed classrooms?"),
 dict(k="transit-rail", kind="para", h="Rail engineers reject contract offer",
  b="Locomotive engineers on the Commuter Line voted down the carrier's final offer, setting up a possible stoppage on December 12 unless a federal mediator brokers a deal. About 40,000 daily riders would be affected. The railroad said it would run a skeleton schedule of express buses.",
  q="Might my train to work stop running in mid-December?"),
 dict(k="fire-sawtooth", kind="exact", h="Sawtooth Ridge fire 80 percent contained",
  b="Crews have contained 80 percent of the Sawtooth Ridge fire, which has burned 14,000 acres and destroyed 31 homes since it started in dry grass on Tuesday. Evacuation orders were lifted for two towns. Investigators are examining a failed power line.",
  q="How many houses did the Sawtooth Ridge fire destroy?"),
 dict(k="biz-fennimore", kind="para", h="Fennimore Foods to be taken private for $2.1 billion",
  b="Regional grocer Fennimore Foods agreed to be acquired by the investment firm Calloway Partners for $2.1 billion in cash, a 28 percent premium to Friday's close. The chain's 214 stores will keep their names. Employees were promised that wages would not be cut for two years.",
  q="Which supermarket chain is being bought by an investment company?"),
 dict(k="culture-vandermeer", kind="exact", h="Vandermeer Quartet brings Shostakovich to Alder Hall",
  b="The Vandermeer Quartet will play three Shostakovich string quartets at Alder Hall on November 28 at 7:30 p.m. Tickets start at $24. The program closes with the Eighth Quartet, written in three days in 1960 beside a ruined city.",
  q="When does the Vandermeer Quartet play at Alder Hall?"),
 dict(k="tech-parley", kind="para", h="Parley fined 38 million euros over contact lists",
  b="European regulators fined the messaging app Parley 38 million euros for copying users' address books to its servers without asking permission. The company must delete the data within 60 days. Parley said the practice ended last year and that it would contest the amount.",
  q="Which chat application was penalized for copying users' contacts?"),
]

# ---- generated distractors: overlapping subjects, with names, numbers and places unlike the targets ----------------
places = ["Ashford", "Brookline Heights", "Cedar Falls", "Dunmore", "Eastbrook", "Fairhaven", "Glenrock", "Halvorsen", "Ironbridge", "Juniper Bay",
          "Kingsport", "Larkspur", "Millbank", "Northgate", "Oakhurst", "Pinecrest", "Quarry Hill", "Rosewood", "Stillwater", "Thornbury",
          "Underhill", "Valemont", "Westmarch", "Yarrow", "Zephyr Cove", "Amberley", "Bellmore", "Corbin", "Delmar", "Elkton"]
surn = ["Alvarado", "Bishop", "Castellano", "Doyle", "Eriksen", "Faulkner", "Garrison", "Hossain", "Iverson", "Jaffe", "Kowalski", "Lindqvist", "Marchetti",
        "Nakamura", "Oduya", "Pellegrino", "Quist", "Rasmussen", "Sandoval", "Tran", "Underwood", "Voss", "Whitlock", "Yilmaz", "Zielinski"]
bills = ["Rural Broadband Act", "Farm Resilience Act", "Rail Safety Modernization Act", "Child Tax Credit Extension", "Border Technology Act",
         "Defense Readiness Act", "Prescription Cost Act", "Clean Water Infrastructure Act", "Veterans Housing Act", "Wildfire Prevention Act",
         "Small Business Relief Act", "Student Loan Fairness Act", "Port Security Act", "Highway Trust Fund Extension", "Telehealth Access Act"]
chips = ["Northfield Semiconductor", "Corvane Devices", "Pellucid Logic", "Bramwell Chips", "Hexagon Wafer Works", "Meridian Photonics", "Quanta Ridge"]
teams = ["Ironclads", "Lakers of Dunmore", "Voyagers", "Foxes", "Stags", "Pioneers", "Comets", "Wolverines", "Kestrels", "Barons"]
subs = ["road improvements", "school lunches", "library hours", "stormwater drains", "bus shelters", "a new fire station", "park lighting", "bike lanes"]
def pick(a): return R.choice(a)
gen = []
def add(h, b, d=None): gen.append(dict(h=h, b=b))

for i in range(int(22*1.45)):  # senate and house votes
    bill, n, m = pick(bills), R.randint(51, 78), None
    m = 100 - n - R.randint(0, 3)
    chamber = pick(["Senate", "House", "Senate", "the upper chamber"])
    t = pick([
     (f"{chamber if chamber[0].isupper() else 'Senate'} approves {bill}", f"The {bill} cleared the {chamber.replace('the upper chamber','Senate')} by {n} votes to {m} on {pick(['Monday','Tuesday','Wednesday','Thursday'])}. Sponsor {pick(['Senator','Representative'])} {pick(surn)} said the vote shows broad support. The measure now goes to a conference committee, where differences over {pick(['funding levels','an amendment on permits','an offset for the cost','rules for states'])} must be settled."),
     (f"{bill} stalls in committee", f"A vote on the {bill} was postponed after {pick(surn)} and {pick(surn)} asked for more time to study its cost, estimated at ${R.randint(2,40)} billion over ten years. Leaders now say a floor vote is unlikely before the new year. Advocates plan to rally outside the Capitol."),
     (f"Senate rejects {bill} on a procedural vote", f"The {bill} failed to advance, getting {n-15} votes where sixty were required. {pick(['Senator','Senator'])} {pick(surn)}, the majority leader, blamed delay tactics. A revised version could return after Thanksgiving."),
    ])
    add(*t)
for i in range(int(20*1.45)):  # bridges and roads
    p = pick(places); nm = pick(["Mill Road", "Canal Street", "Fourth Avenue", "Old Quarry", "Depot Lane", "Riverside Drive", "Lincoln Parkway"])
    t = pick([
     (f"{nm} bridge in {p} closed for {pick(['a weekend','nine days','three weeks','two nights'])}", f"Transportation crews will close the {nm} bridge in {p} for {pick(['expansion joint replacement','repainting','a bearing inspection','emergency crack repair'])}. Drivers should follow signed detours via {pick(['Route 14','the county road','Exit 22','Main Street'])}. Delays of {R.randint(10,25)} minutes are expected at rush hour."),
     (f"Overpass at {nm} reopens early", f"The overpass on {nm} in {p} reopened to traffic {R.randint(2,6)} days ahead of schedule after crews finished pouring the new surface. The ${R.uniform(1.5,9):.1f} million project was paid by a state grant."),
     (f"Lane closures planned on {nm} in {p}", f"Expect one lane of traffic in each direction on {nm} through {p} from Monday until the end of the month, while a contractor lays new water mains. Businesses said they will stay open with signs for parking in back."),
    ])
    add(*t)
for i in range(int(22*1.45)):  # chip and tech companies
    c = pick(chips); t = pick([
     (f"{c} posts quarterly revenue of ${R.uniform(2,19):.1f} billion", f"{c} reported revenue of ${R.uniform(2,19):.1f} billion for the quarter, {pick(['above','below','in line with'])} analyst expectations. Executives pointed to {pick(['automotive demand','strong orders for networking chips','weak consumer electronics','a ramp of its new 3-nanometer process'])}. Shares moved {R.randint(1,9)} percent in after-hours trading."),
     (f"{c} unveils {pick(['a faster graphics processor','a low-power sensor chip','an AI inference accelerator','a memory controller'])}", f"{c} introduced a new product on {pick(['Tuesday','Wednesday','Thursday'])}, promising {R.randint(20,70)} percent better performance per watt. Samples go to customers in {pick(['January','March','the second quarter'])}. Analysts said the launch keeps it competitive."),
     (f"Export curbs squeeze {c}", f"New export rules on advanced chip tools could cost {c} up to ${R.uniform(0.3,2.2):.1f} billion in sales next year, the company warned. It is seeking licenses and shifting some work to {pick(['Malaysia','Texas','Ireland','Japan'])}."),
    ])
    add(*t)
for i in range(int(22*1.45)):  # weather and disasters
    p = pick(places); t = pick([
     (f"Storm knocks out power to {R.randint(8,95)},000 customers", f"A fast-moving storm brought {R.randint(40,75)} mph gusts to {p} and nearby towns, bringing down trees and cutting power. Utilities said most service would be restored by {pick(['Thursday','Friday','the weekend'])}. Shelters opened at two high schools."),
     (f"Flood warning issued for {p}", f"The weather service issued a flood warning for the river at {p}, expected to crest {R.randint(3,9)} feet above flood stage on {pick(['Wednesday','Thursday','Saturday'])}. Residents of low-lying streets were told to move vehicles and valuables."),
     (f"First snow of the season dusts {p}", f"{R.randint(2,9)} inches of wet snow fell on {p} and the surrounding hills overnight, the earliest measurable snowfall in {R.randint(8,30)} years. Plows were out by dawn and several flights were delayed."),
    ])
    add(*t)
for i in range(int(20*1.45)):  # sports
    a, b = pick(places), pick(places); tm1, tm2 = pick(teams), pick(teams)
    if tm1 == tm2: tm2 = "Rapids"
    s1, s2 = R.randint(1, 5), R.randint(0, 4)
    if s1 == s2: s1 += 1
    t = pick([
     (f"{a} {tm1} top {b} {tm2} {s1}-{s2}", f"The {a} {tm1} defeated the {b} {tm2} {s1}-{s2} on {pick(['Friday','Saturday','Sunday'])}, led by {pick(surn)}'s {R.randint(1,3)} goals. The result leaves {a} {pick(['first','third','fifth'])} in the standings."),
     (f"{pick(surn)} signs {R.randint(3,6)}-year deal with {tm1}", f"The {tm1} locked up their star {pick(['forward','defender','pitcher','guard'])}, {pick(surn)}, with a contract reportedly worth ${R.randint(40,180)} million. The player said he wanted to stay in {a}."),
    ])
    add(*t)
for i in range(int(18*1.45)):  # health
    p = pick(places); t = pick([
     (f"Flu season arrives early in {p}", f"Clinics in {p} reported a {R.randint(20,60)} percent rise in influenza visits over the past week. Doctors urged residents to get vaccinated and to stay home when sick. Hospitals have begun limiting visitors."),
     (f"{p} clinic receives ${R.randint(2,12)} million for rural care", f"A federal grant will pay for a mobile clinic, two nurse practitioners and telehealth carts serving farms around {p}. Officials said the nearest emergency room is now {R.randint(35,70)} miles away for some families."),
     (f"New study links {pick(['sleep','walking','coffee','cooking at home'])} to longer life", f"Researchers following {R.randint(8,60)},000 adults for {R.randint(7,18)} years found a lower risk of heart disease among those who {pick(['slept seven hours','walked 8,000 steps daily','drank two cups of coffee','ate home-cooked meals'])}. The authors cautioned that the study shows association, not cause."),
    ])
    add(*t)
for i in range(int(20*1.45)):  # local government, housing, schools, transit
    p = pick(places); t = pick([
     (f"{p} council weighs budget for {pick(subs)}", f"The {p} city council opened hearings on a ${R.uniform(1,9):.1f} million proposal for {pick(subs)}. Residents spoke for two hours; a vote is expected on {pick(['December 3','December 10','the 17th'])}."),
     (f"Rents in {p} rise {R.randint(3,11)} percent in a year", f"Average rent in {p} reached ${R.randint(1300,2900)} a month, according to a new survey. Tenant groups asked the council to consider a cap, while landlords pointed to higher taxes and insurance."),
     (f"{p} schools to start later next fall", f"The {p} school board voted to move first bells {R.randint(30,50)} minutes later for high schoolers, citing studies on teen sleep. Bus routes will be redrawn over the summer, and the district expects to spend ${R.randint(300,900)},000 on the change."),
     (f"Bus drivers in {p} protest schedule changes", f"Drivers for the {p} transit authority wore armbands to work to protest split shifts. The union said it has not called a strike. Service ran normally, though {R.randint(2,9)} routes were cut at rush hour for lack of staff."),
    ])
    add(*t)
for i in range(int(18*1.45)):  # business and economy
    p = pick(places); t = pick([
     (f"{pick(['Hartley','Orlin','Brannigan','Castor','Vesper'])} {pick(['Brewing','Logistics','Outdoor','Pharma','Textiles'])} to open plant in {p}", f"The company will build a ${R.randint(60,400)} million facility in {p}, hiring {R.randint(150,900)} workers by 2028. Local officials approved {pick(['a ten-year tax abatement','road upgrades','a training grant'])}."),
     (f"Inflation slows to {R.uniform(2.1,3.8):.1f} percent", f"Consumer prices rose {R.uniform(2.1,3.8):.1f} percent from a year earlier, the government said, with food and shelter the biggest contributors while gasoline fell. Economists expect the central bank to {pick(['hold rates','cut by a quarter point','wait for more data'])} at its next meeting."),
     (f"Unemployment in {p} falls to {R.uniform(3.2,5.9):.1f} percent", f"The jobless rate in the {p} area fell as {pick(['warehouses','hospitals','construction firms','schools'])} added positions. The labor force participation rate was little changed."),
    ])
    add(*t)
for i in range(int(18*1.45)):  # science, environment, culture, courts
    p = pick(places); t = pick([
     (f"Museum in {p} acquires {pick(['a Dutch landscape','a mastodon skeleton','a trove of jazz records','a medieval manuscript'])}", f"The {p} museum announced the acquisition of {pick(['a seventeenth-century landscape','a nearly complete mastodon skeleton','4,000 jazz records','an illuminated manuscript'])}, funded by an anonymous donor. It will go on view in {pick(['March','the spring','June'])}."),
     (f"Whales return to waters off {p}", f"Marine biologists counted {R.randint(30,200)} humpback whales feeding off {p} this fall, the highest number since surveys began. They credited cleaner water and fewer fishing nets."),
     (f"Judge blocks {pick(['a pipeline','a landfill','a mining permit','a billboard ordinance'])} in {p}", f"A county judge halted work on the project, saying regulators skipped a required public comment period. The developer said it would ask the appeals court for a stay."),
     (f"Researchers in {p} build a battery that charges in {R.randint(4,12)} minutes", f"A university team said its new lithium-sulfur design recharges to 80 percent in under {R.randint(4,12)} minutes and survived {R.randint(500,2000)} cycles. Commercial cells are years away, the authors said."),
     (f"Police in {p} warn of phone scam targeting seniors", f"Callers pretending to be bank investigators have taken ${R.randint(20,260)},000 from {R.randint(6,30)} older residents of {p} this month. Officers urged families to hang up and call the bank directly."),
    ])
    add(*t)

# overlap the three motifs (a senate vote, a bridge closure, a chip company) with the target subjects on purpose
extra = [
 ("Senate committee hears testimony on Tidewater region levees", "Engineers told a Senate subcommittee that levees along the coast are decades old and need $3 billion in repairs. The hearing did not include a vote. Members said they would revisit funding in the next session."),
 ("House passes its own harbor bill", "The House passed a smaller harbor and dredging bill by 220 to 205, setting up negotiations with the Senate over how much to spend on flood pumps. Leaders hope to settle the differences before the holiday recess."),
 ("Solar installers lay off workers as incentives lapse", "Rooftop installers in three states have cut staff in recent weeks, pointing to uncertainty over federal incentives and higher interest rates. Industry groups said orders fell by a third."),
 ("Calder River dam gates opened ahead of rain", "The water authority opened the gates at the Calder River dam on Monday to lower the reservoir before forecast rain. Boaters were warned of strong currents below the dam."),
 ("Bus detour announced on Mill Road overpass", "Mill Road will carry extra buses beginning next month while a bridge elsewhere in the county is rebuilt. The county said it would add a turn lane at the intersection with Fourth Avenue."),
 ("Aldersgate Valley residents fight proposed quarry", "Neighbors in Aldersgate Valley filled a township hearing to oppose a gravel quarry. The owner said it would employ forty people and restore the land after thirty years."),
 ("Veridian Hills hospital names new chief", "Veridian Hills Hospital named Dr. Helen Okafor its chief executive, succeeding a leader who retired after twelve years. She had run the system's cancer program."),
 ("Memory chip prices slide for third straight month", "Spot prices for memory chips fell again in November, which analysts tied to excess inventory at phone and PC makers. Several producers said they would trim output rather than cut prices further."),
 ("Mercy Street bakery wins statewide prize", "A family bakery on Mercy Street in Corbin won the state's best bread award for its sourdough rye, beating forty entrants. The owners said orders doubled after the news."),
 ("Wind farm proposal splits coastal town", "Voters in Stillwater will decide in March whether to allow a six-turbine wind project off their beach. Fishermen worry about nets and nesting birds, while supporters point to tax revenue."),
 ("Quartet of astronomers share prize for dark matter maps", "Four astronomers split a $1 million prize for mapping dark matter through gravitational lensing. They said the maps will be refined by the next generation of survey telescopes."),
 ("Lunar lander company raises $400 million", "A start-up that builds small lunar landers raised $400 million to build a fleet. The firm has one mission planned for next spring and two in the following year."),
 ("Teachers union backs school board candidates", "The teachers union in the Brannigan district endorsed three candidates for the school board ahead of the runoff. Class size and pay headline their platform."),
 ("Commuter ferry service to expand", "The harbor ferry will add two evening sailings from December, officials said, answering requests from commuters who cannot rely on the overcrowded trains."),
 ("Wildfire smoke pushes air quality to unhealthy levels", "Smoke from fires in the mountains drifted over the valley, pushing air quality readings into the unhealthy range. Schools kept children indoors at recess."),
 ("Parley rival launches encrypted chat", "A start-up announced an encrypted messaging service that promises not to upload address books. The company said it has 200,000 early users and no ads."),
 ("County treasurer's office moves to new building", "The county treasurer's office will open in a renovated bank building next month. Residents can pay property taxes by card at the counter for the first time."),
 ("Marathon training groups swell ahead of winter race", "Running clubs report record sign-ups for the winter marathon in Riverton, with some groups adding a second weekly long run. Shoe stores said sales are up."),
 ("Freezing rain advisory posted for the hills", "A winter weather advisory will be in effect from midnight until noon for the hills north of the lake, where light freezing rain may coat untreated surfaces."),
 ("Cooperative grocer expands to fourth store", "The Hillside food cooperative will open its fourth location in January, adding 25 jobs. Members voted to keep their pricing structure unchanged."),
]
for h, b in extra: add(h, b)

# ---- assemble: target + generated, 300 stories, dated across two weeks ---------------------------------------------
stories = []
need = 300 - len(T)
items = [dict(h=t["h"], b=t["b"], key=t["k"]) for t in T] + gen[:need]
R.shuffle(items)
for i, it in enumerate(items):
    d = D0 + datetime.timedelta(days=R.randint(0, 13))
    sid = f"n{i+1:04d}"
    it["id"] = sid; it["date"] = d.isoformat()
    stories.append(it)
byk = {s["key"]: s["id"] for s in stories if "key" in s}
for s in stories: s.pop("key", None)
qs = []
for t in T:
    qs.append(dict(q=t["q"], kind=t["kind"], source="story", answer_ids=[byk[t["k"]]]))
qs += [
 dict(q="What did the label on the little bottle say?", kind="exact", source="book", answer_phrases=["DRINK ME"]),
 dict(q="Who is the Cheshire cat saying everyone in this place is mad?", kind="exact", source="book", answer_phrases=["we're all mad here"]),
 dict(q="What was the first question the smoking caterpillar put to the girl?", kind="exact", source="book", answer_phrases=["Who are you?"]),
 dict(q="Who was the strange hatter that kept asking riddles at the afternoon meal in the garden?", kind="para", source="book", answer_phrases=["raven like a writing-desk"]),
 dict(q="What sport did the furious monarch force her guests to play, with live birds as the clubs?", kind="para", source="book", answer_phrases=["flamingo"]),
 dict(q="How did the heroine become too tall for the house and wedge herself inside it?", kind="para", source="book", answer_phrases=["one arm out of the window"]),
]
assert len(qs) == 30
json.dump(stories, open("stories.json", "w"), indent=0, ensure_ascii=False)
json.dump(qs, open("questions.json", "w"), indent=1, ensure_ascii=False)
print(len(stories), "stories", len(qs), "questions", sum(1 for q in qs if q["kind"] == "exact"), "exact")

# ---- the device fixture: the 24 target stories and their questions as Swift literals (the phone trial's tiny corpus) ----
def sw(s): return '"' + s.replace('\\', '\\\\').replace('"', '\\"') + '"'
rows = ",\n".join(f'    ({sw(byk[t["k"]])}, {sw(t["h"])}, {sw(t["b"])}, {sw(t["q"])}, {sw(t["kind"])})' for t in T)
open("../../AutoradioKit/Sources/AutoradioModels/EmbedTrialFixture.swift", "w").write(f'''// Generated by mobile/EmbedTrial/corpus/make_corpus.py: invented news stories and the question that must find each one.
// The phone's embedding trial (EmbedDeviceTrial) embeds these {len(T)} stories and asks these {len(T)} questions.
enum EmbedTrialFixture {{
    /// (id, headline, body, question, kind) where kind is "exact" (a name BM25 should catch) or "para" (a paraphrase with no shared keywords).
    static let rows: [(id: String, title: String, body: String, question: String, kind: String)] = [
{rows}
    ]
}}
''')
