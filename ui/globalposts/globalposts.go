package globalposts

import (
	"fmt"
	"log"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

var (
	timeStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Foreground(lipgloss.Color(common.COLOR_DIM))

	authorStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Foreground(lipgloss.Color(common.COLOR_USERNAME)).
			Bold(true)

	// Remote author uses secondary color to differentiate from local
	remoteAuthorStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_SECONDARY)).
				Bold(true)

	contentStyle = lipgloss.NewStyle().
			Align(lipgloss.Left)

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_DIM)).
			Italic(true)

	// Inverted styles for selected posts
	selectedTimeStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_WHITE))

	selectedAuthorStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_WHITE)).
				Bold(true)

	selectedContentStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_WHITE))
)

type Model struct {
	AccountId   uuid.UUID
	Posts       []domain.GlobalTimelinePost
	Offset      int  // Pagination offset
	Selected    int  // Currently selected post index
	Width       int
	Height      int
	isActive    bool // Track if this view is currently visible
	LocalDomain string
}

func InitialModel(accountId uuid.UUID, width, height int, localDomain string) Model {
	return Model{
		AccountId:   accountId,
		Posts:       []domain.GlobalTimelinePost{},
		Offset:      0,
		Selected:    0,
		Width:       width,
		Height:      height,
		isActive:    false,
		LocalDomain: localDomain,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

// refreshTickMsg is sent periodically to refresh the timeline
type refreshTickMsg struct{}

// tickRefresh returns a command that sends refreshTickMsg
func tickRefresh() tea.Cmd {
	return tea.Tick(common.TimelineRefreshSeconds*time.Second, func(t time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case common.DeactivateViewMsg:
		m.isActive = false
		return m, nil

	case common.ActivateViewMsg:
		m.isActive = true
		m.Selected = 0
		m.Offset = 0
		return m, loadGlobalPosts()

	case common.SessionState:
		if msg == common.UpdateNoteList {
			return m, loadGlobalPosts()
		}
		return m, nil

	case refreshTickMsg:
		if m.isActive {
			return m, loadGlobalPosts()
		}
		return m, nil

	case postsLoadedMsg:
		m.Posts = msg.posts
		if m.Selected >= len(m.Posts) {
			m.Selected = max(0, len(m.Posts)-1)
		}
		m.Offset = m.Selected

		if m.isActive {
			return m, tickRefresh()
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.Selected > 0 {
				m.Selected--
				m.Offset = m.Selected
			}
		case "down", "j":
			if len(m.Posts) > 0 && m.Selected < len(m.Posts)-1 {
				m.Selected++
				m.Offset = m.Selected
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("global (%d posts)", len(m.Posts))))
	s.WriteString("\n\n")

	if len(m.Posts) == 0 {
		s.WriteString(emptyStyle.Render("No posts yet."))
	} else {
		leftPanelWidth := common.CalculateLeftPanelWidth(m.Width)
		rightPanelWidth := common.CalculateRightPanelWidth(m.Width, leftPanelWidth)
		contentWidth := common.CalculateContentWidth(rightPanelWidth, 2)

		itemsPerPage := common.DefaultItemsPerPage
		start := m.Offset
		end := start + itemsPerPage
		if end > len(m.Posts) {
			end = len(m.Posts)
		}

		for i := start; i < end; i++ {
			post := m.Posts[i]

			timeStr := formatTime(post.CreatedAt)
			if post.ReplyCount == 1 {
				timeStr = fmt.Sprintf("%s · 1 reply", timeStr)
			} else if post.ReplyCount > 1 {
				timeStr = fmt.Sprintf("%s · %d replies", timeStr, post.ReplyCount)
			}
			if post.LikeCount > 0 {
				timeStr = fmt.Sprintf("%s · ⭐ %d", timeStr, post.LikeCount)
			}
			if post.BoostCount > 0 {
				timeStr = fmt.Sprintf("%s · 🔁 %d", timeStr, post.BoostCount)
			}

			author := post.Username
			if !strings.HasPrefix(author, "@") {
				author = "@" + author
			}

			if i == m.Selected {
				selectedBg := lipgloss.NewStyle().
					Background(lipgloss.Color(common.COLOR_ACCENT)).
					Width(contentWidth)

				timeFormatted := selectedBg.Render(selectedTimeStyle.Render(timeStr))
				authorFormatted := selectedBg.Render(selectedAuthorStyle.Render(author))

				processedContent := post.Message
				if !post.IsRemote {
					processedContent = util.UnescapeHTML(processedContent)
					processedContent = util.MarkdownLinksToTerminal(processedContent)
				}
				highlightedContent := util.HighlightHashtagsTerminal(processedContent)
				highlightedContent = util.HighlightMentionsTerminal(highlightedContent, m.LocalDomain)

				contentFormatted := selectedBg.Render(selectedContentStyle.Render(highlightedContent))
				s.WriteString(timeFormatted + "\n")
				s.WriteString(authorFormatted + "\n")
				s.WriteString(contentFormatted)
			} else {
				unselectedStyle := lipgloss.NewStyle().
					Width(contentWidth)

				processedContent := post.Message
				if !post.IsRemote {
					processedContent = util.UnescapeHTML(processedContent)
					processedContent = util.MarkdownLinksToTerminal(processedContent)
				}
				highlightedContent := util.HighlightHashtagsTerminal(processedContent)
				highlightedContent = util.HighlightMentionsTerminal(highlightedContent, m.LocalDomain)

				var authorFormatted string
				if !post.IsRemote {
					authorFormatted = unselectedStyle.Render(authorStyle.Render(author))
				} else {
					authorFormatted = unselectedStyle.Render(remoteAuthorStyle.Render(author))
				}

				timeFormatted := unselectedStyle.Render(timeStyle.Render(timeStr))
				contentFormatted := unselectedStyle.Render(contentStyle.Render(highlightedContent))

				s.WriteString(timeFormatted + "\n")
				s.WriteString(authorFormatted + "\n")
				s.WriteString(contentFormatted)
			}

			s.WriteString("\n\n")
		}
	}

	return s.String()
}

// postsLoadedMsg is sent when posts are loaded
type postsLoadedMsg struct {
	posts []domain.GlobalTimelinePost
}

// loadGlobalPosts loads the global timeline
func loadGlobalPosts() tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, posts := database.ReadGlobalTimelinePosts(common.HomeTimelinePostLimit, 0)
		if err != nil {
			log.Printf("Failed to load global timeline: %v", err)
			return postsLoadedMsg{posts: []domain.GlobalTimelinePost{}}
		}

		return postsLoadedMsg{posts: posts}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func formatTime(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		mins := int(duration.Minutes())
		return fmt.Sprintf("%dm ago", mins)
	} else if duration < common.HoursPerDay*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%dh ago", hours)
	} else {
		days := int(duration.Hours() / common.HoursPerDay)
		return fmt.Sprintf("%dd ago", days)
	}
}
