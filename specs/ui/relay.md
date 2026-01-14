# Relay Management View

This document specifies the Relay Management view, which allows admins to manage ActivityPub relay subscriptions.

---

## Overview

The Relay Management view is an admin-only panel for managing ActivityPub relay subscriptions. It supports:
- Adding new relay subscriptions (FediBuzz and YUKIMOCHI types)
- Unsubscribing from relays
- Pausing/resuming active relays
- Retrying failed relay subscriptions
- Clearing all relay content from the timeline

---

## Data Structure

```go
type Model struct {
    AdminId   uuid.UUID
    AdminAcct *domain.Account
    Config    *util.AppConfig
    Relays    []domain.Relay
    Selected  int
    Offset    int               // Pagination offset
    Width     int
    Height    int
    Status    string            // Success message
    Error     string            // Error message
    Input     textinput.Model   // For entering relay URL
    Adding    bool              // Input mode for adding relay
}
```

---

## View Layout

### Normal Mode

```
┌─────────────────────────────────────────────────────────────┐
│ relay management (3 relays)                                  │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ ▸ relay.fedi.buzz [active]                                   │  ← Selected
│   relay.toot.yukimochi.jp [pending]                          │
│   relay.example.com [failed]                                 │
│                                                              │
│ keys: a add | d delete | p pause/resume | r retry | x clear │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Adding Mode

```
┌─────────────────────────────────────────────────────────────┐
│ relay management (3 relays)                                  │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ Enter relay URL:                                             │
│ ▸ [ relay.example.com                                    ]  │
│                                                              │
│ enter: subscribe | esc: cancel                               │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Keyboard Shortcuts

### Normal Mode

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `a` | Enter add relay mode |
| `d` | Delete/unsubscribe from selected relay |
| `p` | Pause/resume selected relay (active only) |
| `r` | Retry selected relay (failed only) |
| `x` | Clear all relay content from timeline |

### Adding Mode

| Key | Action |
|-----|--------|
| `Enter` | Subscribe to entered relay |
| `Esc` | Cancel and return to normal mode |

---

## Relay Status Badges

```go
var statusBadge string
switch relay.Status {
case "active":
    if relay.Paused {
        statusBadge = common.ListBadgeMutedStyle.Render("[paused]")
    } else {
        statusBadge = common.ListBadgeStyle.Render("[active]")
    }
case "pending":
    statusBadge = common.ListBadgeMutedStyle.Render("[pending]")
case "failed":
    statusBadge = common.ListErrorStyle.Render("[failed]")
default:
    statusBadge = common.ListBadgeMutedStyle.Render("[" + relay.Status + "]")
}
```

---

## URL Normalization

Relay URLs are normalized to full actor URIs:

```go
func normalizeRelayURL(input string) string {
    input = strings.TrimSpace(input)

    // If it already looks like a full URI, use it
    if strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://") {
        return input
    }

    // Otherwise, assume it's a domain and construct the actor URI
    return fmt.Sprintf("https://%s/actor", input)
}
```

### Example Inputs

| Input | Normalized |
|-------|------------|
| `relay.fedi.buzz` | `https://relay.fedi.buzz/actor` |
| `relay.fedi.buzz/tag/music` | `https://relay.fedi.buzz/tag/music` |
| `https://relay.example.com/actor` | `https://relay.example.com/actor` |

---

## Add Relay Flow

```
User presses 'a'
      │
      ▼
Enter Adding Mode
      │
      ├── Input.Focus()
      └── Show text input
            │
            ▼
User enters URL and presses Enter
      │
      ▼
Validate Input
      │
      ├── Empty → Show error
      └── Valid → Subscribe
            │
            ├── Normalize URL
            ├── Send Follow to relay actor
            └── Show "Subscription request sent (pending acceptance)"
                  │
                  ▼
Reload Relays List
```

### Subscribe Command

```go
func subscribeToRelay(adminAcct *domain.Account, relayURL string, config *util.AppConfig) tea.Cmd {
    return func() tea.Msg {
        actorURI := normalizeRelayURL(relayURL)
        err := activitypub.SendRelayFollow(adminAcct, actorURI, config)
        if err != nil {
            return relayAddedMsg{err: err}
        }
        return relayAddedMsg{err: nil}
    }
}
```

---

## Unsubscribe Flow

```
User presses 'd'
      │
      ▼
Send Undo Follow
      │
      ├── ActivityPub Undo(Follow) to relay
      └── Delete from local database
            │
            ▼
Show "Relay unsubscribed"
      │
      ▼
Reload Relays List
```

### Unsubscribe Command

```go
func unsubscribeFromRelay(adminAcct *domain.Account, relay *domain.Relay, config *util.AppConfig) tea.Cmd {
    return func() tea.Msg {
        // Send Undo Follow to relay
        err := activitypub.SendRelayUnfollow(adminAcct, relay, config)
        // Delete locally even if remote fails

        database := db.GetDB()
        database.DeleteRelay(relay.Id)

        return relayDeletedMsg{id: relay.Id, err: nil}
    }
}
```

---

## Pause/Resume

Only active relays can be paused. Paused relays log incoming content but don't save to timeline.

```go
case "p":
    if len(m.Relays) > 0 && m.Selected < len(m.Relays) {
        selectedRelay := m.Relays[m.Selected]
        if selectedRelay.Status == "active" {
            newPaused := !selectedRelay.Paused
            return m, toggleRelayPause(selectedRelay.Id, newPaused)
        } else {
            m.Error = "Only active relays can be paused/resumed"
        }
    }

func toggleRelayPause(relayId uuid.UUID, paused bool) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        err := database.UpdateRelayPaused(relayId, paused)
        return relayPausedMsg{id: relayId, paused: paused, err: err}
    }
}
```

---

## Retry Failed Relay

Only failed relays can be retried. This deletes the old record and sends a new Follow.

```go
case "r":
    if len(m.Relays) > 0 && m.Selected < len(m.Relays) {
        selectedRelay := m.Relays[m.Selected]
        if selectedRelay.Status == "failed" {
            return m, retryRelay(m.AdminAcct, &selectedRelay, m.Config)
        } else {
            m.Error = "Only failed relays can be retried"
        }
    }

func retryRelay(adminAcct *domain.Account, relay *domain.Relay, config *util.AppConfig) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        database.DeleteRelay(relay.Id)

        err := activitypub.SendRelayFollow(adminAcct, relay.ActorURI, config)
        return relayRetryMsg{err: err}
    }
}
```

---

## Clear Relay Content

Deletes all activities from relays in the timeline:

```go
case "x":
    m.Status = "Deleting relay content..."
    return m, deleteRelayContent()

func deleteRelayContent() tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        count, err := database.DeleteRelayActivities()
        return relayContentDeletedMsg{count: count, err: err}
    }
}

case relayContentDeletedMsg:
    if msg.err != nil {
        m.Error = msg.err.Error()
    } else {
        m.Status = fmt.Sprintf("Deleted %d relay activities", msg.count)
    }
```

---

## Domain Extraction

Display friendly domain names from actor URIs:

```go
func extractDomain(uri string) string {
    uri = strings.TrimPrefix(uri, "https://")
    uri = strings.TrimPrefix(uri, "http://")

    parts := strings.SplitN(uri, "/", 2)
    if len(parts) > 0 {
        return parts[0]
    }
    return uri
}
```

---

## Data Loading

```go
func loadRelays() tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        err, relays := database.ReadAllRelays()
        if err != nil || relays == nil {
            return relaysLoadedMsg{relays: []domain.Relay{}}
        }
        return relaysLoadedMsg{relays: *relays}
    }
}
```

---

## Empty State

```go
if len(m.Relays) == 0 {
    s.WriteString(common.ListEmptyStyle.Render("No relays configured."))
    s.WriteString("\n\n")
    s.WriteString(common.HelpStyle.Render("Press 'a' to add a relay"))
}
```

---

## Message Types

```go
type relaysLoadedMsg struct {
    relays []domain.Relay
}

type relayAddedMsg struct {
    err error
}

type relayDeletedMsg struct {
    id  uuid.UUID
    err error
}

type relayRetryMsg struct {
    err error
}

type relayPausedMsg struct {
    id     uuid.UUID
    paused bool
    err    error
}

type relayContentDeletedMsg struct {
    count int64
    err   error
}
```

---

## Initialization

```go
func InitialModel(adminId uuid.UUID, adminAcct *domain.Account, config *util.AppConfig, width, height int) Model {
    ti := textinput.New()
    ti.Placeholder = "relay.example.com or https://relay.example.com/actor"
    ti.CharLimit = 256
    ti.Width = 60

    return Model{
        AdminId:   adminId,
        AdminAcct: adminAcct,
        Config:    config,
        Relays:    []domain.Relay{},
        Selected:  0,
        Offset:    0,
        Width:     width,
        Height:    height,
        Status:    "",
        Error:     "",
        Input:     ti,
        Adding:    false,
    }
}

func (m Model) Init() tea.Cmd {
    return loadRelays()
}
```

---

## Access Control

This view is only accessible to admin users. Access is controlled in supertui.go.

---

## Source Files

- `ui/relay/relay.go` - Relay management view implementation
- `ui/common/styles.go` - List styles
- `activitypub/outbox.go` - SendRelayFollow, SendRelayUnfollow
- `db/db.go` - ReadAllRelays, DeleteRelay, UpdateRelayPaused, DeleteRelayActivities
- `domain/relay.go` - Relay entity with Status, Paused fields
