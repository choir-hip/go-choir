package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"
)

func TestMain(m *testing.M) { os.Exit(dbxtest.Main(m)) }

// Failure modes: --bootstrap cannot create the control and graph schemas on
// empty databases (the C4 rehearsal's first step), or leaves the targets
// without the tables the copier will fill; usage errors exit 0.
func TestBootstrapCreatesBothSchemas(t *testing.T) {
	control, graph := dbxtest.PostgresDSN(t), dbxtest.PostgresDSN(t)
	if err := bootstrapSchemas(context.Background(), control, graph); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	for name, dsn := range map[string]string{"control": control, "graph": graph} {
		db, err := dbx.OpenPostgres(dsn, dbx.PGOptions{MaxOpenConns: 1})
		if err != nil {
			t.Fatal(err)
		}
		tables, err := dbx.Tables(context.Background(), db, dbx.Postgres)
		_ = db.Close()
		if err != nil || len(tables) == 0 {
			t.Errorf("%s has no tables after bootstrap: %v %v", name, tables, err)
		}
	}
}

func TestUsageErrors(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{}, &out, &errb); code != 2 {
		t.Errorf("no DSNs: exit %d, want 2", code)
	}
	if !strings.Contains(errb.String(), "all four DSNs") {
		t.Errorf("stderr = %q", errb.String())
	}
}
