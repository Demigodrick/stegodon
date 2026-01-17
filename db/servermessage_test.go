package db

import (
	"testing"
	"time"
)

func TestServerMessage(t *testing.T) {
	// Setup test database
	testDB := setupTestDB(t)
	defer testDB.db.Close()

	// Create server_message table
	_, err := testDB.db.Exec(sqlCreateServerMessageTable)
	if err != nil {
		t.Fatalf("Failed to create server_message table: %v", err)
	}

	t.Run("ReadServerMessage returns empty message when none exists", func(t *testing.T) {
		err, msg := testDB.ReadServerMessage()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if msg == nil {
			t.Fatal("Expected non-nil message")
		}
		if msg.Id != 1 {
			t.Errorf("Expected ID 1, got %d", msg.Id)
		}
		if msg.Message != "" {
			t.Errorf("Expected empty message, got %s", msg.Message)
		}
		if msg.Enabled {
			t.Error("Expected message to be disabled by default")
		}
	})

	t.Run("UpdateServerMessage creates new message", func(t *testing.T) {
		err := testDB.UpdateServerMessage("Welcome to our server!", true)
		if err != nil {
			t.Fatalf("Failed to update server message: %v", err)
		}

		err, msg := testDB.ReadServerMessage()
		if err != nil {
			t.Fatalf("Failed to read server message: %v", err)
		}
		if msg.Message != "Welcome to our server!" {
			t.Errorf("Expected 'Welcome to our server!', got '%s'", msg.Message)
		}
		if !msg.Enabled {
			t.Error("Expected message to be enabled")
		}
	})

	t.Run("UpdateServerMessage updates existing message", func(t *testing.T) {
		// First update
		err := testDB.UpdateServerMessage("First message", true)
		if err != nil {
			t.Fatalf("Failed to create message: %v", err)
		}

		// Wait a moment to ensure updated_at changes
		time.Sleep(10 * time.Millisecond)

		// Second update
		err = testDB.UpdateServerMessage("Updated message", false)
		if err != nil {
			t.Fatalf("Failed to update message: %v", err)
		}

		err, msg := testDB.ReadServerMessage()
		if err != nil {
			t.Fatalf("Failed to read server message: %v", err)
		}
		if msg.Message != "Updated message" {
			t.Errorf("Expected 'Updated message', got '%s'", msg.Message)
		}
		if msg.Enabled {
			t.Error("Expected message to be disabled")
		}
	})

	t.Run("UpdateServerMessage preserves single row constraint", func(t *testing.T) {
		// Update multiple times
		err := testDB.UpdateServerMessage("Message 1", true)
		if err != nil {
			t.Fatalf("Failed first update: %v", err)
		}

		err = testDB.UpdateServerMessage("Message 2", false)
		if err != nil {
			t.Fatalf("Failed second update: %v", err)
		}

		err = testDB.UpdateServerMessage("Message 3", true)
		if err != nil {
			t.Fatalf("Failed third update: %v", err)
		}

		// Verify it has the latest message (which also confirms only one row)
		err, msg := testDB.ReadServerMessage()
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}
		if msg.Message != "Message 3" {
			t.Errorf("Expected 'Message 3', got '%s'", msg.Message)
		}
		if !msg.Enabled {
			t.Error("Expected message to be enabled")
		}
	})

	t.Run("UpdateServerMessage with empty message", func(t *testing.T) {
		err := testDB.UpdateServerMessage("", false)
		if err != nil {
			t.Fatalf("Failed to update with empty message: %v", err)
		}

		err, msg := testDB.ReadServerMessage()
		if err != nil {
			t.Fatalf("Failed to read server message: %v", err)
		}
		if msg.Message != "" {
			t.Errorf("Expected empty message, got '%s'", msg.Message)
		}
	})

	t.Run("UpdateServerMessage sets updated_at timestamp", func(t *testing.T) {
		beforeUpdate := time.Now()
		time.Sleep(10 * time.Millisecond)

		err := testDB.UpdateServerMessage("Test timestamp", true)
		if err != nil {
			t.Fatalf("Failed to update message: %v", err)
		}

		time.Sleep(10 * time.Millisecond)
		afterUpdate := time.Now()

		err, msg := testDB.ReadServerMessage()
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}

		if msg.UpdatedAt.Before(beforeUpdate) {
			t.Error("UpdatedAt should be after the update started")
		}
		if msg.UpdatedAt.After(afterUpdate) {
			t.Error("UpdatedAt should be before the update completed")
		}
	})
}
