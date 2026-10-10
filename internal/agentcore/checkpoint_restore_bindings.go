package agentcore

import (
	"context"
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

func (rt *Runtime) checkpointRestoreBindings(ctx context.Context, computerID, releaseDigest string, files []updater.ManifestFile) (selfdevprotocol.VMLocalContentWitness, selfdevprotocol.FrontendIdentity, error) {
	report, err := rt.ReplayCompleteness(ctx, computerID)
	if err != nil {
		return selfdevprotocol.VMLocalContentWitness{}, selfdevprotocol.FrontendIdentity{}, fmt.Errorf("self-development checkpoint: replay completeness: %w", err)
	}
	if err := report.Eligibility.Error(); err != nil {
		if og := report.ObjectGraph; og != nil {
			return selfdevprotocol.VMLocalContentWitness{}, selfdevprotocol.FrontendIdentity{}, fmt.Errorf("self-development checkpoint: %w (og_objects live_only=%d replay_only=%d changed=%d by_kind=%v)", err, og.LiveOnlyCount, og.ReplayOnlyCount, og.ChangedCount, og.ByObjectKind)
		}
		return selfdevprotocol.VMLocalContentWitness{}, selfdevprotocol.FrontendIdentity{}, fmt.Errorf("self-development checkpoint: %w", err)
	}
	witness, err := selfdevprotocol.WitnessFromObservationSets(report.Live, report.Replay, report.Result)
	if err != nil {
		return selfdevprotocol.VMLocalContentWitness{}, selfdevprotocol.FrontendIdentity{}, err
	}
	if err := rt.pinCheckpointBase(ctx, computerID, report); err != nil {
		return selfdevprotocol.VMLocalContentWitness{}, selfdevprotocol.FrontendIdentity{}, fmt.Errorf("self-development checkpoint: pin replay base: %w", err)
	}
	releaseFiles := make([]selfdevprotocol.ReleaseFile, 0, len(files))
	for _, file := range files {
		releaseFiles = append(releaseFiles, selfdevprotocol.ReleaseFile{Path: file.Path, SHA256: file.SHA256})
	}
	frontend, err := selfdevprotocol.FrontendIdentityFromReleaseFiles(releaseDigest, releaseFiles)
	if err != nil {
		return selfdevprotocol.VMLocalContentWitness{}, selfdevprotocol.FrontendIdentity{}, err
	}
	return witness, frontend, nil
}
