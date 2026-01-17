package domain

import "time"

// ServerMessage represents a message from the server admin displayed in the TUI and web UI
// Only one server message can be active at a time
type ServerMessage struct {
	Id         int       `json:"id"`          // Always 1 (single row table)
	Message    string    `json:"message"`     // The message text
	Enabled    bool      `json:"enabled"`     // Whether to show the message in TUI
	WebEnabled bool      `json:"web_enabled"` // Whether to show the message in web UI
	UpdatedAt  time.Time `json:"updated_at"`  // Last update timestamp
}
