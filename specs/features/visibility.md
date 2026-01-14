# Visibility Settings

This document specifies note visibility options and their behavior.

---

## Overview

Stegodon supports four visibility levels for notes:
- **Public** - Visible to everyone, federates broadly
- **Unlisted** - Visible to everyone, doesn't appear in public timelines
- **Followers** - Only visible to followers
- **Direct** - Only visible to mentioned users

---

## Data Model

### Note Structure

```go
type Note struct {
    // ... other fields
    Visibility string // "public", "unlisted", "followers", "direct"
    // ...
}
```

### Database Schema

```sql
CREATE TABLE notes (
    -- ... other columns
    visibility TEXT DEFAULT 'public',
    -- ...
);
```

---

## Visibility Options

### Public

| Property | Value |
|----------|-------|
| String | `"public"` |
| Timeline | Yes |
| Federates | Yes |
| ActivityPub `to` | `https://www.w3.org/ns/activitystreams#Public` |
| ActivityPub `cc` | Followers collection |

### Unlisted

| Property | Value |
|----------|-------|
| String | `"unlisted"` |
| Timeline | No (public timelines) |
| Federates | Yes |
| ActivityPub `to` | Followers collection |
| ActivityPub `cc` | `https://www.w3.org/ns/activitystreams#Public` |

### Followers

| Property | Value |
|----------|-------|
| String | `"followers"` |
| Timeline | No |
| Federates | To followers only |
| ActivityPub `to` | Followers collection |
| ActivityPub `cc` | (empty) |

### Direct

| Property | Value |
|----------|-------|
| String | `"direct"` |
| Timeline | No |
| Federates | To mentioned users only |
| ActivityPub `to` | Mentioned actor URIs |
| ActivityPub `cc` | (empty) |

---

## ActivityPub Addressing

### Public Note

```json
{
  "type": "Create",
  "object": {
    "type": "Note",
    "to": ["https://www.w3.org/ns/activitystreams#Public"],
    "cc": ["https://example.com/users/alice/followers"]
  }
}
```

### Unlisted Note

```json
{
  "type": "Create",
  "object": {
    "type": "Note",
    "to": ["https://example.com/users/alice/followers"],
    "cc": ["https://www.w3.org/ns/activitystreams#Public"]
  }
}
```

### Followers-Only Note

```json
{
  "type": "Create",
  "object": {
    "type": "Note",
    "to": ["https://example.com/users/alice/followers"],
    "cc": []
  }
}
```

### Direct Message

```json
{
  "type": "Create",
  "object": {
    "type": "Note",
    "to": ["https://remote.server/users/bob"],
    "cc": []
  }
}
```

---

## Timeline Visibility

### Home Timeline

| Visibility | Shown |
|------------|-------|
| Public | Yes |
| Unlisted | Yes |
| Followers | Yes (if following) |
| Direct | Yes (if mentioned) |

### Public/Federated Timeline

| Visibility | Shown |
|------------|-------|
| Public | Yes |
| Unlisted | No |
| Followers | No |
| Direct | No |

### Profile Page (Web)

| Visibility | Shown |
|------------|-------|
| Public | Yes |
| Unlisted | Yes |
| Followers | No (unless authenticated follower) |
| Direct | No |

---

## Federation Behavior

### Public Posts

- Delivered to all followers
- Included in relay distribution
- Visible in public collections

### Unlisted Posts

- Delivered to all followers
- NOT included in relay distribution
- NOT in public collections
- Accessible by direct URL

### Followers-Only Posts

- Delivered only to followers
- NOT accessible by direct URL (403)
- NOT in any public collection

### Direct Messages

- Delivered only to mentioned users
- NOT accessible by anyone else
- NOT in any collection

---

## Current Status

### Implemented

| Feature | Status |
|---------|--------|
| Data model field | Yes |
| Database storage | Yes |
| Default to public | Yes |
| Parse from ActivityPub | Partial |

### Not Yet Implemented

| Feature | Status |
|---------|--------|
| TUI visibility selector | No |
| Visibility icons in TUI | No |
| Access control enforcement | Partial |
| Visibility in web UI | No |

---

## Default Behavior

### New Notes

All locally created notes default to public:

```go
note := domain.SaveNote{
    UserId:  m.userId,
    Message: value,
    // Visibility defaults to "public" in database
}
```

---

## Visibility Icons

### Planned TUI Display

| Visibility | Icon |
|------------|------|
| Public | (none) |
| Unlisted | `🔓` |
| Followers | `🔒` |
| Direct | `✉️` |

---

## Web Display

### Profile Page

Only public and unlisted posts shown:

```go
func GetProfile(username string) {
    // Query only public and unlisted posts
    notes := database.ReadPublicNotesByUsername(username)
}
```

---

## RSS Feed

### Feed Contents

Only public posts appear in RSS feeds:

```go
func GetRSS(username string) {
    // Only include public posts
    notes := database.ReadPublicNotesByUsername(username)
}
```

---

## Future Enhancements

### Planned Features

1. **TUI Selector** - Dropdown in writenote for visibility
2. **Visibility Icons** - Show lock/envelope icons in timeline
3. **Access Control** - Enforce visibility on web endpoints
4. **DM View** - Separate view for direct messages

---

## Source Files

- `domain/notes.go` - `Visibility` field
- `db/db.go` - Database schema, query filtering
- `activitypub/outbox.go` - ActivityPub addressing
- `web/handlers.go` - Web visibility filtering
