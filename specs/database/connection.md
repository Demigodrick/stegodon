# Connection Management

This document specifies the database connection setup, WAL mode configuration, and connection pooling.

---

## Overview

Stegodon uses SQLite with:
- **modernc.org/sqlite**: Pure Go SQLite implementation (no CGO)
- **WAL/WAL2 mode**: Concurrent read/write access
- **Connection pooling**: Up to 25 concurrent connections
- **Singleton pattern**: Single shared database instance

---

## Database Driver

```go
import (
    "database/sql"
    _ "modernc.org/sqlite"
)
```

Uses `modernc.org/sqlite` for pure Go implementation:
- No CGO dependency
- Cross-platform compatibility
- Embedded in single binary

---

## Singleton Pattern

Database instance is created once and shared:

```go
var (
    dbInstance *DB
    dbOnce     sync.Once
)

type DB struct {
    db *sql.DB
}

func GetDB() *DB {
    dbOnce.Do(func() {
        // Initialize database once
        dbInstance = initializeDB()
    })
    return dbInstance
}
```

**Benefits:**
- Thread-safe initialization
- Single connection pool
- Consistent configuration

---

## Database Path Resolution

```go
dbPath := util.ResolveFilePath("database.db")
```

Path resolution order:
1. `./database.db` (current directory)
2. `~/.config/stegodon/database.db` (user config directory)

---

## Connection Pool Configuration

```go
db.SetMaxOpenConns(25)      // Maximum open connections
db.SetMaxIdleConns(5)       // Maximum idle connections
db.SetConnMaxLifetime(time.Hour)  // Connection lifetime
```

| Setting | Value | Purpose |
|---------|-------|---------|
| `MaxOpenConns` | 25 | Limits concurrent database access |
| `MaxIdleConns` | 5 | Keeps warm connections ready |
| `ConnMaxLifetime` | 1 hour | Prevents stale connections |

---

## Journal Mode (WAL)

### WAL2 Mode (Preferred)

```go
var journalMode string
err = db.QueryRow("PRAGMA journal_mode=WAL2").Scan(&journalMode)
```

WAL2 benefits:
- Better concurrent read/write performance
- Reduced checkpoint overhead
- Improved multi-connection support

### Fallback to WAL

```go
if err != nil || journalMode == "delete" {
    // WAL2 not supported, try regular WAL
    err = db.QueryRow("PRAGMA journal_mode=WAL").Scan(&journalMode)
}
```

If WAL2 fails, falls back to standard WAL mode.

---

## Optimization PRAGMAs

```go
db.Exec("PRAGMA synchronous = NORMAL")      // Reduces fsync calls
db.Exec("PRAGMA cache_size = -64000")       // 64MB cache per connection
db.Exec("PRAGMA temp_store = MEMORY")       // Store temp tables in RAM
db.Exec("PRAGMA busy_timeout = 5000")       // Wait up to 5s for locks
db.Exec("PRAGMA foreign_keys = ON")         // Enable FK constraints
db.Exec("PRAGMA auto_vacuum = INCREMENTAL") // Better performance than FULL
```

| PRAGMA | Value | Description |
|--------|-------|-------------|
| `synchronous` | NORMAL | Balances durability and performance |
| `cache_size` | -64000 | 64MB page cache (negative = KB) |
| `temp_store` | MEMORY | Temp tables in RAM |
| `busy_timeout` | 5000 | 5 second lock wait |
| `foreign_keys` | ON | Enforce foreign key constraints |
| `auto_vacuum` | INCREMENTAL | Gradual space reclamation |

---

## Transaction Wrapper

All write operations use a transaction wrapper with automatic retry:

```go
func (db *DB) wrapTransaction(f func(tx *sql.Tx) error) error {
    ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
    defer cancel()

    tx, err := db.db.BeginTx(ctx, nil)
    if err != nil {
        log.Printf("error starting transaction: %s", err)
        return err
    }

    for {
        err = f(tx)
        if err != nil {
            serr, ok := err.(*sqlite.Error)
            if ok && serr.Code() == sqlitelib.SQLITE_BUSY {
                continue  // Retry on busy
            }
            log.Printf("error in transaction: %s", err)
            return err
        }

        err = tx.Commit()
        if err != nil {
            log.Printf("error committing transaction: %s", err)
            return err
        }
        break
    }
    return nil
}
```

### Features

| Feature | Implementation |
|---------|----------------|
| **Timeout** | 5-second context timeout |
| **Busy Retry** | Automatic retry on `SQLITE_BUSY` |
| **Error Logging** | Logs all transaction errors |
| **Automatic Commit** | Commits on success |

---

## Initialization Sequence

```go
func GetDB() *DB {
    dbOnce.Do(func() {
        // 1. Resolve database path
        dbPath := util.ResolveFilePath("database.db")
        log.Printf("Using database at: %s", dbPath)

        // 2. Open connection
        db, err := sql.Open("sqlite", dbPath)
        if err != nil {
            panic(err)
        }

        // 3. Configure connection pool
        db.SetMaxOpenConns(25)
        db.SetMaxIdleConns(5)
        db.SetConnMaxLifetime(time.Hour)

        // 4. Enable WAL mode
        // Try WAL2, fall back to WAL

        // 5. Set optimization PRAGMAs

        // 6. Create DB wrapper
        dbInstance = &DB{db: db}

        // 7. Run initial schema setup
        err2 := dbInstance.CreateDB()
        if err2 != nil {
            panic(err2)
        }
    })

    return dbInstance
}
```

---

## Schema Initialization

### Core Tables (CreateDB)

```go
func (db *DB) CreateDB() error {
    return db.wrapTransaction(func(tx *sql.Tx) error {
        // Create accounts table
        err := db.createUserTable(tx)
        if err != nil {
            return err
        }

        // Create notes table
        err2 := db.createNotesTable(tx)
        if err2 != nil {
            return err2
        }

        return nil
    })
}
```

### ActivityPub Tables (RunActivityPubMigrations)

```go
func (db *DB) RunActivityPubMigrations() error {
    log.Println("Running ActivityPub migrations...")
    return db.RunMigrations()
}
```

Called from app initialization when ActivityPub is enabled.

---

## Error Handling

### SQLITE_BUSY

SQLite returns `SQLITE_BUSY` when the database is locked:

```go
if serr.Code() == sqlitelib.SQLITE_BUSY {
    continue  // Retry the operation
}
```

The transaction wrapper automatically retries on busy.

### Connection Errors

```go
if err != nil {
    panic(err)  // Fatal on connection failure
}
```

Database connection failures are fatal during startup.

---

## Concurrent Access

### Read Operations

Direct queries without transactions:

```go
row := db.db.QueryRow(sqlSelectUserById, id)
```

Multiple concurrent reads are supported with WAL mode.

### Write Operations

All writes use transactions:

```go
return db.wrapTransaction(func(tx *sql.Tx) error {
    _, err := tx.Exec(sqlInsertNote, ...)
    return err
})
```

Ensures atomicity and handles busy retries.

---

## Performance Characteristics

| Operation | Behavior |
|-----------|----------|
| **Concurrent Reads** | Unlimited with WAL |
| **Concurrent Writes** | Serialized (one at a time) |
| **Lock Wait** | Up to 5 seconds (`busy_timeout`) |
| **Page Cache** | 64MB per connection |
| **Checkpoint** | Automatic (WAL mode) |

---

## Logging

Database operations log to standard output:

```
Using database at: /Users/user/.config/stegodon/database.db
Database journal mode: wal2
Database initialized with connection pooling (max 25 connections)
Table follows created or already exists
Extended existing tables with new columns
```

---

## Source Files

- `db/db.go` - GetDB(), wrapTransaction(), CreateDB()
- `db/migrations.go` - RunMigrations()
- `util/config.go` - ResolveFilePath()
