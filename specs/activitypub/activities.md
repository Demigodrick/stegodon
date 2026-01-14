# Supported Activities

This document specifies all ActivityPub activity types supported by stegodon for sending and receiving.

---

## Overview

Stegodon supports these ActivityPub activity types:

| Activity | Send | Receive | Description |
|----------|------|---------|-------------|
| Follow | ✓ | ✓ | Follow a user or subscribe to relay |
| Accept | ✓ | ✓ | Accept a follow request |
| Undo | ✓ | ✓ | Undo follow, like, or announce |
| Create | ✓ | ✓ | Create a new note/post |
| Update | ✓ | ✓ | Edit a note or update profile |
| Delete | ✓ | ✓ | Delete a note or account |
| Like | ✓ | ✓ | Like a note |
| Announce | - | ✓ | Boost or relay-forwarded content |

---

## Follow Activity

### Purpose

Establishes a subscription relationship between users.

### Outgoing (SendFollow)

```json
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "id": "https://example.com/activities/{uuid}",
  "type": "Follow",
  "actor": "https://example.com/users/alice",
  "object": "https://mastodon.social/users/bob"
}
```

**Behavior:**
1. Fetch remote actor via WebFinger/ActivityPub
2. Check for self-follow (rejected)
3. Check for existing follow (rejected if exists)
4. Store follow as pending (`Accepted: false`)
5. Send Follow activity to target's inbox
6. Wait for Accept activity

### Incoming (handleFollowActivity)

```json
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "id": "https://mastodon.social/activities/{id}",
  "type": "Follow",
  "actor": "https://mastodon.social/users/bob",
  "object": "https://example.com/users/alice"
}
```

**Behavior:**
1. Verify HTTP signature
2. Get or fetch remote actor
3. Check for existing follow (skip creation if exists)
4. Create follow record (`Accepted: true` - auto-accept)
5. Create follow notification
6. Send Accept activity back

---

## Accept Activity

### Purpose

Confirms acceptance of a Follow request.

### Outgoing (SendAccept)

```json
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "id": "https://example.com/activities/{uuid}",
  "type": "Accept",
  "actor": "https://example.com/users/alice",
  "object": {
    "id": "https://mastodon.social/activities/{follow-id}",
    "type": "Follow",
    "actor": "https://mastodon.social/users/bob",
    "object": "https://example.com/users/alice"
  }
}
```

**Sent automatically** when processing incoming Follow activities.

### Incoming (handleAcceptActivity)

**Behavior:**
1. Extract Follow ID from object
2. Check if Accept is from a relay → Update relay status to "active"
3. Otherwise → Mark follow as accepted (`Accepted: true`)

---

## Undo Activity

### Purpose

Reverses a previous activity (unfollow, unlike, unboost).

### Outgoing - Undo Follow (SendUndo)

```json
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "id": "https://example.com/activities/{uuid}",
  "type": "Undo",
  "actor": "https://example.com/users/alice",
  "object": {
    "id": "https://example.com/activities/{original-follow-id}",
    "type": "Follow",
    "actor": "https://example.com/users/alice",
    "object": "https://mastodon.social/users/bob"
  }
}
```

### Outgoing - Undo Like (SendUndoLike)

```json
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "id": "https://example.com/activities/{uuid}",
  "type": "Undo",
  "actor": "https://example.com/users/alice",
  "object": {
    "id": "https://example.com/activities/{original-like-id}",
    "type": "Like",
    "actor": "https://example.com/users/alice",
    "object": "https://mastodon.social/notes/{note-id}"
  }
}
```

### Incoming (handleUndoActivity)

**Supports undoing:**

| Undone Activity | Action |
|-----------------|--------|
| Follow | Delete follow relationship |
| Like | Delete like, decrement like count |
| Announce | Delete boost, decrement boost count |

**Authorization:** Only the original actor can undo their activities.

```go
if remoteActor.ActorURI != undo.Actor {
    return fmt.Errorf("unauthorized: actor cannot undo this activity")
}
```

---

## Create Activity

### Purpose

Publishes a new note/post to followers.

### Outgoing (SendCreate)

```json
{
  "@context": [
    "https://www.w3.org/ns/activitystreams",
    {"Hashtag": "as:Hashtag"}
  ],
  "id": "https://example.com/activities/{uuid}",
  "type": "Create",
  "actor": "https://example.com/users/alice",
  "published": "2024-01-15T10:30:00Z",
  "to": ["https://www.w3.org/ns/activitystreams#Public"],
  "cc": [
    "https://example.com/users/alice/followers",
    "https://mastodon.social/users/bob"
  ],
  "object": {
    "id": "https://example.com/notes/{uuid}",
    "type": "Note",
    "attributedTo": "https://example.com/users/alice",
    "content": "<p>Hello @bob@mastodon.social! Check out #fediverse</p>",
    "mediaType": "text/html",
    "published": "2024-01-15T10:30:00Z",
    "url": "https://example.com/u/alice/{uuid}",
    "to": ["https://www.w3.org/ns/activitystreams#Public"],
    "cc": ["https://example.com/users/alice/followers"],
    "inReplyTo": "https://mastodon.social/notes/{parent-id}",
    "tag": [
      {
        "type": "Hashtag",
        "href": "https://example.com/tags/fediverse",
        "name": "#fediverse"
      },
      {
        "type": "Mention",
        "href": "https://mastodon.social/users/bob",
        "name": "@bob@mastodon.social"
      }
    ]
  }
}
```

**Delivery targets:**
- All followers' inboxes
- Parent author inbox (if reply)
- Mentioned users' inboxes
- Active relay inboxes

### Incoming (handleCreateActivity)

**Acceptance criteria:**
1. Sender is followed by local user, OR
2. Content is from a subscribed relay, OR
3. Content is a reply to local user's post

**Processing:**
1. Verify signature and fetch actor
2. Check acceptance criteria
3. Increment parent's reply count (if reply)
4. Create reply notification (if reply)
5. Process mentions and create notifications
6. Store activity in database

---

## Update Activity

### Purpose

Modifies an existing note or actor profile.

### Outgoing (SendUpdate)

```json
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "id": "https://example.com/activities/{uuid}",
  "type": "Update",
  "actor": "https://example.com/users/alice",
  "to": ["https://www.w3.org/ns/activitystreams#Public"],
  "cc": ["https://example.com/users/alice/followers"],
  "object": {
    "id": "https://example.com/notes/{note-id}",
    "type": "Note",
    "attributedTo": "https://example.com/users/alice",
    "content": "<p>Updated content here</p>",
    "published": "2024-01-15T10:30:00Z",
    "updated": "2024-01-15T11:00:00Z"
  }
}
```

### Incoming (handleUpdateActivity)

**Supported object types:**

| Object Type | Action |
|-------------|--------|
| Person | Re-fetch and update cached remote account |
| Note/Article | Update stored activity's raw JSON |

**Note update handling:**
- If original Create exists: Update the stored raw JSON
- If original not found: Create as new activity (handles late-follow scenario)

---

## Delete Activity

### Purpose

Removes a note or signals account deletion.

### Outgoing (SendDelete)

```json
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "id": "https://example.com/activities/{uuid}",
  "type": "Delete",
  "actor": "https://example.com/users/alice",
  "published": "2024-01-15T12:00:00Z",
  "to": ["https://www.w3.org/ns/activitystreams#Public"],
  "cc": ["https://example.com/users/alice/followers"],
  "object": "https://example.com/notes/{note-id}"
}
```

### Incoming (handleDeleteActivity)

**Two deletion types:**

1. **Actor deletion** (object URI matches actor URI):
   - Delete all follows to/from the actor
   - Delete the remote account record

2. **Object deletion** (post, note):
   - Verify actor authorization (must match object author)
   - Delete the activity from database

**Authorization check:**
```go
if activity.ActorURI != delete.Actor {
    return fmt.Errorf("unauthorized: cannot delete content created by another")
}
```

---

## Like Activity

### Purpose

Expresses appreciation for a note.

### Outgoing (SendLike)

```json
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "id": "https://example.com/activities/{uuid}",
  "type": "Like",
  "actor": "https://example.com/users/alice",
  "object": "https://mastodon.social/notes/{note-id}"
}
```

**Note:** Likes for local notes are not sent via ActivityPub (only stored locally).

### Incoming (handleLikeActivity)

**Processing:**
1. Find local note by object URI
2. Fetch/get remote account for liker
3. Check for duplicate (dedupe by account+note)
4. Create Like record
5. Increment like count on note
6. Create notification for note author

---

## Announce Activity

### Purpose

Boosts/reblogs content or forwards relay content.

### Incoming Only (handleAnnounceActivity)

Stegodon **receives** but does not **send** Announce activities.

**Two types of Announce:**

### 1. Standard Boost

```json
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "id": "https://mastodon.social/activities/{id}",
  "type": "Announce",
  "actor": "https://mastodon.social/users/bob",
  "object": "https://example.com/notes/{note-id}"
}
```

**Processing:**
1. Find local note being boosted
2. Fetch/get remote account for booster
3. Check for duplicate boost
4. Create Boost record
5. Increment boost count on note

### 2. Relay-Forwarded Content

```json
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "id": "https://relay.fedi.buzz/activities/{id}",
  "type": "Announce",
  "actor": "https://relay.fedi.buzz/tag/music",
  "published": "2024-01-15T10:30:00Z",
  "object": {
    "id": "https://mastodon.social/notes/{note-id}",
    "type": "Note",
    "attributedTo": "https://mastodon.social/users/alice",
    "content": "<p>Check out this music!</p>"
  }
}
```

**Processing:**
1. Detect relay (actor matches subscribed relay or domain)
2. Check if relay is paused → Skip if paused
3. Check for duplicate by announce ID or object URI
4. Fetch object content (embedded or via HTTP)
5. Store as Create activity (for timeline display)
6. Mark as `FromRelay: true`

---

## Activity Context

### Standard Context

```json
{
  "@context": "https://www.w3.org/ns/activitystreams"
}
```

### Extended Context (with hashtags)

```json
{
  "@context": [
    "https://www.w3.org/ns/activitystreams",
    {"Hashtag": "as:Hashtag"}
  ]
}
```

---

## Addressing

### Public Addressing

```json
{
  "to": ["https://www.w3.org/ns/activitystreams#Public"],
  "cc": ["https://example.com/users/alice/followers"]
}
```

### Direct Addressing (for replies/mentions)

```json
{
  "cc": [
    "https://example.com/users/alice/followers",
    "https://mastodon.social/users/bob"
  ]
}
```

---

## Activity ID Format

All activity IDs follow this pattern:

```
https://{domain}/activities/{uuid}
```

Example: `https://example.com/activities/550e8400-e29b-41d4-a716-446655440000`

---

## Note Object ID Format

```
https://{domain}/notes/{uuid}
```

Example: `https://example.com/notes/550e8400-e29b-41d4-a716-446655440001`

---

## Source Files

- `activitypub/outbox.go` - Activity creation and sending
- `activitypub/inbox.go` - Activity receiving and processing
- `activitypub/delivery.go` - Queued activity delivery
- `domain/activity.go` - Activity entity definition
