# Threading

This document specifies reply chains and parent-child relationships for notes.

---

## Overview

Stegodon supports threaded conversations:
- Notes can reply to other notes via `InReplyToURI`
- Reply counts are denormalized for performance
- Thread view displays parent + all replies
- Supports both local and remote replies

---

## Data Model

### Note Structure

```go
type Note struct {
    // ... other fields
    InReplyToURI string // URI of the note this is replying to
    ObjectURI    string // This note's ActivityPub URI
    ReplyCount   int    // Number of replies (denormalized)
    // ...
}
```

### SaveNote Structure

```go
type SaveNote struct {
    UserId       uuid.UUID
    Message      string
    InReplyToURI string // URI of parent post (empty for top-level)
}
```

### Database Schema

```sql
CREATE TABLE notes (
    -- ... other columns
    in_reply_to_uri TEXT DEFAULT '',
    object_uri TEXT DEFAULT '',
    reply_count INTEGER DEFAULT 0,
    -- ...
);
```

---

## Reply Creation

### Creating a Reply

```go
case tea.KeyCtrlS:
    if m.isReplying {
        replyURI := m.replyToURI

        // Convert local: prefix to proper URI
        if strings.HasPrefix(replyURI, "local:") {
            noteIdStr := strings.TrimPrefix(replyURI, "local:")
            if conf.Conf.SslDomain != "" && conf.Conf.SslDomain != "example.com" {
                replyURI = fmt.Sprintf("https://%s/notes/%s", conf.Conf.SslDomain, noteIdStr)
            }
        }

        note := domain.SaveNote{
            UserId:       m.userId,
            Message:      value,
            InReplyToURI: replyURI,
        }
        return m, createNoteModelCmd(&note)
    }
```

### Database Insert

```go
func (db *DB) CreateNoteWithReply(userId uuid.UUID, message string, inReplyToURI string) (uuid.UUID, error) {
    noteId := uuid.New()
    _, err := db.db.Exec(`
        INSERT INTO notes (id, created_by, message, in_reply_to_uri, ...)
        VALUES (?, ?, ?, ?, ...)
    `, noteId, userId, message, inReplyToURI, ...)
    return noteId, err
}
```

---

## Reply Count

### Denormalized Counter

Reply counts are stored on the parent note for efficient display:

```go
type Note struct {
    ReplyCount int // Number of replies
}
```

### Counting Functions

```go
// Count replies to a local note by ID
func (db *DB) CountRepliesByNoteId(noteId uuid.UUID) (int, error)

// Count replies by parent URI
func (db *DB) CountRepliesByURI(parentURI string) (int, error)

// Count remote replies from activities table
func (db *DB) CountActivitiesByInReplyTo(parentURI string) (int, error)
```

### Total Reply Count

```go
// Local replies
replyCount, _ := database.CountRepliesByNoteId(note.Id)

// Remote replies (requires domain)
if domain != "" {
    replyURI := fmt.Sprintf("https://%s/notes/%s", domain, note.Id.String())
    remoteReplyCount, _ := database.CountActivitiesByInReplyTo(replyURI)
    replyCount += remoteReplyCount
}
```

---

## Thread View

### ThreadPost Structure

```go
type ThreadPost struct {
    ID         uuid.UUID
    Author     string
    Content    string
    Time       time.Time
    ObjectURI  string
    IsLocal    bool // Local vs federated
    IsParent   bool // Parent vs reply
    IsDeleted  bool // Placeholder for deleted
    ReplyCount int
    LikeCount  int
    BoostCount int
}
```

### Loading Thread

```go
func loadThread(parentURI string) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()

        var parent *ThreadPost
        var replies []ThreadPost

        // 1. Find parent note (local or remote)
        err, localNote := database.ReadNoteByURI(parentURI)
        if err == nil && localNote != nil {
            parent = &ThreadPost{...}
        } else {
            err, activity := database.ReadActivityByObjectURI(parentURI)
            if err == nil && activity != nil {
                parent = &ThreadPost{...}
            }
        }

        // 2. Load local replies
        err, localReplies := database.ReadRepliesByURI(parentURI)
        // ... append to replies

        // 3. Load remote replies
        err, remoteReplies := database.ReadActivitiesByInReplyTo(parentURI)
        // ... append to replies

        // 4. Sort by time
        sort.Slice(replies, func(i, j int) bool {
            return replies[i].Time.Before(replies[j].Time)
        })

        return threadLoadedMsg{parent, replies, nil}
    }
}
```

---

## URI Formats

### Local Note URI

```
https://example.com/notes/550e8400-e29b-41d4-a716-446655440000
```

### Local Prefix (Internal)

```
local:550e8400-e29b-41d4-a716-446655440000
```

Used when domain not configured, converted to full URI before federation.

### Remote Note URI

```
https://mastodon.social/users/bob/statuses/123456789
```

---

## Thread Navigation

### Opening Thread from Timeline

```go
case "enter":
    if selectedPost.ReplyCount > 0 {
        noteURI := selectedPost.ObjectURI
        if noteURI == "" && selectedPost.IsLocal {
            noteURI = "local:" + selectedPost.NoteID.String()
        }
        return m, func() tea.Msg {
            return common.ViewThreadMsg{
                NoteURI:   noteURI,
                NoteID:    selectedPost.NoteID,
                Author:    selectedPost.Author,
                Content:   selectedPost.Content,
                CreatedAt: selectedPost.Time,
                IsLocal:   selectedPost.IsLocal,
            }
        }
    }
```

### ViewThreadMsg

```go
type ViewThreadMsg struct {
    NoteURI   string
    NoteID    uuid.UUID
    Author    string
    Content   string
    CreatedAt time.Time
    IsLocal   bool
}
```

---

## Thread Display

### Visual Structure

```
thread (3 replies)

2h ago · 3 replies · ⭐ 2
@alice
This is the parent post with some content...

    1h ago · 1 reply
    @bob@remote.com
    This is a reply to the parent...

    30m ago
    @carol
    Another reply to the parent...

    10m ago
    @dave@other.server
    A third reply...
```

### Selection

| Index | Selected Item |
|-------|---------------|
| -1 | Parent post |
| 0 | First reply |
| 1 | Second reply |
| n | (n+1)th reply |

### Indentation

```go
const ReplyIndentWidth = 4

replyIndent = strings.Repeat(" ", common.ReplyIndentWidth)
```

Replies are indented 4 spaces from parent.

---

## Reply Actions

### Reply to Parent

```go
case "r":
    if m.Selected == -1 && m.ParentPost != nil {
        replyURI := m.ParentPost.ObjectURI
        if replyURI == "" && m.ParentPost.IsLocal {
            replyURI = "local:" + m.ParentPost.ID.String()
        }
        return m, func() tea.Msg {
            return common.ReplyToNoteMsg{
                NoteURI: replyURI,
                Author:  m.ParentPost.Author,
                Preview: preview,
            }
        }
    }
```

### Reply to Reply

```go
if m.Selected >= 0 && m.Selected < len(m.Replies) {
    reply := m.Replies[m.Selected]
    replyURI := reply.ObjectURI
    if replyURI == "" && reply.IsLocal {
        replyURI = "local:" + reply.ID.String()
    }
    // Send ReplyToNoteMsg...
}
```

---

## Nested Threads

### Drilling Down

Press `Enter` on a reply with replies to open it as new parent:

```go
case "enter":
    if m.Selected >= 0 && reply.ReplyCount > 0 {
        return m, func() tea.Msg {
            return common.ViewThreadMsg{
                NoteURI: reply.ObjectURI,
                NoteID:  reply.ID,
                // ...
            }
        }
    }
```

### Back Navigation

```go
case "esc", "q":
    return m, func() tea.Msg {
        return common.HomeTimelineView
    }
```

---

## ActivityPub Integration

### Outgoing Reply

```json
{
  "type": "Create",
  "object": {
    "type": "Note",
    "id": "https://example.com/notes/new-uuid",
    "inReplyTo": "https://remote.server/users/bob/statuses/123",
    "content": "This is my reply..."
  }
}
```

### Incoming Reply

When receiving a `Create` activity with `inReplyTo`:

1. Store activity in `activities` table
2. Find local parent note
3. Increment parent's `reply_count`
4. Create notification if parent is local

---

## Reply Notifications

### Creating Notification

```go
if note.InReplyToURI != "" {
    // Find parent note
    readErr, parentNote := database.ReadNoteByURI(note.InReplyToURI)
    if readErr == nil && parentNote != nil {
        // Get parent author
        readErr, parentAuthor := database.ReadAccByUsername(parentNote.CreatedBy)
        if parentAuthor.Id != note.UserId {
            notification := &domain.Notification{
                NotificationType: domain.NotificationReply,
                AccountId:        parentAuthor.Id,
                ActorId:          replier.Id,
                NotePreview:      preview,
            }
            database.CreateNotification(notification)
        }
    }
}
```

---

## Deleted Parent Handling

### Placeholder Display

```go
if parent == nil && len(replies) > 0 {
    parent = &ThreadPost{
        Author:    "[deleted]",
        Content:   "This post has been deleted",
        IsParent:  true,
        IsDeleted: true,
    }
}
```

---

## Source Files

- `domain/notes.go` - `InReplyToURI`, `ReplyCount` fields
- `ui/threadview/threadview.go` - Thread display and navigation
- `ui/writenote/writenote.go` - Reply mode handling
- `ui/hometimeline/hometimeline.go` - Thread entry point
- `db/db.go` - Reply queries and counting
- `activitypub/inbox.go` - Incoming reply processing
