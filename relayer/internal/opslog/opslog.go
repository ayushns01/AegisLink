package opslog

import (
	"encoding/json"
	"io"
)

func Write(w io.Writer, level, component, event, message string, fields map[string]any) error {
	payload := map[string]any{
		"level":     level,
		"component": component,
		"event":     event,
		"message":   message,
	}
	for key, value := range fields {
		payload[key] = value
	}
	return json.NewEncoder(w).Encode(payload)
}
