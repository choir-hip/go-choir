package types

import "time"

// MediaProgress is durable user-computer media progress. It is intentionally
// scoped to the owner and source identity so playback can resume across
// devices without browser-local storage.
type MediaProgress struct {
	OwnerID         string    `json:"owner_id"`
	Kind            string    `json:"kind"`
	Identity        string    `json:"identity"`
	CurrentTime     float64   `json:"current_time"`
	Duration        float64   `json:"duration"`
	PlaybackRate    float64   `json:"playback_rate"`
	UpdatedByDevice string    `json:"updated_by_device,omitempty"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// MediaRecent records recently opened media/readers for a user computer.
type MediaRecent struct {
	OwnerID   string    `json:"owner_id"`
	Kind      string    `json:"kind"`
	Identity  string    `json:"identity"`
	Title     string    `json:"title,omitempty"`
	FileName  string    `json:"file_name,omitempty"`
	FilePath  string    `json:"file_path,omitempty"`
	SourceURL string    `json:"source_url,omitempty"`
	MediaType string    `json:"media_type,omitempty"`
	ContentID string    `json:"content_id,omitempty"`
	OpenedAt  time.Time `json:"opened_at"`
}

// UserPreference is a durable owner-scoped preference record.
type UserPreference struct {
	OwnerID       string         `json:"owner_id"`
	PreferenceKey string         `json:"preference_key"`
	Value         map[string]any `json:"value"`
	UpdatedAt     time.Time      `json:"updated_at"`
}
