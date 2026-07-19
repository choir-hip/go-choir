package receiptsigner

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func LoadOrCreateSigningKey(path, domain string) (computerevent.SigningKey, error) {
	path, domain = filepath.Clean(path), filepath.Clean(domain)
	if !filepath.IsAbs(path) || path == string(os.PathSeparator) || domain == "." || domain == string(os.PathSeparator) {
		return computerevent.SigningKey{}, fmt.Errorf("receipt signer: absolute key path and domain are required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return computerevent.SigningKey{}, err
	}
	privateKey, err := readSigningKey(path)
	if errors.Is(err, os.ErrNotExist) {
		_, generated, generateErr := ed25519.GenerateKey(rand.Reader)
		if generateErr != nil {
			return computerevent.SigningKey{}, generateErr
		}
		file, createErr := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if errors.Is(createErr, os.ErrExist) {
			privateKey, err = readSigningKey(path)
		} else if createErr != nil {
			return computerevent.SigningKey{}, createErr
		} else {
			if _, writeErr := file.Write(generated); writeErr != nil {
				file.Close()
				_ = os.Remove(path)
				return computerevent.SigningKey{}, writeErr
			}
			if syncErr := file.Sync(); syncErr != nil {
				file.Close()
				_ = os.Remove(path)
				return computerevent.SigningKey{}, syncErr
			}
			if closeErr := file.Close(); closeErr != nil {
				_ = os.Remove(path)
				return computerevent.SigningKey{}, closeErr
			}
			privateKey, err = generated, nil
		}
	}
	if err != nil {
		return computerevent.SigningKey{}, err
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	digest := sha256.Sum256(publicKey)
	return computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: domain, KeyID: domain + "-" + hex.EncodeToString(digest[:8])}, PrivateKey: privateKey}, nil
}

func readSigningKey(path string) (ed25519.PrivateKey, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return nil, fmt.Errorf("receipt signer: existing key owner mismatch")
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return nil, fmt.Errorf("receipt signer: existing key must be a regular mode-0600 file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("receipt signer: invalid private key size")
	}
	return ed25519.PrivateKey(raw), nil
}
