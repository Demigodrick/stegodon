# Admin Panel View

This document specifies the Admin Panel view, which provides user management capabilities for administrators.

---

## Overview

The Admin Panel is an admin-only view for managing users on the server. It supports:
- Viewing all registered users with their status
- Muting users (deletes their posts)
- Kicking users (deletes their account)

---

## Data Structure

```go
type Model struct {
    AdminId  uuid.UUID
    Users    []domain.Account
    Selected int
    Offset   int               // Pagination offset
    Width    int
    Height   int
    Status   string            // Success message
    Error    string            // Error message
}
```

---

## View Layout

```
┌─────────────────────────────────────────────────────────────┐
│ admin panel (5 users)                                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ ▸ @alice [ADMIN]                                             │  ← Selected, admin badge
│   @bob                                                       │
│   @charlie [MUTED]                                           │  ← Muted user (red text)
│   @diana                                                     │
│   @eve                                                       │
│                                                              │
│ User muted and posts deleted                                 │  ← Status message
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `m` | Mute selected user |
| `K` | Kick selected user (capital K to prevent accidents) |

---

## User Badges

Users display status badges:

```go
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
```

---

## Muting Users

Muting a user deletes their posts and prevents them from posting new content.

### Restrictions

```go
case "m":
    if len(m.Users) > 0 && m.Selected < len(m.Users) {
        selectedUser := m.Users[m.Selected]

        // Can't mute admin
        if selectedUser.IsAdmin {
            m.Error = "Cannot mute admin user"
            return m, nil
        }

        // Can't mute yourself
        if selectedUser.Id == m.AdminId {
            m.Error = "Cannot mute yourself"
            return m, nil
        }

        // Already muted
        if selectedUser.Muted {
            m.Error = "User is already muted"
            return m, nil
        }

        return m, muteUser(selectedUser.Id)
    }
```

### Mute Command

```go
func muteUser(userId uuid.UUID) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        err := database.MuteUser(userId)
        if err != nil {
            log.Printf("Failed to mute user: %v", err)
        }
        return muteUserMsg{userId: userId}
    }
}
```

### Result Handling

```go
case muteUserMsg:
    m.Status = "User muted and posts deleted"
    m.Error = ""
    return m, loadUsers()
```

---

## Kicking Users

Kicking a user completely deletes their account. Uses capital `K` to prevent accidental kicks.

### Restrictions

```go
case "K":  // Capital K for safety
    if len(m.Users) > 0 && m.Selected < len(m.Users) {
        selectedUser := m.Users[m.Selected]

        // Can't kick admin
        if selectedUser.IsAdmin {
            m.Error = "Cannot kick admin user"
            return m, nil
        }

        // Can't kick yourself
        if selectedUser.Id == m.AdminId {
            m.Error = "Cannot kick yourself"
            return m, nil
        }

        return m, kickUser(selectedUser.Id)
    }
```

### Kick Command

```go
func kickUser(userId uuid.UUID) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        err := database.DeleteAccount(userId)
        if err != nil {
            log.Printf("Failed to kick user: %v", err)
        }
        return kickUserMsg{userId: userId}
    }
}
```

### Result Handling

```go
case kickUserMsg:
    m.Status = "User kicked successfully"
    m.Error = ""
    return m, loadUsers()
```

---

## User Rendering

```go
for i := start; i < end; i++ {
    user := m.Users[i]

    username := "@" + user.Username

    // Build badges
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
        // Selected item
        text := common.ListItemSelectedStyle.Render(username + badge)
        s.WriteString(common.ListSelectedPrefix + text)
    } else if user.Muted {
        // Muted users shown in error/red color
        text := username + common.ListBadgeMutedStyle.Render(badge)
        s.WriteString(common.ListUnselectedPrefix + common.ListItemStyle.Render(text))
    } else {
        // Normal item
        text := username + common.ListBadgeStyle.Render(badge)
        s.WriteString(common.ListUnselectedPrefix + common.ListItemStyle.Render(text))
    }
    s.WriteString("\n")
}
```

---

## Data Loading

```go
func loadUsers() tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        err, users := database.ReadAllAccountsAdmin()
        if err != nil || users == nil {
            return usersLoadedMsg{users: []domain.Account{}}
        }
        return usersLoadedMsg{users: *users}
    }
}
```

Note: Uses `ReadAllAccountsAdmin()` which returns additional admin-specific fields.

---

## Message Types

```go
type usersLoadedMsg struct {
    users []domain.Account
}

type muteUserMsg struct {
    userId uuid.UUID
}

type kickUserMsg struct {
    userId uuid.UUID
}
```

---

## Empty State

```go
if len(m.Users) == 0 {
    s.WriteString(common.ListEmptyStyle.Render("No users found."))
}
```

---

## Pagination

```go
start := m.Offset
end := min(start + common.DefaultItemsPerPage, len(m.Users))

// Pagination indicator
if len(m.Users) > common.DefaultItemsPerPage {
    paginationText := fmt.Sprintf("showing %d-%d of %d", start+1, end, len(m.Users))
    s.WriteString(common.ListBadgeStyle.Render(paginationText))
}
```

---

## Selection Bounds

After loading, keep selection within bounds:

```go
case usersLoadedMsg:
    m.Users = msg.users
    m.Selected = 0
    m.Offset = 0
    if m.Selected >= len(m.Users) {
        m.Selected = max(0, len(m.Users)-1)
    }
    return m, nil
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
    if len(m.Users) > 0 && m.Selected < len(m.Users)-1 {
        m.Selected++
        if m.Selected >= m.Offset + common.DefaultItemsPerPage {
            m.Offset = m.Selected - common.DefaultItemsPerPage + 1
        }
    }
```

---

## Initialization

```go
func InitialModel(adminId uuid.UUID, width, height int) Model {
    return Model{
        AdminId:  adminId,
        Users:    []domain.Account{},
        Selected: 0,
        Offset:   0,
        Width:    width,
        Height:   height,
        Status:   "",
        Error:    "",
    }
}

func (m Model) Init() tea.Cmd {
    return loadUsers()
}
```

---

## Access Control

This view is only accessible to admin users. Access is controlled in supertui.go based on the `Account.IsAdmin` field.

---

## Database Operations

### MuteUser

Sets the user's `Muted` flag to true and deletes all their notes.

### DeleteAccount (Kick)

Completely removes the account and all associated data:
- Account record
- Notes
- Follow relationships
- Activities
- Notifications

---

## Source Files

- `ui/admin/admin.go` - Admin panel view implementation
- `ui/common/styles.go` - List styles including ListBadgeMutedStyle
- `db/db.go` - ReadAllAccountsAdmin, MuteUser, DeleteAccount
- `domain/account.go` - Account entity with IsAdmin, Muted fields
