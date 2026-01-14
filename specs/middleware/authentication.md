# SSH Authentication

This document specifies the SSH public key authentication and account lookup system.

---

## Overview

Stegodon uses SSH public key authentication via the [Wish](https://github.com/charmbracelet/wish) framework. Users authenticate with their SSH keys, and accounts are automatically created on first connection (when registration is open).

---

## SSH Server Setup

### Server Configuration

```go
sshServer, err := wish.NewServer(
    wish.WithAddress(fmt.Sprintf("%s:%d", conf.Host, conf.SshPort)),
    wish.WithHostKeyPath(sshKeyPath),
    wish.WithPublicKeyAuth(func(ssh.Context, ssh.PublicKey) bool { return true }),
    wish.WithMiddleware(
        middleware.MainTui(),
        middleware.AuthMiddleware(config),
        logging.MiddlewareWithLogger(log.Default()),
    ),
)
```

### Configuration Options

| Option | Description |
|--------|-------------|
| `WithAddress` | Bind address (host:port) |
| `WithHostKeyPath` | Path to server's SSH host key |
| `WithPublicKeyAuth` | Accept all public keys (account lookup done in middleware) |
| `WithMiddleware` | Stack of middleware handlers |

---

## Middleware Stack

Middleware executes in reverse order (last registered runs first):

1. **Logging Middleware** - Logs connection events
2. **Auth Middleware** - Account lookup and creation
3. **MainTui Middleware** - BubbleTea program initialization

---

## Auth Middleware

### Entry Point

```go
func AuthMiddleware(conf *util.AppConfig) wish.Middleware {
    return func(h ssh.Handler) ssh.Handler {
        return func(s ssh.Session) {
            database := db.GetDB()
            found, acc := database.ReadAccBySession(s)

            switch {
            case found == nil:
                // User exists - check if muted
            default:
                // User not found - check registration rules
            }
            h(s)  // Continue to next middleware
        }
    }
}
```

---

## Account Lookup

### ReadAccBySession

Looks up account by SSH public key hash:

```go
func (db *DB) ReadAccBySession(s ssh.Session) (error, *domain.Account) {
    // Convert public key to string
    publicKeyToString := util.PublicKeyToString(s.PublicKey())

    // Query by hashed public key
    row := db.db.QueryRow(sqlSelectUserByPublicKey, util.PkToHash(publicKeyToString))

    // Scan result into Account struct
    err := row.Scan(&tempAcc.Id, &tempAcc.Username, ...)
    if err == sql.ErrNoRows {
        return err, nil  // User not found
    }
    return nil, &tempAcc  // User found
}
```

### SQL Query

```sql
SELECT id, username, publickey, created_at, first_time_login,
       web_public_key, web_private_key, display_name, summary,
       avatar_url, is_admin, muted
FROM accounts
WHERE publickey = ?
```

---

## Public Key Handling

### Key to String Conversion

```go
func PublicKeyToString(s ssh.PublicKey) string {
    return strings.TrimSpace(string(gossh.MarshalAuthorizedKey(s)))
}
```

Converts SSH public key to OpenSSH authorized_keys format.

### Key Hashing

```go
func PkToHash(pk string) string {
    h := sha256.New()
    h.Write([]byte(pk))
    return hex.EncodeToString(h.Sum(nil))
}
```

**Security**: Public keys are SHA256-hashed before database storage. This provides:
- Privacy (original key not stored)
- Fixed-length storage (64 hex characters)
- Consistent lookup performance

---

## Authentication Flow

### Existing User

```
1. SSH connection established
2. AuthMiddleware receives session
3. ReadAccBySession looks up account by key hash
4. If found and not muted → proceed to TUI
5. If found and muted → display message, close connection
```

### New User (Registration Open)

```
1. SSH connection established
2. AuthMiddleware receives session
3. ReadAccBySession returns sql.ErrNoRows
4. Check registration rules (closed, single-user)
5. If allowed → CreateAccount with random username
6. Proceed to TUI (createuser view)
```

### Blocked Registration

```
1. SSH connection established
2. ReadAccBySession returns sql.ErrNoRows
3. Registration check fails (closed or single-user limit)
4. Display rejection message
5. Close connection
```

---

## Account Creation

### CreateAccount Function

```go
func (db *DB) CreateAccount(s ssh.Session, username string) (error, bool) {
    // Check if account already exists
    err, found := db.ReadAccBySession(s)
    if found != nil {
        return nil, false  // Already exists
    }

    // Generate RSA keypair for ActivityPub
    webKeyPair := util.GenerateRsaKeyPair()

    // Get public key from session
    publicKey := util.PublicKeyToString(s.PublicKey())

    // Insert account with hashed public key
    return db.wrapTransaction(func(tx *sql.Tx) error {
        return db.insertUser(tx, username, publicKey, webKeyPair)
    }), true
}
```

### Initial Username

New accounts get a random 10-character username:

```go
database.CreateAccount(s, util.RandomString(10))
```

Users can change this in the createuser view.

---

## Muted User Handling

### Check and Block

```go
if acc != nil && acc.Muted {
    log.Printf("Blocked login attempt from muted user: %s", acc.Username)
    s.Write([]byte("Your account has been muted by an administrator.\n"))
    s.Close()
    return
}
```

Muted users see a message and are disconnected immediately.

---

## Session Logging

### LogPublicKey

```go
func LogPublicKey(s ssh.Session) {
    log.Printf("%s@%s opened a new ssh-session..", s.User(), s.LocalAddr())
}
```

Logs successful authentication events with:
- SSH username
- Client IP address

---

## Host Key Management

### Auto-Generation

SSH host key is auto-generated on first run:

```
~/.config/stegodon/.ssh/stegodonhostkey
```

or in current directory:

```
.ssh/stegodonhostkey
```

### Key Persistence

The host key persists across restarts, ensuring:
- Consistent server fingerprint
- No "host key changed" warnings for clients

---

## Error Handling

| Error | Action |
|-------|--------|
| Account not found | Proceed to registration check |
| Account muted | Display message, close session |
| Registration closed | Display message, close session |
| Single-user limit reached | Display message, close session |
| Database error | Log error, close session |

---

## Security Considerations

### Public Key Only

Password authentication is disabled. Only SSH public key authentication is supported, providing:
- Strong cryptographic authentication
- No password storage or transmission
- Key-based identity management

### All Keys Accepted

```go
wish.WithPublicKeyAuth(func(ssh.Context, ssh.PublicKey) bool { return true })
```

All public keys are accepted at the SSH layer. Account validation happens in the AuthMiddleware, allowing:
- New user registration flow
- Proper error messages for blocked users
- Graceful handling of registration modes

---

## Source Files

- `middleware/auth.go` - Authentication middleware
- `app/app.go` - SSH server setup
- `db/db.go` - ReadAccBySession, CreateAccount
- `util/util.go` - PkToHash, PublicKeyToString, LogPublicKey
