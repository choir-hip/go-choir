// Command escrowkey inspects, seals and unseals the host escrow private key
// file (station FP9, docs/archive/definitions/choir-launch-fp9-keys-recovery-2026-10-21.md,
// "The escrow key"). It prints states and public-key fingerprints only,
// never key material.
//
//	escrowkey check         state, public_key_sha256, opens (exit 1 unless it opens)
//	escrowkey seal          plaintext -> sealed under the KEK, same public key
//	escrowkey unseal        sealed -> plaintext (the way back)
//	escrowkey kek-generate  a new KEK file, mode 0400, never over an existing one
//
// --key defaults to CHOIR_KEY_ESCROW_KEY_PATH, --kek to CHOIR_KEY_ESCROW_KEK_PATH.
// Stop corpusd and do not run the checkpoint worker while sealing or
// unsealing; both read the file only at start.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/yusefmosiah/go-choir/internal/keyescrow"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}

func run(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: escrowkey check|seal|unseal|kek-generate [--key path] [--kek path]")
		return 2
	}
	sub := args[0]
	fs := flag.NewFlagSet("escrowkey "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	keyPath := fs.String("key", getenv("CHOIR_KEY_ESCROW_KEY_PATH"), "escrow private key file")
	kekPath := fs.String("kek", getenv(keyescrow.EnvKEKPath), "KEK file")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	loadKEK := func() ([]byte, bool) {
		if *kekPath == "" {
			fmt.Fprintf(stderr, "escrowkey %s: no KEK (--kek or %s)\n", sub, keyescrow.EnvKEKPath)
			return nil, false
		}
		kek, err := keyescrow.LoadKEK(*kekPath)
		if err != nil {
			fmt.Fprintf(stderr, "escrowkey %s: %v\n", sub, err)
			return nil, false
		}
		return kek, true
	}
	needKey := func() bool {
		if *keyPath == "" {
			fmt.Fprintln(stderr, "escrowkey: no key path (--key or CHOIR_KEY_ESCROW_KEY_PATH)")
			return false
		}
		return true
	}
	switch sub {
	case "check":
		if !needKey() {
			return 2
		}
		// A KEK that does not load is reported; the file is still inspected.
		var kek []byte
		if *kekPath != "" {
			if k, ok := loadKEK(); ok {
				kek = k
				defer clear(kek)
			}
		}
		insp, err := keyescrow.InspectKeyFile(*keyPath, kek)
		if err != nil {
			fmt.Fprintf(stderr, "escrowkey check: %v\n", err)
			return 1
		}
		opens := "no"
		if insp.Opens {
			opens = "yes"
		}
		fmt.Fprintf(stdout, "state=%s public_key_sha256=%s opens=%s\n", insp.State, insp.PublicKeySHA256, opens)
		if insp.Reason != "" {
			fmt.Fprintf(stdout, "reason=%s\n", insp.Reason)
		}
		if !insp.Opens {
			return 1
		}
		return 0
	case "seal", "unseal":
		if !needKey() {
			return 2
		}
		kek, ok := loadKEK()
		if !ok {
			return 1
		}
		defer clear(kek)
		op := keyescrow.SealKeyFile
		if sub == "unseal" {
			op = keyescrow.UnsealKeyFile
		}
		fp, err := op(*keyPath, kek)
		if err != nil {
			fmt.Fprintf(stderr, "escrowkey %s: %v\n", sub, err)
			return 1
		}
		fmt.Fprintf(stdout, "%sed public_key_sha256=%s\n", sub, fp)
		return 0
	case "kek-generate":
		if *kekPath == "" {
			fmt.Fprintf(stderr, "escrowkey kek-generate: no KEK path (--kek or %s)\n", keyescrow.EnvKEKPath)
			return 2
		}
		if err := keyescrow.GenerateKEKFile(*kekPath); err != nil {
			fmt.Fprintf(stderr, "escrowkey kek-generate: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "kek written path=%s mode=0400; keep an offline copy outside the backup\n", *kekPath)
		return 0
	}
	fmt.Fprintf(stderr, "escrowkey: unknown subcommand %q\n", sub)
	return 2
}
