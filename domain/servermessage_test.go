package domain

import (
	"testing"
	"time"
)

func TestServerMessage(t *testing.T) {
	t.Run("Create ServerMessage with all fields", func(t *testing.T) {
		now := time.Now()
		msg := ServerMessage{
			Id:        1,
			Message:   "Welcome to our server!",
			Enabled:   true,
			UpdatedAt: now,
		}

		if msg.Id != 1 {
			t.Errorf("Expected ID 1, got %d", msg.Id)
		}
		if msg.Message != "Welcome to our server!" {
			t.Errorf("Expected 'Welcome to our server!', got '%s'", msg.Message)
		}
		if !msg.Enabled {
			t.Error("Expected Enabled to be true")
		}
		if msg.UpdatedAt != now {
			t.Error("UpdatedAt mismatch")
		}
	})

	t.Run("Create ServerMessage with defaults", func(t *testing.T) {
		msg := ServerMessage{}

		if msg.Id != 0 {
			t.Errorf("Expected default ID 0, got %d", msg.Id)
		}
		if msg.Message != "" {
			t.Errorf("Expected empty message, got '%s'", msg.Message)
		}
		if msg.Enabled {
			t.Error("Expected Enabled to be false by default")
		}
	})

	t.Run("ServerMessage with empty message", func(t *testing.T) {
		msg := ServerMessage{
			Id:      1,
			Message: "",
			Enabled: false,
		}

		if msg.Message != "" {
			t.Errorf("Expected empty message, got '%s'", msg.Message)
		}
	})

	t.Run("ServerMessage with long message", func(t *testing.T) {
		longMessage := "This is a very long server message that might contain important information for users. " +
			"It could include maintenance notices, feature announcements, or community guidelines. " +
			"The system should handle messages of various lengths without issues."

		msg := ServerMessage{
			Id:      1,
			Message: longMessage,
			Enabled: true,
		}

		if msg.Message != longMessage {
			t.Error("Long message not preserved correctly")
		}
		if len(msg.Message) == 0 {
			t.Error("Long message should not be empty")
		}
	})
}
