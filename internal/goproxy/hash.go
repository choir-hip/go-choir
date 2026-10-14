package goproxy

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// maxZipUnpacked bounds the bytes hashed out of one module zip; Go refuses
// module zips that unpack past 500 MiB, so a larger one is not a module.
const maxZipUnpacked = 500 << 20

// hash1 is the "h1:" directory hash Go records in go.sum
// (golang.org/x/mod/sumdb/dirhash.Hash1): the SHA-256 of the sorted lines
// "<sha256 hex>  <name>\n", base64-encoded.
func hash1(files []string, open func(string) (io.ReadCloser, error)) (string, error) {
	h := sha256.New()
	files = append([]string(nil), files...)
	sort.Strings(files)
	for _, file := range files {
		if strings.Contains(file, "\n") {
			return "", errors.New("goproxy: file name with a newline")
		}
		r, err := open(file)
		if err != nil {
			return "", err
		}
		hf := sha256.New()
		_, err = io.Copy(hf, r)
		r.Close()
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%x  %s\n", hf.Sum(nil), file)
	}
	return "h1:" + base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// hashGoMod is the go.sum hash of a module's go.mod file.
func hashGoMod(data []byte) (string, error) {
	return hash1([]string{"go.mod"}, func(string) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	})
}

// hashZip is the go.sum hash of a module zip.
func hashZip(path string) (string, error) {
	z, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer z.Close()
	files := make([]string, 0, len(z.File))
	byName := make(map[string]*zip.File, len(z.File))
	for _, f := range z.File {
		if _, dup := byName[f.Name]; dup {
			return "", fmt.Errorf("goproxy: zip has duplicate entry %q", f.Name)
		}
		files = append(files, f.Name)
		byName[f.Name] = f
	}
	budget := int64(maxZipUnpacked)
	return hash1(files, func(name string) (io.ReadCloser, error) {
		rc, err := byName[name].Open()
		if err != nil {
			return nil, err
		}
		return &budgetReader{rc: rc, left: &budget}, nil
	})
}

type budgetReader struct {
	rc   io.ReadCloser
	left *int64
}

func (b *budgetReader) Read(p []byte) (int, error) {
	n, err := b.rc.Read(p)
	*b.left -= int64(n)
	if *b.left < 0 {
		return n, errors.New("goproxy: module zip unpacks past 500 MiB")
	}
	return n, err
}

func (b *budgetReader) Close() error { return b.rc.Close() }

// ParseGoSum reads go.sum lines into a map from "mod ver" or
// "mod ver/go.mod" to the h1 hash. Lines with other hash algorithms are
// skipped; a malformed line is an error.
func ParseGoSum(data []byte) (map[string]string, error) {
	sums := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 3 {
			return nil, fmt.Errorf("go.sum line %d: want 3 fields", n)
		}
		if !strings.HasPrefix(f[2], "h1:") {
			continue
		}
		sums[f[0]+" "+f[1]] = f[2]
	}
	return sums, sc.Err()
}
