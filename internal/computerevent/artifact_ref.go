package computerevent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const artifactRefPrefix = "artifact:sha256:"

type SHA256Digest [sha256.Size]byte

func ParseSHA256Digest(value string) (SHA256Digest, error) {
	var digest SHA256Digest
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return digest, fmt.Errorf("SHA256 digest must be 64 lowercase hex characters")
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return digest, fmt.Errorf("SHA256 digest must be 64 lowercase hex characters")
	}
	copy(digest[:], decoded)
	return digest, nil
}

func (d SHA256Digest) String() string { return hex.EncodeToString(d[:]) }

type ArtifactRef struct{ digest SHA256Digest }

func NewArtifactRef(digest SHA256Digest) ArtifactRef { return ArtifactRef{digest: digest} }

func ArtifactRefFromDigest(value string) (ArtifactRef, error) {
	digest, err := ParseSHA256Digest(value)
	if err != nil {
		return ArtifactRef{}, err
	}
	return NewArtifactRef(digest), nil
}

func ParseArtifactRef(value string) (ArtifactRef, error) {
	if !strings.HasPrefix(value, artifactRefPrefix) {
		return ArtifactRef{}, fmt.Errorf("artifact reference must use %s", artifactRefPrefix)
	}
	return ArtifactRefFromDigest(strings.TrimPrefix(value, artifactRefPrefix))
}

// NormalizeArtifactRef accepts a typed reference or a legacy V1 raw digest at
// a projection boundary. New immutable events must always store ref.String().
func NormalizeArtifactRef(value string) (ArtifactRef, error) {
	if strings.HasPrefix(value, artifactRefPrefix) {
		return ParseArtifactRef(value)
	}
	return ArtifactRefFromDigest(value)
}

func (r ArtifactRef) Digest() SHA256Digest { return r.digest }

func (r ArtifactRef) String() string { return artifactRefPrefix + r.digest.String() }
