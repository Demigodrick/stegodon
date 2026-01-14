# TUI Architecture

This document specifies the Terminal User Interface (TUI) architecture, including the BubbleTea MVC pattern, session state management, and view switching.

---

## Overview

Stegodon's TUI is built with [BubbleTea](https://github.com/charmbracelet/bubbletea), a Go framework based on The Elm Architecture (Model-View-Update). The main orchestrator (`supertui.go`) coordinates 13 distinct views, handling navigation, message routing, and lifecycle management.

---

## BubbleTea MVC Pattern

### Model-View-Update Cycle

```
┌─────────────────────────────────────────────────────────────┐
│                      BubbleTea Runtime                       │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│    ┌──────────┐      ┌──────────┐      ┌──────────┐         │
│    │  Model   │─────►│  Update  │─────►│   View   │         │
│    │  (State) │      │  (Logic) │      │ (Render) │         │
│    └──────────┘      └──────────┘      └──────────┘         │
│         ▲                  │                  │              │
│         │                  │                  │              │
│         │    tea.Cmd       │    string        │              │
│         └──────────────────┘                  ▼              │
│                                          Terminal            │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Core Interface

Each view implements the BubbleTea `Model` interface:

```go
type Model interface {
    Init() tea.Cmd
    Update(tea.Msg) (Model, tea.Cmd)
    View() string
}
```

| Method | Purpose |
|--------|---------|
| `Init()` | Return initial command (e.g., load data) |
| `Update(msg)` | Handle input, return new state and commands |
| `View()` | Render current state to string |

---

## Main Model Structure

The `MainModel` in `supertui.go` orchestrates all views:

```go
type MainModel struct {
    // View models
    Header            header.Model
    WriteNote         writenote.Model
    CreateUser        createuser.Model
    MyPosts           myposts.Model
    HomeTimeline      hometimeline.Model
    FollowUser        followuser.Model
    Followers         followers.Model
    Following         following.Model
    LocalUsers        localusers.Model
    AdminPanel        admin.Model
    RelayManagement   relay.Model
    DeleteAccount     deleteaccount.Model
    ThreadView        threadview.Model
    Notifications     notifications.Model

    // State
    Account           *domain.Account
    SessionState      common.SessionState
    PreviousState     common.SessionState
    Terminal          util.TerminalInfo

    // Dependencies
    Conf              *util.Config
    LocalDomain       string
}
```

---

## Session State

Session state determines which view is active:

```go
type SessionState uint

const (
    CreateNoteView     SessionState = iota  // 0 - Default/compose
    HomeTimelineView                        // 1 - Combined timeline
    MyPostsView                             // 2 - User's own posts
    CreateUserView                          // 3 - First-time username
    UpdateNoteList                          // 4 - Internal refresh signal
    FollowUserView                          // 5 - Follow remote users
    FollowersView                           // 6 - View followers
    FollowingView                           // 7 - View following
    LocalUsersView                          // 8 - Browse local users
    AdminPanelView                          // 9 - Admin user management
    RelayManagementView                     // 10 - Admin relay control
    DeleteAccountView                       // 11 - Account deletion
    ThreadView                              // 12 - Thread/conversation
    NotificationsView                       // 13 - Notifications center
)
```

### State Transitions

```
┌─────────────────────────────────────────────────────────────┐
│                    View Navigation                           │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   CreateNoteView ◄──Tab──► HomeTimelineView ◄──Tab──►       │
│         │                        │                           │
│         │                        │ Tab                       │
│         │                        ▼                           │
│   MyPostsView ◄──Tab──► FollowUserView ◄──Tab──►            │
│         │                        │                           │
│         │                        │ Tab                       │
│         │                        ▼                           │
│   FollowersView ◄──Tab──► FollowingView ◄──Tab──►           │
│         │                        │                           │
│         │                        │ Tab                       │
│         │                        ▼                           │
│   LocalUsersView ◄──Tab──► DeleteAccountView ◄──Tab──► ...  │
│                                                              │
│   Special: Ctrl+N → NotificationsView (from anywhere)       │
│   Special: Enter → ThreadView (from timeline views)         │
│   Special: Esc → Return to PreviousState                    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## View Switching

### Tab Navigation

The main navigation cycle is controlled by Tab/Shift+Tab:

```go
func getNextView(currentState SessionState) SessionState {
    switch currentState {
    case CreateNoteView:
        return HomeTimelineView
    case HomeTimelineView:
        return MyPostsView
    case MyPostsView:
        return FollowUserView
    case FollowUserView:
        return FollowersView
    case FollowersView:
        return FollowingView
    case FollowingView:
        return LocalUsersView
    case LocalUsersView:
        return DeleteAccountView
    // ... continues through all navigable views
    }
}
```

### Activation/Deactivation Messages

Views receive lifecycle messages for resource management:

```go
// Sent when view becomes active
type ActivateViewMsg struct{}

// Sent when view becomes inactive
type DeactivateViewMsg struct{}
```

This pattern prevents goroutine leaks in auto-refresh views:

```go
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    switch msg := msg.(type) {
    case common.ActivateViewMsg:
        m.isActive = true
        return m, tea.Batch(loadData(), tickRefresh())
    case common.DeactivateViewMsg:
        m.isActive = false
        return m, nil  // Stop refresh ticker
    case refreshTickMsg:
        if m.isActive {
            return m, tea.Batch(loadData(), tickRefresh())
        }
        return m, nil  // Don't restart if inactive
    }
}
```

---

## Message Routing

The main `Update()` method routes messages to views:

```go
func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Global key handling (Tab, Ctrl+N, Ctrl+C)
        switch msg.String() {
        case "tab":
            // Switch to next view
        case "shift+tab":
            // Switch to previous view
        case "ctrl+n":
            // Open notifications
        case "ctrl+c":
            // Quit application
        }

    case tea.WindowSizeMsg:
        // Handle terminal resize
        m.Terminal.Width = msg.Width
        m.Terminal.Height = msg.Height
    }

    // Route to active view only
    switch m.SessionState {
    case CreateNoteView:
        m.WriteNote, cmd = m.WriteNote.Update(msg)
    case HomeTimelineView:
        m.HomeTimeline, cmd = m.HomeTimeline.Update(msg)
    // ... other views
    }

    return m, tea.Batch(cmds...)
}
```

### Message Types

| Message | Purpose |
|---------|---------|
| `tea.KeyMsg` | Keyboard input |
| `tea.WindowSizeMsg` | Terminal resize |
| `common.EditNoteMsg` | Switch to edit mode |
| `common.DeleteNoteMsg` | Note was deleted |
| `common.ReplyToNoteMsg` | Switch to reply mode |
| `common.ViewThreadMsg` | Open thread view |
| `common.LikeNoteMsg` | Like/unlike a note |
| `notesLoadedMsg` | Notes finished loading |
| `refreshTickMsg` | Auto-refresh timer tick |

---

## View Rendering

The main `View()` method composes the final output:

```go
func (m MainModel) View() string {
    var view string

    // Handle special states
    if m.SessionState == CreateUserView {
        return m.CreateUser.ViewWithWidth(m.Terminal.Width, m.Terminal.Height)
    }

    // Standard layout with header
    header := m.Header.View()
    content := m.renderActiveView()

    return lipgloss.JoinVertical(lipgloss.Left, header, content)
}
```

### Layout Structure

```
┌─────────────────────────────────────────────────────────────┐
│ [stegodon] v1.4.3 | @username | 🔔 3                        │  Header
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────────┐  ┌────────────────────────────────┐   │
│  │                  │  │                                │   │
│  │   Left Panel    │  │        Right Panel             │   │
│  │   (Navigation)  │  │        (Content)               │   │
│  │                  │  │                                │   │
│  └──────────────────┘  └────────────────────────────────┘   │
│                                                              │
│  Hints: Tab: next • Shift+Tab: prev • Ctrl+N: notifications │
└─────────────────────────────────────────────────────────────┘
```

---

## Terminal Requirements

### Minimum Dimensions

```go
const (
    MinTermWidth  = 115
    MinTermHeight = 28
)
```

If terminal is too small, a warning is displayed instead of the TUI.

### Color Support

The application uses ANSI256 color profile for broad compatibility:

```go
func NewProgram(m Model) *tea.Program {
    return tea.NewProgram(
        m,
        tea.WithANSICompression(),
        tea.WithAltScreen(),
        tea.WithOutput(ansi256Writer),
    )
}
```

### Rendering

- 60 FPS default rendering rate
- Alt-screen mode (preserves terminal history)
- ANSI compression for efficient updates

---

## View Lifecycle

### Initialization Flow

```
SSH Connection
      │
      ▼
Account Lookup
      │
      ├── Account exists → HomeTimelineView
      └── No account → CreateUserView
            │
            ▼
      Username Selection
            │
            ▼
      Account Created → HomeTimelineView
```

### View Activation

```go
// When switching views
func switchToView(newState SessionState) tea.Cmd {
    return tea.Batch(
        func() tea.Msg { return common.DeactivateViewMsg{} }, // Old view
        func() tea.Msg { return common.ActivateViewMsg{} },   // New view
    )
}
```

---

## Keyboard Shortcuts

### Global (All Views)

| Key | Action |
|-----|--------|
| `Tab` | Next view |
| `Shift+Tab` | Previous view |
| `Ctrl+N` | Notifications |
| `Ctrl+C` | Quit |

### Navigation Views

| Key | Action |
|-----|--------|
| `↑/k` | Move up |
| `↓/j` | Move down |
| `Enter` | Open/select |
| `Esc` | Go back |

### Content Views

| Key | Action |
|-----|--------|
| `l` | Like/unlike |
| `r` | Reply |
| `u` | Edit (own posts) |
| `d` | Delete (own posts) |

---

## Error Handling

Errors are displayed in the UI without crashing:

```go
type ErrMsg struct {
    Err error
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    switch msg := msg.(type) {
    case util.ErrMsg:
        m.Error = msg.Err.Error()
        return m, nil
    }
}
```

---

## Auto-Refresh Pattern

Views with real-time updates use a safe refresh pattern:

```go
const refreshInterval = 30 * time.Second

type refreshTickMsg struct{}

func tickRefresh() tea.Cmd {
    return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
        return refreshTickMsg{}
    })
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    switch msg := msg.(type) {
    case refreshTickMsg:
        if m.isActive {
            return m, tea.Batch(loadData(), tickRefresh())
        }
        return m, nil  // Stop ticker when inactive
    }
}
```

This prevents goroutine leaks when users navigate away from auto-refreshing views.

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `bubbletea` | TUI framework |
| `bubbles` | Input components (textinput, viewport) |
| `lipgloss` | Styling and layout |

---

## Source Files

- `ui/supertui.go` - Main model and orchestration
- `ui/supertui_test.go` - Tests
- `ui/common/commands.go` - Shared message types
- `ui/common/styles.go` - Shared styles
- `ui/common/layout.go` - Layout calculations
- `ui/header/header.go` - Navigation header
