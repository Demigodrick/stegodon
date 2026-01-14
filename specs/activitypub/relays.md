# Relay Support

This document specifies the ActivityPub relay integration for FediBuzz and YUKIMOCHI relay types.

---

## Overview

Relays amplify content across the fediverse by forwarding posts to subscribers. Stegodon supports:
- **FediBuzz**: Hashtag-based topic relays
- **YUKIMOCHI Activity-Relay**: Firehose relays

Both relay types use the same subscription mechanism but differ in content delivery.

---

## Supported Relay Types

| Relay | URL Pattern | Content Type |
|-------|-------------|--------------|
| FediBuzz | `https://relay.fedi.buzz/tag/{topic}` | Hashtag-specific posts |
| YUKIMOCHI | `https://relay.toot.yukimochi.jp/actor` | All public posts from subscribers |

---

## Relay Entity

```go
type Relay struct {
    Id          uuid.UUID
    ActorURI    string     // Relay actor URI (e.g., https://relay.fedi.buzz/tag/music)
    InboxURI    string     // Relay inbox for sending activities
    FollowURI   string     // Our Follow activity URI (for Undo)
    Name        string     // Display name from actor
    Status      string     // "pending", "active", "failed"
    Paused      bool       // User-toggled pause state
    AcceptedAt  *time.Time // When relay accepted our subscription
    CreatedAt   time.Time
}
```

---

## Subscription Flow

### Subscribe to Relay

```
Admin enters relay URL
      │
      ▼
Normalize URL to Actor URI
      │
      ▼
Fetch Relay Actor
      │
      ▼
Check for Existing Subscription
      │
      ├── Active → Return error "already subscribed"
      ├── Pending → Return error "subscription pending"
      └── Failed → Delete old record, retry
            │
            ▼
Create Relay Record (status: pending)
      │
      ▼
Send Follow Activity
      │
      ▼
Wait for Accept
      │
      ▼
Update Status to "active"
```

### URL Normalization

```go
func normalizeRelayURL(input string) string {
    input = strings.TrimSpace(input)

    // If already a full URI, use as-is
    if strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://") {
        return input
    }

    // Otherwise, assume domain and construct actor URI
    return fmt.Sprintf("https://%s/actor", input)
}
```

| Input | Normalized |
|-------|------------|
| `relay.fedi.buzz` | `https://relay.fedi.buzz/actor` |
| `relay.fedi.buzz/tag/music` | `https://relay.fedi.buzz/tag/music` |
| `https://relay.example.com/actor` | `https://relay.example.com/actor` |

---

## Follow Activity (Subscription)

### Request Format

```json
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "id": "https://example.com/activities/{uuid}",
  "type": "Follow",
  "actor": "https://example.com/users/admin",
  "object": "https://www.w3.org/ns/activitystreams#Public"
}
```

**Key difference from user follows:** The `object` is the public collection, not the relay actor. This is compatible with both FediBuzz and YUKIMOCHI relays.

### SendRelayFollow Implementation

```go
func SendRelayFollowWithDeps(localAccount *domain.Account, relayActorURI string,
                              conf *util.AppConfig, client HTTPClient, database Database) error {
    // Fetch relay actor
    relayActor, err := FetchRemoteActorWithDeps(relayActorURI, client, database)
    if err != nil {
        return fmt.Errorf("failed to fetch relay actor: %w", err)
    }

    // Check for existing subscription
    err, existingRelay := database.ReadRelayByActorURI(relayActorURI)
    if err == nil && existingRelay != nil {
        if existingRelay.Status == "active" {
            return fmt.Errorf("already subscribed to relay %s", relayActorURI)
        }
        if existingRelay.Status == "pending" {
            return fmt.Errorf("subscription to relay %s is pending", relayActorURI)
        }
        // Failed status - delete and retry
        database.DeleteRelay(existingRelay.Id)
    }

    // Build Follow activity
    followID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
    actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)

    follow := map[string]any{
        "@context": "https://www.w3.org/ns/activitystreams",
        "id":       followID,
        "type":     "Follow",
        "actor":    actorURI,
        "object":   "https://www.w3.org/ns/activitystreams#Public",
    }

    // Store relay record as pending
    relay := &domain.Relay{
        Id:        uuid.New(),
        ActorURI:  relayActorURI,
        InboxURI:  relayActor.InboxURI,
        FollowURI: followID,
        Name:      relayActor.DisplayName,
        Status:    "pending",
        CreatedAt: time.Now(),
    }
    database.CreateRelay(relay)

    // Send Follow activity
    return SendActivityWithDeps(follow, relayActor.InboxURI, localAccount, conf, client)
}
```

---

## Accept Handling

When relay accepts our subscription:

```go
case handleAcceptActivity:
    // Check if Accept is from a relay
    err, relay := database.ReadRelayByActorURI(accept.Actor)
    if err == nil && relay != nil {
        // Update relay status to active
        now := time.Now()
        database.UpdateRelayStatus(relay.Id, "active", &now)
        log.Printf("Relay %s accepted our subscription", accept.Actor)
        return nil
    }
```

---

## Unsubscribe Flow

### Undo Follow Activity

```json
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "id": "https://example.com/activities/{uuid}",
  "type": "Undo",
  "actor": "https://example.com/users/admin",
  "object": {
    "id": "https://example.com/activities/{original-follow-id}",
    "type": "Follow",
    "actor": "https://example.com/users/admin",
    "object": "https://www.w3.org/ns/activitystreams#Public"
  }
}
```

### SendRelayUnfollow Implementation

```go
func SendRelayUnfollowWithDeps(localAccount *domain.Account, relay *domain.Relay,
                                conf *util.AppConfig, client HTTPClient) error {
    undoID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
    actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)

    // Use stored Follow URI if available
    followID := relay.FollowURI
    if followID == "" {
        // Fallback for relays created before FollowURI was stored
        followID = fmt.Sprintf("https://%s/relay-follows/%s", conf.Conf.SslDomain, relay.Id.String())
    }

    undo := map[string]any{
        "@context": "https://www.w3.org/ns/activitystreams",
        "id":       undoID,
        "type":     "Undo",
        "actor":    actorURI,
        "object": map[string]any{
            "id":     followID,
            "type":   "Follow",
            "actor":  actorURI,
            "object": "https://www.w3.org/ns/activitystreams#Public",
        },
    }

    return SendActivityWithDeps(undo, relay.InboxURI, localAccount, conf, client)
}
```

---

## Receiving Relay Content

### Content Delivery

Relays forward content via Announce activities:

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
    "content": "<p>Check out this #music!</p>"
  }
}
```

**Key characteristic:** The signer (relay) differs from the actor (content author).

### Relay Detection

```go
// Method 1: Exact match with subscribed relay
err, relay := database.ReadRelayByActorURI(announceActivity.Actor)
isFromRelay := err == nil && relay != nil

// Method 2: Domain-based matching (for tag-specific actors)
if !isFromRelay {
    isFromRelay = isActorFromAnyRelay(announceActivity.Actor, database)
}
```

### Domain-Based Matching

FediBuzz sends Announces from different tag actors (e.g., `/tag/prints` when subscribed to `/tag/music`):

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

func extractDomainFromURI(uri string) string {
    // https://relay.fedi.buzz/tag/music -> relay.fedi.buzz
    start := strings.Index(uri, "://") + 3
    rest := uri[start:]
    end := strings.Index(rest, "/")
    if end == -1 {
        return rest
    }
    return rest[:end]
}
```

---

## Signer vs Actor Verification

For relay content, the HTTP signature signer differs from the activity actor:

```go
// Extract signer from signature header
signerKeyId := extractKeyIdFromSignature(signature)
signerActorURI := strings.Split(signerKeyId, "#")[0]

// Verify signature with signer's key (relay's key)
signerActor, err := GetOrFetchActor(signerActorURI)
_, err = VerifyRequest(r, signerActor.PublicKeyPem)

// Detect relay content
isFromRelay := signerActorURI != activity.Actor

if isFromRelay {
    log.Printf("Activity signed by %s on behalf of %s", signerActorURI, activity.Actor)
    // Also fetch the actual content author for timeline display
    remoteActor, _ = GetOrFetchActor(activity.Actor)
}
```

---

## Handling Relay Announce

```go
func handleRelayAnnounce(announceID, objectURI string, embeddedObject map[string]any, deps *InboxDeps) error {
    database := deps.Database

    // Check for duplicates
    err, existingByAnnounce := database.ReadActivityByURI(announceID)
    if err == nil && existingByAnnounce != nil {
        return nil  // Already have this announce
    }

    err, existingActivity := database.ReadActivityByObjectURI(objectURI)
    if err == nil && existingActivity != nil {
        return nil  // Already have this object
    }

    // Get object content
    var objectContent map[string]any
    var actorURI string

    if embeddedObject != nil {
        objectContent = embeddedObject
        actorURI = embeddedObject["attributedTo"].(string)
    } else {
        objectContent, _ = fetchActivityPubObject(objectURI, deps.HTTPClient)
        actorURI = objectContent["attributedTo"].(string)
    }

    // Validate object type
    objectType := objectContent["type"].(string)
    if objectType != "Note" && objectType != "Article" {
        return nil  // Only store Note/Article
    }

    // Fetch and cache the author
    GetOrFetchActor(actorURI)

    // Store as Create activity for timeline display
    activity := &domain.Activity{
        Id:           uuid.New(),
        ActivityURI:  announceID,  // Unique identifier
        ActivityType: "Create",    // Shows in timeline
        ActorURI:     actorURI,
        ObjectURI:    objectURI,
        RawJSON:      marshalAsCreate(objectContent),
        Processed:    true,
        Local:        false,
        FromRelay:    true,
        CreatedAt:    time.Now(),
    }

    return database.CreateActivity(activity)
}
```

---

## Pause / Resume

### Pausing a Relay

Paused relays:
- Continue receiving content (logged)
- Do NOT save content to timeline
- Can be resumed without resubscribing

```go
// Check if relay is paused before storing content
if isFromRelay {
    relay := findRelayByActorDomain(signerActorURI, database)
    if relay != nil && relay.Paused {
        log.Printf("Relay content from %s skipped (relay %s is paused)",
            activity.Actor, relay.ActorURI)
        w.WriteHeader(http.StatusAccepted)
        return
    }
}
```

### Toggle Pause State

```go
func toggleRelayPause(relayId uuid.UUID, paused bool) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        err := database.UpdateRelayPaused(relayId, paused)
        return relayPausedMsg{id: relayId, paused: paused, err: err}
    }
}
```

---

## Clear Relay Content

Deletes all relay-sourced activities from the timeline:

```go
func deleteRelayContent() tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        count, err := database.DeleteRelayActivities()
        return relayContentDeletedMsg{count: count, err: err}
    }
}
```

### SQL Query

```sql
DELETE FROM activities WHERE from_relay = 1
```

---

## Delivering to Relays

When creating content, active relay inboxes are included in delivery:

```go
// Get active relays and add their inboxes
err, relays := database.ReadActiveRelays()
if err == nil && relays != nil {
    for _, relay := range *relays {
        inboxes[relay.InboxURI] = true
        log.Printf("Will deliver to relay %s", relay.ActorURI)
    }
}
```

---

## Relay Status States

| Status | Description |
|--------|-------------|
| `pending` | Follow sent, awaiting Accept |
| `active` | Subscription confirmed, receiving content |
| `failed` | Subscription failed (can retry) |

---

## Database Operations

```go
type Database interface {
    // Relay CRUD
    CreateRelay(relay *domain.Relay) error
    ReadActiveRelays() (error, *[]domain.Relay)
    ReadActiveUnpausedRelays() (error, *[]domain.Relay)
    ReadRelayByActorURI(actorURI string) (error, *domain.Relay)
    UpdateRelayStatus(id uuid.UUID, status string, acceptedAt *time.Time) error
    UpdateRelayPaused(id uuid.UUID, paused bool) error
    DeleteRelay(id uuid.UUID) error

    // Relay content
    DeleteRelayActivities() (int64, error)
}
```

---

## TUI Management Keys

| Key | Action |
|-----|--------|
| `a` | Add new relay |
| `d` | Delete/unsubscribe from relay |
| `p` | Pause/resume relay |
| `r` | Retry failed relay |
| `x` | Clear all relay content |

---

## Source Files

- `activitypub/outbox.go` - SendRelayFollow, SendRelayUnfollow
- `activitypub/inbox.go` - handleAnnounceActivity, handleRelayAnnounce, relay detection
- `ui/relay/relay.go` - Relay management TUI
- `db/db.go` - Relay database operations
- `domain/relay.go` - Relay entity definition
