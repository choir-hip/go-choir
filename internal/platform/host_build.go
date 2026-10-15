package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/hostbuild"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// ConfigureHostBuild wires the host build service (P5) to this handler's
// platform-control signer and platform-update blob store, and serves its
// routes. Call after ConfigureComputerEvents. The caller starts the worker.
func (h *Handler) ConfigureHostBuild(cfg hostbuild.Config) (*hostbuild.Service, error) {
	if h == nil || h.service == nil || h.checkpointAuthority == nil || h.checkpointAuthority.cas == nil {
		return nil, fmt.Errorf("host build: computer event authority must be configured first")
	}
	cas := h.checkpointAuthority.cas
	cfg.Sign = func(computerID, jobID, resultDigest string, at time.Time) (selfdevprotocol.AuthorityReceipt, error) {
		return selfdevprotocol.NewAuthorityReceipt(selfdevprotocol.ReceiptKindHostBuild, computerID, jobID, resultDigest, cas.issuer, cas.signingKey, at)
	}
	cfg.Store = h.storePlatformUpdateBlobFile
	svc, err := hostbuild.New(cfg)
	if err != nil {
		return nil, err
	}
	h.hostBuild = svc
	return svc, nil
}

// storePlatformUpdateBlobFile streams a file into
// sha256/platform-update/<digest>, verifying the digest as it copies. An
// existing blob is kept (content addressed).
func (h *Handler) storePlatformUpdateBlobFile(digest, source string) error {
	target, err := h.platformUpdateBlobPath(digest)
	if err != nil {
		return err
	}
	if info, statErr := os.Stat(target); statErr == nil && info.Mode().IsRegular() {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := target + ".tmp-" + fmt.Sprintf("%d", time.Now().UnixNano())
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	if _, err := io.Copy(out, io.TeeReader(in, hasher)); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if hex.EncodeToString(hasher.Sum(nil)) != digest {
		os.Remove(tmp)
		return fmt.Errorf("blob digest mismatch")
	}
	return os.Rename(tmp, target)
}

// HandleHostBuilds serves
//
//	POST /internal/computers/selfdev-builds        submit (event:append)
//	GET  /internal/computers/selfdev-builds/<job>   read   (event:read)
//
// for the computer the capability binds. An unconfigured host builder
// answers 503 with the reason code host_builder_unconfigured, which the
// guest treats as a terminal build failure.
func (h *Handler) HandleHostBuilds(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, hostbuild.Route), "/")
	switch {
	case r.Method == http.MethodPost && rest == "":
		var request hostbuild.Request
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid host build request"})
			return
		}
		if !h.authorizeFileCAS(r, request.ComputerID, "event:append") {
			writeJSON(w, http.StatusForbidden, apiError{Error: "computer capability required"})
			return
		}
		if h.hostBuild == nil {
			writeJSON(w, http.StatusServiceUnavailable, apiError{Error: hostbuild.ReasonHostBuilderUnconfigured + ": this host has no build service configured"})
			return
		}
		job, err := h.hostBuild.Submit(request)
		var refusal *hostbuild.RefusalError
		switch {
		case errors.As(err, &refusal):
			// queue_full / computer_busy (retry) and build_quota_exhausted
			// record no job; the reason code leads the error.
			writeJSON(w, http.StatusTooManyRequests, apiError{Error: refusal.Error()})
			return
		case err != nil:
			writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, job)
	case r.Method == http.MethodGet && computerevent.IsSHA256(rest):
		computerID := strings.TrimSpace(r.URL.Query().Get("computer_id"))
		if !h.authorizeFileCAS(r, computerID, "event:read") {
			writeJSON(w, http.StatusForbidden, apiError{Error: "computer capability required"})
			return
		}
		if h.hostBuild == nil {
			writeJSON(w, http.StatusServiceUnavailable, apiError{Error: hostbuild.ReasonHostBuilderUnconfigured + ": this host has no build service configured"})
			return
		}
		job, ok := h.hostBuild.Get(computerID, rest)
		if !ok {
			writeJSON(w, http.StatusNotFound, apiError{Error: "host build job not found"})
			return
		}
		writeJSON(w, http.StatusOK, job)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}
