# HomeTimeline View

This document specifies the HomeTimeline view, which displays the combined local and federated feed with auto-refresh functionality.

---

## Overview

The HomeTimeline is the primary content consumption view, showing a unified timeline of:
- Local posts from users on this server
- Federated posts from followed remote accounts
- Content from relay subscriptions

Posts are sorted in reverse chronological order with automatic refresh every 30 seconds.

---

## Data Structure

```go
type Model struct {
    AccountId   uuid.UUID
    Posts       []domain.HomePost
    Offset      int              // Pagination offset
    Selected    int              // Currently selected post index
    Width       int
    Height      int
    isActive    bool             // Track if view is visible (prevents ticker leaks)
    showingURL  bool             // Toggle between content and URL display
    LocalDomain string           // For mention highlighting
}
```

### HomePost Structure

```go
type HomePost struct {
    NoteID     uuid.UUID    // Local note ID (if local)
    Author     string       // Display name or handle
    Content    string       // Post content
    Time       time.Time    // Creation timestamp
    ObjectURI  string       // ActivityPub object URI
    IsLocal    bool         // Local vs federated post
    ReplyCount int          // Number of replies
    LikeCount  int          // Number of likes
    BoostCount int          // Number of boosts
}
```

---

## View Layout

```
┌─────────────────────────────────────────────────────────────┐
│ home (42 posts)                                              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ ▸ 2h ago · 3 replies · ⭐ 5 · 🔁 2                           │  ← Selected
│   @alice                                                     │  ← Local (primary color)
│   Just posted something interesting! #programming            │
│                                                              │
│   5h ago · ⭐ 12                                              │
│   @bob@mastodon.social                                       │  ← Remote (secondary color)
│   Federated content from followed user                       │
│                                                              │
│   1d ago                                                     │
│   @charlie                                                   │
│   Another local post without engagement yet                  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Author Differentiation

Local and remote authors are styled differently:

```go
// Local authors - primary color
authorStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color(common.COLOR_USERNAME)).
    Bold(true)

// Remote authors - secondary color
remoteAuthorStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color(common.COLOR_SECONDARY)).
    Bold(true)
```

---

## Auto-Refresh Pattern

The timeline auto-refreshes while active to show new content:

```go
const TimelineRefreshSeconds = 30

type refreshTickMsg struct{}

func tickRefresh() tea.Cmd {
    return tea.Tick(TimelineRefreshSeconds * time.Second, func(t time.Time) tea.Msg {
        return refreshTickMsg{}
    })
}
```

### Lifecycle Management

```go
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    switch msg := msg.(type) {
    case common.ActivateViewMsg:
        m.isActive = true
        m.Selected = 0
        m.Offset = 0
        m.showingURL = false
        return m, loadHomePosts(m.AccountId)

    case common.DeactivateViewMsg:
        m.isActive = false
        return m, nil  // Stop ticker chain

    case refreshTickMsg:
        if m.isActive {
            return m, loadHomePosts(m.AccountId)
        }
        return m, nil  // Don't restart if inactive

    case postsLoadedMsg:
        m.Posts = msg.posts
        // Schedule next tick only if still active
        if m.isActive {
            return m, tickRefresh()
        }
        return m, nil
    }
}
```

This pattern prevents goroutine leaks when users navigate away.

---

## Navigation

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `Enter` | Open thread (if has replies) |
| `r` | Reply to selected post |
| `l` | Like/unlike selected post |
| `o` | Toggle URL display |

### Scroll Behavior

```go
case "up", "k":
    if m.Selected > 0 {
        m.Selected--
        m.Offset = m.Selected  // Keep selected at top
    }
    m.showingURL = false  // Reset URL view on navigation

case "down", "j":
    if m.Selected < len(m.Posts)-1 {
        m.Selected++
        m.Offset = m.Selected
    }
    m.showingURL = false
```

---

## URL Toggle

Press `o` to toggle between content and clickable URL:

```go
case "o":
    if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
        selectedPost := m.Posts[m.Selected]
        if util.IsURL(selectedPost.ObjectURI) {
            m.showingURL = !m.showingURL
        }
    }
```

### URL Display Mode

```
┌─────────────────────────────────────────────────────────────┐
│ ▸ 2h ago · ⭐ 5                                              │
│   @alice@mastodon.social                                     │
│   🔗 https://mastodon.social/users/alice/statuses/123456    │
│                                                              │
│   (Cmd+click to open, press 'o' to toggle back)             │
└─────────────────────────────────────────────────────────────┘
```

### URL Display Rendering

```go
if m.showingURL && post.ObjectURI != "" {
    osc8Link := util.FormatClickableURL(post.ObjectURI, common.MaxContentTruncateWidth, "🔗 ")
    hintText := "(Cmd+click to open, press 'o' to toggle back)"
    // ...render with background styling
}
```

Uses `util.FormatClickableURL()` for OSC 8 terminal hyperlinks.

---

## Reply Mode

Press `r` to reply to the selected post:

```go
case "r":
    if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
        selectedPost := m.Posts[m.Selected]
        replyURI := selectedPost.ObjectURI

        // For local posts without ObjectURI, use local: prefix
        if replyURI == "" && selectedPost.IsLocal && selectedPost.NoteID != uuid.Nil {
            replyURI = "local:" + selectedPost.NoteID.String()
        }

        if replyURI != "" {
            return m, func() tea.Msg {
                return common.ReplyToNoteMsg{
                    NoteURI: replyURI,
                    Author:  selectedPost.Author,
                    Preview: truncateFirstLine(selectedPost.Content),
                }
            }
        }
    }
```

---

## Thread View

Press `Enter` to open thread (only if post has replies):

```go
case "enter":
    if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
        selectedPost := m.Posts[m.Selected]

        // Skip if no replies
        if selectedPost.ReplyCount == 0 {
            return m, nil
        }

        return m, func() tea.Msg {
            return common.ViewThreadMsg{
                NoteURI:   selectedPost.ObjectURI,
                NoteID:    selectedPost.NoteID,
                Author:    selectedPost.Author,
                Content:   selectedPost.Content,
                CreatedAt: selectedPost.Time,
                IsLocal:   selectedPost.IsLocal,
            }
        }
    }
```

---

## Like/Unlike

Press `l` to toggle like on the selected post:

```go
case "l":
    if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
        selectedPost := m.Posts[m.Selected]
        noteURI := selectedPost.ObjectURI

        if noteURI == "" && selectedPost.IsLocal && selectedPost.NoteID != uuid.Nil {
            noteURI = "local:" + selectedPost.NoteID.String()
        }

        return m, func() tea.Msg {
            return common.LikeNoteMsg{
                NoteURI: noteURI,
                NoteID:  selectedPost.NoteID,
                IsLocal: selectedPost.IsLocal,
            }
        }
    }
```

---

## Data Loading

```go
func loadHomePosts(accountId uuid.UUID) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        err, posts := database.ReadHomeTimelinePosts(accountId, common.HomeTimelinePostLimit)
        if err != nil {
            return postsLoadedMsg{posts: []domain.HomePost{}}
        }
        return postsLoadedMsg{posts: *posts}
    }
}
```

### Post Limit

```go
const HomeTimelinePostLimit = 100  // From common package
```

---

## Engagement Display

Posts show engagement stats in the timestamp line:

```go
timeStr := formatTime(post.Time)

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
```

---

## Content Processing

### Markdown Links

Local posts have markdown links converted to terminal hyperlinks:

```go
processedContent := post.Content
if post.IsLocal {
    processedContent = util.MarkdownLinksToTerminal(processedContent)
}
```

### Hashtag Highlighting

```go
highlightedContent := util.HighlightHashtagsTerminal(processedContent)
```

### Mention Highlighting

```go
highlightedContent = util.HighlightMentionsTerminal(highlightedContent, m.LocalDomain)
```

### Content Truncation

```go
truncatedContent := util.TruncateVisibleLength(highlightedContent, common.MaxContentTruncateWidth)
```

---

## Empty State

```go
if len(m.Posts) == 0 {
    s.WriteString(emptyStyle.Render("No posts yet.\nFollow some accounts to see their posts here!"))
}
```

---

## Selection Highlighting

Selected posts use inverted colors:

```go
if i == m.Selected {
    selectedBg := lipgloss.NewStyle().
        Background(lipgloss.Color(common.COLOR_ACCENT)).
        Width(contentWidth)

    timeFormatted := selectedBg.Render(selectedTimeStyle.Render(timeStr))
    authorFormatted := selectedBg.Render(selectedAuthorStyle.Render(author))
    contentFormatted := selectedBg.Render(selectedContentStyle.Render(content))
}
```

---

## Pagination

```go
const DefaultItemsPerPage = 10

start := m.Offset
end := min(start + DefaultItemsPerPage, len(m.Posts))

for i := start; i < end; i++ {
    // Render post
}
```

---

## Initialization

```go
func InitialModel(accountId uuid.UUID, width, height int, localDomain string) Model {
    return Model{
        AccountId:   accountId,
        Posts:       []domain.HomePost{},
        Offset:      0,
        Selected:    0,
        Width:       width,
        Height:      height,
        isActive:    false,  // Start inactive
        showingURL:  false,
        LocalDomain: localDomain,
    }
}
```

---

## Source Files

- `ui/hometimeline/hometimeline.go` - HomeTimeline view implementation
- `ui/hometimeline/hometimeline_test.go` - Tests
- `ui/common/commands.go` - ReplyToNoteMsg, ViewThreadMsg, LikeNoteMsg
- `ui/common/constants.go` - TimelineRefreshSeconds, HomeTimelinePostLimit
- `db/db.go` - ReadHomeTimelinePosts
