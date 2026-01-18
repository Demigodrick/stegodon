# Engagement Entities (Like/Boost)

This document specifies the Like and Boost entities, which represent user engagement with notes.

---

## Overview

Engagement entities track user interactions with notes:
- **Like**: A favorite/heart on a note (ActivityPub `Like` activity)
- **Boost**: A reblog/repost of a note (ActivityPub `Announce` activity)

Both can originate from local or remote users and are used to update denormalized counters on notes.

---

## Like Entity

### Data Structure

```go
type Like struct {
    Id        uuid.UUID
    AccountId uuid.UUID // Who liked (local or remote)
    NoteId    uuid.UUID // Which note was liked
    URI       string    // ActivityPub Like activity URI
    CreatedAt time.Time
}
```

### Field Definitions

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| `Id` | `uuid.UUID` | Primary Key | Unique like identifier |
| `AccountId` | `uuid.UUID` | Required | Account who liked |
| `NoteId` | `uuid.UUID` | Required | Note being liked |
| `URI` | `string` | Required | ActivityPub activity URI |
| `CreatedAt` | `time.Time` | Default: now | When like occurred |

### Unique Constraint

One like per account per note:

```sql
UNIQUE(account_id, note_id)
```

### Database Schema

```sql
CREATE TABLE IF NOT EXISTS likes (
    id TEXT NOT NULL PRIMARY KEY,
    account_id TEXT NOT NULL,
    note_id TEXT NOT NULL,
    uri TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(account_id, note_id)
)
```

### Indexes

```sql
CREATE INDEX IF NOT EXISTS idx_likes_note_id ON likes(note_id);
CREATE INDEX IF NOT EXISTS idx_likes_account_id ON likes(account_id);
CREATE INDEX IF NOT EXISTS idx_likes_object_uri ON likes(object_uri);
```

---

## Boost Entity

### Data Structure

```go
type Boost struct {
    Id        uuid.UUID
    AccountId uuid.UUID // Who boosted (local or remote)
    NoteId    uuid.UUID // Which note was boosted
    URI       string    // ActivityPub Announce activity URI
    CreatedAt time.Time
}
```

### Field Definitions

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| `Id` | `uuid.UUID` | Primary Key | Unique boost identifier |
| `AccountId` | `uuid.UUID` | Required | Account who boosted |
| `NoteId` | `uuid.UUID` | Required | Note being boosted |
| `URI` | `string` | Required | ActivityPub activity URI |
| `CreatedAt` | `time.Time` | Default: now | When boost occurred |

### Unique Constraint

One boost per account per note:

```sql
UNIQUE(account_id, note_id)
```

### Database Schema

```sql
CREATE TABLE IF NOT EXISTS boosts (
    id TEXT NOT NULL PRIMARY KEY,
    account_id TEXT NOT NULL,
    note_id TEXT NOT NULL,
    uri TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(account_id, note_id)
)
```

### Indexes

```sql
CREATE INDEX IF NOT EXISTS idx_boosts_note_id ON boosts(note_id);
CREATE INDEX IF NOT EXISTS idx_boosts_account_id ON boosts(account_id);
```

---

## ActivityPub Protocol

### Like Activity

Incoming like from remote user:

```json
{
    "@context": "https://www.w3.org/ns/activitystreams",
    "type": "Like",
    "id": "https://remote.server/likes/uuid",
    "actor": "https://remote.server/users/alice",
    "object": "https://stegodon.example/users/bob/posts/note-uuid"
}
```

### Announce Activity (Boost)

Incoming boost from remote user:

```json
{
    "@context": "https://www.w3.org/ns/activitystreams",
    "type": "Announce",
    "id": "https://remote.server/announces/uuid",
    "actor": "https://remote.server/users/alice",
    "object": "https://stegodon.example/users/bob/posts/note-uuid",
    "to": ["https://www.w3.org/ns/activitystreams#Public"]
}
```

### Undo Like

```json
{
    "@context": "https://www.w3.org/ns/activitystreams",
    "type": "Undo",
    "id": "https://remote.server/undo/uuid",
    "actor": "https://remote.server/users/alice",
    "object": {
        "type": "Like",
        "id": "https://remote.server/likes/original-uuid",
        "actor": "https://remote.server/users/alice",
        "object": "https://stegodon.example/users/bob/posts/note-uuid"
    }
}
```

### Undo Announce (Unboost)

```json
{
    "@context": "https://www.w3.org/ns/activitystreams",
    "type": "Undo",
    "id": "https://remote.server/undo/uuid",
    "actor": "https://remote.server/users/alice",
    "object": {
        "type": "Announce",
        "id": "https://remote.server/announces/original-uuid",
        "actor": "https://remote.server/users/alice",
        "object": "https://stegodon.example/users/bob/posts/note-uuid"
    }
}
```

---

## Processing Flow

### Incoming Like

```
Receive Like Activity
      │
      ├── Verify HTTP signature
      ├── Parse activity
      ├── Find or create remote account
      ├── Find target note by object URI
      │
      ▼
Create Like Record
      │
      ├── Store with URI from activity
      └── Increment note.like_count
            │
            ▼
Create Notification
      │
      └── Type: "like" for note author
```

### Incoming Boost

```
Receive Announce Activity
      │
      ├── Verify HTTP signature
      ├── Parse activity
      ├── Find or create remote account
      ├── Find target note by object URI
      │
      ▼
Create Boost Record
      │
      ├── Store with URI from activity
      └── Increment note.boost_count
```

### Incoming Undo

```
Receive Undo Activity
      │
      ├── Extract inner object (Like or Announce)
      ├── Find existing record by URI
      │
      ▼
Delete Record
      │
      └── Decrement appropriate counter
```

---

## Denormalized Counters

For performance, engagement counts are stored directly on notes:

```go
type Note struct {
    // ...
    LikeCount  int
    BoostCount int
}
```

### Counter Updates

```sql
-- On Like
UPDATE notes SET like_count = like_count + 1 WHERE id = ?

-- On Unlike
UPDATE notes SET like_count = like_count - 1 WHERE id = ?

-- On Boost
UPDATE notes SET boost_count = boost_count + 1 WHERE id = ?

-- On Unboost
UPDATE notes SET boost_count = boost_count - 1 WHERE id = ?
```

### Activity Counters

Remote activities also have denormalized counters and URL fields:

```go
type Activity struct {
    // ...
    ObjectURI  string // ActivityPub object id (canonical URI, returns JSON)
    ObjectURL  string // ActivityPub object url (human-readable web UI link)
    InReplyTo  string // For Create activities, the URI this is a reply to (indexed for fast lookups)
    ReplyCount int    // Denormalized reply count
    LikeCount  int    // Denormalized like count
    BoostCount int    // Denormalized boost count
}
```

---

## Database Operations

### Like Operations

| Function | Description |
|----------|-------------|
| `CreateLike(like)` | Create like and increment counter |
| `DeleteLike(uri)` | Remove like and decrement counter |
| `GetLike(accountId, noteId)` | Check if like exists |
| `GetLikeByURI(uri)` | Lookup by ActivityPub URI |
| `GetLikesForNote(noteId)` | List who liked a note |

### Boost Operations

| Function | Description |
|----------|-------------|
| `CreateBoost(boost)` | Create boost and increment counter |
| `DeleteBoost(uri)` | Remove boost and decrement counter |
| `GetBoost(accountId, noteId)` | Check if boost exists |
| `GetBoostByURI(uri)` | Lookup by ActivityPub URI |
| `GetBoostsForNote(noteId)` | List who boosted a note |

---

## UI Display

### Timeline View

Engagement counts shown per post:

```
┌─────────────────────────────────────┐
│ @alice                              │
│ This is my post content...          │
│                                     │
│ 💬 5  ❤️ 12  🔄 3                    │
└─────────────────────────────────────┘
```

| Icon | Meaning |
|------|---------|
| 💬 | Reply count |
| ❤️ | Like count |
| 🔄 | Boost count |

### Notifications

Likes generate notifications for the note author:

```
❤️ @alice@mastodon.social liked your post
   "First 100 chars of your post..."
```

---

## Account ID Resolution

Engagement can come from local or remote accounts:

### Local Account

```go
// AccountId references accounts.id
like := Like{
    AccountId: localAccount.Id,
    NoteId:    note.Id,
    URI:       generateLikeURI(),
}
```

### Remote Account

```go
// AccountId references remote_accounts.id
like := Like{
    AccountId: remoteAccount.Id,
    NoteId:    note.Id,
    URI:       activity.Id, // From incoming activity
}
```

---

## Cascade Deletion

When a note is deleted:

```sql
-- Delete all likes on this note
DELETE FROM likes WHERE note_id = ?

-- Delete all boosts of this note
DELETE FROM boosts WHERE note_id = ?
```

When an account is deleted:

```sql
-- Delete all likes by this account
DELETE FROM likes WHERE account_id = ?

-- Delete all boosts by this account
DELETE FROM boosts WHERE account_id = ?
```

---

## Relationships

| Related Entity | Relationship | Description |
|----------------|--------------|-------------|
| Account | Many-to-One | Local user who liked/boosted |
| RemoteAccount | Many-to-One | Remote user who liked/boosted |
| Note | Many-to-One | Note being liked/boosted |
| Notification | One-to-One | Like creates notification |

---

## Source Files

- `domain/activitypub.go` - Like and Boost structs
- `db/db.go` - Database operations
- `db/migrations.go` - Table creation
- `activitypub/inbox.go` - Incoming like/boost handling
- `activitypub/outbox.go` - Outgoing like/boost (if implemented)
