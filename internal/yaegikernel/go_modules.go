package yaegikernel

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/gomodule"
)

// The desk's Go module bundle crosses execve in the worker's environment
// (plan MA6.5; docs/design/ma65-yaegi-go-modules-2026-11-06.md section 5):
// GO_MODULES_PARTS=n, GO_MODULES_0 .. GO_MODULES_<n-1> (base64 of the
// canonical encoding, each under execve's 128 KiB per-string limit) and
// GO_MODULES_SHA256. The environment reaches a gVisor worker through its
// process spec unchanged (MA6.4).
const (
	goModulesPartsEnv  = "GO_MODULES_PARTS"
	goModulesPartEnv   = "GO_MODULES_"
	goModulesSHA256Env = "GO_MODULES_SHA256"
	// goModulesPartBytes keeps "NAME=value" under MAX_ARG_STRLEN (131072).
	goModulesPartBytes = 120 << 10
)

// maxGoModulesParts bounds the parts a worker reads.
var maxGoModulesParts = (base64.StdEncoding.EncodedLen(gomodule.MaxEncodedBundleBytes) + goModulesPartBytes - 1) / goModulesPartBytes

// goModulesEnv renders the bundle encoding as worker env entries.
func goModulesEnv(prefix string, raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	enc := base64.StdEncoding.EncodeToString(raw)
	var parts []string
	for len(enc) > 0 {
		n := min(goModulesPartBytes, len(enc))
		parts = append(parts, enc[:n])
		enc = enc[n:]
	}
	sum := sha256.Sum256(raw)
	env := []string{prefix + goModulesPartsEnv + "=" + strconv.Itoa(len(parts)), prefix + goModulesSHA256Env + "=" + hex.EncodeToString(sum[:])}
	for i, p := range parts {
		env = append(env, prefix+goModulesPartEnv+strconv.Itoa(i)+"="+p)
	}
	return env
}

// checkGoModulesCarry refuses, at spawn, a bundle the worker could not read:
// over the encoded cap. The bundle's content was checked where it was built.
func checkGoModulesCarry(raw []byte) error {
	if len(raw) > gomodule.MaxEncodedBundleBytes {
		return fmt.Errorf("desk go modules: the bundle is %d bytes; at most %d", len(raw), gomodule.MaxEncodedBundleBytes)
	}
	return nil
}

// readGoModulesEnv is the worker side: it joins the parts and decodes them.
// An absent carry is no bundle; a partial or malformed one is an error the
// worker refuses to start on.
func readGoModulesEnv(getenv func(string) string, prefix string) ([]byte, string, error) {
	n := getenv(prefix + goModulesPartsEnv)
	sum := getenv(prefix + goModulesSHA256Env)
	if n == "" && sum == "" {
		return nil, "", nil
	}
	parts, err := strconv.Atoi(n)
	if err != nil || parts < 1 || parts > maxGoModulesParts {
		return nil, "", fmt.Errorf("go modules carry: %q parts", n)
	}
	if len(sum) != 64 {
		return nil, "", errors.New("go modules carry: no sha256")
	}
	var b strings.Builder
	for i := 0; i < parts; i++ {
		p := getenv(prefix + goModulesPartEnv + strconv.Itoa(i))
		if p == "" {
			return nil, "", fmt.Errorf("go modules carry: part %d of %d is missing", i, parts)
		}
		b.WriteString(p)
	}
	raw, err := base64.StdEncoding.DecodeString(b.String())
	if err != nil {
		return nil, "", fmt.Errorf("go modules carry: %w", err)
	}
	return raw, sum, nil
}

// goModuleBundle is the worker's check of its carried bundle: a sha256 is
// required whenever bytes are carried (review r1, D3) and must match, then a
// strict decode. NewSessionWithGoModules loads it (re-hash, closure re-walk).
// nil means no bundle.
func (c SessionWorkerConfig) goModuleBundle() (*gomodule.Bundle, error) {
	if c.goModulesErr != nil {
		return nil, c.goModulesErr
	}
	if len(c.GoModules) == 0 && c.GoModulesSHA256 == "" {
		return nil, nil
	}
	if c.GoModulesSHA256 == "" {
		return nil, errors.New("go modules carry: no sha256")
	}
	sum := sha256.Sum256(c.GoModules)
	if hex.EncodeToString(sum[:]) != c.GoModulesSHA256 {
		return nil, errors.New("go modules carry: sha256 mismatch")
	}
	b, err := gomodule.DecodeBundle(c.GoModules)
	if err != nil {
		return nil, err
	}
	return &b, nil
}
