# Note Entity

This document specifies the Note entity, which represents posts/toots created by local users.

---

## Overview

A Note is the primary content unit in stegodon. Notes can be standalone posts or replies to other notes (local or remote). Notes support threading, engagement tracking, and ActivityPub federation.

---

## Data Structures

### SaveNote

Input structure for creating a note:

```go
type SaveNote struct {
    UserId       uuid.UUID
    Message      string
    InReplyToURI string // URI of parent post (empty for top-level)
}
```

### Note

Full note representation:

```go
type Note struct {
    Id             uuid.UUID
    CreatedBy      string     // Username of author
    Message        string
    CreatedAt      time.Time
    EditedAt       *time.Time // nil if never edited
    Visibility     string     // "public", "unlisted", "followers", "direct"
    InReplyToURI   string     // Parent note URI for replies
    ObjectURI      string     // ActivityPub object URI
    Federated      bool       // Whether to federate
    Sensitive      bool       // Contains sensitive content
    ContentWarning string     // CW text
    ReplyCount     int        // Denormalized counter
    LikeCount      int        // Denormalized counter
    BoostCount     int        // Denormalized counter
}
```

### HomePost

Unified timeline representation:

```go
type HomePost struct {
    ID         uuid.UUID
    Author     string     // @user or @user@domain
    Content    string
    Time       time.Time
    ObjectURI  string
    IsLocal    bool       // true = local note
    NoteID     uuid.UUID  // Only for local posts
    ReplyCount int
    LikeCount  int
    BoostCount int
}
```

---

## Field Definitions

### Core Fields

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| `Id` | `uuid.UUID` | Primary Key | Unique note identifier |
| `CreatedBy` | `string` | FK → accounts.username | Author's username |
| `Message` | `string` | Max 1000 chars (DB) | Note content |
| `CreatedAt` | `time.Time` | Default: now | Creation timestamp |
| `EditedAt` | `*time.Time` | Nullable | Last edit timestamp |

### Content Limits

| Limit | Value | Description |
|-------|-------|-------------|
| Visible characters | 150 | UI-enforced limit |
| Database storage | 1000 | Maximum stored characters |
| Truncated preview | 100 | Notification preview length |

### Threading

| Field | Type | Description |
|-------|------|-------------|
| `InReplyToURI` | `string` | ActivityPub URI of parent note |

Empty string indicates a top-level post.

### ActivityPub Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ObjectURI` | `string` | Auto-generated | `https://domain/users/{user}/posts/{id}` |
| `Federated` | `bool` | `true` | Whether to send to followers |
| `Visibility` | `string` | `"public"` | Access level |

### Visibility Levels

| Value | Behavior |
|-------|----------|
| `public` | Visible to everyone, appears in public timelines |
| `unlisted` | Visible to everyone, hidden from public timelines |
| `followers` | Only visible to followers |
| `direct` | Only visible to mentioned users |

### Content Warnings

| Field | Type | Description |
|-------|------|-------------|
| `Sensitive` | `bool` | Flag for sensitive content |
| `ContentWarning` | `string` | Warning text displayed before content |

### Engagement Counters

| Field | Type | Description |
|-------|------|-------------|
| `ReplyCount` | `int` | Number of replies (recursive) |
| `LikeCount` | `int` | Number of likes |
| `BoostCount` | `int` | Number of boosts |

Counters are denormalized for performance.

---

## Database Schema

```sql
CREATE TABLE IF NOT EXISTS notes(
    id uuid NOT NULL PRIMARY KEY,
    user_id uuid NOT NULL,
    message varchar(1000),
    created_at timestamp default current_timestamp,
    edited_at timestamp,
    visibility varchar(20) default 'public',
    in_reply_to_uri text,
    object_uri text,
    federated int default 1,
    sensitive int default 0,
    content_warning text,
    reply_count int default 0,
    like_count int default 0,
    boost_count int default 0
)
```

### Indexes

```sql
CREATE INDEX idx_notes_user_id ON notes(user_id);
CREATE INDEX idx_notes_created_at ON notes(created_at);
CREATE INDEX idx_notes_in_reply_to_uri ON notes(in_reply_to_uri);
```

---

## Lifecycle

### 1. Note Creation

```
User writes in TUI
      │
      ├── Validate length (≤150 visible chars)
      ├── Extract hashtags
      ├── Extract mentions
      ├── Generate UUID
      ├── Generate ObjectURI
      └── Insert note
            │
            ├── Create note_hashtags entries
            ├── Create note_mentions entries
            └── Queue federation (if enabled)
```

### 2. Note Editing

Editing preserves original `CreatedAt` and sets `EditedAt`:

```go
sqlUpdateNote = `UPDATE notes SET message = ?, edited_at = ? WHERE id = ?`
```

Notes can only be edited by their author.

### 3. Note Deletion

Cascade deletes related data:

```
Delete Note
    │
    ├── Delete note_hashtags
    ├── Delete note_mentions
    ├── Delete likes referencing note
    ├── Delete boosts referencing note
    └── Send Delete activity (if federated)
```

### 4. Reply Threading

Replies reference parent by ActivityPub URI:

```go
note := SaveNote{
    UserId:       userId,
    Message:      message,
    InReplyToURI: parentObjectURI,
}
```

Reply counts are recursively updated:

```sql
UPDATE notes SET reply_count = reply_count + 1 WHERE object_uri = ?
UPDATE activities SET reply_count = reply_count + 1 WHERE object_uri = ?
```

---

## Content Processing

### Hashtag Extraction

Hashtags are parsed and stored in `note_hashtags`:

```go
hashtags := util.ExtractHashtags(message)
// Creates: #music → hashtag "music"
```

### Mention Extraction

Mentions are parsed and stored in `note_mentions`:

```go
mentions := util.ExtractMentions(message)
// Creates: @user@domain → NoteMention entry
```

### URL Linkification

URLs in content are converted to clickable links in the web view.

---

## ActivityPub Representation

Note maps to ActivityPub Object:

```json
{
    "@context": "https://www.w3.org/ns/activitystreams",
    "type": "Note",
    "id": "https://domain/users/username/posts/uuid",
    "attributedTo": "https://domain/users/username",
    "content": "Note content here",
    "published": "2024-01-15T10:30:00Z",
    "inReplyTo": "https://remote.server/posts/parent-id",
    "to": ["https://www.w3.org/ns/activitystreams#Public"],
    "cc": ["https://domain/users/username/followers"],
    "sensitive": false,
    "summary": "Content warning text"
}
```

### Federation

Notes are sent to followers via Create activity:

```json
{
    "type": "Create",
    "actor": "https://domain/users/username",
    "object": { /* Note object */ }
}
```

---

## Database Operations

### Read Operations

| Function | Description |
|----------|-------------|
| `ReadNoteById(id)` | Get single note by UUID |
| `ReadNotesByUserId(id)` | Get user's notes (for MyPosts) |
| `ReadNotesByUsername(name)` | Get user's notes by username |
| `ReadNotes()` | Get all notes for timeline |
| `ReadNotesByTag(tag)` | Get notes with hashtag |
| `GetNoteByObjectURI(uri)` | Lookup by ActivityPub URI |
| `GetRepliesTo(uri)` | Get reply thread |

### Write Operations

| Function | Description |
|----------|-------------|
| `SaveNote(note)` | Create new note |
| `UpdateNote(id, message)` | Edit note content |
| `DeleteNote(id)` | Remove note |
| `IncrementReplyCount(uri)` | Bump reply counter |
| `IncrementLikeCount(id)` | Bump like counter |
| `IncrementBoostCount(id)` | Bump boost counter |

---

## UI Integration

### WriteNote View

- Character counter showing remaining (150 - current)
- @mention autocomplete
- Reply mode showing parent context
- Edit mode preserving timestamps

### MyPosts View

- Paginated list of user's notes
- Edit/delete actions
- Engagement counters display

### HomeTimeline View

- Combined local and remote posts
- Thread navigation on Enter
- Auto-refresh every 30 seconds

### ThreadView

- Nested reply display
- Parent-child relationships

---

## Relationships

| Related Entity | Relationship | Description |
|----------------|--------------|-------------|
| Account | Many-to-One | Note belongs to author |
| Hashtag | Many-to-Many | Via note_hashtags |
| NoteMention | One-to-Many | Note contains mentions |
| Like | One-to-Many | Note receives likes |
| Boost | One-to-Many | Note receives boosts |
| Activity | One-to-One | Federated representation |

---

## Source Files

- `domain/notes.go` - Note struct definitions
- `db/db.go` - Database operations
- `ui/writenote/` - Note composition view
- `ui/myposts/` - User's notes view
- `ui/hometimeline/` - Timeline view
- `ui/threadview/` - Thread display
- `activitypub/outbox.go` - Federation sending
