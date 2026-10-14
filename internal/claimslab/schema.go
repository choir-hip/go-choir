package claimslab

// SchemaVersion names the extraction schema the E0 study measured. It is a
// laboratory schema, not a registered object kind: D3 (the new kinds) is an
// open owner decision, and nothing here is written to a store.
const SchemaVersion = "claimslab.extraction.v0"

// QuoteCap is the longest quote a claim may carry (ontology §5.5: the
// shortest grounding span, normally one sentence, capped at 300 characters).
const QuoteCap = 300

// ClaimCap is the most claims kept per capture (ontology §5.1, W0's guard).
const ClaimCap = 12

// Modality is how a claim settles (ontology §6). The values are the
// study's; D8, D12 and D13 may rename or narrow them.
type Modality string

const (
	ModalityAssertion   Modality = "assertion"
	ModalityForecast    Modality = "forecast"
	ModalityCommitment  Modality = "commitment"
	ModalityIntent      Modality = "intent"
	ModalityAttribution Modality = "attribution" // the reporting claim of a relay (§4.1)
	ModalityOpinion     Modality = "opinion"     // opinion, evaluation or question: never scored
)

// Stance of the speaker toward the proposition.
type Stance string

const (
	StanceAffirms Stance = "affirms"
	StanceDenies  Stance = "denies"
	StanceHedges  Stance = "hedges"
)

// Reading is one interpretation of a claim (U2). Weights over a claim's
// readings sum to 1.
type Reading struct {
	Gloss         string  `json:"gloss"`
	Weight        float64 `json:"weight"`
	ContextNeeded string  `json:"context_needed,omitempty"`
}

// Quantity is a measurement inside a proposition.
type Quantity struct {
	Value       string `json:"value"`
	Unit        string `json:"unit,omitempty"`
	Measure     string `json:"measure,omitempty"`
	AsOf        string `json:"as_of,omitempty"`
	Approximate bool   `json:"approximate,omitempty"`
}

// Speaker is the claim's speaker as it appears in the text.
type Speaker struct {
	Surface string `json:"surface"`
	// Type is person, organization, outlet, institution, place, product,
	// anonymous or unknown.
	Type string `json:"type"`
}

// Claim is one attributed assertion anchored to a quote (ontology §4.1),
// with the five uncertainties of §6a. U5 (settlement) lives on settlements,
// never on the claim.
type Claim struct {
	Speaker  Speaker  `json:"speaker"`
	Carrier  string   `json:"carrier,omitempty"` // the outlet when it relays the speaker
	Quote    string   `json:"quote"`
	Block    string   `json:"block,omitempty"` // the model's block hint; the quote check decides
	Stance   Stance   `json:"stance"`
	Modality Modality `json:"modality"`
	// Readings is U2: one to four weighted interpretations.
	Readings []Reading `json:"readings"`
	Quantity *Quantity `json:"quantity,omitempty"`
	Deadline string    `json:"deadline,omitempty"`
	// ExpressedConfidence is U3: an explicit probability ("p=0.7"), an
	// estimative word or hedge ("likely", "may"), or "asserted".
	ExpressedConfidence string `json:"expressed_confidence"`
	// AttributionBasis and PAttr are U1. Basis is verbatim_first_party,
	// direct_quote, paraphrase, relay, anonymous, reportedly or own_voice.
	AttributionBasis string  `json:"attribution_basis"`
	PAttr            float64 `json:"p_attr"`
	// PExtract is U4: the extractor's confidence that span, speaker and
	// modality are right.
	PExtract float64 `json:"p_extract"`
	// UtteredAt is when the speaker said it, often approximate; the
	// capture supplies fetched_at and the outlet published_at.
	UtteredAt string `json:"uttered_at,omitempty"`
	// RelaysIndex, on a reporting claim, names the embedded claim it relays.
	RelaysIndex *int `json:"relays,omitempty"`
}

// Mention is a span that refers to an entity.
type Mention struct {
	Surface string `json:"surface"`
	Type    string `json:"type"`
	// WikidataGuess is the extractor's unverified guess; entity linking is a
	// judgment (§4.3), never the extractor's.
	WikidataGuess string `json:"wikidata_guess,omitempty"`
}

// Event is the extractor's description of the occurrence a capture
// reports, used only by the study's routing diagnostic.
type Event struct {
	Gloss        string   `json:"gloss"`
	When         string   `json:"when,omitempty"`
	Where        string   `json:"where,omitempty"`
	Participants []string `json:"participants,omitempty"`
	Topic        string   `json:"topic,omitempty"`
}

// Extraction is one capture's extraction under one extractor.
type Extraction struct {
	Schema    string    `json:"schema"`
	CaptureID string    `json:"capture_id"`
	Extractor string    `json:"extractor"` // model id plus prompt hash
	Claims    []Claim   `json:"claims"`
	Mentions  []Mention `json:"mentions"`
	Event     *Event    `json:"event,omitempty"`
}
