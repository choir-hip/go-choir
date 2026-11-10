package promptroute

import "strings"

// The input class (lane D, handoff section 4.D): the prompt bar is the
// universal input, and Jev classifies the owner's text as an edit, a comment
// or a request to the app agent. The class is asked at the router's call site
// (prompt_router), as a second typed choice question of the same shape as the
// route question, and decided by the same rule (Decide, Theta). The text is
// host evidence; the question's wording is not.

// The classes. They double as Candidate desks so Decide serves both questions.
const (
	// ClassEdit: an instruction to change the document in view, on a
	// document the owner owns. It goes to the one revise call.
	ClassEdit = "edit"
	// ClassComment: a remark about the whole document, from anyone. On a
	// document the owner owns it goes to the one revise call; on any other
	// it makes a fork the owner owns.
	ClassComment = "comment"
	// ClassRequest: something for the app agent, not about this document
	// (ask for a story, add a source).
	ClassRequest = "request"
)

// ClassQuestionID is the id of the class question.
const ClassQuestionID = "input_class"

// ClassQuestion is the typed choice question.
const ClassQuestion = "Which kind of input is the owner's text? Choose the class whose description fits. The default class takes anything no other class clearly fits."

// DocState is what the prompt bar is aimed at, as the shell states it. The
// revise call and the fork route enforce ownership themselves; this only
// chooses which classes are offered.
type DocState string

const (
	// DocOwned: a document the owner owns is in view.
	DocOwned DocState = "owned"
	// DocForeign: a published version the owner does not own is in view.
	DocForeign DocState = "foreign"
	// DocNone: nothing the text could be about is in view.
	DocNone DocState = "none"
)

// ValidDocState reports whether s is one of the states.
func ValidDocState(s DocState) bool { return s == DocOwned || s == DocForeign || s == DocNone }

// ClassCandidates is the choice set for a state, the default first: edit on
// an owned document, comment on a foreign one (edit is not offered there),
// request with no document.
func ClassCandidates(state DocState) []Candidate {
	switch state {
	case DocOwned:
		return []Candidate{{Desk: ClassEdit, App: ClassEdit, Default: true}, {Desk: ClassComment, App: ClassComment}, {Desk: ClassRequest, App: ClassRequest}}
	case DocForeign:
		return []Candidate{{Desk: ClassComment, App: ClassComment, Default: true}, {Desk: ClassRequest, App: ClassRequest}}
	default:
		return []Candidate{{Desk: ClassRequest, App: ClassRequest, Default: true}}
	}
}

var classDescriptions = map[string]string{
	ClassEdit:    "an instruction to change the document in view: rewrite, shorten, fix, add or remove something in it",
	ClassComment: "a remark, objection, correction or question about the whole document in view, to be answered in a revision",
	ClassRequest: "a request to the app agent that is not about the document in view: ask for a story, add a source, find or open something",
}

// ClassDestinations is the evidence text describing each class, composed by
// the host (never from the owner's text).
func ClassDestinations(candidates []Candidate) string {
	var b strings.Builder
	b.WriteString("The classes:")
	for _, c := range candidates {
		b.WriteString("\n- ")
		b.WriteString(c.Desk)
		b.WriteString(": ")
		b.WriteString(classDescriptions[c.Desk])
	}
	return b.String()
}
