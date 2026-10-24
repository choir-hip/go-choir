package sourceproj

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// U1b (docs/design/computer-update-architecture-rnd-2026-10-24.md, "U1b
// design"): the guest source repository was seeded once, at the computer's
// creation, so it lacks the objects of the commit the computer booted since.
// EnsureBaseObjects fetches that one commit by id from the image's source
// remote. git negotiates against what the repository holds, so only the delta
// moves. Nothing here moves HEAD: Reconcile does, once the objects are there.

const (
	// EnvBaseRemote is where the booted base is fetched from. The guest image
	// sets it to the platform's source remote; empty means no fetch.
	EnvBaseRemote = "CHOIR_SOURCE_BASE_REMOTE"
	// EnvBaseFetch set to "off" turns the fetch off and leaves U1 as built.
	EnvBaseFetch = "CHOIR_SOURCE_BASE_FETCH"
	// RefBootedPrefix + the commit holds a fetched booted base against gc.
	RefBootedPrefix = "refs/choir/booted/"

	// FactBaseFetched and FactBaseFetchFailed are the fact kinds the runtime logs.
	FactBaseFetched     = "source_base_fetched"
	FactBaseFetchFailed = "source_base_fetch_failed"

	// DefaultFetchTimeout bounds one attempt; DefaultMinFreeBytes is the free
	// space the data volume must have before one starts.
	DefaultFetchTimeout = 10 * time.Minute
	DefaultMinFreeBytes = 1 << 30
)

// FetchOutcome is what one attempt did.
type FetchOutcome string

const (
	// FetchDisabled: the switch or the fetch setting is off; nothing ran.
	FetchDisabled FetchOutcome = "disabled"
	// FetchPresent: the objects were already here; no network was used.
	FetchPresent FetchOutcome = "present"
	// FetchFetched: the commit was fetched and is held by its ref.
	FetchFetched FetchOutcome = "fetched"
	// FetchSkipped: nothing to fetch with or into (no remote, no full booted
	// id, no repository, a remote that is refused). Retrying will not help.
	FetchSkipped FetchOutcome = "skipped"
	// FetchFailed: a failure that may pass (network, remote down, low disk).
	FetchFailed FetchOutcome = "failed"
	// FetchPermanent: the remote does not have the commit, or asks for
	// credentials. Retrying this process will not help.
	FetchPermanent FetchOutcome = "failed_permanent"
)

// FetchResult is one attempt's outcome.
type FetchResult struct {
	Outcome  FetchOutcome `json:"outcome"`
	Commit   string       `json:"commit,omitempty"`
	Remote   string       `json:"remote,omitempty"`
	Reason   string       `json:"reason,omitempty"`
	Duration string       `json:"duration,omitempty"`
}

// Retryable reports whether a later attempt may succeed.
func (r FetchResult) Retryable() bool { return r.Outcome == FetchFailed }

// Done reports whether no further attempt is needed or useful.
func (r FetchResult) Done() bool { return !r.Retryable() }

// FetchOptions configure one attempt. The zero Timeout and MinFreeBytes take
// the defaults; a zero Remote is the skipped case.
type FetchOptions struct {
	Remote       string
	Timeout      time.Duration
	MinFreeBytes uint64
	// CABundle is passed to git when it names an existing file.
	CABundle string
}

// BaseRemote is the remote from the environment ("" when none).
func BaseRemote() string { return strings.TrimSpace(os.Getenv(EnvBaseRemote)) }

// DefaultFetchOptions reads the environment.
func DefaultFetchOptions() FetchOptions {
	return FetchOptions{Remote: BaseRemote(), CABundle: "/etc/ssl/certs/ca-certificates.crt"}
}

var fetchMu sync.Mutex

// validRemote admits an https URL with a host and no credentials, or an
// absolute local path (ops, tests, and the credential-disk bundle fallback).
// It reports whether the remote is a local path. Everything else, including
// anything that could read as an option, is refused before git runs (N3).
func validRemote(remote string) (local bool, err error) {
	remote = strings.TrimSpace(remote)
	switch {
	case remote == "":
		return false, errors.New("no source remote is configured")
	case strings.HasPrefix(remote, "-") || strings.ContainsAny(remote, " \t\r\n\x00"):
		return false, errors.New("the source remote is not a plain URL or path")
	case strings.HasPrefix(remote, "/"):
		return true, nil
	}
	u, err := url.Parse(remote)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Hostname() == "" || u.User != nil {
		return false, errors.New("the source remote must be an https URL without credentials, or an absolute path")
	}
	return false, nil
}

// HasCommit reports whether repo already holds commit (no network).
func HasCommit(ctx context.Context, repo, commit string) bool {
	if !commitID.MatchString(strings.ToLower(strings.TrimSpace(commit))) || strings.TrimSpace(repo) == "" {
		return false
	}
	_, ok := resolveCommit(ctx, repo, commit)
	return ok
}

// EnsureBaseObjects makes booted's objects present in repo, fetching them
// from opt.Remote when they are not. It never blocks on the reconcile lock and
// never moves HEAD. See the failure modes N1 to N14 in the design.
func EnsureBaseObjects(ctx context.Context, repo, booted string, opt FetchOptions) FetchResult {
	start := time.Now()
	res := FetchResult{Commit: strings.ToLower(strings.TrimSpace(booted))}
	finish := func(outcome FetchOutcome, reason string) FetchResult {
		res.Outcome, res.Reason = outcome, reason
		res.Duration = time.Since(start).Round(time.Millisecond).String()
		return res
	}
	if !Enabled() || strings.EqualFold(strings.TrimSpace(os.Getenv(EnvBaseFetch)), "off") {
		return finish(FetchDisabled, "")
	}
	if !commitID.MatchString(res.Commit) {
		return finish(FetchSkipped, "the booted image names no full commit id")
	}
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return finish(FetchSkipped, "no source repository is configured")
	}
	if info, err := os.Stat(filepath.Join(repo, ".git")); err != nil || !info.IsDir() {
		return finish(FetchSkipped, "no git repository at "+repo)
	}
	if _, ok := resolveCommit(ctx, repo, res.Commit); ok {
		return finish(FetchPresent, "")
	}
	local, err := validRemote(opt.Remote)
	if err != nil {
		return finish(FetchSkipped, err.Error())
	}
	res.Remote = opt.Remote
	if ctx != nil && ctx.Err() != nil {
		return finish(FetchFailed, ctx.Err().Error())
	}
	minFree := opt.MinFreeBytes
	if minFree == 0 {
		minFree = DefaultMinFreeBytes
	}
	if free, err := freeBytes(repo); err == nil && free < minFree {
		return finish(FetchFailed, fmt.Sprintf("the data volume has %d bytes free; a fetch needs %d", free, minFree))
	}
	fetchMu.Lock()
	defer fetchMu.Unlock()
	// Another attempt may have brought it while this one waited.
	if _, ok := resolveCommit(ctx, repo, res.Commit); ok {
		return finish(FetchPresent, "")
	}
	timeout := opt.Timeout
	if timeout <= 0 {
		timeout = DefaultFetchTimeout
	}
	args := []string{
		// No background maintenance: a catch-up must not start a repack (N6).
		"-c", "gc.auto=0", "-c", "maintenance.auto=false",
		// Objects are checked as they arrive (N3).
		"-c", "transfer.fsckObjects=true",
		// Only the transport this remote needs (N3).
		"-c", "protocol.allow=never",
		// A stalled transfer aborts (N4).
		"-c", "http.lowSpeedLimit=1000", "-c", "http.lowSpeedTime=60",
	}
	if local {
		args = append(args, "-c", "protocol.file.allow=always")
	} else {
		args = append(args, "-c", "protocol.https.allow=always")
		if opt.CABundle != "" {
			if info, err := os.Stat(opt.CABundle); err == nil && info.Mode().IsRegular() {
				args = append(args, "-c", "http.sslCAInfo="+opt.CABundle)
			}
		}
	}
	args = append(args, "fetch", "--quiet", "--no-tags", "--no-write-fetch-head", "--no-recurse-submodules", "--no-auto-gc",
		"--", opt.Remote, "+"+res.Commit+":"+RefBootedPrefix+res.Commit)
	if _, err := gitRawTimeout(ctx, timeout, repo, nil, nil, args...); err != nil {
		reason := err.Error()
		if permanentFetchError(reason) {
			return finish(FetchPermanent, reason)
		}
		return finish(FetchFailed, reason)
	}
	// The remote can only have supplied the commit asked for, but check (N3).
	if got, ok := resolveCommit(ctx, repo, res.Commit); !ok || got != res.Commit {
		_, _ = git(ctx, repo, nil, nil, "update-ref", "-d", RefBootedPrefix+res.Commit)
		return finish(FetchPermanent, "the fetch did not produce the booted commit")
	}
	pruneBootedRefs(ctx, repo, res.Commit)
	return finish(FetchFetched, "")
}

// permanentFetchError recognizes what a remote says when the commit or the
// repository is not there, or when it wants credentials.
func permanentFetchError(reason string) bool {
	reason = strings.ToLower(reason)
	for _, marker := range []string{
		"not our ref", "couldn't find remote ref", "unadvertised object", "no such remote ref",
		"repository not found", "does not appear to be a git repository", "could not read username",
		"authentication failed", "terminal prompts disabled", "returned error: 404", "returned error: 401", "returned error: 403",
	} {
		if strings.Contains(reason, marker) {
			return true
		}
	}
	return false
}

// pruneBootedRefs removes the booted-base refs of earlier boots (F8). Their
// objects stay while anything reaches them, and go with gc otherwise.
func pruneBootedRefs(ctx context.Context, repo, keep string) {
	out, err := git(ctx, repo, nil, nil, "for-each-ref", "--format=%(refname)", RefBootedPrefix)
	if err != nil {
		return
	}
	for _, ref := range strings.Split(out, "\n") {
		ref = strings.TrimSpace(ref)
		if ref == "" || ref == RefBootedPrefix+keep {
			continue
		}
		_, _ = git(ctx, repo, nil, nil, "update-ref", "-d", ref)
	}
}

// freeBytes is the space available to an unprivileged writer on path's volume.
func freeBytes(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
