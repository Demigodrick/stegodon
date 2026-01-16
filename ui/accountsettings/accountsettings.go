package accountsettings

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
	"log"
)

var (
	menuStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_SECONDARY))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_ACCENT)).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_ERROR)).
			Bold(true)

	confirmStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_SECONDARY))

	instructionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(common.COLOR_DIM))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_SUCCESS))

	linkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_LINK)).
			Underline(true)
)

// ViewState represents the current sub-view
type ViewState int

const (
	MenuView ViewState = iota
	EditDisplayNameView
	EditBioView
	AvatarView
	DeleteView
)

// MenuItem represents a menu option
type MenuItem int

const (
	MenuEditDisplayName MenuItem = iota
	MenuEditBio
	MenuChangeAvatar
	MenuDeleteAccount
)

type Model struct {
	Account        *domain.Account
	ViewState      ViewState
	MenuItem       MenuItem
	ConfirmStep    int // For delete: 0 = initial, 1 = first confirmation, 2 = final
	Status         string
	Error          string
	DeletionStatus string
	ShowByeBye     bool
	Width          int

	// Text inputs for editing
	displayNameInput textinput.Model
	bioInput         textinput.Model

	// Avatar upload
	uploadToken string
	uploadURL   string

	// Config for URLs
	conf *util.AppConfig
}

func InitialModel(account *domain.Account) Model {
	// Display name input
	dnInput := textinput.New()
	dnInput.Placeholder = "Display name"
	dnInput.CharLimit = 50
	dnInput.SetValue(account.DisplayName)

	// Bio input
	bioInput := textinput.New()
	bioInput.Placeholder = "Bio/summary"
	bioInput.CharLimit = 200
	bioInput.SetValue(account.Summary)

	conf, _ := util.ReadConf()

	return Model{
		Account:          account,
		ViewState:        MenuView,
		MenuItem:         MenuEditDisplayName,
		ConfirmStep:      0,
		Status:           "",
		Error:            "",
		displayNameInput: dnInput,
		bioInput:         bioInput,
		conf:             conf,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case clearStatusMsg:
		m.Status = ""
		m.Error = ""
		return m, nil

	case showByeByeMsg:
		m.ShowByeBye = true
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tea.Quit()
		})

	case deleteAccountResultMsg:
		if msg.err != nil {
			m.Error = fmt.Sprintf("Failed to delete account: %v", msg.err)
			m.ConfirmStep = 0
		} else {
			m.DeletionStatus = "completed"
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return showByeByeMsg{}
			})
		}
		return m, nil

	case updateProfileResultMsg:
		if msg.err != nil {
			m.Error = fmt.Sprintf("Failed to update: %v", msg.err)
		} else {
			m.Status = "Updated successfully!"
			// Update local account data
			if msg.field == "displayName" {
				m.Account.DisplayName = msg.value
			} else if msg.field == "bio" {
				m.Account.Summary = msg.value
			}
			m.ViewState = MenuView
		}
		return m, clearStatusAfter(3 * time.Second)

	case uploadTokenResultMsg:
		if msg.err != nil {
			m.Error = fmt.Sprintf("Failed to create upload link: %v", msg.err)
		} else {
			m.uploadToken = msg.token
			if m.conf != nil && m.conf.Conf.SslDomain != "" {
				m.uploadURL = fmt.Sprintf("https://%s/upload/%s", m.conf.Conf.SslDomain, msg.token)
			} else {
				m.uploadURL = fmt.Sprintf("http://localhost:%d/upload/%s", m.conf.Conf.HttpPort, msg.token)
			}
		}
		return m, nil

	case tea.KeyMsg:
		switch m.ViewState {
		case MenuView:
			return m.updateMenu(msg)
		case EditDisplayNameView:
			return m.updateEditDisplayName(msg)
		case EditBioView:
			return m.updateEditBio(msg)
		case AvatarView:
			return m.updateAvatar(msg)
		case DeleteView:
			return m.updateDelete(msg)
		}
	}

	return m, cmd
}

func (m Model) updateMenu(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.MenuItem > MenuEditDisplayName {
			m.MenuItem--
		}
	case "down", "j":
		if m.MenuItem < MenuDeleteAccount {
			m.MenuItem++
		}
	case "enter":
		switch m.MenuItem {
		case MenuEditDisplayName:
			m.ViewState = EditDisplayNameView
			m.displayNameInput.Focus()
			return m, textinput.Blink
		case MenuEditBio:
			m.ViewState = EditBioView
			m.bioInput.Focus()
			return m, textinput.Blink
		case MenuChangeAvatar:
			m.ViewState = AvatarView
			m.uploadToken = ""
			m.uploadURL = ""
			return m, nil
		case MenuDeleteAccount:
			m.ViewState = DeleteView
			m.ConfirmStep = 0
			return m, nil
		}
	case "e":
		m.ViewState = EditDisplayNameView
		m.displayNameInput.Focus()
		return m, textinput.Blink
	case "b":
		m.ViewState = EditBioView
		m.bioInput.Focus()
		return m, textinput.Blink
	case "a":
		m.ViewState = AvatarView
		m.uploadToken = ""
		m.uploadURL = ""
		return m, nil
	case "d":
		m.ViewState = DeleteView
		m.ConfirmStep = 0
		return m, nil
	}
	return m, nil
}

func (m Model) updateEditDisplayName(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.ViewState = MenuView
		m.displayNameInput.Blur()
		return m, nil
	case "enter":
		newValue := strings.TrimSpace(m.displayNameInput.Value())
		m.displayNameInput.Blur()
		return m, updateProfileCmd(m.Account.Id, "displayName", newValue)
	}

	var cmd tea.Cmd
	m.displayNameInput, cmd = m.displayNameInput.Update(msg)
	return m, cmd
}

func (m Model) updateEditBio(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.ViewState = MenuView
		m.bioInput.Blur()
		return m, nil
	case "enter":
		newValue := strings.TrimSpace(m.bioInput.Value())
		m.bioInput.Blur()
		return m, updateProfileCmd(m.Account.Id, "bio", newValue)
	}

	var cmd tea.Cmd
	m.bioInput, cmd = m.bioInput.Update(msg)
	return m, cmd
}

func (m Model) updateAvatar(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.ViewState = MenuView
		m.uploadToken = ""
		m.uploadURL = ""
		return m, nil
	case "g", "G":
		// Generate upload link
		return m, createUploadTokenCmd(m.Account.Id)
	}
	return m, nil
}

func (m Model) updateDelete(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if m.ConfirmStep == 0 {
			m.ConfirmStep = 1
			return m, nil
		} else if m.ConfirmStep == 1 {
			m.Status = "Deleting account..."
			return m, deleteAccountCmd(m.Account.Id)
		}
	case "n", "N", "esc":
		if m.ConfirmStep > 0 {
			m.ConfirmStep = 0
			m.Status = "Deletion cancelled"
			return m, clearStatusAfter(2 * time.Second)
		}
		m.ViewState = MenuView
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render("account settings"))
	s.WriteString("\n\n")

	if m.ShowByeBye {
		byeStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_SUCCESS)).
			Bold(true).
			Align(lipgloss.Center)
		s.WriteString("\n\n")
		s.WriteString(byeStyle.Render("Bye bye!"))
		s.WriteString("\n\n")
		return s.String()
	}

	if m.DeletionStatus == "completed" {
		s.WriteString(confirmStyle.Render("Account deleted successfully"))
		s.WriteString("\n\n")
		s.WriteString(instructionStyle.Render("Logging out..."))
		return s.String()
	}

	switch m.ViewState {
	case MenuView:
		s.WriteString(m.renderMenu())
	case EditDisplayNameView:
		s.WriteString(m.renderEditDisplayName())
	case EditBioView:
		s.WriteString(m.renderEditBio())
	case AvatarView:
		s.WriteString(m.renderAvatar())
	case DeleteView:
		s.WriteString(m.renderDelete())
	}

	// Status and error messages
	if m.Status != "" {
		s.WriteString("\n")
		s.WriteString(successStyle.Render(m.Status))
	}
	if m.Error != "" {
		s.WriteString("\n")
		s.WriteString(warningStyle.Render(m.Error))
	}

	return s.String()
}

func (m Model) renderMenu() string {
	var s strings.Builder

	s.WriteString("Current profile:\n")
	s.WriteString(fmt.Sprintf("  Username: @%s\n", m.Account.Username))
	s.WriteString(fmt.Sprintf("  Display name: %s\n", m.Account.DisplayName))
	if m.Account.Summary != "" {
		s.WriteString(fmt.Sprintf("  Bio: %s\n", m.Account.Summary))
	}
	if m.Account.AvatarURL != "" {
		s.WriteString(fmt.Sprintf("  Avatar: %s\n", m.Account.AvatarURL))
	} else {
		s.WriteString("  Avatar: (default)\n")
	}
	s.WriteString("\n")

	items := []struct {
		key   string
		label string
	}{
		{"e", "Edit display name"},
		{"b", "Edit bio"},
		{"a", "Change avatar"},
		{"d", "Delete account"},
	}

	for i, item := range items {
		if MenuItem(i) == m.MenuItem {
			s.WriteString(selectedStyle.Render(fmt.Sprintf("> [%s] %s", item.key, item.label)))
		} else {
			s.WriteString(menuStyle.Render(fmt.Sprintf("  [%s] %s", item.key, item.label)))
		}
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(instructionStyle.Render("Use arrow keys or hotkeys to navigate, Enter to select"))

	return s.String()
}

func (m Model) renderEditDisplayName() string {
	var s strings.Builder

	s.WriteString("Edit Display Name\n\n")
	s.WriteString(m.displayNameInput.View())
	s.WriteString("\n\n")
	s.WriteString(instructionStyle.Render("Enter to save, Esc to cancel"))

	return s.String()
}

func (m Model) renderEditBio() string {
	var s strings.Builder

	s.WriteString("Edit Bio\n\n")
	s.WriteString(m.bioInput.View())
	s.WriteString("\n\n")
	s.WriteString(instructionStyle.Render("Enter to save, Esc to cancel"))

	return s.String()
}

func (m Model) renderAvatar() string {
	var s strings.Builder

	s.WriteString("Change Avatar\n\n")

	if m.uploadURL != "" {
		s.WriteString("Open this link in your browser to upload an image:\n\n")
		s.WriteString(linkStyle.Render(m.uploadURL))
		s.WriteString("\n\n")
		s.WriteString(instructionStyle.Render("Link expires in 10 minutes"))
	} else {
		s.WriteString("Press 'g' to generate an upload link.\n")
		s.WriteString("The link will allow you to upload an avatar image from your browser.\n")
	}

	s.WriteString("\n\n")
	s.WriteString(instructionStyle.Render("Press 'g' to generate link, Esc to go back"))

	return s.String()
}

func (m Model) renderDelete() string {
	var s strings.Builder

	if m.ConfirmStep == 0 {
		s.WriteString(warningStyle.Render("WARNING: This will permanently delete your account!"))
		s.WriteString("\n\n")
		s.WriteString("The following data will be deleted:\n")
		s.WriteString(fmt.Sprintf("  - Your account (@%s)\n", m.Account.Username))
		s.WriteString("  - All your posts and notes\n")
		s.WriteString("  - All follow relationships\n")
		s.WriteString("  - All your activities\n")
		s.WriteString("\n")
		s.WriteString(warningStyle.Render("This action CANNOT be undone!"))
		s.WriteString("\n\n")
		s.WriteString("Are you sure you want to delete your account?\n\n")
		s.WriteString(instructionStyle.Render("Press 'y' to continue or 'n'/'esc' to cancel"))
	} else if m.ConfirmStep == 1 {
		s.WriteString(warningStyle.Render("FINAL WARNING!"))
		s.WriteString("\n\n")
		s.WriteString("You are about to permanently delete account: ")
		s.WriteString(warningStyle.Render("@" + m.Account.Username))
		s.WriteString("\n\n")
		s.WriteString("This is your last chance to cancel.\n")
		s.WriteString("After this, your account and all data will be gone forever.\n\n")
		s.WriteString(instructionStyle.Render("Press 'y' to DELETE PERMANENTLY or 'n'/'esc' to cancel"))
	}

	return s.String()
}

// Message types
type clearStatusMsg struct{}
type showByeByeMsg struct{}

type deleteAccountResultMsg struct {
	err error
}

type updateProfileResultMsg struct {
	field string
	value string
	err   error
}

type uploadTokenResultMsg struct {
	token string
	err   error
}

// Commands
func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

func deleteAccountCmd(accountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err := database.DeleteAccount(accountId)
		if err != nil {
			log.Printf("Failed to delete account %s: %v", accountId, err)
		} else {
			log.Printf("Successfully deleted account %s", accountId)
		}
		return deleteAccountResultMsg{err: err}
	}
}

func updateProfileCmd(accountId uuid.UUID, field, value string) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		var err error

		switch field {
		case "displayName":
			err = database.UpdateAccountDisplayName(accountId, value)
		case "bio":
			err = database.UpdateAccountSummary(accountId, value)
		}

		return updateProfileResultMsg{
			field: field,
			value: value,
			err:   err,
		}
	}
}

func createUploadTokenCmd(accountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		// Generate random token
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			return uploadTokenResultMsg{err: err}
		}
		token := hex.EncodeToString(tokenBytes)

		// Store in database with expiry
		database := db.GetDB()
		err := database.CreateUploadToken(accountId, token, "avatar", 10*time.Minute)
		if err != nil {
			return uploadTokenResultMsg{err: err}
		}

		return uploadTokenResultMsg{token: token}
	}
}
