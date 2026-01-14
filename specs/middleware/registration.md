# Registration Modes

This document specifies the open, closed, and single-user registration modes.

---

## Overview

Stegodon supports three registration modes that control who can create new accounts:

| Mode | Description |
|------|-------------|
| **Open** | Anyone can register (default) |
| **Closed** | No new registrations allowed |
| **Single-User** | Only one user allowed |

---

## Configuration

### Environment Variables

```bash
# Closed registration - no new users
STEGODON_CLOSED=true

# Single-user mode - only one user allowed
STEGODON_SINGLE=true
```

### Config File

```yaml
closed: true   # Close registration
single: true   # Single-user mode
```

### Defaults

| Setting | Default |
|---------|---------|
| `closed` | `false` |
| `single` | `false` |

---

## Mode Behaviors

### Open Registration (Default)

- Anyone with an SSH key can create an account
- New users get a random 10-character username
- Users complete setup in createuser view
- First user becomes admin

```go
// No restrictions - allow registration
database.CreateAccount(s, util.RandomString(10))
```

### Closed Registration

- Existing users can log in normally
- New SSH keys are rejected with a message
- Useful for private instances

```go
if conf.Conf.Closed {
    log.Printf("Rejected new user registration - registration is closed")
    s.Write([]byte("Registration is closed, but you can host your own stegodon!\n"))
    s.Write([]byte("More on: https://github.com/deemkeen/stegodon\n"))
    s.Close()
    return
}
```

### Single-User Mode

- Only one user account can exist
- First connection creates the owner account
- Subsequent new users are rejected
- Ideal for personal blogs

```go
if conf.Conf.Single {
    count, err := database.CountAccounts()
    if err != nil {
        log.Printf("Error counting accounts: %v", err)
        s.Write([]byte("An error occurred. Please try again later.\n"))
        s.Close()
        return
    }
    if count >= 1 {
        log.Printf("Rejected new user registration in single-user mode")
        s.Write([]byte("This blog is in single-user mode, but you can host your own stegodon!\n"))
        s.Write([]byte("More on: https://github.com/deemkeen/stegodon\n"))
        s.Close()
        return
    }
}
```

---

## Registration Check Flow

```
New SSH Connection
        │
        ▼
┌───────────────────┐
│ Account exists?   │──── Yes ──► Allow login
└───────────────────┘              (check muted)
        │
        No
        ▼
┌───────────────────┐
│ CLOSED mode?      │──── Yes ──► Reject with message
└───────────────────┘
        │
        No
        ▼
┌───────────────────┐
│ SINGLE mode?      │──── Yes ─┐
└───────────────────┘          │
        │                      ▼
        No            ┌────────────────┐
        │             │ User count > 0?│─ Yes ─► Reject
        │             └────────────────┘
        │                      │
        │                      No
        ▼                      ▼
┌───────────────────────────────┐
│     Create new account        │
└───────────────────────────────┘
```

---

## Implementation

### AuthMiddleware Registration Check

```go
func AuthMiddleware(conf *util.AppConfig) wish.Middleware {
    return func(h ssh.Handler) ssh.Handler {
        return func(s ssh.Session) {
            database := db.GetDB()
            found, acc := database.ReadAccBySession(s)

            switch {
            case found == nil:
                // User exists - check if muted
                if acc != nil && acc.Muted {
                    // Block muted user
                    return
                }
                // Allow login
            default:
                // User not found - check registration rules
                if conf.Conf.Closed {
                    // Closed mode - reject
                    return
                }
                if conf.Conf.Single {
                    count, _ := database.CountAccounts()
                    if count >= 1 {
                        // Single-user limit reached - reject
                        return
                    }
                }
                // Create new account
                database.CreateAccount(s, util.RandomString(10))
            }
            h(s)
        }
    }
}
```

---

## Rejection Messages

### Closed Registration

```
Registration is closed, but you can host your own stegodon!
More on: https://github.com/deemkeen/stegodon
```

### Single-User Mode

```
This blog is in single-user mode, but you can host your own stegodon!
More on: https://github.com/deemkeen/stegodon
```

### Error During Check

```
An error occurred. Please try again later.
```

---

## NodeInfo Integration

Registration status is exposed via NodeInfo:

```go
// Determine if registrations are open
openRegistrations := !conf.Conf.Closed
if conf.Conf.Single && totalUsers >= 1 {
    openRegistrations = false
}
```

### NodeInfo Response

```json
{
    "openRegistrations": true
}
```

| Mode | Users | openRegistrations |
|------|-------|-------------------|
| Open | any | `true` |
| Closed | any | `false` |
| Single (empty) | 0 | `true` |
| Single (full) | 1+ | `false` |

---

## Mode Combinations

### Valid Combinations

| CLOSED | SINGLE | Behavior |
|--------|--------|----------|
| false | false | Open registration (default) |
| true | false | Closed - no new users |
| false | true | Single-user mode |
| true | true | Closed (CLOSED takes precedence) |

### Precedence

CLOSED mode is checked first:

```go
if conf.Conf.Closed {
    // Reject - closed takes precedence
    return
}
if conf.Conf.Single {
    // Check single-user limit
}
```

---

## First User Admin

The first user to register automatically becomes admin:

```go
func (db *DB) insertUser(tx *sql.Tx, username string, publicKey string, webKeyPair *util.RsaKeyPair) error {
    var count int
    tx.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&count)

    isAdmin := 0
    if count == 0 {
        isAdmin = 1
        log.Println("Creating first user as admin:", username)
    }
    // Insert user and set is_admin
}
```

This works in all modes:
- **Open**: First user of many becomes admin
- **Single**: Only user becomes admin

---

## Use Cases

### Personal Blog (Single-User)

```bash
STEGODON_SINGLE=true ./stegodon
```

- One owner account
- Full admin capabilities
- Others can view via web/RSS
- Others can follow via ActivityPub

### Private Instance (Closed)

```bash
STEGODON_CLOSED=true ./stegodon
```

- Pre-created accounts only
- Admin can create accounts via SSH key import
- Useful for invite-only communities

### Public Instance (Open)

```bash
./stegodon
```

- Anyone can register
- First user is admin
- Admin can mute/remove users

---

## Logging

Registration decisions are logged:

```
Rejected new user registration - registration is closed
Rejected new user registration in single-user mode
Error counting accounts: <error>
```

---

## Source Files

- `middleware/auth.go` - Registration mode checks
- `util/config.go` - Configuration loading
- `web/nodeinfo.go` - NodeInfo openRegistrations
- `db/db.go` - CountAccounts, CreateAccount
