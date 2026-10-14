package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// ogKindComputerSetting is one owner-set switch on one computer (L2a A6 /
// R2-7, docs/reports/l2a-synthesis-intake-design-2026-10-14.md). A setting
// is the owner's act through the product API; no desk sets one.
const ogKindComputerSetting = objectgraph.ObjectKind("choir.computer_setting")

// SettingIntakeLabels turns on Jev's labels for arriving captures and the
// prompt text that tells management about them. Off by default; turning it
// off is the rollback.
const SettingIntakeLabels = "intake.labels"

// KnownComputerSettings are the settings a computer has, with their
// defaults. A key outside this list is refused, so a typo never stores a
// setting nobody reads.
var KnownComputerSettings = map[string]bool{
	SettingIntakeLabels: false,
}

// ComputerSetting is one switch's value on one computer.
type ComputerSetting struct {
	OwnerID    string    `json:"owner_id"`
	ComputerID string    `json:"computer_id"`
	Key        string    `json:"key"`
	Value      bool      `json:"value"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

func computerSettingKey(computerID, key string) string {
	return strings.TrimSpace(computerID) + "\x00" + strings.TrimSpace(key)
}

// PutComputerSetting stores a known setting for one computer.
func (s *Store) PutComputerSetting(ctx context.Context, setting ComputerSetting) error {
	setting.OwnerID, setting.ComputerID, setting.Key = strings.TrimSpace(setting.OwnerID), strings.TrimSpace(setting.ComputerID), strings.TrimSpace(setting.Key)
	if setting.OwnerID == "" || setting.ComputerID == "" {
		return fmt.Errorf("computer setting: owner_id and computer_id are required")
	}
	if _, known := KnownComputerSettings[setting.Key]; !known {
		return fmt.Errorf("computer setting: unknown key %q", setting.Key)
	}
	if setting.UpdatedAt.IsZero() {
		setting.UpdatedAt = time.Now().UTC()
	}
	metadata := map[string]any{
		"computer_id": setting.ComputerID,
		"key":         setting.Key,
		"value":       setting.Value,
		"updated_at":  setting.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	_, err := s.ogPut(ctx, ogKindComputerSetting, setting.OwnerID, computerSettingKey(setting.ComputerID, setting.Key), setting, metadata, setting.UpdatedAt)
	return err
}

// GetComputerSetting returns one stored setting, or ErrNotFound.
func (s *Store) GetComputerSetting(ctx context.Context, ownerID, computerID, key string) (ComputerSetting, error) {
	if s.og == nil {
		return ComputerSetting{}, fmt.Errorf("store: object graph not initialized")
	}
	id, err := objectgraph.BuildCanonicalID(ogKindComputerSetting, strings.TrimSpace(ownerID), objectgraph.StableSuffixFromKey(computerSettingKey(computerID, key)))
	if err != nil {
		return ComputerSetting{}, err
	}
	obj, err := s.og.GetObject(ctx, id)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return ComputerSetting{}, ErrNotFound
	}
	if err != nil {
		return ComputerSetting{}, err
	}
	var setting ComputerSetting
	if err := ogDecode(obj, &setting); err != nil {
		return ComputerSetting{}, err
	}
	return setting, nil
}

// ListComputerSettings returns every known setting of one computer, stored
// or default, sorted by key.
func (s *Store) ListComputerSettings(ctx context.Context, ownerID, computerID string) ([]ComputerSetting, error) {
	out := make([]ComputerSetting, 0, len(KnownComputerSettings))
	for key, def := range KnownComputerSettings {
		setting, err := s.GetComputerSetting(ctx, ownerID, computerID, key)
		if errors.Is(err, ErrNotFound) {
			setting = ComputerSetting{OwnerID: strings.TrimSpace(ownerID), ComputerID: strings.TrimSpace(computerID), Key: key, Value: def}
		} else if err != nil {
			return nil, err
		}
		out = append(out, setting)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}
