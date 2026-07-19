package updater

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

func (u *Updater) SignVerifierCertificate(request selfdevprotocol.VerifierCertificateRequest, key computerevent.SigningKey, now time.Time) (selfdevprotocol.VerifierCertificateResponse, error) {
	if u == nil {
		return selfdevprotocol.VerifierCertificateResponse{}, fmt.Errorf("updater: verifier signer unavailable")
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	requestBytes, err := computerevent.CanonicalJSON(request)
	if err != nil {
		return selfdevprotocol.VerifierCertificateResponse{}, err
	}
	directory := filepath.Join(u.root, "verifier-certificates")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return selfdevprotocol.VerifierCertificateResponse{}, err
	}
	path := filepath.Join(directory, computerevent.DigestBytes(requestBytes)+".json")
	if raw, readErr := os.ReadFile(path); readErr == nil {
		var existing selfdevprotocol.VerifierCertificateResponse
		if json.Unmarshal(raw, &existing) != nil || selfdevprotocol.VerifyVerifierCertificate(existing) != nil {
			return selfdevprotocol.VerifierCertificateResponse{}, fmt.Errorf("updater: durable verifier certificate is invalid")
		}
		existingRequest, canonicalErr := computerevent.CanonicalJSON(existing.Request)
		if canonicalErr != nil || !stringSlicesEqual(existingRequest, requestBytes) {
			return selfdevprotocol.VerifierCertificateResponse{}, fmt.Errorf("updater: durable verifier certificate request changed")
		}
		return existing, nil
	} else if !os.IsNotExist(readErr) {
		return selfdevprotocol.VerifierCertificateResponse{}, readErr
	}
	certificate, err := selfdevprotocol.NewVerifierCertificate(request, key, now.UTC())
	if err != nil {
		return selfdevprotocol.VerifierCertificateResponse{}, err
	}
	response := selfdevprotocol.VerifierCertificateResponse{
		Request: request, Certificate: certificate,
		PublicKey: base64.RawStdEncoding.EncodeToString(key.PrivateKey.Public().(ed25519.PublicKey)),
	}
	canonical, err := computerevent.CanonicalJSON(response)
	if err != nil {
		return selfdevprotocol.VerifierCertificateResponse{}, err
	}
	temporary, err := os.CreateTemp(directory, ".verifier-certificate-")
	if err != nil {
		return selfdevprotocol.VerifierCertificateResponse{}, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return selfdevprotocol.VerifierCertificateResponse{}, err
	}
	if _, err := temporary.Write(canonical); err != nil {
		temporary.Close()
		return selfdevprotocol.VerifierCertificateResponse{}, err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return selfdevprotocol.VerifierCertificateResponse{}, err
	}
	if err := temporary.Close(); err != nil {
		return selfdevprotocol.VerifierCertificateResponse{}, err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return selfdevprotocol.VerifierCertificateResponse{}, err
	}
	if err := syncDir(directory); err != nil {
		return selfdevprotocol.VerifierCertificateResponse{}, err
	}
	return response, nil
}

func stringSlicesEqual(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
