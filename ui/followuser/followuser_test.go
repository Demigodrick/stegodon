package followuser

import (
	"errors"
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

func TestFollowResultMsg_SelfFollow(t *testing.T) {
	// Create a model
	accountId := uuid.New()
	model := InitialModel(accountId)

	// Simulate receiving a self-follow error
	selfFollowErr := fmt.Errorf("self-follow not allowed on stegodon for now")
	msg := followResultMsg{
		username: "alice@stegodon.example",
		err:      selfFollowErr,
	}

	// Update model with the message
	updatedModel, _ := model.Update(msg)

	// Verify that Status is set (not Error) - this is informational
	if updatedModel.Status == "" {
		t.Error("Expected Status to be set for self-follow error")
	}
	if updatedModel.Error != "" {
		t.Error("Expected Error to be empty for self-follow error")
	}

	// Verify the status message contains the informational icon
	if !strings.Contains(updatedModel.Status, "ℹ") {
		t.Error("Expected Status to contain informational icon (ℹ)")
	}
	if !strings.Contains(updatedModel.Status, "Self-follow") {
		t.Error("Expected Status to contain 'Self-follow'")
	}
	if !strings.Contains(updatedModel.Status, "not allowed") {
		t.Error("Expected Status to contain 'not allowed'")
	}
	if !strings.Contains(updatedModel.Status, "stegodon") {
		t.Error("Expected Status to contain 'stegodon'")
	}
}

func TestFollowResultMsg_SelfFollowVariations(t *testing.T) {
	// Test different variations of self-follow error messages
	tests := []struct {
		name     string
		errMsg   string
		wantInfo bool // Should be treated as informational (Status) vs error (Error)
	}{
		{
			name:     "exact self-follow message",
			errMsg:   "self-follow not allowed on stegodon for now",
			wantInfo: true,
		},
		{
			name:     "capitalized Self-follow",
			errMsg:   "Self-follow not allowed on stegodon for now",
			wantInfo: true,
		},
		{
			name:     "with extra context",
			errMsg:   "failed to follow: self-follow not allowed on stegodon for now",
			wantInfo: true,
		},
		{
			name:     "uppercase SELF-FOLLOW",
			errMsg:   "SELF-FOLLOW NOT ALLOWED ON STEGODON FOR NOW",
			wantInfo: true,
		},
		{
			name:     "already following (different error)",
			errMsg:   "already following user@example.com",
			wantInfo: true,
		},
		{
			name:     "network error (should be error, not info)",
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
					t.Errorf("Expected Status to be set for informational message: %s", tt.errMsg)
				}
				if updatedModel.Error != "" {
					t.Errorf("Expected Error to be empty for informational message: %s (got: %s)", tt.errMsg, updatedModel.Error)
				}
			} else {
				// Should be an error (Error, not Status)
				if updatedModel.Error == "" {
					t.Errorf("Expected Error to be set for error message: %s", tt.errMsg)
				}
				if updatedModel.Status != "" {
					t.Errorf("Expected Status to be empty for error message: %s (got: %s)", tt.errMsg, updatedModel.Status)
				}
			}
		})
	}
}

func TestFollowResultMsg_ErrorMessagePriority(t *testing.T) {
	// Test that self-follow is checked before already-following
	// Both should be informational, but verify the correct one is shown
	tests := []struct {
		name        string
		errMsg      string
		wantContain string
	}{
		{
			name:        "self-follow error",
			errMsg:      "self-follow not allowed on stegodon for now",
			wantContain: "Self-follow",
		},
		{
			name:        "already following error",
			errMsg:      "already following bob@mastodon.social",
			wantContain: "Already following",
		},
		{
			name:        "generic error",
			errMsg:      "network timeout",
			wantContain: "Failed:",
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

			// Check that the appropriate field contains the expected text
			combined := updatedModel.Status + updatedModel.Error
			if !strings.Contains(combined, tt.wantContain) {
				t.Errorf("Expected message to contain '%s', got Status='%s' Error='%s'",
					tt.wantContain, updatedModel.Status, updatedModel.Error)
			}
		})
	}
}

func TestFollowResultMsg_UserFriendlyMessages(t *testing.T) {
	// Verify that informational messages are user-friendly (not technical)
	accountId := uuid.New()
	model := InitialModel(accountId)

	selfFollowErr := fmt.Errorf("self-follow not allowed on stegodon for now")
	msg := followResultMsg{
		username: "alice@stegodon.example",
		err:      selfFollowErr,
	}

	updatedModel, _ := model.Update(msg)

	// Should NOT contain technical terms
	technicalTerms := []string{"error", "failed", "exception", "nil", "panic"}
	for _, term := range technicalTerms {
		if strings.Contains(strings.ToLower(updatedModel.Status), term) {
			t.Errorf("Status message should be user-friendly, but contains technical term: %s", term)
		}
	}

	// Should contain informational icon
	if !strings.Contains(updatedModel.Status, "ℹ") {
		t.Error("Expected informational message to have ℹ icon")
	}
}

// TestFollowResultMsg_PendingFollow tests handling of pending follow requests
func TestFollowResultMsg_PendingFollow(t *testing.T) {
	accountId := uuid.New()
	model := InitialModel(accountId)

	tests := []struct {
		name        string
		errMsg      string
		wantStatus  string
		wantContain string
		wantNoError bool
	}{
		{
			name:        "follow pending lowercase",
			errMsg:      "follow pending bob@mastodon.social",
			wantContain: "Follow request pending",
			wantNoError: true,
		},
		{
			name:        "follow pending mixed case",
			errMsg:      "Follow Pending alice@example.com",
			wantContain: "Follow request pending",
			wantNoError: true,
		},
		{
			name:        "follow pending with context",
			errMsg:      "cannot follow: follow pending charlie@pleroma.social",
			wantContain: "Follow request pending",
			wantNoError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := followResultMsg{
				username: "testuser@example.com",
				err:      errors.New(tt.errMsg),
			}

			updatedModel, _ := model.Update(msg)
			m := updatedModel

			if !strings.Contains(m.Status, tt.wantContain) {
				t.Errorf("Expected Status to contain '%s', got: %s", tt.wantContain, m.Status)
			}

			if tt.wantNoError && m.Error != "" {
				t.Errorf("Expected Error to be empty for pending follow, got: %s", m.Error)
			}

			// Should have informational icon
			if !strings.Contains(m.Status, "ℹ") {
				t.Error("Expected Status to have ℹ icon for pending follow")
			}
		})
	}
}
