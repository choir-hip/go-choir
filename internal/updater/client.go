package updater

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

type Client struct {
	socket                    string
	unitEntryMarkerPath       string
	signerMigrationMarkerPath string
	http                      *http.Client
}

const (
	KernelCapabilityFailureUpdaterUnavailable              = "updater_unavailable"
	KernelCapabilityFailureUpdaterUnreachable              = "updater_unreachable"
	KernelCapabilityFailureUpdaterSocketMissing            = "updater_socket_missing"
	KernelCapabilityFailureUpdaterUnitNotStarted           = "updater_unit_not_started"
	KernelCapabilityFailureUpdaterProcessUnavailable       = "updater_process_unavailable"
	KernelCapabilityFailureSignerMigrationUnavailable      = "guest_signer_migration_unavailable"
	KernelCapabilityFailureSignerUnavailableAfterMigration = "guest_signer_unavailable_after_migration"
	KernelCapabilityFailureUpdaterPermissionDenied         = "updater_permission_denied"
	KernelCapabilityFailureUpdaterConnectionRefused        = "updater_connection_refused"
	KernelCapabilityFailureUpdaterTimeout                  = "updater_timeout"
	KernelCapabilityFailureProbeUnavailable                = "probe_unavailable"
	KernelCapabilityFailureReceiptRefused                  = "receipt_refused"
	KernelCapabilityFailureInvalidResponse                 = "invalid_response"
)

type KernelCapabilityUnavailableError struct {
	Code  string
	cause error
}

func (e *KernelCapabilityUnavailableError) Error() string {
	if e == nil || e.cause == nil {
		return "updater client: kernel capabilities unavailable"
	}
	return "updater client: kernel capabilities unavailable: " + e.cause.Error()
}

func (e *KernelCapabilityUnavailableError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func KernelCapabilityFailureCode(err error) string {
	var unavailable *KernelCapabilityUnavailableError
	if errors.As(err, &unavailable) {
		return unavailable.Code
	}
	return KernelCapabilityFailureUpdaterUnavailable
}

func (c *Client) kernelCapabilityTransportFailureCode(err error) string {
	switch {
	case errors.Is(err, os.ErrNotExist):
		if c != nil && c.unitEntryMarkerPath != "" {
			if _, markerErr := os.Stat(c.unitEntryMarkerPath); markerErr == nil {
				return KernelCapabilityFailureUpdaterProcessUnavailable
			} else if errors.Is(markerErr, os.ErrNotExist) {
				if c.signerMigrationMarkerPath != "" {
					if _, migrationErr := os.Stat(c.signerMigrationMarkerPath); migrationErr == nil {
						return KernelCapabilityFailureSignerUnavailableAfterMigration
					} else if errors.Is(migrationErr, os.ErrNotExist) {
						return KernelCapabilityFailureSignerMigrationUnavailable
					}
				}
				return KernelCapabilityFailureUpdaterUnitNotStarted
			}
		}
		return KernelCapabilityFailureUpdaterSocketMissing
	case errors.Is(err, os.ErrPermission):
		return KernelCapabilityFailureUpdaterPermissionDenied
	case errors.Is(err, syscall.ECONNREFUSED):
		return KernelCapabilityFailureUpdaterConnectionRefused
	case errors.Is(err, context.DeadlineExceeded):
		return KernelCapabilityFailureUpdaterTimeout
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return KernelCapabilityFailureUpdaterTimeout
	}
	return KernelCapabilityFailureUpdaterUnreachable
}

func NewClient(socket string) (*Client, error) {
	socket = filepath.Clean(strings.TrimSpace(socket))
	if !filepath.IsAbs(socket) {
		return nil, fmt.Errorf("updater client: absolute socket path is required")
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socket)
		},
		DisableKeepAlives: true,
	}
	return &Client{
		socket:                    socket,
		unitEntryMarkerPath:       "/run/choir/updater-unit-entered",
		signerMigrationMarkerPath: "/run/choir/guest-signer-state-migrated",
		http:                      &http.Client{Transport: transport, Timeout: 2 * time.Minute},
	}, nil
}

func (c *Client) PublicKey(ctx context.Context) (computerevent.SignerRef, ed25519.PublicKey, error) {
	if c == nil || c.http == nil {
		return computerevent.SignerRef{}, nil, fmt.Errorf("updater client: unavailable")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://updater/v1/public-key", nil)
	if err != nil {
		return computerevent.SignerRef{}, nil, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return computerevent.SignerRef{}, nil, err
	}
	defer response.Body.Close()
	var result struct {
		SignerDomain string `json:"signer_domain"`
		KeyID        string `json:"key_id"`
		PublicKey    string `json:"public_key"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&result) != nil {
		return computerevent.SignerRef{}, nil, fmt.Errorf("updater client: public key unavailable")
	}
	publicKey, err := base64.RawStdEncoding.DecodeString(result.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize || result.SignerDomain != "guest-core" || result.KeyID == "" {
		return computerevent.SignerRef{}, nil, fmt.Errorf("updater client: invalid public key")
	}
	return computerevent.SignerRef{SignerDomain: result.SignerDomain, KeyID: result.KeyID}, ed25519.PublicKey(publicKey), nil
}

func (c *Client) KernelCapabilities(ctx context.Context, request KernelCapabilityRequest) (KernelCapabilityReport, error) {
	if c == nil || c.http == nil {
		return KernelCapabilityReport{}, &KernelCapabilityUnavailableError{Code: KernelCapabilityFailureUpdaterUnavailable}
	}
	body, err := json.Marshal(request)
	if err != nil {
		return KernelCapabilityReport{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://updater/v1/kernel-capabilities", bytes.NewReader(body))
	if err != nil {
		return KernelCapabilityReport{}, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(httpRequest)
	if err != nil {
		return KernelCapabilityReport{}, &KernelCapabilityUnavailableError{
			Code: c.kernelCapabilityTransportFailureCode(err), cause: err,
		}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return KernelCapabilityReport{}, kernelCapabilityHTTPFailure(response.Body)
	}
	var report KernelCapabilityReport
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&report); err != nil {
		return KernelCapabilityReport{}, &KernelCapabilityUnavailableError{
			Code: KernelCapabilityFailureInvalidResponse, cause: err,
		}
	}
	return report, nil
}

func kernelCapabilityHTTPFailure(body io.Reader) error {
	var failure struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(body, 64<<10)).Decode(&failure)
	code := KernelCapabilityFailureUpdaterUnavailable
	switch failure.Error {
	case "mandatory kernel capability probe unavailable":
		code = KernelCapabilityFailureProbeUnavailable
	case "mandatory kernel capability receipt refused":
		code = KernelCapabilityFailureReceiptRefused
	}
	return &KernelCapabilityUnavailableError{Code: code}
}

func (c *Client) ImportBaseline(ctx context.Context, request BaselineImportRequest) (ReleaseManifest, error) {
	if c == nil || c.http == nil {
		return ReleaseManifest{}, fmt.Errorf("updater client: unavailable")
	}
	body, err := json.Marshal(request)
	if err != nil {
		return ReleaseManifest{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://updater/v1/import-baseline", bytes.NewReader(body))
	if err != nil {
		return ReleaseManifest{}, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(httpRequest)
	if err != nil {
		return ReleaseManifest{}, fmt.Errorf("updater client: import baseline: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ReleaseManifest{}, fmt.Errorf("updater client: refused baseline import with status %d", response.StatusCode)
	}
	var manifest ReleaseManifest
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&manifest); err != nil {
		return ReleaseManifest{}, err
	}
	return manifest, nil
}

func (c *Client) Apply(ctx context.Context, request ApplyRequest) (ApplyResult, error) {
	if c == nil || c.http == nil {
		return ApplyResult{}, fmt.Errorf("updater client: unavailable")
	}
	body, err := json.Marshal(request)
	if err != nil {
		return ApplyResult{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://updater/v1/apply", bytes.NewReader(body))
	if err != nil {
		return ApplyResult{}, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(httpRequest)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("updater client: apply: %w", err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return ApplyResult{}, err
	}
	if response.StatusCode == http.StatusOK {
		var result ApplyResult
		if err := json.Unmarshal(raw, &result); err != nil {
			return ApplyResult{}, err
		}
		return result, nil
	}
	var failed struct {
		Result ApplyResult `json:"result"`
		Error  string      `json:"error"`
	}
	if err := json.Unmarshal(raw, &failed); err == nil && failed.Error != "" {
		return failed.Result, fmt.Errorf("updater client: %s", failed.Error)
	}
	return ApplyResult{}, fmt.Errorf("updater client: refused apply with status %d", response.StatusCode)
}
