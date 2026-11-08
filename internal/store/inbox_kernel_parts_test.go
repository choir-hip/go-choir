package store

import (
	"context"
	"errors"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// F6 S3 review: a kernel-declared inbox batch re-attributes only the two kinds
// the crash closure settles. Any other custodian kind stays under the kernel
// writer, so the gate refuses it at enforce.
func TestKernelDeclaredInboxBatchReattributesOnlyTheSettledKinds(t *testing.T) {
	defer objectgraph.SetModeForTest(objectgraph.WriterGateEnforce)()
	s := &Store{}
	ctx := objectgraph.WithWriter(context.Background(), objectgraph.WriterKernel)
	objs := []objectgraph.Object{
		{CanonicalID: "t1", ObjectKind: "choir.management_task"},
		{CanonicalID: "d1", ObjectKind: "choir.capture_disposition"},
		{CanonicalID: "x1", ObjectKind: "choir.texture_document"},
		{CanonicalID: "c1", ObjectKind: "choir.inbox_head"},
	}
	parts := s.inboxCommandParts(ctx, objs)
	sawOther := false
	for _, part := range parts {
		for _, o := range part.Batch.Objects {
			err := objectgraph.CheckWriteAs(objectgraph.WithWriter(context.Background(), part.Writer), []objectgraph.Object{o}, nil)
			switch o.ObjectKind {
			case "choir.management_task", "choir.capture_disposition":
				if err != nil {
					t.Errorf("%s refused under %s: %v", o.ObjectKind, part.Writer, err)
				}
			case "choir.inbox_head":
				if err != nil {
					t.Errorf("a commons kind was refused: %v", err)
				}
			case "choir.texture_document":
				sawOther = true
				if part.Writer != objectgraph.WriterKernel {
					t.Errorf("texture_document was re-attributed to %s", part.Writer)
				}
				if !errors.Is(err, objectgraph.ErrWrongWriter) {
					t.Errorf("texture_document in a kernel batch: err = %v, want ErrWrongWriter", err)
				}
			}
		}
	}
	if !sawOther {
		t.Fatal("the other custodian kind vanished from the parts")
	}
}
