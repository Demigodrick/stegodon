# LocalUsers View

This document specifies the LocalUsers view, which allows browsing and following local users on the server.

---

## Overview

The LocalUsers view displays all users on the local server, allowing users to browse and follow/unfollow other local accounts. It supports pagination and immediate UI feedback for follow actions.

---

## Data Structure

```go
type Model struct {
    AccountId uuid.UUID
    Users     []domain.Account
    Following map[uuid.UUID]bool  // Track which users are being followed
    Selected  int
    Offset    int                 // Pagination offset
    Width     int
    Height    int
    Status    string              // Success message
    Error     string              // Error message
}
```

---

## View Layout

```
┌─────────────────────────────────────────────────────────────┐
│ local users (5)                                              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   @alice [you]                                               │  ← Current user (not selectable)
│ ▸ @bob [following]                                           │  ← Selected, already following
│   @charlie                                                   │  ← Not following
│   @diana [following]                                         │
│   @eve                                                       │
│                                                              │
│ Following @bob                                               │  ← Status message
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## User Display

### Current User

The current user is always shown first and is not selectable:

```go
for _, user := range m.Users {
    if user.Id == m.AccountId {
        text := "@" + user.Username + common.ListBadgeStyle.Render(" [you]")
        s.WriteString(common.ListUnselectedPrefix + common.ListItemStyle.Render(text))
        s.WriteString("\n")
        break
    }
}
```

### Other Users

Other users are displayed in a paginated, selectable list:

```go
for i := start; i < end; i++ {
    user := otherUsers[i]

    username := "@" + user.Username
    badge := ""
    if m.Following[user.Id] {
        badge = " [following]"
    }

    if i == m.Selected {
        text := common.ListItemSelectedStyle.Render(username + badge)
        s.WriteString(common.ListSelectedPrefix + text)
    } else {
        text := username + common.ListBadgeStyle.Render(badge)
        s.WriteString(common.ListUnselectedPrefix + common.ListItemStyle.Render(text))
    }
}
```

---

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `Enter` / `f` | Toggle follow/unfollow selected user |

---

## Follow/Unfollow Flow

```
User presses 'f' or 'Enter'
      │
      ▼
Check Current State
      │
      ├── Already following
      │     └── Unfollow (delete from database)
      │
      └── Not following
            ├── Follow (create in database)
            └── Create follow notification
                  │
                  ▼
Update Local State Immediately
      │
      ├── Update Following map
      ├── Show status message
      └── Clear status after 2s
```

---

## Follow Logic

```go
case "enter", "f":
    if len(otherUsers) > 0 && m.Selected < len(otherUsers) {
        selectedUser := otherUsers[m.Selected]
        isFollowing := m.Following[selectedUser.Id]

        go func() {
            database := db.GetDB()
            if isFollowing {
                // Unfollow
                database.DeleteLocalFollow(m.AccountId, selectedUser.Id)
            } else {
                // Follow
                database.CreateLocalFollow(m.AccountId, selectedUser.Id)

                // Create notification for the followed user
                err, follower := database.ReadAccById(m.AccountId)
                if err == nil && follower != nil {
                    notification := &domain.Notification{
                        Id:               uuid.New(),
                        AccountId:        selectedUser.Id,
                        NotificationType: domain.NotificationFollow,
                        ActorId:          follower.Id,
                        ActorUsername:    follower.Username,
                        ActorDomain:      "",  // Empty for local users
                        Read:             false,
                        CreatedAt:        time.Now(),
                    }
                    database.CreateNotification(notification)
                }
            }
        }()

        // Update local state immediately
        if isFollowing {
            delete(m.Following, selectedUser.Id)
            m.Status = fmt.Sprintf("Unfollowed @%s", selectedUser.Username)
        } else {
            m.Following[selectedUser.Id] = true
            m.Status = fmt.Sprintf("Following @%s", selectedUser.Username)
        }

        return m, clearStatusAfter(2 * time.Second)
    }
```

---

## Notification Creation

When a user follows another local user, a notification is created:

```go
notification := &domain.Notification{
    Id:               uuid.New(),
    AccountId:        selectedUser.Id,        // Recipient
    NotificationType: domain.NotificationFollow,
    ActorId:          follower.Id,            // Who followed
    ActorUsername:    follower.Username,
    ActorDomain:      "",                     // Empty for local
    Read:             false,
    CreatedAt:        time.Now(),
}
database.CreateNotification(notification)
```

---

## Data Loading

### On Initialization

```go
func loadLocalUsers(accountId uuid.UUID) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()

        // Load all local users
        err, users := database.ReadAllAccounts()

        // Load local follows to see who we're following
        err, follows := database.ReadLocalFollowsByAccountId(accountId)
        following := make(map[uuid.UUID]bool)
        if err == nil && follows != nil {
            for _, follow := range *follows {
                following[follow.TargetAccountId] = true
            }
        }

        return usersLoadedMsg{users: *users, following: following}
    }
}
```

---

## Status Clearing

Status messages auto-clear after 2 seconds:

```go
type clearStatusMsg struct{}

func clearStatusAfter(d time.Duration) tea.Cmd {
    return tea.Tick(d, func(t time.Time) tea.Msg {
        return clearStatusMsg{}
    })
}

case clearStatusMsg:
    m.Status = ""
    m.Error = ""
    return m, nil
```

---

## Pagination

```go
const DefaultItemsPerPage = 10

start := m.Offset
end := min(start + common.DefaultItemsPerPage, len(otherUsers))

for i := start; i < end; i++ {
    // Render user
}

// Pagination indicator
if len(otherUsers) > common.DefaultItemsPerPage {
    paginationText := fmt.Sprintf("showing %d-%d of %d", start+1, end, len(otherUsers))
    s.WriteString(common.ListBadgeStyle.Render(paginationText))
}
```

---

## Empty States

### No Users

```go
if len(m.Users) == 0 {
    s.WriteString(common.ListEmptyStyle.Render("No local users found."))
}
```

### Only Current User

```go
if len(otherUsers) == 0 {
    s.WriteString(common.ListEmptyStyle.Render("No other local users yet."))
}
```

---

## Scroll Behavior

```go
case "up", "k":
    if m.Selected > 0 {
        m.Selected--
        if m.Selected < m.Offset {
            m.Offset = m.Selected
        }
    }

case "down", "j":
    if m.Selected < len(otherUsers)-1 {
        m.Selected++
        if m.Selected >= m.Offset + common.DefaultItemsPerPage {
            m.Offset = m.Selected - common.DefaultItemsPerPage + 1
        }
    }
```

---

## User Filtering

The view filters out the current user from the selectable list:

```go
func (m Model) getOtherUsers() []domain.Account {
    var others []domain.Account
    for _, user := range m.Users {
        if user.Id != m.AccountId {
            others = append(others, user)
        }
    }
    return others
}
```

---

## Initialization

```go
func InitialModel(accountId uuid.UUID, width, height int) Model {
    return Model{
        AccountId: accountId,
        Users:     []domain.Account{},
        Following: make(map[uuid.UUID]bool),
        Selected:  0,
        Offset:    0,
        Width:     width,
        Height:    height,
        Status:    "",
        Error:     "",
    }
}

func (m Model) Init() tea.Cmd {
    return loadLocalUsers(m.AccountId)
}
```

---

## Source Files

- `ui/localusers/localusers.go` - LocalUsers view implementation
- `ui/common/styles.go` - List styles
- `db/db.go` - ReadAllAccounts, ReadLocalFollowsByAccountId, CreateLocalFollow, DeleteLocalFollow
- `domain/notification.go` - Notification type constants
