package followuser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestFollowResultMsg_AlreadyFollowing(t *testing.T) {
	// Create a model
	accountId := uuid.New()
	model := InitialModel(accountId)

	// Simulate receiving a "already following" error
	alreadyFollowingErr := fmt.Errorf("already following user@example.com")
	msg := followResultMsg{
		username: "user@example.com",
		err:      alreadyFollowingErr,
	}

	// Update model with the message
	updatedModel, _ := model.Update(msg)

	// Verify that Status is set (not Error)
	if updatedModel.Status == "" {
		t.Error("Expected Status to be set for 'already following' error")
	}
	if updatedModel.Error != "" {
		t.Error("Expected Error to be empty for 'already following' error")
	}

	// Verify the status message contains the informational icon and username
	if !strings.Contains(updatedModel.Status, "ℹ") {
		t.Error("Expected Status to contain informational icon (ℹ)")
	}
	if !strings.Contains(updatedModel.Status, "user@example.com") {
		t.Error("Expected Status to contain username")
	}
	if !strings.Contains(updatedModel.Status, "Already following") {
		t.Error("Expected Status to contain 'Already following'")
	}
}

func TestFollowResultMsg_OtherError(t *testing.T) {
	// Create a model
	accountId := uuid.New()
	model := InitialModel(accountId)

	// Simulate receiving a different error
	otherErr := fmt.Errorf("network error: connection timeout")
	msg := followResultMsg{
		username: "user@example.com",
		err:      otherErr,
	}

	// Update model with the message
	updatedModel, _ := model.Update(msg)

	// Verify that Error is set (not Status)
	if updatedModel.Error == "" {
		t.Error("Expected Error to be set for non-'already following' error")
	}
	if updatedModel.Status != "" {
		t.Error("Expected Status to be empty for non-'already following' error")
	}

	// Verify the error message contains "Failed:"
	if !strings.Contains(updatedModel.Error, "Failed:") {
		t.Error("Expected Error to contain 'Failed:'")
	}
	if !strings.Contains(updatedModel.Error, "network error") {
		t.Error("Expected Error to contain the actual error message")
	}
}

func TestFollowResultMsg_Success(t *testing.T) {
	// Create a model
	accountId := uuid.New()
	model := InitialModel(accountId)

	// Simulate successful follow
	msg := followResultMsg{
		username: "user@example.com",
		err:      nil,
	}

	// Update model with the message
	updatedModel, _ := model.Update(msg)

	// Verify that Status is set (not Error)
	if updatedModel.Status == "" {
		t.Error("Expected Status to be set for successful follow")
	}
	if updatedModel.Error != "" {
		t.Error("Expected Error to be empty for successful follow")
	}

	// Verify the status message contains success indicator
	if !strings.Contains(updatedModel.Status, "✓") {
		t.Error("Expected Status to contain checkmark (✓)")
	}
	if !strings.Contains(updatedModel.Status, "user@example.com") {
		t.Error("Expected Status to contain username")
	}
	if !strings.Contains(updatedModel.Status, "Sent follow request") {
		t.Error("Expected Status to contain 'Sent follow request'")
	}
}

func TestFollowResultMsg_AlreadyFollowingVariations(t *testing.T) {
	// Test different variations of "already following" error messages
	tests := []struct {
		name     string
		errMsg   string
		wantInfo bool // Should be treated as informational (Status) vs error (Error)
	}{
		{
			name:     "exact match",
			errMsg:   "already following user@example.com",
			wantInfo: true,
		},
		{
			name:     "capitalized",
			errMsg:   "Already following user@example.com",
			wantInfo: true,
		},
		{
			name:     "with extra text",
			errMsg:   "failed to send follow: already following user@example.com",
			wantInfo: true,
		},
		{
			name:     "different error",
			errMsg:   "webfinger resolution failed: 404 not found",
			wantInfo: false,
		},
		{
			name:     "network error",
			errMsg:   "failed to connect to remote server",
			wantInfo: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accountId := uuid.New()
			model := InitialModel(accountId)

			msg := followResultMsg{
				username: "user@example.com",
				err:      fmt.Errorf("%s", tt.errMsg),
			}

			updatedModel, _ := model.Update(msg)

			if tt.wantInfo {
				// Should be informational (Status, not Error)
				if updatedModel.Status == "" {
					t.Error("Expected Status to be set for informational message")
				}
				if updatedModel.Error != "" {
					t.Error("Expected Error to be empty for informational message")
				}
			} else {
				// Should be an error (Error, not Status)
				if updatedModel.Error == "" {
					t.Error("Expected Error to be set for error message")
				}
				if updatedModel.Status != "" {
					t.Error("Expected Status to be empty for error message")
				}
			}
		})
	}
}

func TestClearStatusMsg(t *testing.T) {
	// Create a model with status and error set
	accountId := uuid.New()
	model := InitialModel(accountId)
	model.Status = "Some status"
	model.Error = "Some error"
	model.TextInput.SetValue("user@example.com")

	// Send clearStatusMsg
	msg := clearStatusMsg{}
	updatedModel, _ := model.Update(msg)

	// Verify everything is cleared
	if updatedModel.Status != "" {
		t.Errorf("Expected Status to be cleared, got: %s", updatedModel.Status)
	}
	if updatedModel.Error != "" {
		t.Errorf("Expected Error to be cleared, got: %s", updatedModel.Error)
	}
	if updatedModel.TextInput.Value() != "" {
		t.Errorf("Expected TextInput to be cleared, got: %s", updatedModel.TextInput.Value())
	}
}
