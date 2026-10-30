// Command ledger-witness prints where inbox records sit on a computer's
// event chain, read-only from the host's event artifacts: L3's witness (a)
// (docs/archive/definitions/choir-ledger-l3-hypotheses-settled-2026-10-13.md).
//
//	ledger-witness --computer computer-... --head <sha256> --records id1,id2 \
//	  [--artifacts-root /var/lib/go-choir/platform-artifacts] [--max-events 50000]
//
// The head comes from GET /api/computers/{id}/self-development/head. It
// opens no database and never touches a guest. Exit 0 when every record
// was found, 3 when some are missing (the caller fails closed), 1 on error.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/ledgerwitness"
)

func main() {
	root := flag.String("artifacts-root", "/var/lib/go-choir/platform-artifacts", "the host's platform-artifacts directory")
	computer := flag.String("computer", "", "computer id")
	head := flag.String("head", "", "canonical event head to walk back from")
	records := flag.String("records", "", "comma-separated record ids")
	maxEvents := flag.Int("max-events", 50000, "at most this many events are read")
	flag.Parse()
	res, err := ledgerwitness.Walk(context.Background(), ledgerwitness.Config{ArtifactsRoot: *root, ComputerID: *computer, Head: *head, MaxEvents: *maxEvents},
		strings.Split(*records, ","))
	out, _ := json.MarshalIndent(res, "", "  ")
	fmt.Println(string(out))
	if err != nil {
		fmt.Fprintln(os.Stderr, "ledger-witness:", err)
		os.Exit(1)
	}
	if len(res.Missing) > 0 {
		os.Exit(3)
	}
}
