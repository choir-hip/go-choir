package entitytext

// The word lists of the extractor. Keys are match keys (lower case, folded).
// They are deliberately short: a word missing here costs one System 1
// question, and System 1's "not an entity" answers feed the learned stoplist
// (slice c0 §6), which is where the long tail belongs.

func set(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// stopwords neither start nor end a span, split a run of capitals, and never
// match or are proposed alone: function words, pronouns, sentence openers,
// days and months.
var stopwords = set(
	"a", "an", "the", "and", "or", "but", "nor", "so", "yet", "if", "as", "at", "by", "for", "from", "in",
	"into", "on", "onto", "to", "with", "without", "about", "after", "before", "during", "since", "until",
	"while", "when", "where", "why", "how", "what", "which", "who", "whom", "whose", "that", "this",
	"these", "those", "there", "here", "it", "its", "he", "she", "they", "we", "you", "i", "me", "him",
	"her", "his", "hers", "their", "theirs", "our", "ours", "my", "your", "us", "them", "one", "some",
	"many", "most", "several", "both", "all", "any", "each", "every", "no", "not", "none", "other",
	"another", "such", "two", "three", "four", "five", "first", "second", "third", "last", "next",
	"new", "former", "late", "later", "earlier", "yesterday", "today", "tonight", "tomorrow", "now",
	"then", "meanwhile", "however", "still", "also", "instead", "even", "only", "just", "already",
	"according", "despite", "although", "though", "because", "unlike", "like", "under", "over",
	"against", "between", "among", "through", "across", "within", "officials", "critics", "analysts",
	"experts", "reporters", "sources", "people", "residents", "ok", "yes", "is", "are", "was", "were",
	"be", "been", "has", "have", "had", "do", "does", "did", "would", "could", "might", "should",
	"must", "said", "says", "say",
	"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday",
	"january", "february", "march", "april", "june", "july", "august", "september", "october",
	"november", "december", "jan", "feb", "mar", "apr", "jun", "jul", "aug", "sep", "sept", "oct",
	"nov", "dec",
)

// titleWords are dropped from the start of a span ("President Joe Biden" is
// Joe Biden) and never match or are proposed alone.
var titleWords = set(
	"mr", "mrs", "ms", "mx", "dr", "prof", "professor", "sir", "dame", "lord", "lady", "president",
	"vice", "senator", "sen", "rep", "representative", "congressman", "congresswoman", "gov",
	"governor", "minister", "prime", "deputy", "chancellor", "king", "queen", "prince", "princess",
	"pope", "judge", "general", "gen", "colonel", "col", "captain", "capt", "lieutenant", "lt",
	"sergeant", "sgt", "secretary", "chairman", "chairwoman", "chair", "ceo", "cfo", "cto", "chief",
	"director", "mayor", "ambassador", "spokesman", "spokeswoman", "spokesperson", "founder",
	"leader", "speaker", "attorney", "commissioner", "bishop", "cardinal", "rabbi", "imam", "sheikh",
	"saint", "st", "jr", "sr",
)

// connectors may sit inside a span ("Bank of England", "Institute for
// the Study of War", "Ludwig van Beethoven") but never start or end one.
var connectors = set("of", "for", "de", "da", "del", "della", "der", "den", "van", "von", "du", "di", "la", "le", "al", "el", "bin", "ibn", "y")

// abbreviations end with a period that does not end a sentence.
var abbreviations = set(
	"mr", "mrs", "ms", "mx", "dr", "prof", "sr", "jr", "st", "gen", "sen", "rep", "gov", "lt", "col",
	"capt", "sgt", "rev", "hon", "no", "vs", "etc", "inc", "corp", "co", "ltd", "dept", "est", "approx",
	"fig", "vol", "mt", "ft", "jan", "feb", "mar", "apr", "jun", "jul", "aug", "sep", "sept", "oct",
	"nov", "dec",
)

// closedAcronyms are all-capital words that are not names.
var closedAcronyms = set("ok", "am", "pm", "ad", "bc", "tv")

// closedWord reports a word that never stands as a name by itself.
func closedWord(key string) bool { return stopwords[key] || titleWords[key] || connectors[key] }

// closed reports a token that never stands as a name by itself. An acronym
// is open even when its letters spell a function word ("US", "IT").
func closed(t Token) bool {
	if t.Acronym {
		return closedAcronyms[t.Key]
	}
	return closedWord(t.Key)
}
