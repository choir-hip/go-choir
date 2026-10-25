package texturewrite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/quotematch"
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Capture is a captured source as a block document: the capture's document
// id and version (its doc_hash), the page it captured, and its blocks with
// their cids.
type Capture struct {
	Doc     string
	Version string
	URL     string
	Title   string
	Blocks  []quotematch.Block
}

// ErrNotCaptured is what a CaptureSource returns when a handle names no
// capture. Write turns it into a line-located refusal (A2).
var ErrNotCaptured = errors.New("not captured")

// CaptureSource resolves a citation handle (a capture id or a URL) to the
// owner's capture of it. It is the seam to the capture store, which is built
// as block documents in Cut 2 (ii); until it is wired every new citation is
// refused as not captured.
type CaptureSource interface {
	CaptureBlocks(ctx context.Context, ownerID, handleOrURL string) (Capture, error)
}

// Source is one source the frame shows under a record, by handle.
type Source struct {
	Handle    string `json:"handle"`
	URL       string `json:"url,omitempty"`
	CaptureID string `json:"capture_id,omitempty"`
	Title     string `json:"title,omitempty"`
	Excerpt   string `json:"excerpt,omitempty"`
}

// Key is what a CaptureSource resolves the source by.
func (s Source) Key() string {
	if s.CaptureID != "" {
		return s.CaptureID
	}
	return s.URL
}

// DroppedCitation is a citation a Write did not carry: its block changed and
// the quote no longer appears in it. The next frame lists it.
type DroppedCitation struct {
	BlockID string `json:"block_id"`
	Token   string `json:"token"`
	Quote   string `json:"quote,omitempty"`
}

// Input is one Write against a base document.
type Input struct {
	OwnerID string
	DocID   string
	// Seed makes new block ids deterministic (the base revision id), so a
	// replayed cell materializes the same revision.
	Seed     string
	Base     texturedoc.StructuredTextureDoc
	Entities []texturedoc.SourceEntity
	Script   string
	// Handles are the frame's record sources (r2.c1 ...).
	Handles  map[string]Source
	Resolver CaptureSource
}

// Result is the next document.
type Result struct {
	Doc      texturedoc.StructuredTextureDoc
	Entities []texturedoc.SourceEntity
	Dropped  []DroppedCitation
	Ops      int
}

// Apply materializes a Write. Every refusal is an Errors of line-located
// refusals; nothing is partially applied.
func Apply(ctx context.Context, in Input) (Result, error) {
	var errs Errors
	ops := parse(in.Script, &errs)
	base := DocBlocks(in.Base)
	headings := map[string]bool{}
	for _, b := range base {
		if b.Kind == "heading" {
			headings[b.Alias] = true
		}
	}
	checkOps(ops, len(base), headings, &errs)
	if len(errs) > 0 {
		return Result{}, errs.sorted()
	}
	w := &writer{ctx: ctx, in: in, base: base, errs: &errs, entities: map[string]texturedoc.SourceEntity{}, ids: map[string]bool{}}
	for _, e := range in.Entities {
		w.entities[e.SourceEntityID] = e
	}
	w.indexBase()
	content := w.apply(ops)
	if len(errs) > 0 {
		return Result{}, errs.sorted()
	}
	if kept := dropEmbedOnlyBlocks(content); len(kept) > 0 {
		content = kept
	}
	if err := keepEmbedBlocks(base, content, ops[0].Line); err != nil {
		return Result{}, err
	}
	if len(content) == 0 {
		return Result{}, Errors{{Line: ops[0].Line, Msg: "the Write leaves the document empty; a document keeps at least one block"}}
	}
	doc := texturedoc.StructuredTextureDoc{Schema: texturedoc.SchemaV1, Doc: in.Base.Doc}
	doc.Doc.Content = content
	if doc.Doc.Type == "" {
		doc.Doc.Type = "doc"
	}
	if nodeID(doc.Doc) == "" {
		doc.Doc.Attrs = map[string]any{"id": "doc-" + in.DocID}
	}
	if strings.TrimSpace(doc.Schema) == "" {
		doc.Schema = texturedoc.SchemaV1
	}
	return Result{Doc: doc, Entities: w.usedEntities(doc), Dropped: w.dropped, Ops: len(ops)}, nil
}

type writer struct {
	ctx      context.Context
	in       Input
	base     []Block
	errs     *Errors
	entities map[string]texturedoc.SourceEntity
	// existing indexes the base's Write citations by token text, so a token
	// copied from the frame reuses its citation without the capture store.
	existing map[string]texturedoc.Node
	ids      map[string]bool
	seq      int
	dropped  []DroppedCitation
	captures map[string]Capture
}

func (w *writer) indexBase() {
	w.existing = map[string]texturedoc.Node{}
	var walk func(texturedoc.Node)
	walk = func(n texturedoc.Node) {
		if id := nodeID(n); id != "" {
			w.ids[id] = true
		}
		if n.Type == "source_ref" {
			w.existing[tokenKey(tokenOf(n))] = n
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	for _, b := range w.base {
		walk(b.Node)
	}
}

func tokenKey(t Token) string {
	return t.Kind + "\x00" + t.Handle + "\x00" + normalizeText(t.Quote)
}

// apply lays the base blocks out with the ops applied. Targets and anchors
// always name base blocks, so ops do not depend on their order.
func (w *writer) apply(ops []Op) []texturedoc.Node {
	if len(ops) == 1 && ops[0].Kind == OpAll {
		return w.reconcile(w.base, ops[0].Body)
	}
	index := map[string]int{}
	for i, b := range w.base {
		index[b.Alias] = i
	}
	replace := map[string][]texturedoc.Node{}
	gone := map[string]bool{}
	movedAway := map[string]bool{}
	before := map[string][]func() []texturedoc.Node{}
	after := map[string][]func() []texturedoc.Node{}
	var start, end []func() []texturedoc.Node
	claimed := map[string]int{}
	for _, op := range ops {
		for _, t := range op.Targets {
			claimed[t] = op.Line
		}
	}
	place := func(op Op, emit func() []texturedoc.Node) {
		switch op.Place {
		case PlaceStart:
			start = append(start, emit)
		case PlaceEnd:
			end = append(end, emit)
		case PlaceBefore:
			before[op.Anchor] = append(before[op.Anchor], emit)
		case PlaceAfter:
			after[op.Anchor] = append(after[op.Anchor], emit)
		}
	}
	var emitUnit func(alias string) []texturedoc.Node
	for _, op := range ops {
		switch op.Kind {
		case OpReplace:
			first := op.Targets[0]
			span := make([]Block, 0, len(op.Targets))
			for i, t := range op.Targets {
				span = append(span, w.base[index[t]])
				if i > 0 {
					gone[t] = true
				}
			}
			replace[first] = w.reconcile(span, op.Body)
		case OpDelete:
			for _, t := range op.Targets {
				gone[t] = true
			}
		case OpSection:
			head := op.Targets[0]
			i := index[head]
			level := intAttr(w.base[i].Node, "level")
			span := []Block{w.base[i]}
			for j := i + 1; j < len(w.base); j++ {
				b := w.base[j]
				if b.Kind == "heading" && intAttr(b.Node, "level") <= level {
					break
				}
				if line, ok := claimed[b.Alias]; ok && line != op.Line {
					w.errs.add(op.Line, "@section %s includes %s, which the directive on line %d changes; one directive per block", head, b.Alias, line)
				}
				span = append(span, b)
				gone[b.Alias] = true
			}
			replace[head] = w.reconcile(span, op.Body)
		case OpInsert:
			body := op.Body
			place(op, func() []texturedoc.Node { return w.fresh(body) })
		case OpMove:
			alias := op.Targets[0]
			movedAway[alias] = true
			place(op, func() []texturedoc.Node { return emitUnit(alias) })
		}
	}
	emitted := map[string]bool{}
	emitUnit = func(alias string) []texturedoc.Node {
		if emitted[alias] {
			return nil
		}
		emitted[alias] = true
		var out []texturedoc.Node
		for _, f := range before[alias] {
			out = append(out, f()...)
		}
		switch {
		case replace[alias] != nil:
			out = append(out, replace[alias]...)
		case !gone[alias]:
			out = append(out, w.base[index[alias]].Node)
		}
		for _, f := range after[alias] {
			out = append(out, f()...)
		}
		return out
	}
	var out []texturedoc.Node
	for _, f := range start {
		out = append(out, f()...)
	}
	for _, b := range w.base {
		if movedAway[b.Alias] {
			continue
		}
		out = append(out, emitUnit(b.Alias)...)
	}
	for _, f := range end {
		out = append(out, f()...)
	}
	return out
}

// reconcile lays new text over old blocks. A new block whose text equals an
// old block's (whitespace aside) is that old block, kept exactly. Between
// those anchors, a new block takes the id of the next unmatched old block of
// its kind, and that old block's citations carry onto it only while their
// quotes still appear in its text; the rest are dropped and listed. Other
// new blocks are new.
func (w *writer) reconcile(old []Block, body []BodyBlock) []texturedoc.Node {
	newText := make([]string, len(body))
	for i, b := range body {
		newText[i] = normalizeText(bodyText(b))
	}
	oldText := make([]string, len(old))
	for i, b := range old {
		oldText[i] = normalizeText(b.Text)
	}
	pairs := lcs(oldText, newText)
	var out []texturedoc.Node
	oi, ni := 0, 0
	gap := func(oEnd, nEnd int) {
		unmatched := append([]Block(nil), old[oi:oEnd]...)
		for ; ni < nEnd; ni++ {
			var paired *Block
			for k := range unmatched {
				if unmatched[k].Kind == body[ni].Kind && unmatched[k].ID != "" {
					b := unmatched[k]
					paired = &b
					unmatched = append(unmatched[:k], unmatched[k+1:]...)
					break
				}
			}
			if paired == nil {
				out = append(out, w.node(body[ni], ""))
				continue
			}
			node := w.node(body[ni], paired.ID)
			out = append(out, w.carry(*paired, node))
		}
		oi = oEnd
	}
	for _, p := range pairs {
		gap(p[0], p[1])
		out = append(out, old[p[0]].Node)
		oi, ni = p[0]+1, p[1]+1
	}
	gap(len(old), len(body))
	return out
}

// lcs aligns equal texts in order and returns (old, new) index pairs.
func lcs(a, b []string) [][2]int {
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else {
				dp[i][j] = max(dp[i+1][j], dp[i][j+1])
			}
		}
	}
	var out [][2]int
	for i, j := 0, 0; i < n && j < m; {
		switch {
		case a[i] == b[j]:
			out = append(out, [2]int{i, j})
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			i++
		default:
			j++
		}
	}
	return out
}

// carry adds old's citations that the new text dropped but whose quote it
// still says; the others are listed as dropped.
func (w *writer) carry(old Block, node texturedoc.Node) texturedoc.Node {
	present := map[string]bool{}
	for _, r := range sourceRefs(node) {
		present[tokenKey(tokenOf(r))] = true
	}
	for _, r := range sourceRefs(old.Node) {
		tok := tokenOf(r)
		if present[tokenKey(tok)] {
			continue
		}
		ref, ok := RefOf(r)
		if ok && ref.Quote != "" && attachWhereQuoted(&node, r, ref.Quote) {
			present[tokenKey(tok)] = true
			continue
		}
		w.dropped = append(w.dropped, DroppedCitation{BlockID: old.ID, Token: TokenText(tok), Quote: tok.Quote})
	}
	return node
}

// attachWhereQuoted appends ref to the innermost text block of node whose
// text still holds quote.
func attachWhereQuoted(node *texturedoc.Node, ref texturedoc.Node, quote string) bool {
	for i := range node.Content {
		if isBlockType(node.Content[i].Type) && attachWhereQuoted(&node.Content[i], ref, quote) {
			return true
		}
	}
	switch node.Type {
	case "paragraph", "heading":
		if quotematch.Match(quote, []quotematch.Block{{ID: "x", Text: plainText(*node)}}).Outcome == quotematch.Matched {
			node.Content = append(node.Content, ref)
			return true
		}
	}
	return false
}

// fresh builds new blocks with new ids.
func (w *writer) fresh(body []BodyBlock) []texturedoc.Node {
	out := make([]texturedoc.Node, 0, len(body))
	for _, b := range body {
		out = append(out, w.node(b, ""))
	}
	return out
}

func bodyText(b BodyBlock) string {
	switch b.Kind {
	case "heading":
		return strings.Repeat("#", b.Level) + " " + joinLines(b.Lines)
	case "bullet_list", "ordered_list":
		lines := make([]string, len(b.Lines))
		for i, l := range b.Lines {
			marker := "- "
			if b.Kind == "ordered_list" {
				marker = strconv.Itoa(b.Start+i) + ". "
			}
			lines[i] = marker + strings.TrimSpace(l.Text)
		}
		return strings.Join(lines, "\n")
	case "blockquote":
		return "> " + joinLines(b.Lines)
	case "code_block":
		raw := make([]string, len(b.Lines))
		for i, l := range b.Lines {
			raw[i] = l.Text
		}
		return "```" + b.Lang + "\n" + strings.Join(raw, "\n") + "\n```"
	case "horizontal_rule":
		return "---"
	}
	return joinLines(b.Lines)
}

func joinLines(lines []Line) string {
	parts := make([]string, len(lines))
	for i, l := range lines {
		parts[i] = strings.TrimSpace(l.Text)
	}
	return strings.Join(parts, " ")
}

// newID mints a deterministic, unused node id.
func (w *writer) newID(prefix, preferred string) string {
	if preferred != "" {
		w.ids[preferred] = true
		return preferred
	}
	for {
		w.seq++
		sum := sha256.Sum256([]byte(w.in.Seed + "\x00" + w.in.DocID + "\x00" + strconv.Itoa(w.seq)))
		id := prefix + "-" + hex.EncodeToString(sum[:6])
		if !w.ids[id] {
			w.ids[id] = true
			return id
		}
	}
}

// node builds one block; id, when set, is the paired old block's id.
func (w *writer) node(b BodyBlock, id string) texturedoc.Node {
	switch b.Kind {
	case "heading":
		return texturedoc.Node{Type: "heading", Attrs: map[string]any{"id": w.newID("h", id), "level": b.Level}, Content: w.inline(b.Lines)}
	case "bullet_list", "ordered_list":
		attrs := map[string]any{"id": w.newID("list", id)}
		if b.Kind == "ordered_list" {
			attrs["start"] = b.Start
		}
		items := make([]texturedoc.Node, 0, len(b.Lines))
		for _, l := range b.Lines {
			items = append(items, texturedoc.Node{Type: "list_item", Attrs: map[string]any{"id": w.newID("li", "")}, Content: []texturedoc.Node{
				{Type: "paragraph", Attrs: map[string]any{"id": w.newID("p", "")}, Content: w.inline([]Line{l})},
			}})
		}
		return texturedoc.Node{Type: b.Kind, Attrs: attrs, Content: items}
	case "blockquote":
		return texturedoc.Node{Type: "blockquote", Attrs: map[string]any{"id": w.newID("quote", id)}, Content: []texturedoc.Node{
			{Type: "paragraph", Attrs: map[string]any{"id": w.newID("p", "")}, Content: w.inline(b.Lines)},
		}}
	case "code_block":
		attrs := map[string]any{"id": w.newID("code", id)}
		if b.Lang != "" {
			attrs["language"] = b.Lang
		}
		var content []texturedoc.Node
		raw := make([]string, len(b.Lines))
		for i, l := range b.Lines {
			raw[i] = l.Text
		}
		if text := strings.Join(raw, "\n"); text != "" {
			content = []texturedoc.Node{{Type: "text", Text: text}}
		}
		return texturedoc.Node{Type: "code_block", Attrs: attrs, Content: content}
	case "horizontal_rule":
		return texturedoc.Node{Type: "horizontal_rule", Attrs: map[string]any{"id": w.newID("hr", id)}}
	}
	return texturedoc.Node{Type: "paragraph", Attrs: map[string]any{"id": w.newID("p", id)}, Content: w.inline(b.Lines)}
}

// inline builds a text run from lines joined by spaces, resolving tokens.
// Emphasis may span the lines of one block; a token may not.
func (w *writer) inline(lines []Line) []texturedoc.Node {
	var out []texturedoc.Node
	appendText := func(s string, marks []string) {
		if s == "" {
			return
		}
		if k := len(out) - 1; k >= 0 && out[k].Type == "text" && sameMarks(nodeMarks(out[k]), marks) {
			out[k].Text += s
			return
		}
		n := texturedoc.Node{Type: "text", Text: s}
		for _, m := range marks {
			n.Marks = append(n.Marks, texturedoc.Mark{Type: m})
		}
		out = append(out, n)
	}
	var pieces []piece
	for _, l := range lines {
		text := strings.TrimSpace(l.Text)
		if text == "" {
			continue
		}
		ps, err := scanInline(text, l.N)
		if err != nil {
			*w.errs = append(*w.errs, err)
			continue
		}
		for _, p := range ps {
			if p.token == nil && !p.code && legacyMarkerRE.MatchString(p.text) {
				w.errs.add(l.N, "numeric markers such as [12] and source: links are not citations; cite with {{cite <handle> \"quote\"}}")
				break
			}
		}
		if len(pieces) > 0 {
			pieces = append(pieces, piece{text: " ", pair: -1})
		}
		pieces = append(pieces, ps...)
	}
	for _, s := range resolveInline(pieces) {
		if s.Token == nil {
			appendText(s.Text, s.Marks)
			continue
		}
		if ref, ok := w.citation(*s.Token); ok {
			out = append(out, ref)
		}
	}
	// A citation is an atom after the text it supports; trim the space a
	// token at the end of a line leaves.
	if k := len(out) - 1; k >= 0 && out[k].Type == "text" {
		out[k].Text = strings.TrimRight(out[k].Text, " ")
		if out[k].Text == "" {
			out = out[:k]
		}
	}
	return out
}

// captureKeyRE is the unique tail of a capture id: its content key.
var captureKeyRE = regexp.MustCompile(`:key-[0-9a-f]{64}$`)

// frameSourceByCaptureKey finds the one frame source a mistyped capture id
// means. A capture id is about 140 characters: an owner segment the same
// for every capture in the computer, then key-<sha256>. A model copying it
// can drop a chunk of the owner segment (T5 in
// docs/problems/tool-call-failures-dual-model-2026-10-21.md). The match is
// the whole key suffix, and every matching frame source must name the
// same capture; anything else is no match.
func (w *writer) frameSourceByCaptureKey(id string) (Source, bool) {
	suffix := captureKeyRE.FindString(id)
	if suffix == "" {
		return Source{}, false
	}
	var found Source
	for _, src := range w.in.Handles {
		if src.CaptureID == "" || !strings.HasSuffix(src.CaptureID, suffix) {
			continue
		}
		if found.CaptureID != "" && found.CaptureID != src.CaptureID {
			return Source{}, false
		}
		found = src
	}
	return found, found.CaptureID != ""
}

// frameHandleList names the frame's citable handles for a refusal.
func (w *writer) frameHandleList() string {
	handles := make([]string, 0, len(w.in.Handles))
	for h := range w.in.Handles {
		handles = append(handles, h)
	}
	if len(handles) == 0 {
		return "your frame lists none"
	}
	sort.Strings(handles)
	if len(handles) > 6 {
		return strings.Join(handles[:6], ", ") + fmt.Sprintf(" and %d more", len(handles)-6)
	}
	return strings.Join(handles, ", ")
}

// citation resolves one token to a source_ref node.
func (w *writer) citation(t Token) (texturedoc.Node, bool) {
	if prior, ok := w.existing[tokenKey(t)]; ok {
		n := prior
		n.Attrs = copyAttrs(prior.Attrs)
		n.Attrs["id"] = w.newID("ref", "")
		return n, true
	}
	if t.Kind == TokenRef {
		if _, ok := w.entities[t.Handle]; !ok {
			w.errs.add(t.Line, "{{ref %s}} names no source of this document; {{ref}} only keeps a citation the frame shows, by its source id (%s). To cite a source from your records, write {{cite rN.cM \"exact quote\"}} with its frame handle", t.Handle, w.entityIDList())
			return texturedoc.Node{}, false
		}
		return texturedoc.Node{Type: "source_ref", Attrs: map[string]any{"id": w.newID("ref", ""), "source_entity_id": t.Handle, "display_mode": "numbered_ref"}}, true
	}
	key, label := t.Handle, t.Handle
	if src, ok := w.in.Handles[t.Handle]; ok {
		key = src.Key()
		label = t.Handle + " (" + key + ")"
	} else if frameHandleRE.MatchString(t.Handle) {
		w.errs.add(t.Line, "%s is not a source in your frame; frame sources are listed under the records as rN.cM", t.Handle)
		return texturedoc.Node{}, false
	}
	capture, err := w.capture(key)
	if errors.Is(err, ErrNotCaptured) {
		if src, ok := w.frameSourceByCaptureKey(key); ok {
			key = src.Key()
			capture, err = w.capture(key)
		}
	}
	if err != nil {
		if errors.Is(err, ErrNotCaptured) {
			w.errs.add(t.Line, "%s is not captured: a citation needs a captured copy of the source to check its quote against. Cite a frame source by its handle (%s), or ask research to Read it", label, w.frameHandleList())
		} else {
			w.errs.add(t.Line, "%s could not be read from the capture store: %v", label, err)
		}
		return texturedoc.Node{}, false
	}
	ref := types.Ref{Doc: capture.Doc, Version: capture.Version}
	tier := ""
	if strings.TrimSpace(t.Quote) != "" {
		match := quotematch.Match(t.Quote, capture.Blocks)
		if match.Outcome != quotematch.Matched {
			w.errs.add(t.Line, "%s", quoteRefusal(t, label, capture))
			return texturedoc.Node{}, false
		}
		ref.CID = match.BlockID
		ref.Quote = match.Excerpt
		ref.Hash = blockHash(capture.Blocks, match.BlockID)
		ref.Span = RefSpanOf(match, blockText(capture.Blocks, match.BlockID))
		tier = string(match.Tier)
	} else {
		w.errs.add(t.Line, "{{cite %s}} needs the quoted source words", t.Handle)
		return texturedoc.Node{}, false
	}
	entityID := w.entityFor(capture)
	attrs := map[string]any{"id": w.newID("ref", ""), "source_entity_id": entityID, "display_mode": "numbered_ref", refAttr: refAttrValue(ref)}
	if tier != "" {
		attrs[tierAttr] = tier
	}
	return texturedoc.Node{Type: "source_ref", Attrs: attrs}, true
}

// quoteRefusal explains a quote that matched no block: a cross-block
// ellipsis quote (A4) is told to split.
func quoteRefusal(t Token, label string, c Capture) string {
	segments := ellipsisRE.Split(t.Quote, -1)
	kept := 0
	inSomeBlock := true
	for _, seg := range segments {
		if strings.TrimSpace(seg) == "" {
			continue
		}
		kept++
		if quotematch.Match(seg, c.Blocks).Outcome != quotematch.Matched {
			inSomeBlock = false
		}
	}
	if kept > 1 && inSomeBlock {
		return fmt.Sprintf("the quote in {{%s %s}} joins words from different blocks of the source with an ellipsis; a citation quotes one block, so split it into two cites, one per passage", t.Kind, t.Handle)
	}
	return fmt.Sprintf("quote %q was not found in the capture of %s; quote the source's own words from one passage", t.Quote, label)
}

func (w *writer) capture(key string) (Capture, error) {
	if c, ok := w.captures[key]; ok {
		return c, nil
	}
	if w.in.Resolver == nil {
		return Capture{}, ErrNotCaptured
	}
	ctx := w.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	c, err := w.in.Resolver.CaptureBlocks(ctx, w.in.OwnerID, key)
	if err != nil {
		return Capture{}, err
	}
	if c.Doc == "" {
		c.Doc = key
	}
	if w.captures == nil {
		w.captures = map[string]Capture{}
	}
	w.captures[key] = c
	return c, nil
}

// entityFor is the document's source entity for a capture: one per
// captured version, shared by every citation of it.
func (w *writer) entityFor(c Capture) string {
	sum := sha256.Sum256([]byte(c.Doc + "\x00" + c.Version))
	id := "src_" + hex.EncodeToString(sum[:8])
	if _, ok := w.entities[id]; ok {
		return id
	}
	kind := "content_item"
	if c.URL != "" {
		kind = "web_url"
	}
	w.entities[id] = texturedoc.SourceEntity{
		SourceEntityID: id,
		Target: texturedoc.SourceTarget{Kind: kind, URI: c.URL, ID: c.Doc, Metadata: map[string]any{
			"capture_doc": c.Doc, "capture_version": c.Version,
		}},
		Selectors:  []texturedoc.SourceSelector{{Kind: sourcecontract.SelectorKindWholeResource}},
		Display:    texturedoc.SourceDisplay{Mode: "numbered_ref", Title: c.Title},
		Evidence:   texturedoc.SourceEvidence{State: sourcecontract.EvidenceStateAvailable, OpenSurface: sourcecontract.OpenSurfaceSource},
		Provenance: texturedoc.SourceEntityProvenance{CreatedBy: "texture_write"},
	}
	return id
}

// usedEntities keeps the source entities the document cites, in a stable
// order: base order first, then new ones by id.
func (w *writer) usedEntities(doc texturedoc.StructuredTextureDoc) []texturedoc.SourceEntity {
	used := map[string]bool{}
	for _, r := range sourceRefs(doc.Doc) {
		used[stringAttr(r, "source_entity_id")] = true
	}
	var out []texturedoc.SourceEntity
	seen := map[string]bool{}
	for _, e := range w.in.Entities {
		if used[e.SourceEntityID] && !seen[e.SourceEntityID] {
			out = append(out, e)
			seen[e.SourceEntityID] = true
		}
	}
	var fresh []string
	for id := range w.entities {
		if used[id] && !seen[id] {
			fresh = append(fresh, id)
		}
	}
	sort.Strings(fresh)
	for _, id := range fresh {
		out = append(out, w.entities[id])
	}
	return out
}

func blockHash(blocks []quotematch.Block, cid string) string {
	for _, b := range blocks {
		if b.ID == cid {
			return TextHash(b.Text)
		}
	}
	return ""
}

func blockText(blocks []quotematch.Block, cid string) string {
	for _, b := range blocks {
		if b.ID == cid {
			return b.Text
		}
	}
	return ""
}

// RefSpanOf is the span a matched quote covers in its block's original
// text (H-OG3): quotematch's byte offsets converted to code points, one
// range per ellipsis segment when there is more than one, and the hash of
// the covered text. It returns nil for a result that did not match or
// whose offsets do not fall on character boundaries of text, so a Ref
// never carries a span that points between the bytes of a character.
func RefSpanOf(match quotematch.Result, text string) *types.RefSpan {
	if match.Outcome != quotematch.Matched || len(match.Spans) == 0 {
		return nil
	}
	codePoint := func(b int) (int, bool) {
		if b < 0 || b > len(text) || (b < len(text) && !utf8.RuneStart(text[b])) {
			return 0, false
		}
		return utf8.RuneCountInString(text[:b]), true
	}
	start, ok1 := codePoint(match.Start)
	end, ok2 := codePoint(match.End)
	if !ok1 || !ok2 || start >= end {
		return nil
	}
	span := &types.RefSpan{Start: start, End: end, TextHash: TextHash(text[match.Start:match.End])}
	if len(match.Spans) > 1 {
		for _, s := range match.Spans {
			a, okA := codePoint(s.Start)
			b, okB := codePoint(s.End)
			if !okA || !okB || a >= b {
				return nil
			}
			span.Segments = append(span.Segments, types.RefRange{Start: a, End: b})
		}
	}
	return span
}

func refAttrValue(ref types.Ref) map[string]any {
	attrs := map[string]any{"doc": ref.Doc, "version": ref.Version, "cid": ref.CID, "hash": ref.Hash, "quote": ref.Quote}
	if ref.Span != nil {
		// Stored in its decoded JSON form, as the attrs read back after a
		// revision round-trips.
		var span map[string]any
		if raw, err := json.Marshal(ref.Span); err == nil && json.Unmarshal(raw, &span) == nil {
			attrs["span"] = span
		}
	}
	return attrs
}

func copyAttrs(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// dropEmbedOnlyBlocks drops every stored embed of a whole capture (an
// expanded_ref with no quote, from before {{embed}} was retired) from the
// document, and each paragraph left with no words. Such an embed grounds no
// sentence: it drew as a lone marker under the headline and counted its
// outlet toward the publish gate
// (docs/problems/articles-relay-outlets-instead-of-grounding-them-2026-10-19.md).
// A quoted stored embed cites a passage and stays.
func dropEmbedOnlyBlocks(blocks []texturedoc.Node) []texturedoc.Node {
	out := make([]texturedoc.Node, 0, len(blocks))
	for _, b := range blocks {
		before := len(sourceRefs(b))
		b.Content = dropWholeCaptureEmbeds(b.Content)
		if b.Type == "paragraph" && len(sourceRefs(b)) < before && strings.TrimSpace(plainText(b)) == "" && len(sourceRefs(b)) == 0 {
			continue
		}
		out = append(out, b)
	}
	return out
}

// dropWholeCaptureEmbeds removes the whole-capture embeds among nodes,
// at any depth.
func dropWholeCaptureEmbeds(nodes []texturedoc.Node) []texturedoc.Node {
	if len(nodes) == 0 {
		return nodes
	}
	out := make([]texturedoc.Node, 0, len(nodes))
	for _, n := range nodes {
		if n.Type == "source_ref" && stringAttr(n, "display_mode") == "expanded_ref" {
			if ref, ok := RefOf(n); ok && strings.TrimSpace(ref.Quote) == "" {
				continue
			}
		}
		n.Content = dropWholeCaptureEmbeds(n.Content)
		out = append(out, n)
	}
	return out
}

// entityIDList names this document's source ids for a refusal, a few at
// most, sorted.
func (w *writer) entityIDList() string {
	ids := make([]string, 0, len(w.entities))
	for id := range w.entities {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return "this document has none yet"
	}
	sort.Strings(ids)
	more := ""
	if len(ids) > 6 {
		more = fmt.Sprintf(" and %d more", len(ids)-6)
		ids = ids[:6]
	}
	return strings.Join(ids, ", ") + more
}

// keepEmbedBlocks refuses a Write that removes or changes an embed block of
// the base. Embed blocks are evidence that Texture's evidence path wrote from
// a frozen candidate's bundle (docs/design/texture-evidence-embeds-te-
// 2026-10-25.md); a desk's Write writes around them.
func keepEmbedBlocks(base []Block, content []texturedoc.Node, line int) error {
	after := map[string]string{}
	for _, n := range content {
		if n.Type == "embed" {
			raw, _ := json.Marshal(n)
			after[nodeID(n)] = string(raw)
		}
	}
	for _, b := range base {
		if b.Kind != "embed" {
			continue
		}
		raw, _ := json.Marshal(b.Node)
		if after[b.ID] != string(raw) {
			return Errors{{Line: line, Msg: fmt.Sprintf("the Write removes or changes evidence embed %s (%s); an evidence embed stays as written: leave it out of @replace, @delete and @section, and copy its line unchanged under @all", b.Alias, b.Text)}}
		}
	}
	return nil
}
