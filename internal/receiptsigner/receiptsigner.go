package receiptsigner

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	selfdevprotocol "github.com/yusefmosiah/go-choir/internal/verifierprotocol"
)

const (
	ModeGuestCore = "guest-core"
	ModeVerifier  = "verifier-control"
)

type SignReceiptRequest struct {
	ReceiptKind string         `json:"receipt_kind"`
	Issuer      string         `json:"issuer"`
	KindFields  map[string]any `json:"kind_fields"`
	IssuedAt    string         `json:"issued_at"`
}

type Handler struct {
	mu         sync.Mutex
	mode       string
	computerID string
	stateRoot  string
	key        computerevent.SigningKey
	now        func() time.Time
}

func NewHandler(mode, computerID, stateRoot string, key computerevent.SigningKey) (*Handler, error) {
	mode, computerID, stateRoot = strings.TrimSpace(mode), strings.TrimSpace(computerID), filepath.Clean(stateRoot)
	if (mode != ModeGuestCore && mode != ModeVerifier) || computerID == "" || !filepath.IsAbs(stateRoot) ||
		key.SignerDomain != mode || key.KeyID == "" || len(key.PrivateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("receipt signer: complete isolated identity and key are required")
	}
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		return nil, err
	}
	return &Handler{mode: mode, computerID: computerID, stateRoot: stateRoot, key: key, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/public-key":
		publicKey := h.key.PrivateKey.Public().(ed25519.PublicKey)
		writeJSON(w, http.StatusOK, map[string]string{"signer_domain": h.key.SignerDomain, "key_id": h.key.KeyID, "public_key": base64.RawStdEncoding.EncodeToString(publicKey)})
	case r.Method == http.MethodPost && r.URL.Path == "/v1/sign-receipt" && h.mode == ModeGuestCore:
		h.signReceipt(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/sign-verifier-certificate" && h.mode == ModeVerifier:
		h.signVerifierCertificate(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "signing operation unavailable"})
	}
}

func (h *Handler) signReceipt(w http.ResponseWriter, r *http.Request) {
	var request SignReceiptRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || request.Issuer != "choir-updater" || !allowedGuestReceipt(request.ReceiptKind) || fmt.Sprint(request.KindFields["computer_id"]) != h.computerID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "typed guest receipt refused"})
		return
	}
	issuedAt, err := time.Parse(time.RFC3339Nano, request.IssuedAt)
	if err != nil || issuedAt.Location() != time.UTC || issuedAt.Before(h.now().Add(-10*time.Minute)) || issuedAt.After(h.now().Add(time.Minute)) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "guest receipt time refused"})
		return
	}
	var receipt computerevent.Receipt
	if err := h.cached("guest-receipts", request, &receipt, func() (any, error) {
		return computerevent.NewSignedReceipt(request.ReceiptKind, request.Issuer, request.KindFields, []computerevent.SigningKey{h.key}, issuedAt)
	}); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "typed guest receipt refused"})
		return
	}
	writeJSON(w, http.StatusOK, receipt)
}

func allowedGuestReceipt(kind string) bool {
	switch kind {
	case "HealthReceipt", "ActivationIntentReceipt", "MaterializationReceipt", "UpdaterRecoveryReceipt", "KernelCapabilityReceipt":
		return true
	default:
		return false
	}
}

func (h *Handler) signVerifierCertificate(w http.ResponseWriter, r *http.Request) {
	var request selfdevprotocol.VerifierCertificateRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || request.ComputerID != h.computerID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "verifier certificate refused"})
		return
	}
	var response selfdevprotocol.VerifierCertificateResponse
	if err := h.cached("verifier-certificates", request, &response, func() (any, error) {
		certificate, err := selfdevprotocol.NewVerifierCertificate(request, h.key, h.now())
		if err != nil {
			return nil, err
		}
		return selfdevprotocol.VerifierCertificateResponse{Request: request, Certificate: certificate, PublicKey: base64.RawStdEncoding.EncodeToString(h.key.PrivateKey.Public().(ed25519.PublicKey))}, nil
	}); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "verifier certificate refused"})
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) cached(namespace string, request, destination any, create func() (any, error)) error {
	canonical, err := computerevent.CanonicalJSON(request)
	if err != nil {
		return err
	}
	directory := filepath.Join(h.stateRoot, namespace)
	path := filepath.Join(directory, computerevent.DigestBytes(canonical)+".json")
	h.mu.Lock()
	defer h.mu.Unlock()
	if raw, readErr := os.ReadFile(path); readErr == nil {
		return json.Unmarshal(raw, destination)
	} else if !os.IsNotExist(readErr) {
		return readErr
	}
	value, err := create()
	if err != nil {
		return err
	}
	raw, err := computerevent.CanonicalJSON(value)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".receipt-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(raw)
	}
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(temporaryPath, path)
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, destination)
}

type Client struct {
	mode string
	http *http.Client
}

func NewClient(socket, mode string) (*Client, error) {
	socket, mode = filepath.Clean(strings.TrimSpace(socket)), strings.TrimSpace(mode)
	if !filepath.IsAbs(socket) || (mode != ModeGuestCore && mode != ModeVerifier) {
		return nil, fmt.Errorf("receipt signer client: absolute socket and known mode are required")
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", socket)
	}, DisableKeepAlives: true}
	return &Client{mode: mode, http: &http.Client{Transport: transport, Timeout: 30 * time.Second}}, nil
}

func (c *Client) PublicKey(ctx context.Context) (computerevent.SignerRef, ed25519.PublicKey, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://signer/v1/public-key", nil)
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
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&result) != nil || result.SignerDomain != c.mode {
		return computerevent.SignerRef{}, nil, fmt.Errorf("receipt signer client: public key unavailable")
	}
	publicKey, err := base64.RawStdEncoding.DecodeString(result.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize || result.KeyID == "" {
		return computerevent.SignerRef{}, nil, fmt.Errorf("receipt signer client: invalid public key")
	}
	return computerevent.SignerRef{SignerDomain: result.SignerDomain, KeyID: result.KeyID}, ed25519.PublicKey(publicKey), nil
}

func (c *Client) SignReceipt(ctx context.Context, kind, issuer string, fields map[string]any, issuedAt time.Time) (computerevent.Receipt, error) {
	if c == nil || c.mode != ModeGuestCore {
		return computerevent.Receipt{}, fmt.Errorf("receipt signer client: guest signer unavailable")
	}
	request := SignReceiptRequest{ReceiptKind: kind, Issuer: issuer, KindFields: fields, IssuedAt: issuedAt.UTC().Format(time.RFC3339Nano)}
	var receipt computerevent.Receipt
	if err := c.post(ctx, "/v1/sign-receipt", request, &receipt); err != nil {
		return computerevent.Receipt{}, err
	}
	return receipt, nil
}

func (c *Client) SignVerifierCertificate(ctx context.Context, request selfdevprotocol.VerifierCertificateRequest) (selfdevprotocol.VerifierCertificateResponse, error) {
	if c == nil || c.mode != ModeVerifier {
		return selfdevprotocol.VerifierCertificateResponse{}, fmt.Errorf("receipt signer client: verifier unavailable")
	}
	var response selfdevprotocol.VerifierCertificateResponse
	if err := c.post(ctx, "/v1/sign-verifier-certificate", request, &response); err != nil || !reflect.DeepEqual(response.Request, request) || selfdevprotocol.VerifyVerifierCertificate(response) != nil {
		return selfdevprotocol.VerifierCertificateResponse{}, fmt.Errorf("receipt signer client: verifier certificate refused")
	}
	return response, nil
}

func (c *Client) post(ctx context.Context, path string, request, response any) error {
	body, err := computerevent.CanonicalJSON(request)
	if err != nil {
		return err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://signer"+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	result, err := c.http.Do(httpRequest)
	if err != nil {
		return err
	}
	defer result.Body.Close()
	if result.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(result.Body, 1<<20)).Decode(response) != nil {
		return fmt.Errorf("receipt signer client: request refused")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
