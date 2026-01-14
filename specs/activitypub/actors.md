# Actor Fetching

This document specifies the actor fetching and caching system for remote ActivityPub users.

---

## Overview

The actor module handles fetching and caching remote ActivityPub actors. It provides:
- Remote actor profile fetching
- 24-hour cache with automatic refresh
- Profile data extraction (username, display name, avatar, public key)
- Storage in `remote_accounts` table

---

## Actor Response Structure

```go
type ActorResponse struct {
    Context           any    `json:"@context"`
    ID                string `json:"id"`
    Type              string `json:"type"`
    PreferredUsername string `json:"preferredUsername"`
    Name              string `json:"name"`
    Summary           string `json:"summary"`
    Inbox             string `json:"inbox"`
    Outbox            string `json:"outbox"`
    Icon              struct {
        Type      string `json:"type"`
        MediaType string `json:"mediaType"`
        URL       string `json:"url"`
    } `json:"icon"`
    PublicKey struct {
        ID           string `json:"id"`
        Owner        string `json:"owner"`
        PublicKeyPem string `json:"publicKeyPem"`
    } `json:"publicKey"`
}
```

---

## Required Fields

Actors must have these fields to be valid:

| Field | Description |
|-------|-------------|
| `id` | Actor's unique URI |
| `inbox` | Inbox endpoint for receiving activities |
| `publicKey.publicKeyPem` | RSA public key for signature verification |

```go
if actor.ID == "" || actor.Inbox == "" || actor.PublicKey.PublicKeyPem == "" {
    return nil, fmt.Errorf("actor missing required fields")
}
```

---

## Cache TTL

**Cache Duration:** 24 hours

```go
const cacheTTL = 24 * time.Hour

if time.Since(cached.LastFetchedAt) < cacheTTL {
    return cached, nil  // Use cached data
}

// Cache stale, fetch fresh data
return FetchRemoteActor(actorURI)
```

---

## GetOrFetchActor

Primary function for retrieving actors with cache support.

### Flow

```
GetOrFetchActor(actorURI)
      │
      ▼
Check Cache (remote_accounts table)
      │
      ├── Found + Fresh (< 24h) → Return cached
      └── Not Found or Stale → Fetch fresh
            │
            ▼
FetchRemoteActor(actorURI)
      │
      ▼
Return actor
```

### Implementation

```go
func GetOrFetchActorWithDeps(actorURI string, client HTTPClient, database Database) (*domain.RemoteAccount, error) {
    // Check cache first
    err, cached := database.ReadRemoteAccountByURI(actorURI)
    if err == nil && cached != nil {
        // Check if cache is fresh (< 24 hours)
        if time.Since(cached.LastFetchedAt) < 24*time.Hour {
            return cached, nil
        }
    }

    // Fetch fresh data
    return FetchRemoteActorWithDeps(actorURI, client, database)
}
```

---

## FetchRemoteActor

Fetches actor from remote server and stores/updates in cache.

### Request

```go
req, err := http.NewRequest("GET", actorURI, nil)
req.Header.Set("Accept", "application/activity+json")
req.Header.Set("User-Agent", "stegodon/1.0 ActivityPub")

resp, err := client.Do(req)
if resp.StatusCode != http.StatusOK {
    return nil, fmt.Errorf("actor fetch failed with status: %d", resp.StatusCode)
}
```

### Domain Extraction

```go
func extractDomain(actorURI string) (string, error) {
    parsed, err := url.Parse(actorURI)
    if err != nil {
        return "", fmt.Errorf("invalid actor URI: %w", err)
    }
    return parsed.Host, nil
}

// Example: "https://mastodon.social/users/alice" -> "mastodon.social"
```

### Storage

```go
// Check if actor already exists
err, existingAcc := database.ReadRemoteAccountByURI(actor.ID)

if err == nil && existingAcc != nil {
    // Update existing record (reuse ID)
    remoteAcc = &domain.RemoteAccount{
        Id:            existingAcc.Id,  // Keep same ID
        Username:      actor.PreferredUsername,
        Domain:        domainName,
        ActorURI:      actor.ID,
        DisplayName:   actor.Name,
        Summary:       actor.Summary,
        InboxURI:      actor.Inbox,
        OutboxURI:     actor.Outbox,
        PublicKeyPem:  actor.PublicKey.PublicKeyPem,
        AvatarURL:     actor.Icon.URL,
        LastFetchedAt: time.Now(),
    }
    database.UpdateRemoteAccount(remoteAcc)
} else {
    // Create new record
    remoteAcc = &domain.RemoteAccount{
        Id:            uuid.New(),
        // ... same fields
    }
    database.CreateRemoteAccount(remoteAcc)
}
```

---

## RemoteAccount Entity

```go
type RemoteAccount struct {
    Id            uuid.UUID
    Username      string      // preferredUsername from actor
    Domain        string      // Extracted from actor URI
    ActorURI      string      // Full actor URI (id)
    DisplayName   string      // name from actor
    Summary       string      // Bio/description
    InboxURI      string      // inbox endpoint
    OutboxURI     string      // outbox endpoint
    PublicKeyPem  string      // RSA public key for verification
    AvatarURL     string      // icon.url
    LastFetchedAt time.Time   // Cache timestamp
}
```

---

## Username Extraction

Helper for extracting username from URI:

```go
func extractUsername(uri string) string {
    parts := strings.Split(uri, "/")
    if len(parts) > 0 {
        username := parts[len(parts)-1]
        return strings.TrimPrefix(username, "@")
    }
    return ""
}

// Examples:
// "https://example.com/users/alice" -> "alice"
// "https://example.com/@alice" -> "alice"
```

---

## HTTP Client

### Default Configuration

```go
var defaultHTTPClient HTTPClient = NewDefaultHTTPClient(10 * time.Second)

type DefaultHTTPClient struct {
    client *http.Client
}

func NewDefaultHTTPClient(timeout time.Duration) *DefaultHTTPClient {
    return &DefaultHTTPClient{
        client: &http.Client{Timeout: timeout},
    }
}
```

### Timeout

**Request Timeout:** 10 seconds

---

## Dependency Injection

For testing, dependencies can be injected:

```go
// Production
func FetchRemoteActor(actorURI string) (*domain.RemoteAccount, error) {
    return FetchRemoteActorWithDeps(actorURI, defaultHTTPClient, NewDBWrapper())
}

// Testing
func FetchRemoteActorWithDeps(actorURI string, client HTTPClient, database Database) (*domain.RemoteAccount, error) {
    // ...
}
```

---

## Database Interface

Required operations for actor management:

```go
type Database interface {
    // Read by various identifiers
    ReadRemoteAccountByURI(uri string) (error, *domain.RemoteAccount)
    ReadRemoteAccountById(id uuid.UUID) (error, *domain.RemoteAccount)
    ReadRemoteAccountByActorURI(actorURI string) (error, *domain.RemoteAccount)

    // Create and update
    CreateRemoteAccount(acc *domain.RemoteAccount) error
    UpdateRemoteAccount(acc *domain.RemoteAccount) error

    // Delete (used when actor deletes account)
    DeleteRemoteAccount(id uuid.UUID) error
}
```

---

## Error Handling

| Error | Cause |
|-------|-------|
| Request creation failed | Invalid URI |
| Request failed | Network error, timeout |
| Non-200 status | Remote server error |
| JSON parse failed | Invalid response format |
| Missing required fields | Actor missing id, inbox, or publicKey |
| Invalid actor URI | Cannot extract domain |

---

## Usage Examples

### Signature Verification

```go
// In inbox handler
signerActor, err := GetOrFetchActor(signerActorURI)
if err != nil {
    http.Error(w, "Failed to verify signer", http.StatusBadRequest)
    return
}

// Verify signature with signer's public key
_, err = VerifyRequest(r, signerActor.PublicKeyPem)
```

### Follow Request

```go
// Fetch target actor before following
remoteActor, err := GetOrFetchActor(remoteActorURI)
if err != nil {
    return fmt.Errorf("failed to fetch remote actor: %w", err)
}

// Send follow to their inbox
SendActivity(follow, remoteActor.InboxURI, localAccount, conf)
```

### Create Activity Delivery

```go
// Get inbox for mentioned user
mentionedAccount, err := GetOrFetchActor(mentionActorURI)
if err == nil {
    inboxes[mentionedAccount.InboxURI] = true
}
```

---

## Cache Behavior

### Fresh Cache Hit

```
Request for actor@mastodon.social
      │
      ▼
Found in remote_accounts table
      │
      ▼
LastFetchedAt: 5 hours ago (< 24h)
      │
      ▼
Return cached data (no HTTP request)
```

### Stale Cache / Cache Miss

```
Request for actor@mastodon.social
      │
      ▼
Found in remote_accounts table
      │
      ▼
LastFetchedAt: 36 hours ago (> 24h)
      │
      ▼
Fetch from https://mastodon.social/users/actor
      │
      ▼
Update remote_accounts record
      │
      ▼
Return fresh data
```

---

## Actor URI Patterns

Common formats encountered:

| Platform | Pattern |
|----------|---------|
| Mastodon | `https://domain/users/username` |
| Pleroma | `https://domain/users/username` |
| Misskey | `https://domain/users/id` |
| Pixelfed | `https://domain/users/username` |
| Relay | `https://relay.domain/actor` |

---

## Source Files

- `activitypub/actors.go` - Actor fetching and caching
- `activitypub/deps.go` - Database and HTTP client interfaces
- `activitypub/inbox.go` - Uses actors for signature verification
- `activitypub/outbox.go` - Uses actors for follow/like delivery
- `domain/remote_account.go` - RemoteAccount entity definition
