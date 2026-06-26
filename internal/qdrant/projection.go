package qdrant

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

type ObjectSource interface {
	ListObjects(ctx context.Context, filter objectgraph.ListFilter) ([]objectgraph.Object, error)
}

type IndexedObject struct {
	CanonicalID string
	ObjectKind  objectgraph.ObjectKind
	ContentHash string
	OwnerID     string
	ComputerID  string
	VersionID   string
	Text        string
	Metadata    json.RawMessage
}

func ListIndexableObjects(ctx context.Context, source ObjectSource, filter objectgraph.ListFilter) ([]IndexedObject, error) {
	objects, err := source.ListObjects(ctx, filter)
	if err != nil {
		return nil, err
	}
	return ProjectObjects(objects)
}

func ProjectObjects(objects []objectgraph.Object) ([]IndexedObject, error) {
	out := make([]IndexedObject, 0, len(objects))
	for _, obj := range objects {
		indexed, ok, err := ProjectObject(obj)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, indexed)
		}
	}
	return out, nil
}

func ProjectObject(obj objectgraph.Object) (IndexedObject, bool, error) {
	if obj.Tombstone {
		return IndexedObject{}, false, nil
	}
	if obj.CanonicalID == "" {
		return IndexedObject{}, false, fmt.Errorf("object missing canonical_id")
	}
	if obj.ObjectKind == "" {
		return IndexedObject{}, false, fmt.Errorf("object %s missing object_kind", obj.CanonicalID)
	}
	if obj.ContentHash == "" {
		return IndexedObject{}, false, fmt.Errorf("object %s missing content_hash", obj.CanonicalID)
	}
	if obj.OwnerID == "" {
		return IndexedObject{}, false, fmt.Errorf("object %s missing owner_id", obj.CanonicalID)
	}
	if len(obj.Body) > 0 && !utf8.Valid(obj.Body) {
		return IndexedObject{}, false, nil
	}
	text := strings.TrimSpace(string(obj.Body))
	if text == "" {
		return IndexedObject{}, false, nil
	}
	metadata := obj.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	return IndexedObject{
		CanonicalID: obj.CanonicalID,
		ObjectKind:  obj.ObjectKind,
		ContentHash: obj.ContentHash,
		OwnerID:     obj.OwnerID,
		ComputerID:  obj.ComputerID,
		VersionID:   obj.VersionID,
		Text:        text,
		Metadata:    metadata,
	}, true, nil
}
