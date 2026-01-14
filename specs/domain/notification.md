# Notification Entity

This document specifies the Notification entity, which represents user notifications for various events.

---

## Overview

Notifications inform users about interactions with their account or content. Stegodon supports four notification types:
- **Follow**: Someone followed you
- **Like**: Someone liked your post
- **Reply**: Someone replied to your post
- **Mention**: Someone mentioned you in a post

Notifications track read status and include denormalized actor/note information for efficient display.

---

## Data Structure

```go
type NotificationType string

const (
    NotificationFollow  NotificationType = "follow"
    NotificationLike    NotificationType = "like"
    NotificationReply   NotificationType = "reply"
    NotificationMention NotificationType = "mention"
)

type Notification struct {
    Id               uuid.UUID
    AccountId        uuid.UUID        // Local user receiving notification
    NotificationType NotificationType // follow, like, reply, mention
    ActorId          uuid.UUID        // Account that triggered it
    ActorUsername    string           // Denormalized: "alice"
    ActorDomain      string           // Denormalized: "mastodon.social" or ""
    NoteId           uuid.UUID        // Related note (for like/reply/mention)
    NoteURI          string           // ActivityPub URI of note
    NotePreview      string           // First 100 chars of note
    Read             bool             // Has been viewed
    CreatedAt        time.Time
}
```

---

## Field Definitions

### Identity Fields

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| `Id` | `uuid.UUID` | Primary Key | Unique notification identifier |
| `AccountId` | `uuid.UUID` | FK → accounts.id | Recipient user |
| `NotificationType` | `string` | Required | Type of notification |

### Actor Fields (Denormalized)

| Field | Type | Description |
|-------|------|-------------|
| `ActorId` | `uuid.UUID` | Account that triggered notification |
| `ActorUsername` | `string` | Username for display |
| `ActorDomain` | `string` | Domain (empty for local users) |

### Note Fields (Denormalized)

| Field | Type | Description |
|-------|------|-------------|
| `NoteId` | `uuid.UUID` | Related note (nullable for follow) |
| `NoteURI` | `string` | ActivityPub URI |
| `NotePreview` | `string` | First 100 characters of content |

### State

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Read` | `bool` | `false` | Whether viewed |
| `CreatedAt` | `time.Time` | now | When created |

---

## Notification Types

### Follow

Created when someone follows the user:

| Field | Value |
|-------|-------|
| `NotificationType` | `"follow"` |
| `NoteId` | Empty/null |
| `NoteURI` | Empty |
| `NotePreview` | Empty |

### Like

Created when someone likes the user's post:

| Field | Value |
|-------|-------|
| `NotificationType` | `"like"` |
| `NoteId` | The liked note's ID |
| `NoteURI` | The liked note's URI |
| `NotePreview` | First 100 chars of liked note |

### Reply

Created when someone replies to the user's post:

| Field | Value |
|-------|-------|
| `NotificationType` | `"reply"` |
| `NoteId` | The reply note's ID |
| `NoteURI` | The reply note's URI |
| `NotePreview` | First 100 chars of reply |

### Mention

Created when someone mentions the user in a post:

| Field | Value |
|-------|-------|
| `NotificationType` | `"mention"` |
| `NoteId` | The note containing mention |
| `NoteURI` | The note's URI |
| `NotePreview` | First 100 chars of note |

---

## Database Schema

```sql
CREATE TABLE IF NOT EXISTS notifications (
    id TEXT NOT NULL PRIMARY KEY,
    account_id TEXT NOT NULL,
    notification_type TEXT NOT NULL,
    actor_id TEXT,
    actor_username TEXT,
    actor_domain TEXT,
    note_id TEXT,
    note_uri TEXT,
    note_preview TEXT,
    read INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
)
```

### Indexes

```sql
CREATE INDEX IF NOT EXISTS idx_notifications_account_id ON notifications(account_id);
CREATE INDEX IF NOT EXISTS idx_notifications_read ON notifications(read);
CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at);
```

---

## Helper Methods

### ActorHandle

Returns formatted handle for display:

```go
func (n *Notification) ActorHandle() string {
    if n.ActorDomain == "" {
        return "@" + n.ActorUsername
    }
    return "@" + n.ActorUsername + "@" + n.ActorDomain
}
```

Examples:
- Local: `@alice`
- Remote: `@alice@mastodon.social`

### TypeLabel

Returns human-readable description:

```go
func (n *Notification) TypeLabel() string {
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
        return ""
    }
}
```

### TypeIcon

Returns emoji icon for type:

```go
func (n *Notification) TypeIcon() string {
    switch n.NotificationType {
    case NotificationFollow:
        return "👤"
    case NotificationLike:
        return "❤️"
    case NotificationReply:
        return "💬"
    case NotificationMention:
        return "@"
    default:
        return "•"
    }
}
```

### Summary

Returns one-line summary:

```go
func (n *Notification) Summary() string {
    return fmt.Sprintf("%s %s %s", n.TypeIcon(), n.ActorHandle(), n.TypeLabel())
}
```

Example: `❤️ @alice@mastodon.social liked your post`

---

## Creation Flow

### From Follow

```
Incoming Follow Activity
      │
      ├── Create Follow record
      └── Create Notification
            │
            ├── Type: "follow"
            ├── ActorId: follower's account
            ├── ActorUsername: from remote account
            └── ActorDomain: from remote account
```

### From Like

```
Incoming Like Activity
      │
      ├── Create Like record
      ├── Find liked note
      └── Create Notification
            │
            ├── Type: "like"
            ├── ActorId: liker's account
            ├── NoteId: liked note
            └── NotePreview: truncate(note.Message, 100)
```

### From Reply

```
Incoming Create Activity (reply)
      │
      ├── Create Activity record
      ├── Find parent note
      └── Create Notification
            │
            ├── Type: "reply"
            ├── ActorId: replier's account
            ├── NoteId: the reply
            └── NotePreview: truncate(reply.Content, 100)
```

### From Mention

```
Incoming Create Activity
      │
      ├── Parse mentions in content
      ├── For each mentioned local user
      └── Create Notification
            │
            ├── Type: "mention"
            ├── ActorId: post author
            ├── NoteId: the note
            └── NotePreview: truncate(note.Content, 100)
```

---

## Database Operations

### Read Operations

| Function | Description |
|----------|-------------|
| `GetNotifications(accountId, limit, offset)` | Paginated notifications |
| `GetUnreadCount(accountId)` | Count unread notifications |
| `GetNotificationById(id)` | Single notification |

### Write Operations

| Function | Description |
|----------|-------------|
| `CreateNotification(notification)` | Create new notification |
| `MarkAsRead(id)` | Mark single as read |
| `MarkAllAsRead(accountId)` | Mark all as read |
| `DeleteNotification(id)` | Remove notification |
| `DeleteNotificationsForAccount(accountId)` | Clear all for user |

---

## UI Integration

### Notifications View

Paginated list with 50 per page:

```
┌─────────────────────────────────────────────┐
│ Notifications (3 unread)                    │
├─────────────────────────────────────────────┤
│ • ❤️ @alice@mastodon.social liked your post │
│     "This is the preview of my note..."    │
│                                             │
│   👤 @bob followed you                      │
│                                             │
│   💬 @charlie replied to your post          │
│     "Great point! I think..."              │
└─────────────────────────────────────────────┘
```

### Unread Indicator

- Unread notifications shown with bullet (•)
- Read notifications without bullet
- Viewing marks notifications as read

### Header Badge

Unread count shown in header:

```
[stegodon] v1.4.3 | @username | 🔔 3
```

Badge updates via:
- 30-second auto-refresh in notifications view
- Periodic background check

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Ctrl+N` | Open notifications view |
| `Enter` | View related note/thread |
| `Esc` | Return to previous view |

---

## Auto-Refresh

Notifications view refreshes every 30 seconds when active:

```go
const notificationRefreshInterval = 30 * time.Second

func tickRefresh() tea.Cmd {
    return tea.Tick(notificationRefreshInterval, func(t time.Time) tea.Msg {
        return refreshTickMsg{}
    })
}
```

---

## Cascade Deletion

Notifications are deleted when:

1. **Account deleted**: All notifications for that account

```sql
-- Foreign key with ON DELETE CASCADE
FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
```

2. **Note deleted**: Notifications referencing that note should be cleaned up

---

## Denormalization Rationale

Actor and note fields are denormalized because:

1. **Performance**: Avoid joins for every notification display
2. **Availability**: Show notifications even if remote account cache expired
3. **Simplicity**: Single query returns display-ready data

Tradeoff: Data may become stale if actor updates their profile.

---

## Relationships

| Related Entity | Relationship | Description |
|----------------|--------------|-------------|
| Account | Many-to-One | Recipient of notification |
| Account/RemoteAccount | Many-to-One | Actor who triggered it |
| Note | Many-to-One | Related note (optional) |
| Follow | One-to-One | For follow notifications |
| Like | One-to-One | For like notifications |

---

## Source Files

- `domain/notification.go` - Notification struct and helpers
- `db/db.go` - Database operations
- `db/migrations.go` - Table creation
- `ui/notifications/` - Notifications view
- `ui/common/commands.go` - Notification-related commands
- `activitypub/inbox.go` - Notification creation on activities
