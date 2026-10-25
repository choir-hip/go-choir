// Command choir-capture takes viewport screenshots of the built release
// frontend inside an engineering capsule, as a capsule job
// (docs/design/visual-evidence-slice-2026-10-15.md):
//
//	choir.Start([]string{"choir-capture", "--route", "/"}, map[string]any{"minutes": 5})
//
// It serves /var/lib/artifact/release/frontend on the capsule's loopback,
// runs headless Chromium once per route, and writes each PNG (named by its
// sha256) and visual-evidence.json into /var/lib/artifact/evidence/visual,
// where Freeze finds them. It prints a JSON summary, which is the job's
// tail. Exit: 0 captured; 1 a capture failed; 2 usage; 3 this computer's
// grant is off.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/visualcapture"
)

type routeList []string

func (r *routeList) String() string     { return strings.Join(*r, ",") }
func (r *routeList) Set(v string) error { *r = append(*r, v); return nil }

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet(visualcapture.ToolName, flag.ContinueOnError)
	var routes routeList
	fs.Var(&routes, "route", "a route to capture, such as / (repeatable; default /; at most 6)")
	width := fs.Int("width", visualcapture.DefaultWidth, "viewport width")
	height := fs.Int("height", visualcapture.DefaultHeight, "viewport height")
	wait := fs.Int("wait-ms", visualcapture.DefaultWaitMS, "virtual time Chromium lets the page run before the shot")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "%s: unexpected arguments %q\n", visualcapture.ToolName, fs.Args())
		return 2
	}
	res, err := visualcapture.RunReport(context.Background(), visualcapture.Options{
		Routes: routes, Width: *width, Height: *height, WaitMS: *wait,
		Browser:    os.Getenv(visualcapture.BrowserEnv),
		Fontconfig: os.Getenv(visualcapture.FontconfigEnv),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", visualcapture.ToolName, err)
		if errors.Is(err, visualcapture.ErrNotGranted) {
			return 3
		}
		return 1
	}
	// The summary names the manifest's digest (visualcapture.Summary, V17).
	manifestRaw, err := os.ReadFile(filepath.Join(visualcapture.OutputDir, visualcapture.ManifestFile))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: read back the manifest: %v\n", visualcapture.ToolName, err)
		return 1
	}
	fmt.Println(string(visualcapture.Summary(res.Manifest, manifestRaw, res.AssetFailures)))
	return 0
}
