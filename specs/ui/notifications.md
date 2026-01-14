# Notifications View

This document specifies the Notifications view, which displays the user's notification center with unread badges.

---

## Overview

The Notifications view displays a paginated list of notifications including follows, likes, replies, and mentions. It features:
- Unread badge count displayed in the header
- 30-second auto-refresh to keep badge updated
- Single notification deletion and "delete all" functionality
- Note preview for engagement notifications

---

## Data Structure

```go
type Model struct {
    AccountId     uuid.UUID
    Notifications []domain.Notification
    Selected      int
    Offset        int
    Width         int
    Height        int
    isActive      bool
    UnreadCount   int
}
```

---

## Constants

```go
const (
    notificationsLimit = 50
    refreshInterval    = 30 * time.Second
)
```

---

## View Layout

```
┌─────────────────────────────────────────────────────────────┐
│ 🔔 Notifications (3 unread)                                  │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ ▸ 👋 @alice followed you                          2h ago    │  ← Selected
│                                                              │
│   ⭐ @bob@mastodon.social liked your post         5h ago    │
│   "This is the beginning of my post..."                     │  ← Note preview
│                                                              │
│   💬 @charlie replied to your post                1d ago    │
│   "Great point about..."                                     │
│                                                              │
│   📢 @diana mentioned you                         2d ago    │
│   "Hey @alice, check this out..."                           │
│                                                              │
│ Showing 1-4 of 12                                            │
└─────────────────────────────────────────────────────────────┘
```

---

## Notification Types

Notifications use type-specific icons and labels:

```go
// From domain/notification.go
const (
    NotificationFollow  = "follow"
    NotificationLike    = "like"
    NotificationReply   = "reply"
    NotificationMention = "mention"
)

func (n Notification) TypeIcon() string {
    switch n.NotificationType {
    case NotificationFollow:
        return "👋"
    case NotificationLike:
        return "⭐"
    case NotificationReply:
        return "💬"
    case NotificationMention:
        return "📢"
    default:
        return "🔔"
    }
}

func (n Notification) TypeLabel() string {
    switch n.NotificationType {
    case NotificationFollow:
        return "followed you"
    case NotificationLike:
        return "liked your post"
    case NotificationReply:
        return "replied to your post"
    case NotificationMention:
        return "mentioned you"
    default:
        return "notification"
    }
}
```

---

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `Enter` | Delete selected notification |
| `a` | Delete all notifications |

---

## Auto-Refresh Pattern

The notifications view keeps refreshing to update the unread badge count:

```go
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    switch msg := msg.(type) {
    case common.ActivateViewMsg:
        m.isActive = true
        return m, loadNotifications(m.AccountId)

    case common.DeactivateViewMsg:
        // Don't actually deactivate - keep refreshing for badge
        m.isActive = false
        return m, nil

    case notificationsLoadedMsg:
        m.Notifications = msg.notifications
        m.UnreadCount = msg.unreadCount
        // Schedule next tick to keep badge updated
        return m, tickRefresh()

    case refreshTickMsg:
        // Always refresh to keep badge count updated
        return m, loadNotifications(m.AccountId)
    }
}
```

### Tick Refresh

```go
func tickRefresh() tea.Cmd {
    return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
        return refreshTickMsg{}
    })
}
```

---

## Data Loading

```go
func loadNotifications(accountId uuid.UUID) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        err, notifications := database.ReadNotificationsByAccountId(accountId, notificationsLimit)
        if err != nil {
            return notificationsLoadedMsg{notifications: []domain.Notification{}, unreadCount: 0}
        }

        // Get unread count for badge
        unreadCount, err := database.ReadUnreadNotificationCount(accountId)
        if err != nil {
            unreadCount = 0
        }

        return notificationsLoadedMsg{
            notifications: *notifications,
            unreadCount:   unreadCount,
        }
    }
}
```

---

## Notification Deletion

### Single Notification

```go
case "enter":
    if m.Selected < len(m.Notifications) {
        notif := m.Notifications[m.Selected]
        return m, deleteNotification(notif.Id, m.AccountId)
    }

func deleteNotification(notificationId uuid.UUID, accountId uuid.UUID) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        database.DeleteNotification(notificationId)
        // Reload to update the view
        return loadNotifications(accountId)()
    }
}
```

### All Notifications

```go
case "a":
    return m, deleteAllNotifications(m.AccountId)

func deleteAllNotifications(accountId uuid.UUID) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        database.DeleteAllNotifications(accountId)
        // Reload to update the view
        return loadNotifications(accountId)()
    }
}
```

---

## Notification Rendering

```go
for i := start; i < end; i++ {
    notif := m.Notifications[i]
    selected := i == m.Selected

    // Format notification line
    line1 := fmt.Sprintf("%s %s %s", notif.TypeIcon(), notif.ActorHandle(), notif.TypeLabel())
    timeAgo := formatTimeAgo(notif.CreatedAt)

    if selected {
        if !notif.Read {
            // Unread + selected: bold with username color
            s.WriteString(common.ListSelectedPrefix +
                lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(common.COLOR_USERNAME)).Render(line1) +
                "  " + common.ListBadgeStyle.Render(timeAgo))
        } else {
            // Read + selected
            s.WriteString(common.ListSelectedPrefix +
                common.ListItemSelectedStyle.Render(line1) +
                "  " + common.ListBadgeStyle.Render(timeAgo))
        }
    } else {
        if !notif.Read {
            // Unread: bold
            s.WriteString(common.ListUnselectedPrefix +
                lipgloss.NewStyle().Bold(true).Render(line1) +
                "  " + common.ListBadgeStyle.Render(timeAgo))
        } else {
            // Read: normal
            s.WriteString(common.ListUnselectedPrefix +
                common.ListItemStyle.Render(line1) +
                "  " + common.ListBadgeStyle.Render(timeAgo))
        }
    }
    s.WriteString("\n")

    // Show preview for engagement notifications (not follows)
    if notif.NotePreview != "" && notif.NotificationType != domain.NotificationFollow {
        preview := truncate(notif.NotePreview, 60)
        s.WriteString("  " + common.ListBadgeStyle.Render("\""+preview+"\""))
        s.WriteString("\n")
    }
}
```

---

## Time Formatting

```go
func formatTimeAgo(t time.Time) string {
    duration := time.Since(t)

    if duration < time.Minute {
        return "just now"
    } else if duration < time.Hour {
        return fmt.Sprintf("%dm ago", int(duration.Minutes()))
    } else if duration < 24*time.Hour {
        return fmt.Sprintf("%dh ago", int(duration.Hours()))
    } else if duration < 7*24*time.Hour {
        return fmt.Sprintf("%dd ago", int(duration.Hours()/24))
    } else if duration < 30*24*time.Hour {
        return fmt.Sprintf("%dw ago", int(duration.Hours()/24/7))
    } else if duration < 365*24*time.Hour {
        return fmt.Sprintf("%dmo ago", int(duration.Hours()/24/30))
    } else {
        return fmt.Sprintf("%dy ago", int(duration.Hours()/24/365))
    }
}
```

---

## Header Badge Integration

The unread count is passed to the header for badge display:

```go
// In header.go
if unreadCount > 0 {
    badgePlain = fmt.Sprintf(" [%d]", unreadCount)
    leftTextPlain += badgePlain
}

// Uses warning color (orange) for visibility
leftText += common.ANSI_WARNING_START + badgePlain + common.ANSI_COLOR_RESET
```

---

## Empty State

```go
if len(m.Notifications) == 0 {
    s.WriteString(common.ListEmptyStyle.Render("No notifications yet."))
}
```

---

## Pagination

```go
itemsPerPage := common.DefaultItemsPerPage
start := m.Offset
end := start + itemsPerPage
if end > len(m.Notifications) {
    end = len(m.Notifications)
}

// Pagination info
if len(m.Notifications) > itemsPerPage {
    pageInfo := fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(m.Notifications))
    s.WriteString("\n" + common.ListBadgeStyle.Render(pageInfo))
}
```

---

## Selection Bounds

After loading, keep selection within bounds:

```go
case notificationsLoadedMsg:
    m.Notifications = msg.notifications
    m.UnreadCount = msg.unreadCount
    if m.Selected >= len(m.Notifications) {
        m.Selected = len(m.Notifications) - 1
    }
    if m.Selected < 0 {
        m.Selected = 0
    }
    return m, tickRefresh()
```

---

## Initialization

```go
func InitialModel(accountId uuid.UUID, width, height int) Model {
    return Model{
        AccountId:     accountId,
        Notifications: []domain.Notification{},
        Selected:      0,
        Offset:        0,
        Width:         width,
        Height:        height,
        isActive:      false,
        UnreadCount:   0,
    }
}

func (m Model) Init() tea.Cmd {
    return nil  // Model starts inactive, loads on activation
}
```

---

## Accessing Notifications

Press `Ctrl+N` from any view to open notifications (handled in supertui.go).

---

## Source Files

- `ui/notifications/notifications.go` - Notifications view implementation
- `ui/header/header.go` - Header with notification badge
- `ui/common/styles.go` - List styles
- `domain/notification.go` - Notification type with TypeIcon, TypeLabel, ActorHandle methods
- `db/db.go` - ReadNotificationsByAccountId, ReadUnreadNotificationCount, DeleteNotification, DeleteAllNotifications
