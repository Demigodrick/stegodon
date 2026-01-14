# Application Lifecycle

This document specifies the stegodon application lifecycle, including initialization, startup, and graceful shutdown.

---

## Overview

The application follows a structured lifecycle pattern managed by the `App` struct in `app/app.go`. The lifecycle consists of three phases:

1. **Creation** - Instantiate the App with configuration
2. **Initialization** - Run migrations, setup servers
3. **Start/Shutdown** - Run servers, handle signals, graceful stop

---

## App Struct

```go
type App struct {
    config              *util.AppConfig
    sshServer           *ssh.Server
    httpServer          *http.Server
    done                chan os.Signal
    stopDeliveryWorker  func()
}
```

| Field | Type | Description |
|-------|------|-------------|
| `config` | `*util.AppConfig` | Application configuration |
| `sshServer` | `*ssh.Server` | Wish SSH server instance |
| `httpServer` | `*http.Server` | Standard HTTP server |
| `done` | `chan os.Signal` | Signal channel for shutdown |
| `stopDeliveryWorker` | `func()` | ActivityPub delivery worker stop function |

---

## Lifecycle Phases

### Phase 1: Creation

**Function**: `app.New(conf *util.AppConfig) (*App, error)`

Creates a new App instance with the provided configuration. Initializes the signal channel for shutdown handling.

**Entry Point**: `main.go`

```go
application, err := app.New(conf)
```

---

### Phase 2: Initialization

**Function**: `app.Initialize() error`

Performs all setup tasks before starting servers.

#### 2.1 Database Migrations

| Migration | Description | Error Handling |
|-----------|-------------|----------------|
| `RunActivityPubMigrations()` | Creates ActivityPub-related tables | Warning logged, continues |
| `MigrateKeysToPKCS8()` | Converts RSA keys from PKCS#1 to PKCS#8 | Warning logged, continues |
| `MigrateDuplicateFollows()` | Removes duplicate follow relationships | Warning logged, continues |
| `MigrateLocalReplyCounts()` | Recalculates reply counts for local posts | Warning logged, continues |
| `MigratePerformanceIndexes()` | Creates database indexes | Warning logged, continues |

All migrations are idempotent and safe to run multiple times.

#### 2.2 SSH Server Setup

```go
sshServer, err := wish.NewServer(
    wish.WithAddress(fmt.Sprintf("%s:%d", conf.Conf.Host, conf.Conf.SshPort)),
    wish.WithHostKeyPath(sshKeyPath),
    wish.WithPublicKeyAuth(func(ssh.Context, ssh.PublicKey) bool { return true }),
    wish.WithMiddleware(
        middleware.MainTui(),
        middleware.AuthMiddleware(conf),
        logging.MiddlewareWithLogger(log.Default()),
    ),
)
```

**SSH Host Key**: Auto-generated at `~/.config/stegodon/.ssh/stegodonhostkey` or `./.ssh/stegodonhostkey`

#### 2.3 HTTP Server Setup

```go
router, err := web.Router(conf)
httpServer = &http.Server{
    Addr:    fmt.Sprintf(":%d", conf.Conf.HttpPort),
    Handler: router,
}
```

---

### Phase 3: Start

**Function**: `app.Start() error`

Starts all servers and blocks until shutdown signal.

#### 3.1 ActivityPub Delivery Worker

Started only if `WithAp` is enabled:

```go
if conf.Conf.WithAp {
    stopDeliveryWorker = activitypub.StartDeliveryWorker(conf)
}
```

#### 3.2 Signal Handling

Listens for shutdown signals:

```go
signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
```

| Signal | Trigger |
|--------|---------|
| `SIGINT` | Ctrl+C |
| `SIGTERM` | `kill` command, Docker stop |
| `os.Interrupt` | Platform interrupt |

#### 3.3 Server Goroutines

Both servers run in separate goroutines:

```go
go func() {
    sshServer.ListenAndServe()
}()

go func() {
    httpServer.ListenAndServe()
}()
```

#### 3.4 Blocking

Main goroutine blocks on signal channel:

```go
<-done
```

---

### Phase 4: Shutdown

**Function**: `app.Shutdown() error`

Gracefully stops all components with a 30-second timeout.

#### Shutdown Order

1. **ActivityPub Delivery Worker** - Stop processing queue
2. **HTTP Server** - Stop accepting new requests, finish existing
3. **SSH Server** - Disconnect clients gracefully

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// 1. Stop delivery worker
if stopDeliveryWorker != nil {
    stopDeliveryWorker()
}

// 2. Stop HTTP server
httpServer.Shutdown(ctx)

// 3. Stop SSH server
sshServer.Shutdown(ctx)
```

#### Timeout Behavior

- **30-second grace period** for in-flight requests
- After timeout, forceful termination
- Errors logged but don't prevent shutdown

---

## Entry Point Flow

```
main.go
    │
    ├── Parse flags (-v for version)
    ├── Load configuration (util.ReadConf)
    ├── Setup logging
    ├── Start pprof server (if enabled)
    │
    ├── app.New(conf)
    ├── app.Initialize()
    │       ├── Run migrations
    │       ├── Create SSH server
    │       └── Create HTTP server
    │
    └── app.Start()
            ├── Start delivery worker (if AP enabled)
            ├── Start SSH server goroutine
            ├── Start HTTP server goroutine
            ├── Wait for signal
            └── Shutdown()
```

---

## Error Handling

| Phase | Error Behavior |
|-------|----------------|
| Configuration | `log.Fatalln` - exit immediately |
| App Creation | `log.Fatalf` - exit with message |
| Initialization | `log.Fatalf` - exit with message |
| Migrations | Warning logged, continues |
| Server Start | `log.Fatalf` in goroutine |
| Shutdown | Errors logged, returns first error |

---

## Logging

All lifecycle events are logged:

```
stegodon v1.4.3
Configuration: {...}
Running database migrations...
Database migrations complete
Checking for key format migration...
Key format migration complete
Using SSH host key at: ~/.config/stegodon/.ssh/stegodonhostkey
Starting SSH server on 127.0.0.1:23232
Starting HTTP server on 127.0.0.1:9999
```

On shutdown:

```
Shutdown signal received
Initiating graceful shutdown...
Stopping ActivityPub delivery worker...
Stopping HTTP server...
HTTP server stopped gracefully
Stopping SSH server...
SSH server stopped gracefully
All servers stopped
```

---

## Source Files

- `app/app.go` - App struct and lifecycle methods
- `main.go` - Entry point and flag handling
