// Guest-reach decisions of this package, moved here whole from handlers.go so the
// security-epoch gate (nix/guest-security-surfaces.txt) sees a change to them. A
// change to this file changes what a guest computer may reach on the host.
// Keep it to those decisions; put anything else in another file.

package platform

import (
	"net/http"

	"github.com/yusefmosiah/go-choir/internal/hostbuild"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// admitPublishedRead is the admission of the published pieces' reads: a GET by
// a host-internal caller or by any guest vmctl binds to a live computer. It
// writes the refusal and returns false when the caller is not admitted.
func (h *Handler) admitPublishedRead(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return false
	}
	if !trustedInternalCaller(r) {
		if _, err := bindTapCaller(r.Context(), h.guestLookup, r); err != nil {
			writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
			return false
		}
	}
	return true
}

func RegisterRoutes(s *server.Server, h *Handler) {
	s.SetHealthHandler(h.HandleHealth)
	s.HandleFunc("/internal/computers/credentials/exchange", h.HandleComputerCredentialExchange)
	s.HandleFunc("/internal/computers/credentials/consume", h.HandleComputerCredentialConsume)
	s.HandleFunc("/internal/computers/credentials/issue", h.HandleComputerCredentialIssue)
	s.HandleFunc("/internal/computers/credentials/renew", h.HandleComputerCredentialRenew)
	s.HandleFunc("/internal/computers/lifecycle/control", h.HandleComputerLifecycleControl)
	s.HandleFunc("/internal/computers/self-development/mode", h.HandleSelfDevelopmentMode)
	s.HandleFunc("/internal/computers/self-development/consensus", h.HandleSelfDevelopmentConsensus)
	s.HandleFunc("/internal/computers/self-development/package-publish", h.HandlePackagePublishByDecision)
	s.HandleFunc("/internal/computers/events/head", h.HandleComputerEventHead)
	s.HandleFunc("/internal/computers/events/pin", h.HandleComputerEventPin)
	s.HandleFunc("/internal/computers/events/append", h.HandleComputerEventAppend)
	s.HandleFunc("/internal/computers/events/payload", h.HandleComputerEventPayload)
	s.HandleFunc("/internal/computers/events/replay", h.HandleComputerEventReplay)
	s.HandleFunc(tapeFollowRoute, h.HandleTapeFollowBundle)
	s.HandleFunc("/internal/computers/checkpoints", h.HandleComputerCheckpoint)
	s.HandleFunc("/internal/computers/checkpoints/verdict", h.HandleCheckpointVerdict)
	s.HandleFunc("/internal/computers/checkpoints/list", h.HandleCheckpointList)
	s.HandleFunc(verifierKeyPreflightRoute, h.HandleVerifierKeyPreflight)
	s.HandleFunc(verifierKeyRepinRoute, h.HandleVerifierKeyRepin)
	s.HandleFunc("/internal/computers/route-projection-certificates", h.HandleRouteProjectionCertificate)
	s.HandleFunc("/internal/computers/platform-updates/offer", h.HandlePlatformUpdateOfferMint)
	s.HandleFunc("/internal/computers/platform-updates/blob/", h.HandlePlatformUpdateBlob)
	s.HandleFunc(hostbuild.Route, h.HandleHostBuilds)
	s.HandleFunc(hostbuild.Route+"/", h.HandleHostBuilds)
	s.HandleFunc(packagesRoute+"/", h.HandlePackages)
	s.HandleFunc(listingsRoute+"/", h.HandleListings)
	s.HandleFunc(platformOperatorYankRoute, h.HandlePlatformYank)
	s.HandleFunc(PlatformOperatorCheckRoute, h.HandlePlatformOperatorCheck)
	s.HandleFunc("/internal/computers/keys/escrow-public-key", h.HandleKeyEscrowPublicKey)
	s.HandleFunc("/internal/computers/keys/escrow", h.HandleKeyEscrow)
	s.HandleFunc("/internal/computers/keys/escrow/status", h.HandleKeyEscrowStatus)
	s.HandleFunc("/internal/computers/keys/realization-delivery", h.HandleRealizationKeyDelivery)
	s.HandleFunc("/internal/computers/keys/unwrap-requests", h.HandleKeyUnwrapRequests)
	s.HandleFunc("/internal/computers/keys/unwrap-requests/", h.HandleKeyUnwrapRequestAction)
	s.HandleFunc("/internal/computers/keys/transparency-head", h.HandleKeyEscrowTransparencyHead)
	s.HandleFunc("/internal/computers/files/chunks/", h.HandleFileCASChunk)
	s.HandleFunc("/internal/computers/files/root", h.HandleFileCASRoot)
	s.HandleFunc("/internal/computers/files/snapshot-root", h.HandleSnapshotRoot)
	s.HandleFunc("/internal/computers/files/watermark", h.HandleFileCASWatermark)
	s.HandleFunc("/internal/computers/files/roots", h.HandleFileCASRoots)
	s.HandleFunc("/internal/platform/control-key", h.HandlePlatformControlPublicKey)
	s.HandleFunc("/internal/platform/execution-identity/attest", h.HandleExecutionIdentityAttestation)
	s.HandleFunc("/internal/platform/publications/texture", h.HandleInternalPublishTexture)
	s.HandleFunc(types.StoryVerdictsPath, h.HandleSupportVerdicts)
	s.HandleFunc(types.StoryRelationsPath, h.HandleStoryRelations)
	s.HandleFunc(types.StoryEntitiesPath, h.HandleStoryEntities)
	s.HandleFunc(types.StoryEntityCandidatesPath, h.HandleStoryEntityCandidates)
	s.HandleFunc(entityGrowthInternalPath, h.HandleInternalEntityGrowth)
	s.HandleFunc(EntityPageInternalPrefix, h.HandleInternalEntityPage)
	s.HandleFunc("/internal/platform/publications/resolve", h.HandleInternalResolvePublication)
	s.HandleFunc("/internal/platform/publications/export", h.HandleInternalExportPublication)
	s.HandleFunc("/internal/platform/publications/versions/", h.HandleInternalPublicationVersion)
	s.HandleFunc("/internal/platform/publications/front", h.HandleInternalFrontStories)
	s.HandleFunc(PodcastClipsPath, h.HandlePodcastClips)
	s.HandleFunc(PodcastEpisodesPath, h.HandlePodcastEpisodes)
	s.HandleFunc(PodcastTranscriptsPath, h.HandlePodcastTranscripts)
	s.HandleFunc(PodcastClipsWritePath, h.HandlePodcastClipsWrite)
	s.HandleFunc(PodcastFeedsListedPath, h.HandlePodcastFeedsListed)
	s.HandleFunc(PodcastPopularPath, h.HandlePodcastPopular)
	s.HandleFunc(PodcastTranscriptPath, h.HandlePodcastTranscript)
	s.HandleFunc(TakesPath, h.HandleTakes)
	s.HandleFunc(TakesAudioPath, h.HandleTakeAudio)
	s.HandleFunc(TakesDeletePath, h.HandleTakeDelete)
	s.HandleFunc(TakesRecordPath, h.HandleTakeRecord)
	s.HandleFunc(TakesOutcomePath, h.HandleTakeOutcome)
	s.HandleFunc(TakesYankPath, h.HandleTakeYank)
	s.HandleFunc(AccountSuspendPath, h.HandleAccountSuspend)
	s.HandleFunc(AccountPurgePath, h.HandleAccountPurge)
	s.HandleFunc(AccountPurgeTapePath, h.HandleAccountPurgeTape)
	s.HandleFunc("/internal/platform/publications/owned", h.HandleInternalOwnedPublication)
	s.HandleFunc("/internal/platform/publications/expansion-source", h.HandleInternalExpansionSource)
	s.HandleFunc("/internal/platform/publications/related", h.HandleInternalRelatedStories)
	s.HandleFunc("/internal/platform/publications/related-backfill", h.HandleInternalRelatedBackfill)
	s.HandleFunc("/internal/platform/retrieval/search", h.HandleInternalRetrievalSearch)
	s.HandleFunc("/internal/platform/published/search", h.HandleInternalPublishedSearch)
	s.HandleFunc("/internal/platform/published/ask", h.HandleInternalPublishedAsk)
	s.HandleFunc(opedFeedPath, h.HandleInternalOpeds)
	s.HandleFunc(noticeFeedPath, h.HandleInternalNotices)
	s.HandleFunc(opedVerdictsPath, h.HandleInternalOpedVerdicts)
	s.HandleFunc("/internal/platform/proposal-deliveries/state", h.HandleInternalProposalDeliveryState)
	s.HandleFunc("/internal/platform/publications/", h.HandleInternalPublicationProposal)
	s.HandleFunc("/internal/platform/texture/sync", h.HandleInternalSyncTextureDocument)
	s.HandleFunc("/internal/platform/texture/revisions/", h.HandleInternalGetTextureRevision)
	s.HandleFunc("/internal/platform/texture/documents/", h.HandleInternalListTextureRevisions)
	s.HandleFunc("/internal/platform/artifact-gc", h.HandleArtifactGC)
	s.HandleFunc("/internal/platform/artifact-gc/tape", h.HandleTapeSweep)
	s.HandleFunc("/internal/platform/artifact-gc/tape/extra-refs", h.HandleTapeExtraRefs)
	s.HandleFunc("/internal/platform/vectors/import", h.HandleVectorImport)
}
