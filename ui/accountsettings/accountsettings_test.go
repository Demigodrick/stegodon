package accountsettings

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

func createTestAccount() *domain.Account {
	return &domain.Account{
		Id:          uuid.New(),
		Username:    "testuser",
		DisplayName: "Test User",
		Summary:     "Test bio",
		AvatarURL:   "",
		CreatedAt:   time.Now(),
	}
}

func TestInitialModel(t *testing.T) {
	acc := createTestAccount()
	model := InitialModel(acc)

	if model.Account != acc {
		t.Error("Account should be set")
	}
	if model.ViewState != MenuView {
		t.Error("Initial view state should be MenuView")
	}
	if model.MenuItem != MenuEditDisplayName {
		t.Error("Initial menu item should be MenuEditDisplayName")
	}
	if model.ConfirmStep != 0 {
		t.Error("Initial confirm step should be 0")
	}
}

func TestMenuNavigation(t *testing.T) {
	acc := createTestAccount()
	model := InitialModel(acc)

	// Test down navigation
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if model.MenuItem != MenuEditBio {
		t.Errorf("Expected MenuEditBio after down, got %d", model.MenuItem)
	}

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if model.MenuItem != MenuChangeAvatar {
		t.Errorf("Expected MenuChangeAvatar after down, got %d", model.MenuItem)
	}

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if model.MenuItem != MenuDeleteAccount {
		t.Errorf("Expected MenuDeleteAccount after down, got %d", model.MenuItem)
	}

	// Test that we can't go past the last item
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if model.MenuItem != MenuDeleteAccount {
		t.Error("Should not go past MenuDeleteAccount")
	}

	// Test up navigation
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if model.MenuItem != MenuChangeAvatar {
		t.Errorf("Expected MenuChangeAvatar after up, got %d", model.MenuItem)
	}

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	if model.MenuItem != MenuEditBio {
		t.Errorf("Expected MenuEditBio after up, got %d", model.MenuItem)
	}
}

func TestMenuHotkeys(t *testing.T) {
	acc := createTestAccount()

	tests := []struct {
		key           rune
		expectedState ViewState
	}{
		{'e', EditDisplayNameView},
		{'b', EditBioView},
		{'a', AvatarView},
		{'d', DeleteView},
	}

	for _, tt := range tests {
		model := InitialModel(acc)
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.key}})
		if model.ViewState != tt.expectedState {
			t.Errorf("Hotkey '%c' should switch to state %d, got %d", tt.key, tt.expectedState, model.ViewState)
		}
	}
}

func TestEditDisplayNameView(t *testing.T) {
	acc := createTestAccount()
	model := InitialModel(acc)

	// Enter edit display name view
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if model.ViewState != EditDisplayNameView {
		t.Error("Should be in EditDisplayNameView")
	}

	// Test escape returns to menu
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if model.ViewState != MenuView {
		t.Error("Escape should return to MenuView")
	}
}

func TestEditBioView(t *testing.T) {
	acc := createTestAccount()
	model := InitialModel(acc)

	// Enter edit bio view
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if model.ViewState != EditBioView {
		t.Error("Should be in EditBioView")
	}

	// Test escape returns to menu
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if model.ViewState != MenuView {
		t.Error("Escape should return to MenuView")
	}
}

func TestAvatarView(t *testing.T) {
	acc := createTestAccount()
	model := InitialModel(acc)

	// Enter avatar view
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if model.ViewState != AvatarView {
		t.Error("Should be in AvatarView")
	}
	if model.uploadToken != "" {
		t.Error("Upload token should be empty initially")
	}
	if model.uploadURL != "" {
		t.Error("Upload URL should be empty initially")
	}

	// Test escape returns to menu
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if model.ViewState != MenuView {
		t.Error("Escape should return to MenuView")
	}
}

func TestDeleteConfirmation(t *testing.T) {
	acc := createTestAccount()
	model := InitialModel(acc)

	// Enter delete view
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if model.ViewState != DeleteView {
		t.Error("Should be in DeleteView")
	}
	if model.ConfirmStep != 0 {
		t.Error("Initial confirm step should be 0")
	}

	// First confirmation
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if model.ConfirmStep != 1 {
		t.Error("After first 'y', confirm step should be 1")
	}

	// Cancel with 'n'
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if model.ConfirmStep != 0 {
		t.Error("After 'n', confirm step should be reset to 0")
	}

	// Test escape from delete view
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if model.ViewState != MenuView {
		t.Error("Escape should return to MenuView")
	}
}

func TestDeleteConfirmationWithEscape(t *testing.T) {
	acc := createTestAccount()
	model := InitialModel(acc)

	// Enter delete view and confirm once
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	if model.ConfirmStep != 1 {
		t.Error("Should be at confirm step 1")
	}

	// Escape should reset confirm step
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if model.ConfirmStep != 0 {
		t.Error("Escape should reset confirm step")
	}
}

func TestViewRendering(t *testing.T) {
	acc := createTestAccount()
	model := InitialModel(acc)

	// Test menu view renders
	view := model.View()
	if view == "" {
		t.Error("Menu view should not be empty")
	}
	if !contains(view, "account settings") {
		t.Error("Menu view should contain 'account settings'")
	}
	if !contains(view, acc.Username) {
		t.Error("Menu view should contain username")
	}

	// Test edit display name view renders
	model.ViewState = EditDisplayNameView
	view = model.View()
	if !contains(view, "Edit Display Name") {
		t.Error("Edit display name view should contain 'Edit Display Name'")
	}

	// Test edit bio view renders
	model.ViewState = EditBioView
	view = model.View()
	if !contains(view, "Edit Bio") {
		t.Error("Edit bio view should contain 'Edit Bio'")
	}

	// Test avatar view renders
	model.ViewState = AvatarView
	view = model.View()
	if !contains(view, "Change Avatar") {
		t.Error("Avatar view should contain 'Change Avatar'")
	}

	// Test delete view renders
	model.ViewState = DeleteView
	view = model.View()
	if !contains(view, "WARNING") {
		t.Error("Delete view should contain 'WARNING'")
	}
}

func TestClearStatusMessage(t *testing.T) {
	acc := createTestAccount()
	model := InitialModel(acc)

	model.Status = "Test status"
	model.Error = "Test error"

	model, _ = model.Update(clearStatusMsg{})

	if model.Status != "" {
		t.Error("Status should be cleared")
	}
	if model.Error != "" {
		t.Error("Error should be cleared")
	}
}

func TestUploadTokenResult(t *testing.T) {
	acc := createTestAccount()
	model := InitialModel(acc)
	model.ViewState = AvatarView

	// Test successful token result
	model, _ = model.Update(uploadTokenResultMsg{token: "test-token-123"})
	if model.uploadToken != "test-token-123" {
		t.Error("Upload token should be set")
	}

	// Test error token result
	model2 := InitialModel(acc)
	model2.ViewState = AvatarView
	model2, _ = model2.Update(uploadTokenResultMsg{err: errTestError{}})
	if model2.Error == "" {
		t.Error("Error should be set on token error")
	}
}

// Helper error type for testing
type errTestError struct{}

func (e errTestError) Error() string {
	return "test error"
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
