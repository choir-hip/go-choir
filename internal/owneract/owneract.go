// Package owneract is the owner-act receipt (S1 of
// docs/reports/principal-identity-and-signed-acts-2026-10-15.md): a
// statement, signed by the proxy's own host-owner-act key, that the proxy
// authenticated the owner of one computer, by one method, for one act over
// one exact subject. corpusd verifies it before it lets the owner's
// authority change anything (docs/problems/effect-accepted-admitted-without-authority-2026-10-15.md).
//
// The receipt is a computerevent.Receipt: RFC 8785 canonical JSON, the
// "choir-receipt-v1\0" preimage prefix, a UUIDv7 receipt id and a canonical
// UTC issue time, so its kind, issuer and signer domain are signed. The
// private key lives only in the proxy (systemd LoadCredential=); corpusd
// holds the public key.
package owneract

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

const (
	// ReceiptKind is the receipt_kind of every owner-act receipt.
	ReceiptKind = "OwnerActReceipt"
	// Issuer is the issuer of every owner-act receipt.
	Issuer = "proxy"
	// SignerDomain is the proxy owner-act key's signer domain; no other
	// domain may sign an owner act.
	SignerDomain = "host-owner-act"
	// Audience names the verifier a receipt is minted for.
	Audience = "corpusd"
	// Header carries the receipt (base64url of its canonical bytes) on the
	// proxy's request to corpusd.
	Header = "X-Choir-Owner-Act-Receipt"
	// TTL is how long a receipt may be used after it is issued.
	TTL = 5 * time.Minute
	// MaxFutureSkew bounds how far ahead of the verifier's clock issued_at
	// may be (the proxy and corpusd share a host).
	MaxFutureSkew = time.Minute

	// CredentialName is the proxy's systemd credential holding the private
	// key (PEM PKCS#8).
	CredentialName = "choir-owner-act-key"
	// PublicCredentialName is corpusd's systemd credential holding the
	// public key (PEM SPKI).
	PublicCredentialName = "choir-owner-act-public-key"
	// EnvKeyFile overrides the proxy's private key path.
	EnvKeyFile = "CHOIR_OWNER_ACT_KEY_FILE"
	// EnvPublicKeyFile overrides corpusd's public key path.
	EnvPublicKeyFile = "CHOIR_OWNER_ACT_PUBLIC_KEY_FILE"
)

// Act kinds.
const (
	// ActSelfDevApprove is arming accept_once: the owner's approval of
	// exactly one operation and bundle at exact heads.
	ActSelfDevApprove = "selfdev.approve"
	// ActSelfDevModeSet is every other self-development mode write.
	ActSelfDevModeSet = "selfdev.mode.set"
	// ActSelfDevAutonomyArm is arming standing_consensus: the owner's
	// standing grant of autonomous acceptance under one policy, on one
	// test-tier computer, until an expiry (P4,
	// docs/problems/p4-standing-consensus-autonomy-2026-10-15.md). Its
	// subject carries computer_tier, which the proxy reads from vmctl.
	ActSelfDevAutonomyArm = "selfdev.autonomy.arm"
	// ActPackagePublish is publishing one package release at scope owner
	// (Library S2, docs/design/library-package-propagation-2026-10-26.md
	// 4.1). Its subject binds the release id and the scopes.
	ActPackagePublish = "package.publish"
	// ActPackageListing is one change to a package release's listing: its
	// scopes (share), or its state (withdraw, reinstate) (Library S3,
	// design 4.1). Its subject binds the release id and the change.
	ActPackageListing = "package.listing"
)

// Auth methods.
const (
	AuthMethodSession = "session"
	AuthMethodAPIKey  = "api_key"
)

// Act is what the proxy attests.
type Act struct {
	Kind          string
	OwnerUserID   string
	AuthMethod    string
	CredentialRef string
	ComputerID    string
	Subject       map[string]any
}

// KeyID names a host-owner-act public key: a rotation is a new key id.
func KeyID(public ed25519.PublicKey) string {
	sum := sha256.Sum256(public)
	return "host-owner-act-" + hex.EncodeToString(sum[:8])
}

// Signer is the proxy's owner-act key.
type Signer struct {
	key computerevent.SigningKey
}

// NewSigner wraps an Ed25519 private key.
func NewSigner(private ed25519.PrivateKey) (*Signer, error) {
	if len(private) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("owner act: invalid private key")
	}
	public := private.Public().(ed25519.PublicKey)
	return &Signer{key: computerevent.SigningKey{
		SignerRef:  computerevent.SignerRef{SignerDomain: SignerDomain, KeyID: KeyID(public)},
		PrivateKey: append(ed25519.PrivateKey(nil), private...),
	}}, nil
}

// KeyID is the signer's key id.
func (s *Signer) KeyID() string { return s.key.KeyID }

// PublicKey is the signer's public key.
func (s *Signer) PublicKey() ed25519.PublicKey {
	return s.key.PrivateKey.Public().(ed25519.PublicKey)
}

// Mint signs an owner-act receipt valid from now for TTL.
func (s *Signer) Mint(act Act, now time.Time) (computerevent.Receipt, error) {
	if s == nil {
		return computerevent.Receipt{}, fmt.Errorf("owner act: no signer")
	}
	if strings.TrimSpace(act.Kind) == "" || strings.TrimSpace(act.OwnerUserID) == "" || strings.TrimSpace(act.ComputerID) == "" ||
		(act.AuthMethod != AuthMethodSession && act.AuthMethod != AuthMethodAPIKey) || strings.TrimSpace(act.CredentialRef) == "" || len(act.Subject) == 0 {
		return computerevent.Receipt{}, fmt.Errorf("owner act: incomplete act")
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return computerevent.Receipt{}, fmt.Errorf("owner act: nonce: %w", err)
	}
	now = now.UTC().Truncate(time.Microsecond)
	fields := map[string]any{
		"act_kind":       act.Kind,
		"owner_user_id":  act.OwnerUserID,
		"auth_method":    act.AuthMethod,
		"credential_ref": act.CredentialRef,
		"audience":       Audience,
		"computer_id":    act.ComputerID,
		"nonce":          hex.EncodeToString(nonce[:]),
		"not_after":      now.Add(TTL).Format(time.RFC3339Nano),
		"subject":        act.Subject,
	}
	return computerevent.NewSignedReceipt(ReceiptKind, Issuer, fields, []computerevent.SigningKey{s.key}, now)
}

// Encode renders a receipt for the Header.
func Encode(receipt computerevent.Receipt) (string, error) {
	raw, err := receipt.CanonicalBytes()
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// Decode parses a Header value.
func Decode(value string) (computerevent.Receipt, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return computerevent.Receipt{}, ErrMissing
	}
	if len(value) > 16<<10 {
		return computerevent.Receipt{}, fmt.Errorf("owner act: receipt too large")
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return computerevent.Receipt{}, fmt.Errorf("owner act: receipt encoding: %w", err)
	}
	var receipt computerevent.Receipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return computerevent.Receipt{}, fmt.Errorf("owner act: receipt: %w", err)
	}
	return receipt, nil
}

// ErrMissing is a request with no receipt.
var ErrMissing = errors.New("owner act: no receipt")

// Verified is a receipt that passed Verify.
type Verified struct {
	Receipt       computerevent.Receipt
	Digest        string
	ReceiptID     string
	Nonce         string
	ActKind       string
	OwnerUserID   string
	AuthMethod    string
	CredentialRef string
	ComputerID    string
	NotAfter      time.Time
	Subject       map[string]any
}

// SubjectString reads a string subject field.
func (v Verified) SubjectString(name string) string {
	value, _ := v.Subject[name].(string)
	return value
}

// Verifier holds corpusd's copy of the proxy's public key.
type Verifier struct {
	keyID  string
	public ed25519.PublicKey
}

// NewVerifier pins one public key.
func NewVerifier(public ed25519.PublicKey) (*Verifier, error) {
	if len(public) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("owner act: invalid public key")
	}
	return &Verifier{keyID: KeyID(public), public: append(ed25519.PublicKey(nil), public...)}, nil
}

// KeyID is the pinned key's id.
func (v *Verifier) KeyID() string { return v.keyID }

func (v *Verifier) ResolveReceiptKey(domain, _ string, keyID string, _ uint64, _ time.Time) (ed25519.PublicKey, error) {
	if domain != SignerDomain || keyID != v.keyID {
		return nil, fmt.Errorf("owner act: unknown signer %s/%s", domain, keyID)
	}
	return v.public, nil
}

// Verify checks the envelope, the one host-owner-act signature, the
// audience, the computer, the act kind and the validity window. The
// caller checks the subject.
func (v *Verifier) Verify(receipt computerevent.Receipt, computerID, actKind string, now time.Time) (Verified, error) {
	if v == nil {
		return Verified{}, fmt.Errorf("owner act: no verifier key configured")
	}
	if receipt.ReceiptKind != ReceiptKind || receipt.Issuer != Issuer {
		return Verified{}, fmt.Errorf("owner act: not an owner-act receipt")
	}
	if len(receipt.RequiredSigners) != 1 || receipt.RequiredSigners[0].SignerDomain != SignerDomain {
		return Verified{}, fmt.Errorf("owner act: receipt must carry exactly one %s signature", SignerDomain)
	}
	if err := receipt.Verify(v); err != nil {
		return Verified{}, fmt.Errorf("owner act: %w", err)
	}
	field := func(name string) string {
		value, _ := receipt.KindFields[name].(string)
		return value
	}
	subject, _ := receipt.KindFields["subject"].(map[string]any)
	out := Verified{
		Receipt: receipt, ReceiptID: receipt.ReceiptID, Nonce: field("nonce"),
		ActKind: field("act_kind"), OwnerUserID: field("owner_user_id"), AuthMethod: field("auth_method"),
		CredentialRef: field("credential_ref"), ComputerID: field("computer_id"), Subject: subject,
	}
	if field("audience") != Audience {
		return Verified{}, fmt.Errorf("owner act: audience %q is not %s", field("audience"), Audience)
	}
	if out.ComputerID == "" || out.ComputerID != computerID {
		return Verified{}, fmt.Errorf("owner act: receipt is for another computer")
	}
	if out.ActKind != actKind {
		return Verified{}, fmt.Errorf("owner act: act %q is not %q", out.ActKind, actKind)
	}
	if out.OwnerUserID == "" || (out.AuthMethod != AuthMethodSession && out.AuthMethod != AuthMethodAPIKey) || out.CredentialRef == "" || subject == nil {
		return Verified{}, fmt.Errorf("owner act: incomplete principal or subject")
	}
	if nonce, err := hex.DecodeString(out.Nonce); err != nil || len(nonce) != 16 {
		return Verified{}, fmt.Errorf("owner act: nonce must be 128 bits of hex")
	}
	issuedAt, err := time.Parse(time.RFC3339Nano, receipt.IssuedAt)
	if err != nil {
		return Verified{}, fmt.Errorf("owner act: issued_at: %w", err)
	}
	notAfter, err := time.Parse(time.RFC3339Nano, field("not_after"))
	if err != nil || notAfter.Location() != time.UTC || notAfter.Format(time.RFC3339Nano) != field("not_after") {
		return Verified{}, fmt.Errorf("owner act: not_after must be canonical UTC")
	}
	if notAfter.Sub(issuedAt) > TTL || !notAfter.After(issuedAt) {
		return Verified{}, fmt.Errorf("owner act: validity window exceeds %s", TTL)
	}
	now = now.UTC()
	if issuedAt.After(now.Add(MaxFutureSkew)) {
		return Verified{}, fmt.Errorf("owner act: issued in the future")
	}
	if !now.Before(notAfter) {
		return Verified{}, fmt.Errorf("owner act: receipt expired at %s", field("not_after"))
	}
	out.NotAfter = notAfter
	raw, err := receipt.CanonicalBytes()
	if err != nil {
		return Verified{}, err
	}
	out.Digest = computerevent.DigestBytes(raw)
	return out, nil
}

// credentialPath resolves an explicit path, else the systemd credential.
func credentialPath(env, name string) string {
	if path := strings.TrimSpace(os.Getenv(env)); path != "" {
		return path
	}
	if dir := strings.TrimSpace(os.Getenv("CREDENTIALS_DIRECTORY")); dir != "" {
		return filepath.Join(dir, name)
	}
	return ""
}

// LoadSignerFromEnv loads the proxy's key from EnvKeyFile or
// $CREDENTIALS_DIRECTORY/CredentialName. It returns (nil, nil) when
// neither is set: the proxy then mints no receipts.
func LoadSignerFromEnv() (*Signer, string, error) {
	path := credentialPath(EnvKeyFile, CredentialName)
	if path == "" {
		return nil, "", nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, path, nil
	}
	if err != nil {
		return nil, path, fmt.Errorf("owner act: read key: %w", err)
	}
	signer, err := ParsePrivateKeyPEM(data)
	return signer, path, err
}

// ParsePrivateKeyPEM parses `openssl genpkey -algorithm ed25519` output.
func ParsePrivateKeyPEM(data []byte) (*Signer, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("owner act: key is not a PEM PKCS#8 private key")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("owner act: parse key: %w", err)
	}
	private, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("owner act: key is not Ed25519")
	}
	return NewSigner(private)
}

// LoadVerifierFromEnv loads corpusd's copy of the public key from
// EnvPublicKeyFile or $CREDENTIALS_DIRECTORY/PublicCredentialName. It
// returns (nil, path, nil) when the file is absent.
func LoadVerifierFromEnv() (*Verifier, string, error) {
	path := credentialPath(EnvPublicKeyFile, PublicCredentialName)
	if path == "" {
		return nil, "", nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, path, nil
	}
	if err != nil {
		return nil, path, fmt.Errorf("owner act: read public key: %w", err)
	}
	verifier, err := ParsePublicKeyPEM(data)
	return verifier, path, err
}

// ParsePublicKeyPEM parses `openssl pkey -pubout` output.
func ParsePublicKeyPEM(data []byte) (*Verifier, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("owner act: public key is not PEM SPKI")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("owner act: parse public key: %w", err)
	}
	public, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("owner act: public key is not Ed25519")
	}
	return NewVerifier(public)
}

// MarshalPrivateKeyPEM and MarshalPublicKeyPEM produce the same formats as
// openssl, for tests and tooling.
func MarshalPrivateKeyPEM(private ed25519.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

func MarshalPublicKeyPEM(public ed25519.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}
