package agentcore

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/imageblob"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Station TE (docs/design/texture-evidence-embeds-te-2026-10-25.md): a
// fresh Freeze that observed visual captures asks the Texture owner to embed
// them in the operation's trajectory document. Texture's package writes the
// embed blocks from the frozen bundle (CandidateVisualEvidence); this side
// only says which operation froze, from the runtime's own observation, and
// only when the computer setting texture.evidence_embeds is on.

// TextureEvidenceEmbedder embeds a frozen candidate's captures in its
// trajectory's Texture document. autoputer binds the Texture owner handler;
// the desk carrier never imports the owner package, so this is the seam (the
// same shape as TextureCellAuthorizer). computerID scopes the Texture
// document; selfdevComputerID scopes the self-development operation (the
// same computer, named by the self-development surface). It returns a JSON
// receipt.
type TextureEvidenceEmbedder interface {
	EmbedCandidateEvidence(ctx context.Context, ownerID, computerID, selfdevComputerID, operationID string) (string, error)
}

// evidenceEmbedding holds the bound embedder and the jobs started so far.
type evidenceEmbedding struct {
	mu       sync.Mutex
	embedder TextureEvidenceEmbedder
	jobs     sync.WaitGroup
}

// evidenceEmbedTimeout bounds one embed job (a listing read and one Texture
// commit with its retries).
const evidenceEmbedTimeout = 60 * time.Second

// SetTextureEvidenceEmbedder binds the Texture owner's evidence-embed path.
func (rt *Runtime) SetTextureEvidenceEmbedder(e TextureEvidenceEmbedder) {
	if rt == nil {
		return
	}
	rt.evidenceEmbeds.mu.Lock()
	defer rt.evidenceEmbeds.mu.Unlock()
	rt.evidenceEmbeds.embedder = e
}

// WaitEvidenceEmbeds waits for the evidence-embed jobs started so far.
func (rt *Runtime) WaitEvidenceEmbeds() {
	if rt != nil {
		rt.evidenceEmbeds.jobs.Wait()
	}
}

// AssetStore is this computer's content-addressed asset store, where a
// Texture revision pins the digests it embeds.
func (rt *Runtime) AssetStore() *imageblob.Store {
	if rt == nil {
		return nil
	}
	return rt.imageStore()
}

// requestCandidateEvidenceEmbeds asks the Texture owner, off the reduction
// path, to embed operationID's captures. The switch is read here: anything
// but a stored true (no setting, a read error) is off. A failure is logged;
// the Freeze never fails for it, and nothing retries it (no boot resumer:
// a crash between the Freeze and the write leaves the listing as the source,
// design E11).
func (rt *Runtime) requestCandidateEvidenceEmbeds(ctx context.Context, ownerID, computerID, selfdevComputerID, operationID string) {
	if rt == nil {
		return
	}
	ownerID, computerID, operationID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(operationID)
	selfdevComputerID = firstNonEmpty(strings.TrimSpace(selfdevComputerID), strings.TrimSpace(rt.selfdevComputerID), computerID)
	if ownerID == "" || computerID == "" || operationID == "" {
		return
	}
	if !rt.computerSettingOn(ctx, ownerID, computerID, store.SettingTextureEvidenceEmbeds) {
		return
	}
	rt.evidenceEmbeds.mu.Lock()
	embedder := rt.evidenceEmbeds.embedder
	rt.evidenceEmbeds.mu.Unlock()
	if embedder == nil {
		log.Printf("texture evidence: %s froze with captures, but no Texture owner is bound to embed them", operationID)
		return
	}
	rt.evidenceEmbeds.jobs.Add(1)
	go func() {
		defer rt.evidenceEmbeds.jobs.Done()
		jobCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), evidenceEmbedTimeout)
		defer cancel()
		receipt, err := embedder.EmbedCandidateEvidence(jobCtx, ownerID, computerID, selfdevComputerID, operationID)
		if err != nil {
			log.Printf("texture evidence: embed %s's captures: %v", operationID, err)
			return
		}
		log.Printf("texture evidence: %s: %s", operationID, receipt)
	}()
}
