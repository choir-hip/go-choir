package podcastmatch

// Fixtures for the automatic matcher: invented stories and invented spoken
// passages (no third-party transcript text). Each case is one episode: filler,
// a passage, filler. Positives report the story's event in other words;
// negatives are near misses.

import (
	"fmt"
	"strings"
	"time"
)

type fixStory struct{ id, title, dek string }

var fixStories = []fixStory{
	{"ferry", "Harborview ferry Marlin strikes pier in Tessaly Bay, 40 hurt", "The coast guard says the vessel lost steering during its morning crossing."},
	{"bridge", "Valdoria bans night freight on Merrow Bridge after cracks found", "Inspectors found fatigue cracks in the steel deck of the 1960s crossing."},
	{"ebike", "Kestrel Dynamics recalls 90,000 Orbis e-bikes over battery fires", "The company has received 31 reports of packs overheating while charging."},
	{"water", "Norland parliament passes water rationing law after record drought", "Households face fines for garden watering as reservoirs fall below 20 percent."},
	{"deal", "Lindqvist Group agrees to buy Sablewood Foods for $4.2 billion", "The all-cash deal would create the region's largest packaged food maker."},
	{"typhoon", "Typhoon Amara makes landfall near Port Calder, 200,000 evacuated", "Winds of 170 kilometres an hour tore roofs from homes along the coast."},
	{"bank", "Brevia central bank raises rates to 6.5 percent in surprise move", "Governor Ilse Maran cited stubborn wage growth and a weak currency."},
	{"vaccine", "Wolfram Institute reports malaria vaccine R9 worked in Ghanvale trial", "The late-stage study found 78 percent protection in children under five."},
	{"mayor", "Eastmere mayor Dale Prentiss resigns amid contracting inquiry", "Prosecutors are examining road repair contracts awarded to a friend's firm."},
	{"rail", "Ostrava Rail strike enters third day, halting freight to Kovnia", "Unions reject a 4 percent pay offer as port stockpiles grow."},
	{"lunar", "Titan Lunar's Selene-2 lander misses landing site, contact lost", "Mission controllers lost the signal minutes after the descent engine fired."},
	{"marathon", "Halverson wins Great Basin Marathon in record 2:03:41", "The 27-year-old broke the course record by almost two minutes."},
}

type fixCase struct {
	name     string
	want     string // the story id the passage is for; "" when no story should take it
	ageHours float64
	text     []string
}

func s(x ...string) []string { return x }

var fixFiller = []string{
	"Good morning and welcome to the programme.",
	"The weather across the south stays mild with light wind through the evening.",
	"Stock indexes closed a little higher after a quiet day of trading in most sectors.",
	"A listener wrote in to ask about gardening tips for the coming season.",
	"The next segment looks at how schools are planning their calendars.",
	"We will be back after a short break.",
	"Traffic on the ring road is moving normally this hour.",
	"Our guest this week has written a book about the history of canals.",
	"The county fair opens on Saturday with music and food stalls.",
	"Sports now, and the local team won again at home last night.",
	"Thanks for staying with us through the hour.",
	"Researchers say the new library catalogue will be searchable by voice.",
	"The museum has extended its opening hours for the holidays.",
	"Coming up, a look at what is on at the cinema this weekend.",
	"Farmers report a good harvest of apples in the western valleys.",
	"That is all from us for now, more at the top of the hour.",
}

var fixCases = []fixCase{
	// ---- positives (the passage reports the story's event in other words)
	{"ferry+1", "ferry", -3, s("A passenger ferry called the Marlin hit a pier in Tessaly Bay this morning.", "Forty people were hurt, most of them with minor injuries.", "The coast guard says the Harborview vessel lost steering during its crossing.", "Rescue boats reached the ferry within minutes of the collision.", "Investigators will examine the steering gear in the coming days.")},
	{"ferry+2", "ferry", -6, s("Tessaly Bay was closed to traffic for hours after the Marlin ferry struck a pier.", "Harborview Lines confirmed that about forty passengers needed treatment.", "A coast guard spokesman said the captain reported a loss of steering.", "The pier itself is badly damaged.", "Officials expect a statement from the operator tonight.")},
	{"bridge+1", "bridge", -4, s("Valdoria has banned heavy freight from Merrow Bridge between dusk and dawn.", "Inspectors found fatigue cracks in the steel deck of the sixties crossing.", "The transport minister said trucks must use the southern ring road at night.", "Engineers will test the cracked sections over the next month.", "Hauliers warn the ban will add hours to deliveries.")},
	{"ebike+1", "ebike", -5, s("Kestrel Dynamics is recalling ninety thousand Orbis e-bikes.", "The firm says batteries in the bikes can overheat while charging and catch fire.", "It has had thirty-one reports of overheating packs so far.", "Owners are told to stop charging the Orbis bikes and contact the company.", "A refund or a free replacement battery will be offered.")},
	{"water+1", "water", -8, s("Lawmakers in Norland have passed a water rationing law after the record drought.", "Reservoirs have fallen below twenty percent of capacity.", "Households that water their gardens will face fines under the new rules.", "The Norland parliament voted late on Thursday.", "Farmers are exempt for now but must report their usage.")},
	{"deal+1", "deal", -2, s("Lindqvist Group has agreed to buy Sablewood Foods in an all-cash deal.", "The price is about four point two billion dollars.", "Together the two firms would be the largest packaged food maker in the region.", "Sablewood shareholders are expected to vote in the new year.", "Analysts say Lindqvist paid a generous premium.")},
	{"typhoon+1", "typhoon", -1, s("Typhoon Amara came ashore near Port Calder overnight.", "More than two hundred thousand people had been evacuated beforehand.", "Winds of a hundred and seventy kilometres an hour tore roofs from homes along the coast.", "Power is out across much of the Port Calder region.", "Rescue crews are only now reaching the worst hit villages.")},
	{"typhoon+2", "typhoon", -9, s("Amara, the typhoon, has hit the coast near Port Calder with ferocious winds.", "Officials say two hundred thousand residents left their homes ahead of it.", "Roofs were stripped from houses all along the shoreline.", "The storm is now weakening as it moves inland.", "Aid agencies are preparing shelters for those evacuated.")},
	{"bank+1", "bank", -2, s("Brevia's central bank has raised interest rates to six and a half percent.", "The move surprised markets, which had expected rates to hold.", "Governor Ilse Maran pointed to stubborn wage growth and a weak currency.", "The currency rose sharply after the announcement.", "Mortgage holders will feel the increase within weeks.")},
	{"vaccine+1", "vaccine", -12, s("The Wolfram Institute says its malaria vaccine R9 worked in a late-stage trial in Ghanvale.", "Protection reached seventy-eight percent among children under five.", "It is the strongest result yet for a malaria vaccine in young children.", "Regulators will now review the data.", "The institute hopes for approval within the year.")},
	{"mayor+1", "mayor", -3, s("The mayor of Eastmere, Dale Prentiss, has resigned.", "Prosecutors are examining road repair contracts awarded to a firm run by a friend.", "Mr Prentiss denies wrongdoing but said the inquiry was a distraction for the town.", "The deputy mayor will serve until an election.", "Residents gathered outside the town hall this afternoon.")},
	{"rail+1", "rail", -4, s("The strike at Ostrava Rail has now entered its third day.", "No freight is moving to Kovnia and stockpiles at the port are growing.", "Unions rejected a four percent pay offer on Tuesday night.", "The company says it is ready to talk again.", "Businesses warn of shortages if the strike drags on.")},
	{"lunar+1", "lunar", -2, s("Titan Lunar's Selene-2 lander has missed its landing site.", "Mission controllers lost the signal minutes after the descent engine fired.", "The company says it is still trying to reach the spacecraft.", "Selene-2 was meant to be the first private lander to touch down near the pole.", "A statement is expected within hours.")},
	{"marathon+1", "marathon", -5, s("Halverson has won the Great Basin Marathon in a record time of two hours three minutes and forty-one seconds.", "The twenty-seven-year-old broke the course record by almost two minutes.", "She pulled away from the field after the thirty kilometre mark.", "Crowds lined the finish in cold sunshine.", "Halverson said she had dreamed of this race since childhood.")},

	// ---- negatives: an earlier, different event of the same kind
	{"ferry-prior", "", -3, s("Ten years ago another ferry, the Gannet, ran aground off the coast of Brevia.", "Nobody was badly hurt, and the inquiry blamed a faulty chart.", "The operator there replaced its whole fleet afterward.", "Maritime safety groups still cite it in their training.", "The lessons from that grounding remain relevant for crews today.")},
	{"bridge-prior", "", -3, s("A footbridge in the Kessel valley was closed last spring after inspectors found rust.", "Hikers had to take a long detour for weeks.", "The council repaired it with new timber and reopened it in summer.", "Nobody was hurt and no cracks were found in the stone piers.", "Walkers say the path is nicer than before.")},
	{"ebike-prior", "", -3, s("Electric scooters were recalled last year by another maker over brake faults.", "Customers were offered a free repair at any dealer.", "The recall covered a few thousand units and ended quietly.", "Consumer groups say recalls like that are common.", "Always check the label before buying a used scooter.")},
	{"water-prior", "", -3, s("Water bills are going up across the country next spring.", "Utilities say the pipes need replacing and the cost has to be shared.", "Customer groups want the regulator to cap the increase.", "A decision is expected in the summer.", "Meanwhile people are advised to fix leaking taps.")},
	{"deal-prior", "", -3, s("A food maker in the north has agreed to buy a small bakery chain.", "The price was not disclosed but the chain has forty shops.", "The bakery's founder will stay on as an adviser.", "Staff were told the shops would keep their names.", "The deal is expected to close before the end of the year.")},
	{"typhoon-prior", "", -3, s("Last year a typhoon hit the islands in the south and thousands were evacuated.", "Rebuilding is still not finished in some villages.", "The government promised new shelters that have yet to appear.", "Residents say they feel forgotten.", "A memorial service was held this week for the victims.")},
	{"bank-prior", "", -3, s("The central bank of a neighbouring country held its rates steady today.", "Officials said inflation is easing slowly and wages are calm.", "Markets had expected exactly that and barely moved.", "The next meeting is in six weeks.", "Economists expect a cut early next year.")},
	{"vaccine-prior", "", -3, s("Another institute reported an early trial of a flu vaccine this week.", "The study was small and enrolled only adults.", "Protection was modest and the authors urge caution.", "A larger trial will start in spring.", "Experts say flu vaccines are hard to improve.")},
	{"mayor-prior", "", -3, s("The mayor of a coastal town opened a new library this week.", "Hundreds of residents came to see the children's reading room.", "The project was paid for by a local charity.", "There were speeches and a small orchestra.", "The library opens daily from nine.")},
	{"rail-prior", "", -3, s("A strike on the northern buses ended after two days with a modest pay rise.", "Commuters were relieved to see services back to normal.", "The union said it had won guarantees on safety.", "The company said the deal was fair.", "Fares will not change this year.")},
	{"lunar-prior", "", -3, s("A space agency lander touched down on the moon years ago and sent back pictures.", "The mission lasted only a week before the cold ended it.", "Engineers still study the data to this day.", "Scientists say the dust was the biggest surprise.", "Another mission is planned for the end of the decade.")},
	{"marathon-prior", "", -3, s("The city half marathon takes place next Sunday and roads will close early.", "Organisers expect twelve thousand runners this year.", "Volunteers are still needed at the water stations.", "Entry is closed but spectators are welcome.", "Buses will run a special timetable.")},

	// ---- negatives: the name of the place or firm, in unrelated news
	{"ferry-name", "", -3, s("Harborview is known for its seafood restaurants and its summer festival.", "The town council announced a new market hall this week.", "Residents say parking is the biggest worry.", "The festival organisers expect record crowds next year.", "Local bakeries are hiring extra staff.")},
	{"bridge-name", "", -3, s("Valdoria won its football match against Norland last night by two goals.", "The captain scored twice in the second half.", "Fans celebrated in the capital until late.", "The coach praised the defence after the game.", "The team plays again on Sunday.")},
	{"ebike-name", "", -3, s("Kestrel Dynamics opened a new office in the capital this week.", "The company says it will hire two hundred engineers.", "Its chief executive spoke about training and apprenticeships.", "The mayor attended the opening.", "Tours of the office are planned for students.")},
	{"water-name", "", -3, s("The parliament of Norland returned from its summer break today.", "Members will debate the budget and a new education bill.", "The opposition promised to fight the tax proposals.", "The prime minister opened the session with a short speech.", "Committees start work on Monday.")},
	{"deal-name", "", -3, s("Sablewood Foods will sponsor the school sports day this year.", "The company has supported the event for a decade.", "Pupils will receive free fruit at the finish line.", "Teachers thanked the firm for its generosity.", "The day starts at ten on Friday.")},
	{"typhoon-name", "", -3, s("Port Calder is a busy trading harbour on the eastern coast.", "Its fish market is one of the largest in the region.", "Tourists come for the old town and the lighthouse.", "A new tram line is planned for next year.", "The harbour festival is held each spring.")},
	{"bank-name", "", -3, s("Ilse Maran spoke at a university about the history of banking.", "She described how coins were minted in the middle ages.", "Students asked about careers in public service.", "She said patience is the main virtue of a regulator.", "The lecture was recorded for the library.")},
	{"vaccine-name", "", -3, s("The Wolfram Institute opened an exhibition on the history of medicine.", "Visitors can see old microscopes and handwritten lab books.", "School groups are welcome on weekdays.", "Entry is free for children.", "The exhibition runs until the spring.")},
	{"mayor-name", "", -3, s("Dale Prentiss is a keen gardener and grows prize onions.", "He won first place at the Eastmere flower show again this year.", "He credits his soil and a good deal of patience.", "The show raised money for the hospital.", "Next year's show will have a new tent.")},
	{"rail-name", "", -3, s("Ostrava Rail published its new timetable for the coming year.", "Most services keep the same times as before.", "A few late trains have been added on Fridays.", "Passengers can download the timetable from the website.", "Printed copies are available at stations.")},
	{"lunar-name", "", -3, s("Titan Lunar's chief executive gave a talk to students about careers in space.", "She encouraged them to study engineering and physics.", "Questions came from a packed hall.", "The talk ended with a round of applause.", "Recordings will be published next week.")},
	{"marathon-name", "", -3, s("The Great Basin is a high desert region with long dry summers.", "Wildlife there includes pronghorn and desert foxes.", "Hikers are advised to carry plenty of water.", "A national park protects its ancient pines.", "Visitors can camp at several sites.")},

	// ---- negatives: same subject, a different event with some shared names
	{"ferry-other", "", -3, s("A ferry in the Norland islands was delayed by fog this morning.", "Passengers waited two hours at the terminal before boarding.", "The operator apologised and offered refunds.", "No one was hurt and there was no damage to the vessel.", "Services returned to normal by noon.")},
	{"bridge-other", "", -3, s("Traffic on the Merrow Bridge was slow today because of roadworks.", "Lane closures will last until the end of the month.", "Drivers are advised to leave early.", "The work is routine repainting and nothing more.", "Cyclists can still cross as normal.")},
	{"ebike-other", "", -3, s("Orbis is the name of a new cycling festival in the highlands.", "Riders from five countries will take part this summer.", "Many will arrive by e-bike to climb the steep roads.", "Organisers say the route is safe and well marked.", "Tickets go on sale next week.")},
	{"water-other", "", -3, s("A drought in the highlands has left farmers short of feed for cattle.", "Many are selling stock early to cut costs.", "Rain is forecast for the weekend which may help.", "Charities are collecting hay for the worst affected farms.", "The government has promised to look at relief.")},
	{"deal-other", "", -3, s("Lindqvist Group reported quarterly profits that beat expectations.", "Revenue rose four percent thanks to strong sales abroad.", "The chief executive said the outlook remains stable.", "Shares gained slightly in morning trading.", "A dividend will be paid next month.")},
	{"typhoon-other", "", -3, s("A tropical storm is forming far out in the ocean and may become a typhoon.", "Forecasters say it is too early to know where it will go.", "Ships have been told to avoid the area.", "Coastal towns are watching the forecasts.", "An update will follow tomorrow.")},
	{"bank-other", "", -3, s("Interest rates on savings accounts have risen in Brevia this year.", "Banks compete for deposits as the cost of borrowing grows.", "Savers are advised to shop around for the best rate.", "Fixed accounts now pay more than instant access ones.", "A guide is on our website.")},
	{"vaccine-other", "", -3, s("Malaria remains one of the biggest killers of children in the world.", "Bed nets and medicines have saved millions of lives.", "Rainy seasons bring the highest risk.", "Aid groups say funding is falling short.", "Better tools would change everything.")},
	{"mayor-other", "", -3, s("Contracting rules for road repairs were tightened across the country this year.", "Councils must now publish every bid.", "Campaigners welcomed the change.", "Some small firms fear more paperwork.", "The rules take effect in January.")},
	{"rail-other", "", -3, s("Freight volumes on the railways have grown this year despite high costs.", "Companies are shifting loads from trucks to trains.", "New terminals are planned at several ports.", "Environmental groups welcome the change.", "Capacity is the main constraint.")},
	{"lunar-other", "", -3, s("A lunar eclipse will be visible across the country on Friday night.", "The moon will turn a deep red for about an hour.", "Clear skies are expected in the west.", "Astronomy clubs are holding viewing evenings.", "No special equipment is needed.")},
	{"marathon-other", "", -3, s("Runners are reminded to stretch properly before a long race.", "Cramps are a common problem in cold weather.", "Drink small amounts often rather than a lot at once.", "A good pair of shoes matters more than any gadget.", "Rest days are part of training.")},

	// ---- negatives: a roundup of headlines (each story gets one sentence)
	{"roundup-1", "", -3, s("In other news a ferry hit a pier in Tessaly Bay and forty people were hurt.", "Valdoria banned night freight on Merrow Bridge after cracks were found.", "Kestrel Dynamics recalled ninety thousand Orbis e-bikes over battery fires.", "Norland passed a water rationing law after the drought.", "And Lindqvist Group agreed to buy Sablewood Foods.")},
	{"roundup-2", "", -3, s("Typhoon Amara hit Port Calder and two hundred thousand were evacuated.", "Brevia's central bank raised rates to six and a half percent.", "The Wolfram Institute reported a malaria vaccine trial in Ghanvale.", "The mayor of Eastmere resigned over a contracting inquiry.", "More on all of those in a moment.")},
	{"roundup-3", "", -3, s("Ostrava Rail strike in its third day halts freight to Kovnia.", "Titan Lunar's Selene-2 lander missed its landing site.", "Halverson won the Great Basin Marathon in a record time.", "A ferry struck a pier in Tessaly Bay and the coast guard is investigating.", "Those are the headlines this hour.")},
	{"roundup-4", "", -3, s("Brevia raised rates, Typhoon Amara hit Port Calder, and Eastmere's mayor Dale Prentiss resigned.", "Lindqvist Group will buy Sablewood Foods and Kestrel Dynamics recalled Orbis e-bikes.", "The Wolfram Institute reported a malaria vaccine result in Ghanvale.", "Norland passed water rationing and Valdoria banned night freight on Merrow Bridge.", "We will go through each of them after the break.")},

	// ---- negatives: the right event, but the episode is old
	{"ferry-old", "", -216, s("A passenger ferry called the Marlin hit a pier in Tessaly Bay this morning.", "Forty people were hurt, most of them with minor injuries.", "The coast guard says the Harborview vessel lost steering during its crossing.", "Rescue boats reached the ferry within minutes of the collision.", "Investigators will examine the steering gear in the coming days.")},
	{"bank-old", "", -300, s("Brevia's central bank has raised interest rates to six and a half percent.", "The move surprised markets, which had expected rates to hold.", "Governor Ilse Maran pointed to stubborn wage growth and a weak currency.", "The currency rose sharply after the announcement.", "Mortgage holders will feel the increase within weeks.")},
	{"lunar-old", "", -120, s("Titan Lunar's Selene-2 lander has missed its landing site.", "Mission controllers lost the signal minutes after the descent engine fired.", "The company says it is still trying to reach the spacecraft.", "Selene-2 was meant to be the first private lander to touch down near the pole.", "A statement is expected within hours.")},
	{"mayor-old", "", -400, s("The mayor of Eastmere, Dale Prentiss, has resigned.", "Prosecutors are examining road repair contracts awarded to a firm run by a friend.", "Mr Prentiss denies wrongdoing but said the inquiry was a distraction for the town.", "The deputy mayor will serve until an election.", "Residents gathered outside the town hall this afternoon.")},
}

// fixture builds the stories and episodes. Every case is an episode of 7 second
// segments with sentence-final punctuation: 8 filler sentences, the passage and
// 8 more. Stories were updated at fixNow.
var fixNow = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

func fixture() ([]Story, []Episode, map[string]string) {
	var stories []Story
	for _, f := range fixStories {
		stories = append(stories, Story{ID: f.id, Title: f.title, Dek: f.dek, Updated: fixNow})
	}
	var eps []Episode
	want := map[string]string{}
	for ci, c := range fixCases {
		var segs []Segment
		add := func(t string) {
			at := float64(len(segs)) * 7
			segs = append(segs, Segment{Start: at, End: at + 6.5, Text: t})
		}
		for k := 0; k < 8; k++ {
			add(fixFiller[(ci+k)%len(fixFiller)])
		}
		for _, t := range c.text {
			add(t)
		}
		for k := 0; k < 8; k++ {
			add(fixFiller[(ci*3+k+5)%len(fixFiller)])
		}
		eps = append(eps, Episode{Key: c.name, Published: fixNow.Add(time.Duration(c.ageHours * float64(time.Hour))), Segs: segs})
		if c.want != "" {
			want[c.name] = c.want
		}
	}
	// A corpus of the usual size: many episodes that carry none of the stories.
	for k := 0; k < 40; k++ {
		var segs []Segment
		for n := 0; n < 120; n++ {
			at := float64(n) * 7
			segs = append(segs, Segment{Start: at, End: at + 6.5, Text: fixFiller[(k*7+n*5)%len(fixFiller)]})
		}
		eps = append(eps, Episode{Key: fmt.Sprintf("pad-%d", k), Published: fixNow.Add(-5 * time.Hour), Segs: segs})
	}
	return stories, eps, want
}

func joinText(t []string) string { return strings.Join(t, " ") }
