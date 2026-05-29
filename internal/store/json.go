package store

import "encoding/json"

func rawJSONOrDefault(raw json.RawMessage, fallback string) string {
	if len(raw) == 0 || !json.Valid(raw) {
		return fallback
	}
	return string(raw)
}
