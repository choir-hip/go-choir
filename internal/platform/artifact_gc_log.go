package platform

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// artifactGCLogAppender appends records to one day's log.
type artifactGCLogAppender struct {
	file *os.File
	w    *bufio.Writer
}

// openArtifactGCLogForAppend opens (creating it) the day's log under the
// artifacts root.
func openArtifactGCLogForAppend(artifactsRoot string, day time.Time) (*artifactGCLogAppender, error) {
	dir := filepath.Join(artifactsRoot, artifactGCLogDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, day.UTC().Format("2006-01-02")+".jsonl"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	return &artifactGCLogAppender{file: f, w: bufio.NewWriter(f)}, nil
}

func (a *artifactGCLogAppender) append(rec artifactGCLogRecord) error {
	rec.At = time.Now().UTC().Format(time.RFC3339Nano)
	line, _ := json.Marshal(rec)
	_, err := a.w.Write(append(line, '\n'))
	return err
}

func (a *artifactGCLogAppender) sync() error {
	if err := a.w.Flush(); err != nil {
		return err
	}
	return a.file.Sync()
}

func (a *artifactGCLogAppender) Close() error {
	_ = a.w.Flush()
	return a.file.Close()
}
