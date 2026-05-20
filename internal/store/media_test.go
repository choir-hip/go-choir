package store

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestMediaProgressAndRecentsRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	saved, err := s.UpsertMediaProgress(ctx, types.MediaProgress{
		OwnerID:         "user-media",
		Kind:            "podcast",
		Identity:        "podcast:episode:one",
		CurrentTime:     123.5,
		Duration:        456.7,
		PlaybackRate:    1.25,
		UpdatedByDevice: "device-a",
		UpdatedAt:       time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("upsert progress: %v", err)
	}
	if saved.PlaybackRate != 1.25 {
		t.Fatalf("playback rate = %v, want 1.25", saved.PlaybackRate)
	}

	got, err := s.GetMediaProgress(ctx, "user-media", "podcast", "podcast:episode:one")
	if err != nil {
		t.Fatalf("get progress: %v", err)
	}
	if got.CurrentTime != 123.5 || got.Duration != 456.7 || got.UpdatedByDevice != "device-a" {
		t.Fatalf("progress round trip = %#v", got)
	}

	_, err = s.UpsertMediaRecent(ctx, types.MediaRecent{
		OwnerID:   "user-media",
		Kind:      "pdf",
		Identity:  "/files/report.pdf",
		Title:     "Report",
		FileName:  "report.pdf",
		FilePath:  "report.pdf",
		MediaType: "application/pdf",
		OpenedAt:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("upsert recent: %v", err)
	}

	recents, err := s.ListMediaRecents(ctx, "user-media", "pdf", 10)
	if err != nil {
		t.Fatalf("list recents: %v", err)
	}
	if len(recents) != 1 || recents[0].Identity != "/files/report.pdf" || recents[0].FilePath != "report.pdf" {
		t.Fatalf("recents = %#v", recents)
	}
}

func TestUserPreferenceRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.SaveUserPreference(ctx, types.UserPreference{
		OwnerID:       "user-theme",
		PreferenceKey: "theme",
		Value: map[string]any{
			"id":   "classic-mac",
			"name": "Classic Mac",
		},
		UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("save preference: %v", err)
	}

	got, err := s.GetUserPreference(ctx, "user-theme", "theme")
	if err != nil {
		t.Fatalf("get preference: %v", err)
	}
	if got.Value["id"] != "classic-mac" {
		t.Fatalf("theme preference = %#v", got.Value)
	}
}
