# Account Entity

This document specifies the Account entity, which represents local user accounts in stegodon.

---

## Overview

An Account represents a registered user who connects via SSH. Each account is uniquely identified by their SSH public key and has associated profile information and cryptographic keys for ActivityPub federation.

---

## Data Structure

```go
type Account struct {
    Id             uuid.UUID
    Username       string
    Publickey      string
    CreatedAt      time.Time
    FirstTimeLogin dbBool
    WebPublicKey   string
    WebPrivateKey  string
    DisplayName    string
    Summary        string
    AvatarURL      string
    IsAdmin        bool
    Muted          bool
}
```

---

## Field Definitions

### Identity Fields

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| `Id` | `uuid.UUID` | Primary Key | Unique account identifier |
| `Username` | `string` | Unique, max 100 chars | Public username (WebFinger handle) |
| `Publickey` | `string` | Unique, max 1000 chars | SHA256 hash of SSH public key |
| `CreatedAt` | `time.Time` | Default: now | Account creation timestamp |

### Login State

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `FirstTimeLogin` | `dbBool` | `TRUE` (1) | Whether user needs to set username |

The `dbBool` type maps to SQLite integers:
- `FALSE` = 0
- `TRUE` = 1

### ActivityPub Keys

| Field | Type | Description |
|-------|------|-------------|
| `WebPublicKey` | `string` | RSA public key (PKCS#8 PEM format) |
| `WebPrivateKey` | `string` | RSA private key (PKCS#8 PEM format) |

These keys are used for:
- HTTP signature signing (outgoing requests)
- HTTP signature verification (incoming requests)
- Actor public key endpoint

### Profile Fields

| Field | Type | Max Length | Description |
|-------|------|------------|-------------|
| `DisplayName` | `string` | 50 chars | Human-readable name |
| `Summary` | `string` | 200 chars | Bio/description (supports mentions) |
| `AvatarURL` | `string` | - | Profile image URL |

### Administration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `IsAdmin` | `bool` | `false` | Has admin privileges |
| `Muted` | `bool` | `false` | Blocked from logging in |

---

## Database Schema

```sql
CREATE TABLE IF NOT EXISTS accounts(
    id uuid NOT NULL PRIMARY KEY,
    username varchar(100) UNIQUE NOT NULL,
    publickey varchar(1000) UNIQUE,
    created_at timestamp default current_timestamp,
    first_time_login int default 1,
    web_public_key text,
    web_private_key text,
    display_name text,
    summary text,
    avatar_url text,
    is_admin int default 0,
    muted int default 0
)
```

---

## Lifecycle

### 1. Account Creation

Triggered when a new SSH public key connects:

```
SSH Connection
      │
      ▼
AuthMiddleware
      │
      ├── Check registration mode (open/closed/single)
      ├── Generate UUID
      ├── Generate RSA keypair (PKCS#8)
      ├── Hash SSH public key (SHA256)
      └── Insert account (first_time_login = 1)
```

### 2. First Login

User must choose a username on first connection:

```
TUI Shows CreateUser View
      │
      ├── User enters username
      ├── Validate WebFinger format
      ├── Check uniqueness
      └── Update: first_time_login = 0
```

### 3. Profile Updates

Users can update via TUI:
- Display name (optional)
- Summary/bio (optional)
- Avatar URL (not implemented in TUI)

### 4. Account Muting

Admins can mute accounts:

```go
// Muted users are blocked at SSH connection
if acc != nil && acc.Muted {
    s.Write([]byte("Your account has been muted...\n"))
    s.Close()
    return
}
```

---

## SSH Public Key Handling

### Hashing

Public keys are hashed before storage for privacy:

```go
// Hash the SSH public key
hash := sha256.Sum256(pubKey.Marshal())
hashStr := base64.StdEncoding.EncodeToString(hash[:])
```

### Authentication

Account lookup by hashed public key:

```sql
SELECT ... FROM accounts WHERE publickey = ?
```

---

## RSA Keypair

### Generation

2048-bit RSA keys generated on account creation:

```go
privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
```

### Format

Keys stored in PKCS#8 PEM format:

```
-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASC...
-----END PRIVATE KEY-----

-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A...
-----END PUBLIC KEY-----
```

### Migration

Legacy PKCS#1 keys are migrated to PKCS#8:

```go
database.MigrateKeysToPKCS8()
```

---

## Username Validation

Usernames must be valid WebFinger handles:

| Rule | Constraint |
|------|------------|
| Length | 1-15 characters |
| Characters | Alphanumeric and underscores only |
| Format | No @ symbol (local users) |
| Uniqueness | Case-sensitive unique |

```go
valid, errMsg := util.IsValidWebFingerUsername(username)
```

---

## ActivityPub Representation

Account maps to ActivityPub Actor:

```json
{
    "@context": "https://www.w3.org/ns/activitystreams",
    "type": "Person",
    "id": "https://domain/users/username",
    "preferredUsername": "username",
    "name": "Display Name",
    "summary": "Bio text",
    "inbox": "https://domain/users/username/inbox",
    "outbox": "https://domain/users/username/outbox",
    "publicKey": {
        "id": "https://domain/users/username#main-key",
        "owner": "https://domain/users/username",
        "publicKeyPem": "-----BEGIN PUBLIC KEY-----\n..."
    }
}
```

---

## Database Operations

### Read Operations

| Function | Description |
|----------|-------------|
| `ReadAccBySession(s)` | Lookup by SSH session public key |
| `ReadAccById(id)` | Lookup by UUID |
| `ReadAccByUsername(name)` | Lookup by username |
| `GetAllAccounts()` | List all accounts (admin) |
| `CountAccounts()` | Count total accounts |

### Write Operations

| Function | Description |
|----------|-------------|
| `CreateAccount(s, tempName)` | Create new account from SSH session |
| `UpdateLoginUser(...)` | Set username after first login |
| `SetMuted(id, muted)` | Toggle mute status |
| `DeleteAccount(id)` | Remove account and related data |

---

## Relationships

| Related Entity | Relationship | Description |
|----------------|--------------|-------------|
| Note | One-to-Many | Account creates notes |
| Follow | Many-to-Many | Account follows/followed by others |
| Like | One-to-Many | Account likes notes |
| Boost | One-to-Many | Account boosts notes |
| Notification | One-to-Many | Account receives notifications |

---

## Source Files

- `domain/accounts.go` - Account struct definition
- `db/db.go` - Database operations (CRUD)
- `middleware/auth.go` - Account creation on SSH connect
- `ui/createuser/` - Username selection view
- `util/crypto.go` - RSA key generation
