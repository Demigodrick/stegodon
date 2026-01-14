package infoboxes

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/google/uuid"
	"log"
)

type Model struct {
	InfoBoxes []domain.InfoBox
	Selected  int
	Offset    int
	Width     int
	Height    int
	Status    string
	Error     string
	Editing   bool         // Are we in edit mode?
	EditBox   *domain.InfoBox // The box being edited
	EditField int          // Which field is being edited (0=title, 1=content, 2=order)
	EditValue string       // Current value being edited
}

func InitialModel(width, height int) Model {
	return Model{
		InfoBoxes: []domain.InfoBox{},
		Selected:  0,
		Offset:    0,
		Width:     width,
		Height:    height,
		Status:    "",
		Error:     "",
		Editing:   false,
	}
}

func (m Model) Init() tea.Cmd {
	return loadInfoBoxes()
}

type infoBoxesLoadedMsg struct {
	boxes []domain.InfoBox
}

type infoBoxSavedMsg struct{}
type infoBoxDeletedMsg struct{}
type infoBoxToggledMsg struct{}

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
			// New box
			box.Id = uuid.New()
			box.CreatedAt = time.Now()
			box.UpdatedAt = time.Now()
			err = database.CreateInfoBox(box)
		} else {
			// Update existing
			err = database.UpdateInfoBox(box)
		}
		if err != nil {
			log.Printf("Failed to save info box: %v", err)
			return common.SessionState(0) // Error, but reload anyway
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
	case infoBoxesLoadedMsg:
		m.InfoBoxes = msg.boxes
		m.Selected = 0
		m.Offset = 0
		if m.Selected >= len(m.InfoBoxes) {
			m.Selected = max(0, len(m.InfoBoxes)-1)
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

		// Handle editing mode
		if m.Editing {
			return m.handleEditingKeys(msg)
		}

		// Handle normal navigation
		switch msg.String() {
		case "up", "k":
			if m.Selected > 0 {
				m.Selected--
				if m.Selected < m.Offset {
					m.Offset = m.Selected
				}
			}
		case "down", "j":
			if len(m.InfoBoxes) > 0 && m.Selected < len(m.InfoBoxes)-1 {
				m.Selected++
				if m.Selected >= m.Offset+common.DefaultItemsPerPage {
					m.Offset = m.Selected - common.DefaultItemsPerPage + 1
				}
			}
		case "n":
			// Add new info box
			m.Editing = true
			m.EditBox = &domain.InfoBox{
				Title:     "",
				Content:   "",
				OrderNum:  len(m.InfoBoxes) + 1,
				Enabled:   true,
			}
			m.EditField = 0
			m.EditValue = ""
		case "e":
			// Edit selected info box
			if len(m.InfoBoxes) > 0 && m.Selected < len(m.InfoBoxes) {
				box := m.InfoBoxes[m.Selected]
				m.Editing = true
				m.EditBox = &box
				m.EditField = 0
				m.EditValue = box.Title
			}
		case "d":
			// Delete selected info box
			if len(m.InfoBoxes) > 0 && m.Selected < len(m.InfoBoxes) {
				return m, deleteInfoBox(m.InfoBoxes[m.Selected].Id)
			}
		case "t":
			// Toggle enabled/disabled
			if len(m.InfoBoxes) > 0 && m.Selected < len(m.InfoBoxes) {
				return m, toggleInfoBox(m.InfoBoxes[m.Selected].Id)
			}
		}
	}

	return m, nil
}

func (m Model) handleEditingKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel editing
		m.Editing = false
		m.EditBox = nil
		m.Status = "Edit cancelled"
		return m, nil

	case "tab":
		// Move to next field
		m.saveCurrentField()
		m.EditField++
		if m.EditField > 2 {
			m.EditField = 0
		}
		m.loadFieldValue()
		return m, nil

	case "enter":
		// Save if on last field, otherwise move to next
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
		// Add character to current field
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
		// Parse order number
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

	if m.Editing {
		return m.renderEditView()
	}

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("website info boxes (%d boxes)", len(m.InfoBoxes))))
	s.WriteString("\n\n")

	if len(m.InfoBoxes) == 0 {
		s.WriteString(common.ListEmptyStyle.Render("No info boxes found. Press 'n' to add one."))
		s.WriteString("\n\n")
		s.WriteString(common.ListBadgeStyle.Render("Keys: n: add • tab: back"))
		return s.String()
	}

	start := m.Offset
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

		if i == m.Selected {
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

	s.WriteString("\n")

	if m.Status != "" {
		s.WriteString(common.ListStatusStyle.Render(m.Status))
		s.WriteString("\n")
	}

	if m.Error != "" {
		s.WriteString(common.ListErrorStyle.Render("Error: " + m.Error))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(common.ListBadgeStyle.Render("Keys: ↑/↓: navigate • n: add • e: edit • d: delete • t: toggle • tab: back"))

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

	// Show fields
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
