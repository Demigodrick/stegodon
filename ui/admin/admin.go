package admin

import (
	"fmt"
	"strings"
	"time"

	"log"

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
	AdminId       uuid.UUID
	CurrentView   AdminView
	MenuSelected  int // Which menu item is selected
	
	// User management
	Users    []domain.Account
	Selected int
	Offset   int
	
	// Info boxes management
	InfoBoxes   []domain.InfoBox
	BoxSelected int
	BoxOffset   int
	Editing     bool
	EditBox     *domain.InfoBox
	EditField   int
	EditValue   string
	
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
				return m.handleEditingKeys(msg)
			}
			return m.handleInfoBoxesKeys(msg)
		}
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
		m.Editing = true
		m.EditBox = &domain.InfoBox{
			Title:    "",
			Content:  "",
			OrderNum: len(m.InfoBoxes) + 1,
			Enabled:  true,
		}
		m.EditField = 0
		m.EditValue = ""
	case "e":
		if len(m.InfoBoxes) > 0 && m.BoxSelected < len(m.InfoBoxes) {
			box := m.InfoBoxes[m.BoxSelected]
			m.Editing = true
			m.EditBox = &box
			m.EditField = 0
			m.EditValue = box.Title
		}
	case "d":
		if len(m.InfoBoxes) > 0 && m.BoxSelected < len(m.InfoBoxes) {
			return m, deleteInfoBox(m.InfoBoxes[m.BoxSelected].Id)
		}
	case "t":
		if len(m.InfoBoxes) > 0 && m.BoxSelected < len(m.InfoBoxes) {
			return m, toggleInfoBox(m.InfoBoxes[m.BoxSelected].Id)
		}
	}
	return m, nil
}

func (m Model) handleEditingKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.Editing = false
		m.EditBox = nil
		m.Status = "Edit cancelled"
		return m, nil
	case "tab":
		m.saveCurrentField()
		m.EditField++
		if m.EditField > 2 {
			m.EditField = 0
		}
		m.loadFieldValue()
		return m, nil
	case "enter":
		if m.EditField == 2 {
			m.saveCurrentField()
			return m, saveInfoBox(m.EditBox)
		} else {
			m.saveCurrentField()
			m.EditField++
			m.loadFieldValue()
		}
		return m, nil
	case "backspace":
		if len(m.EditValue) > 0 {
			m.EditValue = m.EditValue[:len(m.EditValue)-1]
		}
		return m, nil
	default:
		if len(msg.String()) == 1 {
			m.EditValue += msg.String()
		}
		return m, nil
	}
}

func (m *Model) saveCurrentField() {
	if m.EditBox == nil {
		return
	}
	switch m.EditField {
	case 0:
		m.EditBox.Title = m.EditValue
	case 1:
		m.EditBox.Content = m.EditValue
	case 2:
		var order int
		fmt.Sscanf(m.EditValue, "%d", &order)
		m.EditBox.OrderNum = order
	}
}

func (m *Model) loadFieldValue() {
	if m.EditBox == nil {
		return
	}
	switch m.EditField {
	case 0:
		m.EditValue = m.EditBox.Title
	case 1:
		m.EditValue = m.EditBox.Content
	case 2:
		m.EditValue = fmt.Sprintf("%d", m.EditBox.OrderNum)
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
			status = common.ListBadgeStyle.Render(" [ON]")
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
	s.WriteString(common.ListBadgeStyle.Render("Keys: ↑/↓: navigate • n: add • e: edit • d: delete • t: toggle • esc: back"))

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

	fieldNames := []string{"Title", "Content (Markdown)", "Order Number"}
	for i, name := range fieldNames {
		if i == m.EditField {
			s.WriteString(common.ListItemSelectedStyle.Render(fmt.Sprintf("▶ %s: %s_", name, m.EditValue)))
		} else {
			var value string
			switch i {
			case 0:
				value = m.EditBox.Title
			case 1:
				value = m.EditBox.Content
				if len(value) > 50 {
					value = value[:47] + "..."
				}
			case 2:
				value = fmt.Sprintf("%d", m.EditBox.OrderNum)
			}
			s.WriteString(common.ListItemStyle.Render(fmt.Sprintf("  %s: %s", name, value)))
		}
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(common.ListBadgeStyle.Render("Keys: tab: next field • enter: save • esc: cancel"))
	s.WriteString("\n\n")
	s.WriteString(common.ListBadgeStyle.Render("Note: Content supports Markdown. Use {{SSH_PORT}} for port substitution."))

	return s.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
