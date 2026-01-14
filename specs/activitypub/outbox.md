# Outbox / Sending Activities

This document specifies the outbox handlers for sending ActivityPub activities to remote servers.

---

## Overview

The outbox module handles sending activities from local users to remote servers. It provides:
- Activity construction and serialization
- HTTP signature signing
- Delivery queue for reliability
- Mention resolution via WebFinger
- Inbox collection for followers, replies, and relays

---

## Delivery Architecture

```
Local Action (create note, follow, like)
      │
      ▼
Build Activity Object
      │
      ▼
Collect Target Inboxes
      │
      ├── Followers' inboxes
      ├── Parent author inbox (for replies)
      ├── Mentioned users' inboxes
      └── Active relay inboxes
            │
            ▼
Queue Delivery Items
      │
      ▼
Background Worker Processes Queue
      │
      ▼
Sign and Send HTTP Request
      │
      ├── Success → Remove from queue
      └── Failure → Retry with backoff
```

---

## Supported Activities

| Activity | Function | Description |
|----------|----------|-------------|
| Create | `SendCreate()` | New note publication |
| Update | `SendUpdate()` | Note edit |
| Delete | `SendDelete()` | Note deletion |
| Follow | `SendFollow()` | Follow remote user |
| Undo (Follow) | `SendUndo()` | Unfollow |
| Like | `SendLike()` | Like remote note |
| Undo (Like) | `SendUndoLike()` | Unlike |
| Accept | `SendAccept()` | Accept incoming follow |
| Follow (Relay) | `SendRelayFollow()` | Subscribe to relay |
| Undo (Relay) | `SendRelayUnfollow()` | Unsubscribe from relay |

---

## SendActivity (Base Function)

### Signature

```go
func SendActivityWithDeps(activity any, inboxURI string, localAccount *domain.Account,
                          conf *util.AppConfig, client HTTPClient) error
```

### Request Construction

```go
// Marshal activity to JSON
activityJSON, err := json.Marshal(activity)

// Calculate digest for HTTP signature
hash := sha256.Sum256(activityJSON)
digest := "SHA-256=" + base64.StdEncoding.EncodeToString(hash[:])

// Create HTTP request
req, err := http.NewRequest("POST", inboxURI, bytes.NewReader(activityJSON))

// Set headers
req.Header.Set("Content-Type", "application/activity+json")
req.Header.Set("Accept", "application/activity+json")
req.Header.Set("User-Agent", "stegodon/1.0 ActivityPub")
req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
req.Header.Set("Host", req.URL.Host)
req.Header.Set("Digest", digest)

// Sign request
keyID := fmt.Sprintf("https://%s/users/%s#main-key", conf.Conf.SslDomain, localAccount.Username)
SignRequest(req, privateKey, keyID)

// Send
resp, err := client.Do(req)
```

---

## Create Activity

### Activity Structure

```go
create := map[string]any{
    "@context":  context,
    "id":        createID,
    "type":      "Create",
    "actor":     actorURI,
    "published": note.CreatedAt.Format(time.RFC3339),
    "to": []string{
        "https://www.w3.org/ns/activitystreams#Public",
    },
    "cc": ccList,
    "object": map[string]any{
        "id":           noteURI,
        "type":         "Note",
        "attributedTo": actorURI,
        "content":      contentHTML,
        "mediaType":    "text/html",
        "published":    note.CreatedAt.Format(time.RFC3339),
        "url":          noteURL,
        "to":           []string{"https://www.w3.org/ns/activitystreams#Public"},
        "cc":           ccList,
        "tag":          tags,      // Hashtags and mentions
        "inReplyTo":    parentURI, // If reply
    },
}
```

### CC List Construction

```go
ccList := []string{
    fmt.Sprintf("https://%s/users/%s/followers", conf.Conf.SslDomain, localAccount.Username),
}

// Add parent author for replies
if note.InReplyToURI != "" {
    parentAuthorURI := extractAuthorFromURI(note.InReplyToURI, database, conf)
    if parentAuthorURI != "" && parentAuthorURI != actorURI {
        ccList = append(ccList, parentAuthorURI)
    }
}

// Add mentioned actors
for _, mentionActorURI := range mentionedActors {
    ccList = append(ccList, mentionActorURI)
}
```

### Tag Array

```go
tags := make([]map[string]any, 0)

// Hashtags
for _, tag := range hashtags {
    tags = append(tags, map[string]any{
        "type": "Hashtag",
        "href": fmt.Sprintf("https://%s/tags/%s", conf.Conf.SslDomain, tag),
        "name": "#" + tag,
    })
}

// Mentions
for _, mention := range mentions {
    tags = append(tags, map[string]any{
        "type": "Mention",
        "href": actorURI,
        "name": fmt.Sprintf("@%s@%s", mention.Username, mention.Domain),
    })
}
```

### Content Transformation

```go
// Convert Markdown links to HTML
contentHTML := util.MarkdownLinksToHTML(note.Message)

// Convert hashtags to ActivityPub-compliant HTML links
contentHTML = util.HashtagsToActivityPubHTML(contentHTML, baseURL)

// Convert mentions to HTML (after URI resolution)
contentHTML = util.MentionsToActivityPubHTML(contentHTML, mentionURIs)
```

---

## Inbox Collection

### Sources

```go
inboxes := make(map[string]bool)  // Dedupe with map

// 1. Followers
err, followers := database.ReadFollowersByAccountId(localAccount.Id)
for _, follower := range *followers {
    if follower.IsLocal {
        continue  // Skip local followers
    }
    err, remoteActor := database.ReadRemoteAccountById(follower.AccountId)
    inboxes[remoteActor.InboxURI] = true
}

// 2. Parent author (for replies)
if parentAuthorURI != "" && parentAuthorURI != actorURI {
    err, parentAccount := database.ReadRemoteAccountByActorURI(parentAuthorURI)
    if err == nil && parentAccount != nil {
        inboxes[parentAccount.InboxURI] = true
    }
}

// 3. Mentioned actors
for _, mentionActorURI := range mentionedActors {
    err, mentionedAccount := database.ReadRemoteAccountByActorURI(mentionActorURI)
    if err == nil && mentionedAccount != nil {
        inboxes[mentionedAccount.InboxURI] = true
    }
}

// 4. Active relays
err, relays := database.ReadActiveRelays()
for _, relay := range *relays {
    inboxes[relay.InboxURI] = true
}
```

### Queue Delivery

```go
for inboxURI := range inboxes {
    queueItem := &domain.DeliveryQueueItem{
        Id:           uuid.New(),
        InboxURI:     inboxURI,
        ActivityJSON: mustMarshal(create),
        Attempts:     0,
        NextRetryAt:  time.Now(),
        CreatedAt:    time.Now(),
    }
    database.EnqueueDelivery(queueItem)
}
```

---

## Update Activity

### Activity Structure

```go
update := map[string]any{
    "@context": context,
    "id":       updateID,
    "type":     "Update",
    "actor":    actorURI,
    "to":       []string{"https://www.w3.org/ns/activitystreams#Public"},
    "cc":       ccList,
    "object": map[string]any{
        // Same as Create object, plus:
        "updated": updatedTime.Format(time.RFC3339),
    },
}
```

### Timestamp Handling

```go
updatedTime := note.CreatedAt
if note.EditedAt != nil {
    updatedTime = *note.EditedAt
}
```

---

## Delete Activity

### Activity Structure

```go
deleteActivity := map[string]any{
    "@context":  "https://www.w3.org/ns/activitystreams",
    "id":        deleteID,
    "type":      "Delete",
    "actor":     actorURI,
    "published": time.Now().Format(time.RFC3339),
    "to":        []string{"https://www.w3.org/ns/activitystreams#Public"},
    "cc":        []string{followersURI},
    "object":    noteURI,
}
```

---

## Follow Activity

### Activity Structure

```go
follow := map[string]any{
    "@context": "https://www.w3.org/ns/activitystreams",
    "id":       followID,
    "type":     "Follow",
    "actor":    actorURI,
    "object":   remoteActorURI,
}
```

### Pre-Flight Checks

```go
// Check if trying to follow yourself
if remoteActor.Domain == conf.Conf.SslDomain && remoteActor.Username == localAccount.Username {
    return fmt.Errorf("self-follow not allowed")
}

// Check if already following
err, existingFollow := database.ReadFollowByAccountIds(localAccount.Id, remoteActor.Id)
if existingFollow != nil {
    if existingFollow.Accepted {
        return fmt.Errorf("already following %s@%s", remoteActor.Username, remoteActor.Domain)
    } else {
        return fmt.Errorf("follow pending %s@%s", remoteActor.Username, remoteActor.Domain)
    }
}
```

### Store Pending Follow

```go
followRecord := &domain.Follow{
    Id:              uuid.New(),
    AccountId:       localAccount.Id,
    TargetAccountId: remoteActor.Id,
    URI:             followID,
    Accepted:        false,  // Pending until Accept received
    CreatedAt:       time.Now(),
}
database.CreateFollow(followRecord)
```

---

## Undo Activity (Unfollow)

### Activity Structure

```go
undo := map[string]any{
    "@context": "https://www.w3.org/ns/activitystreams",
    "id":       undoID,
    "type":     "Undo",
    "actor":    actorURI,
    "object": map[string]any{
        "id":     follow.URI,
        "type":   "Follow",
        "actor":  actorURI,
        "object": remoteActor.ActorURI,
    },
}
```

---

## Like Activity

### Activity Structure

```go
like := map[string]any{
    "@context": "https://www.w3.org/ns/activitystreams",
    "id":       likeID,
    "type":     "Like",
    "actor":    actorURI,
    "object":   noteURI,
}
```

### Local Note Skip

```go
// Don't send ActivityPub for local likes
if strings.Contains(authorURI, conf.Conf.SslDomain) {
    log.Printf("Skipping Like delivery for local note %s", noteURI)
    return nil
}
```

---

## Accept Activity

### Activity Structure

```go
accept := map[string]any{
    "@context": "https://www.w3.org/ns/activitystreams",
    "id":       acceptID,
    "type":     "Accept",
    "actor":    actorURI,
    "object": map[string]any{
        "id":     followID,
        "type":   "Follow",
        "actor":  remoteActor.ActorURI,
        "object": actorURI,
    },
}
```

---

## Relay Follow

### Activity Structure

```go
follow := map[string]any{
    "@context": "https://www.w3.org/ns/activitystreams",
    "id":       followID,
    "type":     "Follow",
    "actor":    actorURI,
    "object":   "https://www.w3.org/ns/activitystreams#Public",  // Special for relays
}
```

### Store Relay Record

```go
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
```

---

## Mention Resolution

### WebFinger Lookup

```go
func resolveMentionURI(username, domain string) (string, error) {
    webfingerURL := fmt.Sprintf("https://%s/.well-known/webfinger?resource=acct:%s@%s",
        domain, username, domain)

    req, err := http.NewRequest("GET", webfingerURL, nil)
    req.Header.Set("Accept", "application/jrd+json")

    // Parse response
    var result struct {
        Subject string `json:"subject"`
        Links   []struct {
            Rel  string `json:"rel"`
            Type string `json:"type"`
            Href string `json:"href"`
        } `json:"links"`
    }

    // Find ActivityPub actor link
    for _, link := range result.Links {
        if link.Rel == "self" {
            if link.Type == "application/activity+json" ||
               link.Type == "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\"" {
                return link.Href, nil
            }
        }
    }
}
```

---

## Delivery Queue Worker

### Configuration

```go
const (
    pollInterval = 10 * time.Second
    batchSize    = 50
    maxAttempts  = 10
)
```

### Exponential Backoff

```go
backoffMinutes := []int{1, 5, 15, 60, 240, 1440}[min(item.Attempts-1, 5)]
item.NextRetryAt = time.Now().Add(time.Duration(backoffMinutes) * time.Minute)
```

| Attempt | Retry After |
|---------|-------------|
| 1 | 1 minute |
| 2 | 5 minutes |
| 3 | 15 minutes |
| 4 | 1 hour |
| 5 | 4 hours |
| 6+ | 24 hours |

### Worker Loop

```go
func StartDeliveryWorker(conf *util.AppConfig) func() {
    ticker := time.NewTicker(10 * time.Second)
    stop := make(chan struct{})

    go func() {
        for {
            select {
            case <-ticker.C:
                processDeliveryQueue(conf)
            case <-stop:
                ticker.Stop()
                return
            }
        }
    }()

    return func() {
        close(stop)
    }
}
```

### Process Queue

```go
err, items := database.ReadPendingDeliveries(50)

for _, item := range *items {
    if err := deliverActivity(&item, conf); err != nil {
        item.Attempts++
        if item.Attempts >= 10 {
            database.DeleteDelivery(item.Id)  // Give up
        } else {
            database.UpdateDeliveryAttempt(item.Id, item.Attempts, nextRetry)
        }
    } else {
        database.DeleteDelivery(item.Id)  // Success
    }
}
```

---

## Context Building

### With Hashtags

```go
if len(hashtags) > 0 {
    context = []any{
        "https://www.w3.org/ns/activitystreams",
        map[string]any{
            "Hashtag": "as:Hashtag",
        },
    }
} else {
    context = "https://www.w3.org/ns/activitystreams"
}
```

---

## Author Extraction

For replies, extract the parent author:

```go
func extractAuthorFromURI(objectURI string, database Database, conf *util.AppConfig) string {
    // Try stored activity
    err, activity := database.ReadActivityByObjectURI(objectURI)
    if err == nil && activity != nil {
        return activity.ActorURI
    }

    // Try local note
    err, localNote := database.ReadNoteByURI(objectURI)
    if err == nil && localNote != nil {
        return fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localNote.CreatedBy)
    }

    return ""
}
```

---

## Source Files

- `activitypub/outbox.go` - Activity construction and sending
- `activitypub/delivery.go` - Background delivery worker
- `activitypub/httpsig.go` - Request signing
- `activitypub/deps.go` - Database and HTTP client interfaces
