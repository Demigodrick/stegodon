# Relay Entity

This document specifies the Relay entity, which represents ActivityPub relay subscriptions for receiving federated content.

---

## Overview

A Relay is a subscription to an ActivityPub relay server that forwards content from across the fediverse. Relays allow small instances to receive content without direct follow relationships.

Stegodon supports two relay types:
- **FediBuzz**: Hashtag-based relays (e.g., `relay.fedi.buzz/tag/music`)
- **YUKIMOCHI**: Firehose relays (e.g., `relay.toot.yukimochi.jp`)

---

## Data Structure

```go
type Relay struct {
    Id         uuid.UUID
    ActorURI   string     // Relay's actor URI
    InboxURI   string     // Relay's inbox for activities
    FollowURI  string     // Our Follow activity URI (for Undo)
    Name       string     // Display name from relay
    Status     string     // pending, active, failed
    Paused     bool       // If true, log but don't save content
    CreatedAt  time.Time
    AcceptedAt *time.Time // When relay accepted our Follow
}
```

---

## Field Definitions

### Identity Fields

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| `Id` | `uuid.UUID` | Primary Key | Unique relay identifier |
| `ActorURI` | `string` | Unique, Required | Relay's ActivityPub actor URL |
| `InboxURI` | `string` | Required | Where to send activities |

### Subscription Fields

| Field | Type | Description |
|-------|------|-------------|
| `FollowURI` | `string` | URI of our Follow activity |
| `Name` | `string` | Relay display name |

### State Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Status` | `string` | `"pending"` | Subscription status |
| `Paused` | `bool` | `false` | Content reception paused |
| `CreatedAt` | `time.Time` | now | When subscription created |
| `AcceptedAt` | `*time.Time` | null | When relay accepted |

---

## Status Values

| Status | Description |
|--------|-------------|
| `pending` | Follow sent, waiting for Accept |
| `active` | Relay accepted, receiving content |
| `failed` | Subscription failed or rejected |

---

## Database Schema

```sql
CREATE TABLE IF NOT EXISTS relays (
    id TEXT NOT NULL PRIMARY KEY,
    actor_uri TEXT UNIQUE NOT NULL,
    inbox_uri TEXT NOT NULL,
    follow_uri TEXT,
    name TEXT,
    status TEXT DEFAULT 'pending',
    paused INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    accepted_at TIMESTAMP
)
```

### Indexes

```sql
CREATE INDEX IF NOT EXISTS idx_relays_status ON relays(status);
```

---

## Relay Types

### FediBuzz Relays

Hashtag-based content filtering:

```
URL: https://relay.fedi.buzz/tag/music
```

- Subscribe to specific hashtags
- Content wrapped in Announce activities
- Actor: the relay server
- Object: the original post

### YUKIMOCHI Relays

Firehose of all content:

```
URL: https://relay.toot.yukimochi.jp
```

- Receive all public content
- Raw Create activities forwarded
- Uses shared inbox (`/inbox`)
- Follow object: `https://www.w3.org/ns/activitystreams#Public`

---

## Subscription Protocol

### Sending Follow

```
Admin adds relay URL
      │
      ▼
Fetch Relay Actor
      │
      ├── GET relay actor URI
      └── Extract inbox, name
            │
            ▼
Create Follow Activity
      │
      ├── Generate Follow URI
      ├── Object: relay actor (or #Public for YUKIMOCHI)
      └── Queue delivery
            │
            ▼
Store Relay Record
      │
      └── Status: "pending"
```

### Follow Activity

```json
{
    "@context": "https://www.w3.org/ns/activitystreams",
    "type": "Follow",
    "id": "https://stegodon.example/follows/relay-uuid",
    "actor": "https://stegodon.example/actor",
    "object": "https://relay.fedi.buzz/actor"
}
```

### For YUKIMOCHI

```json
{
    "@context": "https://www.w3.org/ns/activitystreams",
    "type": "Follow",
    "id": "https://stegodon.example/follows/relay-uuid",
    "actor": "https://stegodon.example/actor",
    "object": "https://www.w3.org/ns/activitystreams#Public"
}
```

### Receiving Accept

```
Receive Accept Activity
      │
      ├── Find relay by Follow URI
      ├── Update status: "active"
      └── Set accepted_at timestamp
```

---

## Content Reception

### FediBuzz Content

Wrapped in Announce:

```json
{
    "type": "Announce",
    "actor": "https://relay.fedi.buzz/actor",
    "object": {
        "type": "Note",
        "id": "https://remote.server/posts/uuid",
        "attributedTo": "https://remote.server/users/alice",
        "content": "Post content..."
    }
}
```

### YUKIMOCHI Content

Raw Create forwarded:

```json
{
    "type": "Create",
    "actor": "https://remote.server/users/alice",
    "object": {
        "type": "Note",
        "id": "https://remote.server/posts/uuid",
        "content": "Post content..."
    }
}
```

### Signature Verification

For relay content, signature verification uses:
- **Signer's key**: The relay server's public key
- **Not actor's key**: Content author may differ from signer

```go
// Check if activity is from relay
if activity.FromRelay {
    // Verify against relay's public key
    verifySignature(request, relay.PublicKey)
} else {
    // Verify against actor's public key
    verifySignature(request, actor.PublicKey)
}
```

---

## Pause/Resume

### Pausing

When paused:
- Incoming activities are logged
- Content is NOT saved to timeline
- Useful for high-volume relays

```go
if relay.Paused {
    log.Printf("Relay %s paused, skipping content", relay.Name)
    return
}
```

### Resuming

Unpausing resumes normal content reception. Previously skipped content is not retroactively fetched.

---

## Unsubscribe Protocol

### Sending Undo

```
Admin deletes relay
      │
      ▼
Create Undo Activity
      │
      ├── Reference original Follow URI
      └── Queue delivery
            │
            ▼
Delete Relay Record
```

### Undo Activity

```json
{
    "@context": "https://www.w3.org/ns/activitystreams",
    "type": "Undo",
    "id": "https://stegodon.example/undo/uuid",
    "actor": "https://stegodon.example/actor",
    "object": {
        "type": "Follow",
        "id": "https://stegodon.example/follows/original-relay-uuid",
        "actor": "https://stegodon.example/actor",
        "object": "https://relay.fedi.buzz/actor"
    }
}
```

---

## Content Deletion

Admin can delete all content from a specific relay:

```sql
-- Delete activities from relay
DELETE FROM activities WHERE from_relay = 1 AND actor_uri LIKE '%relay.fedi.buzz%'
```

This removes all posts received via that relay from the timeline.

---

## Database Operations

### Read Operations

| Function | Description |
|----------|-------------|
| `GetRelays()` | List all relay subscriptions |
| `GetActiveRelays()` | List active (accepted) relays |
| `GetRelayByActorURI(uri)` | Lookup by actor URI |
| `GetRelayById(id)` | Lookup by ID |

### Write Operations

| Function | Description |
|----------|-------------|
| `CreateRelay(relay)` | Add new subscription |
| `UpdateRelayStatus(id, status)` | Change status |
| `SetRelayPaused(id, paused)` | Toggle pause state |
| `SetRelayAccepted(id)` | Mark as accepted |
| `DeleteRelay(id)` | Remove subscription |
| `DeleteRelayContent(actorURI)` | Remove relay's content |

---

## UI Integration

### Relay Management View (Admin)

```
┌─────────────────────────────────────────────┐
│ Relay Management                            │
├─────────────────────────────────────────────┤
│ ● relay.fedi.buzz/tag/music     [active]    │
│ ○ relay.toot.yukimochi.jp       [paused]    │
│ ⚠ relay.example.com             [failed]    │
└─────────────────────────────────────────────┘
```

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `a` | Add new relay |
| `d` | Delete/unsubscribe |
| `p` | Pause/resume relay |
| `r` | Retry failed relay |
| `x` | Clear relay content |

### Status Indicators

| Symbol | Status |
|--------|--------|
| ● | Active |
| ○ | Paused |
| ⏳ | Pending |
| ⚠ | Failed |

---

## Activity Tracking

Activities from relays are marked:

```go
type Activity struct {
    // ...
    FromRelay bool // true if forwarded by relay
}
```

This enables:
- Filtering relay content from timeline
- Bulk deletion of relay content
- Different signature verification

---

## Retry Logic

For failed relays, admin can retry:

```
Retry subscription
      │
      ├── Reset status to "pending"
      ├── Generate new Follow URI
      └── Queue Follow delivery
```

---

## Security Considerations

### Signature Verification

Relay-forwarded content uses relay's signature:
- Signer = relay server
- Actor = original post author

Must verify against signer's key, not actor's key.

### Content Trust

Relay content is less trusted than direct follows:
- No direct relationship with content author
- Relay could forward malicious content
- Consider pausing high-volume relays

### Rate Limiting

Relays can send high volumes. Consider:
- Pausing during maintenance
- Deleting old relay content
- Monitoring database size

---

## Relationships

| Related Entity | Relationship | Description |
|----------------|--------------|-------------|
| Activity | One-to-Many | Relay forwards activities |
| RemoteAccount | Indirect | Content authors cached |

---

## Source Files

- `domain/activitypub.go` - Relay struct
- `db/db.go` - Database operations
- `db/migrations.go` - Table creation
- `activitypub/inbox.go` - Relay content handling
- `activitypub/outbox.go` - Follow/Undo for relays
- `ui/relay/` - Relay management view
