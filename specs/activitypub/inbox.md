# Inbox Processing

This document specifies the inbox handler for processing incoming ActivityPub activities.

---

## Overview

The inbox handler receives and processes incoming ActivityPub activities from remote servers. It provides:
- HTTP signature verification
- Activity deduplication via UNIQUE constraint
- Handler dispatch based on activity type
- Relay content detection and pausing
- Notification creation for engagement activities

---

## Endpoint

```
POST /users/{username}/inbox
POST /inbox  (shared inbox for relays)
```

---

## Request Processing Flow

```
Incoming POST Request
      │
      ▼
Verify HTTP Signature Present
      │
      ├── Missing → 401 Unauthorized
      └── Present → Continue
            │
            ▼
Extract keyId from Signature
      │
      ▼
Read Request Body (max 1MB)
      │
      ├── Too Large → 413 Request Too Large
      └── OK → Continue
            │
            ▼
Parse Activity JSON
      │
      ▼
Fetch Signer's Actor
      │
      ▼
Verify HTTP Signature
      │
      ├── Invalid → 401 Unauthorized
      └── Valid → Continue
            │
            ▼
Check if Signer != Actor (relay content)
      │
      ├── Yes → Check if relay is paused
      │           ├── Paused → 202 Accepted (skip)
      │           └── Active → Continue
      └── No → Continue
            │
            ▼
Store Activity in Database
      │
      ├── Duplicate → 202 Accepted (skip)
      └── New → Continue
            │
            ▼
Dispatch to Activity Handler
      │
      ▼
Mark Activity as Processed
      │
      ▼
Return 202 Accepted
```

---

## Security Limits

```go
const maxBodySize = 1 * 1024 * 1024  // 1MB max body size
```

Prevents denial-of-service attacks via large payloads.

---

## Activity Deduplication

Activities are stored with a UNIQUE constraint on `activity_uri`:

```go
if err := database.CreateActivity(activityRecord); err != nil {
    if strings.Contains(err.Error(), "UNIQUE constraint failed") {
        log.Printf("Inbox: Activity %s already processed", activity.ID)
        w.WriteHeader(http.StatusAccepted)
        return
    }
}
```

This prevents duplicate processing of the same activity.

---

## Supported Activity Types

| Type | Handler | Description |
|------|---------|-------------|
| Follow | `handleFollowActivity` | Remote user following local user |
| Undo | `handleUndoActivity` | Undo Follow, Like, or Announce |
| Create | `handleCreateActivity` | New post/note from followed user |
| Like | `handleLikeActivity` | Remote user liking local post |
| Announce | `handleAnnounceActivity` | Boost or relay-forwarded content |
| Accept | `handleAcceptActivity` | Confirmation of Follow request |
| Update | `handleUpdateActivity` | Profile or post edit |
| Delete | `handleDeleteActivity` | Post or account deletion |

---

## Follow Activity

### Processing Flow

```
Follow Activity Received
      │
      ▼
Get Local Account by Username
      │
      ▼
Check if Follow Already Exists
      │
      ├── Exists → Skip creation, still send Accept
      └── New → Create Follow record
            │
            ▼
Create Follow Notification
      │
      ▼
Send Accept Activity
```

### Follow Record

```go
followRecord := &domain.Follow{
    Id:              uuid.New(),
    AccountId:       remoteActor.Id,   // The follower
    TargetAccountId: localAccount.Id,  // Being followed
    URI:             follow.ID,
    Accepted:        true,             // Auto-accept
    CreatedAt:       time.Now(),
}
```

### Notification

```go
notification := &domain.Notification{
    Id:               uuid.New(),
    AccountId:        localAccount.Id,
    NotificationType: domain.NotificationFollow,
    ActorId:          remoteActor.Id,
    ActorUsername:    remoteActor.Username,
    ActorDomain:      remoteActor.Domain,
    Read:             false,
    CreatedAt:        time.Now(),
}
```

---

## Undo Activity

Supports undoing:
- **Follow**: Deletes follow relationship
- **Like**: Deletes like and decrements count
- **Announce**: Deletes boost and decrements count

### Authorization

Only the original actor can undo their activities:

```go
if remoteActor.ActorURI != undo.Actor {
    return fmt.Errorf("unauthorized: actor %s cannot undo like", undo.Actor)
}
```

### Undo Follow

```go
// Verify actor matches
err, followActor := database.ReadRemoteAccountById(follow.AccountId)
if followActor.ActorURI != undo.Actor {
    return fmt.Errorf("unauthorized: cannot undo follow created by another")
}

// Delete the follow
database.DeleteFollowByURI(obj.ID)
```

---

## Create Activity

### Acceptance Rules

Content is accepted if:
1. Local user follows the remote actor, OR
2. Content is from a relay (signer != actor), OR
3. Content is a reply to a local user's post

```go
isFollowing := err == nil && follow != nil

if isFollowing {
    // Accept from followed user
} else if isFromRelay {
    // Accept relay-forwarded content
} else if isReplyToOurPost {
    // Accept reply to our content
} else {
    return fmt.Errorf("not following this actor")
}
```

### Reply Handling

For replies, the handler:
1. Increments parent note's reply count
2. Creates reply notification for parent author

```go
if create.Object.InReplyTo != "" {
    database.IncrementReplyCountByURI(create.Object.InReplyTo)

    // Create notification for parent author
    notification := &domain.Notification{
        NotificationType: domain.NotificationReply,
        NoteURI:          create.Object.ID,
        NotePreview:      preview,
    }
}
```

### Mention Processing

Mentions are extracted from the `tag` array and stored:

```go
for _, tag := range create.Object.Tag {
    switch tag.Type {
    case "Mention":
        mention := &domain.NoteMention{
            Id:                uuid.New(),
            NoteId:            activityRecord.Id,
            MentionedActorURI: tag.Href,
            MentionedUsername: parts[0],
            MentionedDomain:   parts[1],
        }
        database.CreateNoteMention(mention)

        // Create notification if mentioned user is local
        if parts[1] == conf.Conf.SslDomain {
            // Create mention notification
        }
    case "Hashtag":
        log.Printf("Post contains hashtag %s", tag.Name)
    }
}
```

---

## Like Activity

### Processing

```go
var likeActivity struct {
    ID     string `json:"id"`
    Actor  string `json:"actor"`
    Object string `json:"object"`  // URI of liked note
}
```

### Steps

1. Find local note by object URI
2. Get or fetch remote account for liker
3. Check for duplicate like (dedupe by account+note)
4. Create Like record
5. Increment like count on note
6. Create notification for note author

```go
like := &domain.Like{
    Id:        uuid.New(),
    AccountId: remoteAcc.Id,
    NoteId:    note.Id,
    URI:       likeActivity.ID,
    CreatedAt: time.Now(),
}
database.CreateLike(like)
database.IncrementLikeCountByNoteId(note.Id)
```

---

## Announce Activity

### Two Types

1. **Standard Boost**: User boosting a local post
2. **Relay Content**: Relay forwarding content to subscribers

### Relay Detection

```go
// Check if from subscribed relay
err, relay := database.ReadRelayByActorURI(announceActivity.Actor)
isFromRelay := err == nil && relay != nil

// Also check domain-based matching for tag-specific relays
if !isFromRelay {
    isFromRelay = isActorFromAnyRelay(announceActivity.Actor, database)
}
```

### Relay Content Handling

```go
func handleRelayAnnounce(announceID, objectURI string, embeddedObject map[string]any, deps *InboxDeps) error {
    // Check for duplicate by announce ID or object URI

    // Get object content (embedded or fetch)
    var objectContent map[string]any
    if embeddedObject != nil {
        objectContent = embeddedObject
    } else {
        objectContent = fetchActivityPubObject(objectURI)
    }

    // Store as Create activity for timeline display
    activity := &domain.Activity{
        ActivityURI:  announceID,
        ActivityType: "Create",
        ActorURI:     actorURI,
        ObjectURI:    objectURI,
        FromRelay:    true,
    }
}
```

### Standard Boost Handling

```go
boost := &domain.Boost{
    Id:        uuid.New(),
    AccountId: remoteAcc.Id,
    NoteId:    note.Id,
    URI:       announceActivity.ID,
    CreatedAt: time.Now(),
}
database.CreateBoost(boost)
database.IncrementBoostCountByNoteId(note.Id)
```

---

## Accept Activity

### Processing

Handles Accept responses to:
1. Follow requests to remote users
2. Relay subscription requests

```go
// Check if Accept is from a relay
err, relay := database.ReadRelayByActorURI(accept.Actor)
if err == nil && relay != nil {
    // Update relay status to active
    database.UpdateRelayStatus(relay.Id, "active", &now)
    return nil
}

// Standard follow acceptance
database.AcceptFollowByURI(followID)
```

---

## Update Activity

### Supported Object Types

| Type | Action |
|------|--------|
| Person | Re-fetch and update cached actor |
| Note/Article | Update stored activity content |

### Note Update

```go
err, existingActivity := database.ReadActivityByObjectURI(objectType.ID)
if err != nil || existingActivity == nil {
    // Create as new post if original not found
    newActivity := &domain.Activity{
        ActivityURI:  update.ID,
        ActivityType: "Create",
        ActorURI:     update.Actor,
        ObjectURI:    objectType.ID,
        RawJSON:      string(body),
    }
    database.CreateActivity(newActivity)
} else {
    // Update existing activity content
    existingActivity.RawJSON = string(body)
    database.UpdateActivity(existingActivity)
}
```

---

## Delete Activity

### Object Types

1. **Actor Deletion**: Actor URI matches object URI
2. **Object Deletion**: Post, note, or other content

### Actor Deletion

```go
if objectURI == delete.Actor {
    // Delete remote account and all associated data
    database.DeleteFollowsByRemoteAccountId(remoteAcc.Id)
    database.DeleteRemoteAccount(remoteAcc.Id)
}
```

### Object Deletion (Authorization Required)

```go
// Verify actor can delete this content
if activity.ActorURI != delete.Actor {
    return fmt.Errorf("unauthorized: actor %s cannot delete content created by %s",
        delete.Actor, activity.ActorURI)
}

database.DeleteActivity(activity.Id)
```

---

## Relay Content Detection

### Signer vs Actor Check

```go
signerActorURI := strings.Split(signerKeyId, "#")[0]
isFromRelay := signerActorURI != activity.Actor

if isFromRelay {
    log.Printf("Activity signed by %s on behalf of %s", signerActorURI, activity.Actor)
}
```

### Domain-Based Relay Matching

For relays like FediBuzz with multiple tag actors:

```go
func isActorFromAnyRelay(actorURI string, database Database) bool {
    actorDomain := extractDomainFromURI(actorURI)

    err, relays := database.ReadActiveRelays()
    for _, relay := range *relays {
        relayDomain := extractDomainFromURI(relay.ActorURI)
        if relayDomain == actorDomain {
            return true
        }
    }
    return false
}
```

### Paused Relay Check

```go
if isFromRelay {
    relay := findRelayByActorDomain(signerActorURI, database)
    if relay != nil && relay.Paused {
        log.Printf("Relay content skipped (relay %s is paused)", relay.ActorURI)
        w.WriteHeader(http.StatusAccepted)
        return
    }
}
```

---

## Dependency Injection

```go
type InboxDeps struct {
    Database   Database
    HTTPClient HTTPClient
}

// Production
func HandleInbox(w http.ResponseWriter, r *http.Request, username string, conf *util.AppConfig) {
    deps := &InboxDeps{
        Database:   NewDBWrapper(),
        HTTPClient: defaultHTTPClient,
    }
    HandleInboxWithDeps(w, r, username, conf, deps)
}
```

---

## HTTP Response Codes

| Code | Meaning |
|------|---------|
| 202 Accepted | Activity received and queued for processing |
| 400 Bad Request | Invalid activity format or failed to fetch signer |
| 401 Unauthorized | Missing or invalid signature |
| 413 Request Entity Too Large | Body exceeds 1MB limit |
| 500 Internal Server Error | Processing failure |

---

## Source Files

- `activitypub/inbox.go` - Main inbox handler and activity processors
- `activitypub/httpsig.go` - Signature verification
- `activitypub/actors.go` - Actor fetching for verification
- `activitypub/deps.go` - Database interface definitions
