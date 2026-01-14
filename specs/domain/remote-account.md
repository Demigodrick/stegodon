# Remote Account Entity

This document specifies the RemoteAccount entity, which represents cached federated user profiles from other ActivityPub servers.

---

## Overview

A RemoteAccount is a cached copy of a user profile from a remote ActivityPub server. These profiles are fetched when:
- A local user follows a remote user
- A remote user interacts with the server
- Actor information is needed for display or verification

Remote accounts have a 24-hour TTL (Time To Live) and are re-fetched when stale.

---

## Data Structure

```go
type RemoteAccount struct {
    Id            uuid.UUID
    Username      string
    Domain        string
    ActorURI      string
    DisplayName   string
    Summary       string
    InboxURI      string
    OutboxURI     string
    PublicKeyPem  string
    AvatarURL     string
    LastFetchedAt time.Time
}
```

---

## Field Definitions

### Identity Fields

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| `Id` | `uuid.UUID` | Primary Key | Internal identifier |
| `Username` | `string` | Required | Remote username (e.g., `user`) |
| `Domain` | `string` | Required | Remote server domain (e.g., `mastodon.social`) |
| `ActorURI` | `string` | Unique, Required | Full ActivityPub actor URL |

### Unique Constraint

Remote accounts are unique by `(username, domain)` pair:

```sql
UNIQUE(username, domain)
```

### Profile Fields

| Field | Type | Description |
|-------|------|-------------|
| `DisplayName` | `string` | Human-readable name |
| `Summary` | `string` | Bio/description (may contain HTML) |
| `AvatarURL` | `string` | Profile image URL |

### ActivityPub Endpoints

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `InboxURI` | `string` | Yes | Where to send activities |
| `OutboxURI` | `string` | No | Activity feed endpoint |

### Cryptography

| Field | Type | Description |
|-------|------|-------------|
| `PublicKeyPem` | `string` | RSA public key for signature verification |

### Cache Management

| Field | Type | Description |
|-------|------|-------------|
| `LastFetchedAt` | `time.Time` | When profile was last fetched |

---

## Database Schema

```sql
CREATE TABLE IF NOT EXISTS remote_accounts (
    id TEXT NOT NULL PRIMARY KEY,
    username TEXT NOT NULL,
    domain TEXT NOT NULL,
    actor_uri TEXT UNIQUE NOT NULL,
    display_name TEXT,
    summary TEXT,
    inbox_uri TEXT NOT NULL,
    outbox_uri TEXT,
    public_key_pem TEXT NOT NULL,
    avatar_url TEXT,
    last_fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(username, domain)
)
```

### Indexes

```sql
CREATE INDEX IF NOT EXISTS idx_remote_accounts_actor_uri ON remote_accounts(actor_uri);
CREATE INDEX IF NOT EXISTS idx_remote_accounts_domain ON remote_accounts(domain);
CREATE INDEX IF NOT EXISTS idx_remote_accounts_username_domain ON remote_accounts(username, domain);
```

---

## Cache Lifecycle

### 1. Initial Fetch

Triggered by WebFinger resolution or incoming activity:

```
User enters: @user@mastodon.social
      │
      ▼
WebFinger Lookup
      │
      ├── GET https://mastodon.social/.well-known/webfinger?resource=acct:user@mastodon.social
      └── Extract actor URI from links
            │
            ▼
      Actor Fetch
            │
            ├── GET https://mastodon.social/users/user
            ├── Parse JSON-LD
            └── Create RemoteAccount
```

### 2. Cache Check

Before fetching, check if cached profile is fresh:

```go
const CacheTTL = 24 * time.Hour

if time.Since(account.LastFetchedAt) > CacheTTL {
    // Re-fetch from remote server
}
```

### 3. Cache Refresh

Update existing record when re-fetching:

```go
// Update all fields except Id
UPDATE remote_accounts SET
    display_name = ?,
    summary = ?,
    inbox_uri = ?,
    outbox_uri = ?,
    public_key_pem = ?,
    avatar_url = ?,
    last_fetched_at = ?
WHERE actor_uri = ?
```

---

## WebFinger Resolution

WebFinger is used to discover the actor URI from a handle:

### Request

```http
GET /.well-known/webfinger?resource=acct:user@domain HTTP/1.1
Host: domain
Accept: application/jrd+json
```

### Response

```json
{
    "subject": "acct:user@domain",
    "links": [
        {
            "rel": "self",
            "type": "application/activity+json",
            "href": "https://domain/users/user"
        }
    ]
}
```

### Flow

```go
func ResolveWebFinger(handle string) (string, error) {
    // Parse handle: user@domain
    // Fetch WebFinger document
    // Extract self link with type application/activity+json
    // Return actor URI
}
```

---

## Actor Fetching

### Request

```http
GET /users/user HTTP/1.1
Host: domain
Accept: application/activity+json
```

### Response Parsing

```json
{
    "@context": "https://www.w3.org/ns/activitystreams",
    "type": "Person",
    "id": "https://domain/users/user",
    "preferredUsername": "user",
    "name": "Display Name",
    "summary": "<p>Bio here</p>",
    "inbox": "https://domain/users/user/inbox",
    "outbox": "https://domain/users/user/outbox",
    "icon": {
        "type": "Image",
        "url": "https://domain/avatars/user.png"
    },
    "publicKey": {
        "id": "https://domain/users/user#main-key",
        "owner": "https://domain/users/user",
        "publicKeyPem": "-----BEGIN PUBLIC KEY-----\n..."
    }
}
```

### Field Mapping

| JSON Field | RemoteAccount Field |
|------------|---------------------|
| `id` | `ActorURI` |
| `preferredUsername` | `Username` |
| (extracted from id) | `Domain` |
| `name` | `DisplayName` |
| `summary` | `Summary` |
| `inbox` | `InboxURI` |
| `outbox` | `OutboxURI` |
| `icon.url` | `AvatarURL` |
| `publicKey.publicKeyPem` | `PublicKeyPem` |

---

## Public Key Handling

### Storage Format

Public keys are stored in PEM format:

```
-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...
-----END PUBLIC KEY-----
```

### Supported Formats

The system supports both:
- PKIX format (preferred, standard)
- PKCS#1 format (legacy, some servers)

### Usage

Public keys are used to verify HTTP signatures on incoming activities:

```go
func VerifySignature(req *http.Request, publicKeyPem string) bool {
    // Parse PEM to public key
    // Verify signature header against key
}
```

---

## Database Operations

### Read Operations

| Function | Description |
|----------|-------------|
| `GetRemoteAccount(actorURI)` | Lookup by actor URI |
| `GetRemoteAccountByHandle(user, domain)` | Lookup by handle |
| `GetRemoteAccountById(id)` | Lookup by internal UUID |

### Write Operations

| Function | Description |
|----------|-------------|
| `SaveRemoteAccount(account)` | Create or update cached profile |
| `UpdateRemoteAccountFetchTime(uri)` | Touch last_fetched_at |

---

## Handle Formats

### Display Format

Remote users are displayed as `@username@domain`:

```go
func (ra *RemoteAccount) Handle() string {
    return fmt.Sprintf("@%s@%s", ra.Username, ra.Domain)
}
```

### Input Parsing

Users can enter handles in various formats:

| Input | Parsed Username | Parsed Domain |
|-------|-----------------|---------------|
| `@user@domain.com` | `user` | `domain.com` |
| `user@domain.com` | `user` | `domain.com` |
| `https://domain.com/users/user` | (fetched) | (fetched) |

---

## Error Handling

### Fetch Failures

| Error | Behavior |
|-------|----------|
| Network timeout | Retry with backoff |
| 404 Not Found | User doesn't exist |
| 410 Gone | Account deleted |
| Invalid JSON | Log and skip |
| Missing required fields | Log and skip |

### Stale Cache

If remote server is unreachable, stale cached data is used with warning.

---

## Relationships

| Related Entity | Relationship | Description |
|----------------|--------------|-------------|
| Follow | One-to-Many | Remote account is followed/follows |
| Activity | One-to-Many | Remote account creates activities |
| Like | One-to-Many | Remote account likes notes |
| Boost | One-to-Many | Remote account boosts notes |

---

## Security Considerations

### Signature Verification

All incoming activities are verified using the remote account's public key.

### Actor URI Validation

Actor URIs are validated to prevent impersonation:
- Must be HTTPS
- Domain must match claimed domain
- URI must resolve to valid actor

### Key Rotation

When a remote account's public key changes:
1. Old signatures become invalid
2. Cache is refreshed on next interaction
3. New key is stored

---

## Source Files

- `domain/activitypub.go` - RemoteAccount struct
- `activitypub/actors.go` - Actor fetching and WebFinger
- `db/db.go` - Database operations
- `activitypub/inbox.go` - Signature verification
