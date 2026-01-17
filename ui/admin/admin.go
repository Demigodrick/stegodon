package admin

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"log"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/google/uuid"
)

type AdminView int

const (
	MenuView AdminView = iota
	UsersView
	InfoBoxesView
)

type Model struct {
	AdminId      uuid.UUID
	CurrentView  AdminView
	MenuSelected int // Which menu item is selected

	// User management
	Users    []domain.Account
	Selected int
	Offset   int

	// Info boxes management
	InfoBoxes     []domain.InfoBox
	BoxSelected   int
	BoxOffset     int
	Editing       bool
	EditBox       *domain.InfoBox
	EditField     int            // 0=Title, 1=Content, 2=Order
	TitleInput    textarea.Model // Textarea for title
	ContentInput  textarea.Model // Textarea for content
	OrderInput    textarea.Model // Textarea for order number
	ConfirmDelete bool           // True when confirming deletion
	DeleteBoxId   uuid.UUID      // ID of box to delete

	Width  int
	Height int
	Status string
	Error  string
}

func InitialModel(adminId uuid.UUID, width, height int) Model {
	return Model{
		AdminId:      adminId,
		CurrentView:  MenuView,
		MenuSelected: 0,
		Users:        []domain.Account{},
		InfoBoxes:    []domain.InfoBox{},
		Selected:     0,
		Offset:       0,
		BoxSelected:  0,
		BoxOffset:    0,
		Width:        width,
		Height:       height,
		Status:       "",
		Error:        "",
		Editing:      false,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadUsers(), loadInfoBoxes())
}

// createTextarea creates a new textarea with standard settings
func createTextarea(placeholder string, maxHeight int) textarea.Model {
	t := textarea.New()
	t.Placeholder = placeholder
	t.CharLimit = 0
	t.ShowLineNumbers = false
	t.SetWidth(50)
	t.SetHeight(maxHeight)
	t.Cursor.SetMode(cursor.CursorBlink)
	return t
}

// initializeTextareas sets up the textareas when entering edit mode
func (m *Model) initializeTextareas(box *domain.InfoBox) {
	// Title input - single line
	m.TitleInput = createTextarea("Enter title", 1)
	m.TitleInput.SetValue(box.Title)
	// Don't focus yet - let user navigate fields first

	// Content input - multi-line
	m.ContentInput = createTextarea("Enter content (Markdown)", 8)
	m.ContentInput.SetValue(box.Content)

	// Order input - single line
	m.OrderInput = createTextarea("Enter order number", 1)
	m.OrderInput.SetValue(fmt.Sprintf("%d", box.OrderNum))

	// Start with field 0 selected but not focused (user can navigate with arrows)
	m.EditField = 0
}

// Message types
type usersLoadedMsg struct {
	users []domain.Account
}

type muteUserMsg struct{}
type kickUserMsg struct{}

type infoBoxesLoadedMsg struct {
	boxes []domain.InfoBox
}

type infoBoxSavedMsg struct{}
type infoBoxDeletedMsg struct{}
type infoBoxToggledMsg struct{}

// User management commands
func loadUsers() tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, users := database.ReadAllAccountsAdmin()
		if err != nil {
			log.Printf("Failed to load users: %v", err)
			return usersLoadedMsg{users: []domain.Account{}}
		}
		if users == nil {
			return usersLoadedMsg{users: []domain.Account{}}
		}
		return usersLoadedMsg{users: *users}
	}
}

func muteUser(userId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err := database.MuteUser(userId)
		if err != nil {
			log.Printf("Failed to mute user: %v", err)
		}
		return muteUserMsg{}
	}
}

func kickUser(userId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err := database.DeleteAccount(userId)
		if err != nil {
			log.Printf("Failed to kick user: %v", err)
		}
		return kickUserMsg{}
	}
}

// Info box management commands
func loadInfoBoxes() tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, boxes := database.ReadAllInfoBoxes()
		if err != nil {
			log.Printf("Failed to load info boxes: %v", err)
			return infoBoxesLoadedMsg{boxes: []domain.InfoBox{}}
		}
		if boxes == nil {
			return infoBoxesLoadedMsg{boxes: []domain.InfoBox{}}
		}
		return infoBoxesLoadedMsg{boxes: *boxes}
	}
}

func saveInfoBox(box *domain.InfoBox) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		var err error
		if box.Id == uuid.Nil {
			box.Id = uuid.New()
			box.CreatedAt = time.Now()
			box.UpdatedAt = time.Now()
			err = database.CreateInfoBox(box)
		} else {
			box.UpdatedAt = time.Now()
			err = database.UpdateInfoBox(box)
		}
		if err != nil {
			log.Printf("Failed to save info box: %v", err)
		}
		return infoBoxSavedMsg{}
	}
}

func deleteInfoBox(id uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err := database.DeleteInfoBox(id)
		if err != nil {
			log.Printf("Failed to delete info box: %v", err)
		}
		return infoBoxDeletedMsg{}
	}
}

func toggleInfoBox(id uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err := database.ToggleInfoBoxEnabled(id)
		if err != nil {
			log.Printf("Failed to toggle info box: %v", err)
		}
		return infoBoxToggledMsg{}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case usersLoadedMsg:
		m.Users = msg.users
		if m.Selected >= len(m.Users) && len(m.Users) > 0 {
			m.Selected = len(m.Users) - 1
		}
		return m, nil

	case muteUserMsg:
		m.Status = "User muted and posts deleted"
		m.Error = ""
		return m, loadUsers()

	case kickUserMsg:
		m.Status = "User kicked successfully"
		m.Error = ""
		return m, loadUsers()

	case infoBoxesLoadedMsg:
		m.InfoBoxes = msg.boxes
		if m.BoxSelected >= len(m.InfoBoxes) && len(m.InfoBoxes) > 0 {
			m.BoxSelected = len(m.InfoBoxes) - 1
		}
		return m, nil

	case infoBoxSavedMsg:
		m.Status = "Info box saved successfully"
		m.Error = ""
		m.Editing = false
		m.EditBox = nil
		return m, loadInfoBoxes()

	case infoBoxDeletedMsg:
		m.Status = "Info box deleted successfully"
		m.Error = ""
		return m, loadInfoBoxes()

	case infoBoxToggledMsg:
		m.Status = "Info box toggled"
		m.Error = ""
		return m, loadInfoBoxes()

	case tea.KeyMsg:
		m.Status = ""
		m.Error = ""

		// Route to appropriate handler based on current view
		switch m.CurrentView {
		case MenuView:
			return m.handleMenuKeys(msg)
		case UsersView:
			return m.handleUsersKeys(msg)
		case InfoBoxesView:
			if m.Editing {
				// Handle editing keys first (intercepts tab/shift+tab/ctrl+s/esc)
				newModel, editCmd := m.handleEditingKeys(msg)
				if editCmd != nil {
					// Key was handled by editor (tab, save, cancel, etc.)
					return newModel, editCmd
				}
				// Key wasn't handled, pass to textarea
				m = newModel
			} else {
				return m.handleInfoBoxesKeys(msg)
			}
		}
	}

	// Update active textarea when in edit mode (only if focused and key not blocked)
	if m.Editing {
		// Check if textarea is focused before passing keys
		isFocused := m.TitleInput.Focused() || m.ContentInput.Focused() || m.OrderInput.Focused()

		if isFocused {
			// Block enter key in single-line fields
			if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
				if m.EditField == 0 || m.EditField == 2 {
					// Don't pass enter to title or order textareas
					return m, nil
				}
			}

			// Pass key to active textarea
			switch m.EditField {
			case 0:
				m.TitleInput, cmd = m.TitleInput.Update(msg)
				cmds = append(cmds, cmd)
			case 1:
				m.ContentInput, cmd = m.ContentInput.Update(msg)
				cmds = append(cmds, cmd)
			case 2:
				m.OrderInput, cmd = m.OrderInput.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	if len(cmds) > 0 {
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

func (m Model) handleMenuKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.MenuSelected > 0 {
			m.MenuSelected--
		}
	case "down", "j":
		if m.MenuSelected < 1 { // We have 2 menu items (0 and 1)
			m.MenuSelected++
		}
	case "enter":
		// Open the selected submenu
		switch m.MenuSelected {
		case 0:
			m.CurrentView = UsersView
		case 1:
			m.CurrentView = InfoBoxesView
		}
	}
	return m, nil
}

func (m Model) handleUsersKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Go back to menu
		m.CurrentView = MenuView
		return m, nil
	case "up", "k":
		if m.Selected > 0 {
			m.Selected--
			if m.Selected < m.Offset {
				m.Offset = m.Selected
			}
		}
	case "down", "j":
		if len(m.Users) > 0 && m.Selected < len(m.Users)-1 {
			m.Selected++
			if m.Selected >= m.Offset+common.DefaultItemsPerPage {
				m.Offset = m.Selected - common.DefaultItemsPerPage + 1
			}
		}
	case "m":
		if len(m.Users) > 0 && m.Selected < len(m.Users) {
			selectedUser := m.Users[m.Selected]
			if selectedUser.IsAdmin {
				m.Error = "Cannot mute admin user"
				return m, nil
			}
			if selectedUser.Id == m.AdminId {
				m.Error = "Cannot mute yourself"
				return m, nil
			}
			if selectedUser.Muted {
				m.Error = "User is already muted"
				return m, nil
			}
			return m, muteUser(selectedUser.Id)
		}
	case "K":
		if len(m.Users) > 0 && m.Selected < len(m.Users) {
			selectedUser := m.Users[m.Selected]
			if selectedUser.IsAdmin {
				m.Error = "Cannot kick admin user"
				return m, nil
			}
			if selectedUser.Id == m.AdminId {
				m.Error = "Cannot kick yourself"
				return m, nil
			}
			return m, kickUser(selectedUser.Id)
		}
	}
	return m, nil
}

func (m Model) handleInfoBoxesKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Handle delete confirmation
	if m.ConfirmDelete {
		switch msg.String() {
		case "y", "Y":
			// Confirm deletion
			m.ConfirmDelete = false
			return m, deleteInfoBox(m.DeleteBoxId)
		case "n", "N", "esc":
			// Cancel deletion
			m.ConfirmDelete = false
			m.DeleteBoxId = uuid.Nil
			m.Status = "Deletion cancelled"
		}
		return m, nil
	}

	// Normal navigation
	switch msg.String() {
	case "esc":
		// Go back to menu
		m.CurrentView = MenuView
		return m, nil
	case "up", "k":
		if m.BoxSelected > 0 {
			m.BoxSelected--
			if m.BoxSelected < m.BoxOffset {
				m.BoxOffset = m.BoxSelected
			}
		}
	case "down", "j":
		if len(m.InfoBoxes) > 0 && m.BoxSelected < len(m.InfoBoxes)-1 {
			m.BoxSelected++
			if m.BoxSelected >= m.BoxOffset+common.DefaultItemsPerPage {
				m.BoxOffset = m.BoxSelected - common.DefaultItemsPerPage + 1
			}
		}
	case "n":
		// Create new info box
		m.Editing = true
		m.EditBox = &domain.InfoBox{
			Title:    "",
			Content:  "",
			OrderNum: len(m.InfoBoxes) + 1,
			Enabled:  true,
		}
		m.initializeTextareas(m.EditBox)
		return m, textarea.Blink
	case "enter", "e":
		// Edit existing info box
		if len(m.InfoBoxes) > 0 && m.BoxSelected < len(m.InfoBoxes) {
			box := m.InfoBoxes[m.BoxSelected]
			m.Editing = true
			m.EditBox = &box
			m.initializeTextareas(m.EditBox)
			return m, textarea.Blink
		}
	case "d":
		if len(m.InfoBoxes) > 0 && m.BoxSelected < len(m.InfoBoxes) {
			// Show confirmation prompt
			m.ConfirmDelete = true
			m.DeleteBoxId = m.InfoBoxes[m.BoxSelected].Id
		}
	case "t":
		if len(m.InfoBoxes) > 0 && m.BoxSelected < len(m.InfoBoxes) {
			return m, toggleInfoBox(m.InfoBoxes[m.BoxSelected].Id)
		}
	}
	return m, nil
}

func (m Model) handleEditingKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Check if any textarea is focused
	isFocused := m.TitleInput.Focused() || m.ContentInput.Focused() || m.OrderInput.Focused()

	if !isFocused {
		// No textarea focused - handle field navigation
		switch msg.String() {
		case "esc":
			// Cancel editing and go back
			m.Editing = false
			m.EditBox = nil
			m.Status = "Edit cancelled"
			return m, nil

		case "ctrl+s":
			// Save the info box
			m.saveFromTextareas()
			return m, saveInfoBox(m.EditBox)

		case "up", "k":
			// Move to previous field
			m.saveFromTextareas()
			if m.EditField > 0 {
				m.EditField--
			}
			return m, nil

		case "down", "j":
			// Move to next field
			m.saveFromTextareas()
			if m.EditField < 2 {
				m.EditField++
			}
			return m, nil

		case "enter":
			// Start editing the current field
			m.focusCurrentField()
			return m, nil
		}
	} else {
		// Textarea is focused - handle editing controls
		switch msg.String() {
		case "esc":
			// Blur current textarea (exit editing mode for field)
			m.saveFromTextareas()
			m.TitleInput.Blur()
			m.ContentInput.Blur()
			m.OrderInput.Blur()
			return m, nil

		case "ctrl+s":
			// Save the info box
			m.saveFromTextareas()
			return m, saveInfoBox(m.EditBox)

		case "tab":
			// Move to next field
			m.saveFromTextareas()
			m.EditField = (m.EditField + 1) % 3
			m.focusCurrentField()
			return m, nil

		case "shift+tab":
			// Move to previous field
			m.saveFromTextareas()
			m.EditField = (m.EditField + 2) % 3
			m.focusCurrentField()
			return m, nil

		case "enter":
			// Prevent newlines in single-line fields (title and order)
			if m.EditField == 0 || m.EditField == 2 {
				// Block enter in title and order fields
				return m, nil
			}
			// Allow enter in content field (field 1) - pass to textarea
		}
	}

	// Let the active textarea handle the key (if focused)
	return m, nil
}

// saveFromTextareas copies textarea values to EditBox
func (m *Model) saveFromTextareas() {
	if m.EditBox == nil {
		return
	}
	m.EditBox.Title = m.TitleInput.Value()
	m.EditBox.Content = m.ContentInput.Value()

	// Parse order number
	orderStr := strings.TrimSpace(m.OrderInput.Value())
	if order, err := strconv.Atoi(orderStr); err == nil {
		m.EditBox.OrderNum = order
	}
}

// focusCurrentField focuses the textarea for the current field
func (m *Model) focusCurrentField() {
	m.TitleInput.Blur()
	m.ContentInput.Blur()
	m.OrderInput.Blur()

	switch m.EditField {
	case 0:
		m.TitleInput.Focus()
	case 1:
		m.ContentInput.Focus()
	case 2:
		m.OrderInput.Focus()
	}
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render("admin panel"))
	s.WriteString("\n\n")

	// Render appropriate view
	switch m.CurrentView {
	case MenuView:
		s.WriteString(m.renderMenu())
	case UsersView:
		s.WriteString(m.renderUsersView())
	case InfoBoxesView:
		if m.Editing {
			s.WriteString(m.renderEditView())
		} else {
			s.WriteString(m.renderInfoBoxesView())
		}
	}

	// Status messages
	if m.Status != "" {
		s.WriteString("\n")
		s.WriteString(common.ListStatusStyle.Render(m.Status))
	}

	if m.Error != "" {
		s.WriteString("\n")
		s.WriteString(common.ListErrorStyle.Render("Error: " + m.Error))
	}

	return s.String()
}

func (m Model) renderMenu() string {
	var s strings.Builder

	menuItems := []string{"Manage Users", "Manage Info Boxes"}

	for i, item := range menuItems {
		if i == m.MenuSelected {
			text := common.ListItemSelectedStyle.Render(item)
			s.WriteString(common.ListSelectedPrefix + text)
		} else {
			s.WriteString(common.ListUnselectedPrefix + common.ListItemStyle.Render(item))
		}
		s.WriteString("\n")
	}

	s.WriteString("\n\n")
	s.WriteString(common.ListBadgeStyle.Render("select an option to continue"))

	return s.String()
}

func (m Model) renderUsersView() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("manage users (%d users)", len(m.Users))))
	s.WriteString("\n\n")

	if len(m.Users) == 0 {
		s.WriteString(common.ListEmptyStyle.Render("No users found."))
		s.WriteString("\n\n")
		s.WriteString(common.ListBadgeStyle.Render("Keys: esc: back"))
		return s.String()
	}

	start := m.Offset
	end := min(start+common.DefaultItemsPerPage, len(m.Users))

	for i := start; i < end; i++ {
		user := m.Users[i]
		username := "@" + user.Username
		var badges []string

		if user.IsAdmin {
			badges = append(badges, "[ADMIN]")
		}
		if user.Muted {
			badges = append(badges, "[MUTED]")
		}

		badge := ""
		if len(badges) > 0 {
			badge = " " + strings.Join(badges, " ")
		}

		if i == m.Selected {
			text := common.ListItemSelectedStyle.Render(username + badge)
			s.WriteString(common.ListSelectedPrefix + text)
		} else if user.Muted {
			text := username + common.ListBadgeMutedStyle.Render(badge)
			s.WriteString(common.ListUnselectedPrefix + common.ListItemStyle.Render(text))
		} else {
			text := username + common.ListBadgeStyle.Render(badge)
			s.WriteString(common.ListUnselectedPrefix + common.ListItemStyle.Render(text))
		}
		s.WriteString("\n")
	}

	if len(m.Users) > common.DefaultItemsPerPage {
		s.WriteString("\n")
		paginationText := fmt.Sprintf("showing %d-%d of %d", start+1, end, len(m.Users))
		s.WriteString(common.ListBadgeStyle.Render(paginationText))
	}

	s.WriteString("\n\n")
	s.WriteString(common.ListBadgeStyle.Render("Keys: ↑/↓: navigate • m: mute • K: kick • esc: back"))

	return s.String()
}

func (m Model) renderInfoBoxesView() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("manage info boxes (%d boxes)", len(m.InfoBoxes))))
	s.WriteString("\n\n")

	if len(m.InfoBoxes) == 0 {
		s.WriteString(common.ListEmptyStyle.Render("No info boxes found. Press 'n' to add one."))
		s.WriteString("\n\n")
		s.WriteString(common.ListBadgeStyle.Render("Keys: n: add • esc: back"))
		return s.String()
	}

	start := m.BoxOffset
	end := min(start+common.DefaultItemsPerPage, len(m.InfoBoxes))

	for i := start; i < end; i++ {
		box := m.InfoBoxes[i]
		title := box.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}

		status := ""
		if box.Enabled {
			status = common.ListBadgeEnabledStyle.Render(" [ON]")
		} else {
			status = common.ListBadgeMutedStyle.Render(" [OFF]")
		}

		order := common.ListBadgeStyle.Render(fmt.Sprintf("#%d", box.OrderNum))

		if i == m.BoxSelected {
			text := common.ListItemSelectedStyle.Render(order + " " + title + status)
			s.WriteString(common.ListSelectedPrefix + text)
		} else {
			text := order + " " + title + status
			s.WriteString(common.ListUnselectedPrefix + common.ListItemStyle.Render(text))
		}
		s.WriteString("\n")
	}

	if len(m.InfoBoxes) > common.DefaultItemsPerPage {
		s.WriteString("\n")
		paginationText := fmt.Sprintf("showing %d-%d of %d", start+1, end, len(m.InfoBoxes))
		s.WriteString(common.ListBadgeStyle.Render(paginationText))
	}

	s.WriteString("\n\n")

	// Show confirmation prompt if deleting
	if m.ConfirmDelete {
		s.WriteString(common.ListErrorStyle.Render("Delete this info box? (y/n)"))
	} else {
		s.WriteString(common.ListBadgeStyle.Render("Keys: ↑/↓: navigate • n: add • enter: edit • d: delete • t: toggle • esc: back"))
	}

	return s.String()
}

func (m Model) renderEditView() string {
	var s strings.Builder

	if m.EditBox.Id == uuid.Nil {
		s.WriteString(common.CaptionStyle.Render("add new info box"))
	} else {
		s.WriteString(common.CaptionStyle.Render("edit info box"))
	}
	s.WriteString("\n\n")

	// Render textareas with labels and visual separation
	fieldNames := []string{"Title", "Content (Markdown)", "Order Number"}
	textareas := []textarea.Model{m.TitleInput, m.ContentInput, m.OrderInput}

	for i, name := range fieldNames {
		// Add space between fields (except before first)
		if i > 0 {
			s.WriteString("\n")
		}

		// Add indicator for focused field
		indicator := "  "
		if i == m.EditField {
			indicator = "▶ "
		}

		// Render label
		labelStyle := common.ListItemStyle
		if i == m.EditField {
			labelStyle = common.ListItemSelectedStyle
		}
		s.WriteString(labelStyle.Render(indicator + name + ":"))
		s.WriteString("\n")

		// Render textarea
		s.WriteString(textareas[i].View())
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(common.ListBadgeStyle.Render("Keys: tab/shift+tab: switch field • ctrl+s: save • esc: cancel"))
	s.WriteString("\n\n")
	s.WriteString(common.ListBadgeStyle.Render("Note: Content supports Markdown. Use {{SSH_PORT}} for port substitution."))

	return s.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
