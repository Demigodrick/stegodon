# Logging

This document specifies standard logging and journald integration.

---

## Overview

Stegodon supports two logging modes:
- **Standard logging** - stdout/stderr with timestamps (default)
- **Journald logging** - Linux systemd journal integration

---

## Configuration

### Environment Variable

```bash
STEGODON_WITH_JOURNALD=true
```

### Config File

```yaml
withJournald: true
```

### Default

Standard logging (stdout/stderr) with timestamps.

---

## Standard Logging

### Output

- All logs go to stderr by default
- Timestamps included in log format
- Compatible with Docker, terminal, file redirection

### Example Output

```
2024/01/15 10:30:00 stegodon v1.4.3
2024/01/15 10:30:00 Config file not found, using embedded defaults
2024/01/15 10:30:00 Initializing HTTP router on port 9999
2024/01/15 10:30:00 Starting SSH server on 127.0.0.1:23232
```

### Log Format

Go standard library format:
```
YYYY/MM/DD HH:MM:SS message
```

---

## Journald Logging

### Linux Only

Journald integration only works on Linux systems with systemd.

### Build Tags

```go
//go:build linux
// +build linux

package util

import "github.com/coreos/go-systemd/v22/journal"
```

Non-Linux builds use a stub implementation.

### journaldWriter

```go
type journaldWriter struct{}

func (w *journaldWriter) Write(p []byte) (n int, err error) {
    msg := string(p)
    // Remove trailing newline (journald adds its own)
    if len(msg) > 0 && msg[len(msg)-1] == '\n' {
        msg = msg[:len(msg)-1]
    }

    err = journal.Send(msg, journal.PriInfo, map[string]string{
        "SYSLOG_IDENTIFIER": "stegodon",
    })
    if err != nil {
        // Fallback to stderr
        return fmt.Fprintf(os.Stderr, "%s", p)
    }
    return len(p), nil
}
```

### Properties

| Property | Value |
|----------|-------|
| Priority | INFO |
| Syslog Identifier | `stegodon` |
| Fallback | stderr on failure |

---

## Setup Function

### SetupLogging (Linux)

```go
func SetupLogging(withJournald bool) {
    if withJournald {
        if !journal.Enabled() {
            log.Println("Warning: Journald not available; using standard logging")
            return
        }

        writer := &journaldWriter{}
        logWriter = writer
        log.SetOutput(writer)
        log.SetFlags(0)  // journald adds timestamps
        log.Println("Logging initialized with journald support")
    }
}
```

### SetupLogging (Non-Linux)

```go
func SetupLogging(withJournald bool) {
    if withJournald {
        log.Println("Warning: Journald logging is not supported on this operating system")
        log.Println("Falling back to standard logging (stdout/stderr)")
    }
}
```

---

## Initialization

### In main.go

```go
func main() {
    conf, _ := util.ReadConf()

    // Setup logging (journald if enabled, otherwise standard)
    util.SetupLogging(conf.Conf.WithJournald)

    log.Printf("stegodon v%s", util.GetVersion())
    // ...
}
```

Called early in startup, before other logging.

---

## Log Writer Access

### GetLogWriter Function

```go
var logWriter io.Writer = os.Stderr

func GetLogWriter() io.Writer {
    return logWriter
}
```

Allows other packages to access the configured log writer.

---

## Journald Commands

### View Logs

```bash
journalctl -u stegodon -f
```

### Filter by Time

```bash
journalctl -u stegodon --since "1 hour ago"
```

### Filter by Priority

```bash
journalctl -u stegodon -p info
```

### Show Last N Lines

```bash
journalctl -u stegodon -n 100
```

---

## Log Categories

### Application Events

| Event | Example |
|-------|---------|
| Startup | `stegodon v1.4.3` |
| Config | `Config file not found, using embedded defaults` |
| Server | `Starting SSH server on 127.0.0.1:23232` |
| Migration | `Running ActivityPub migrations...` |

### User Events

| Event | Example |
|-------|---------|
| Login | `alice@192.168.1.1 opened a new ssh-session..` |
| Registration | `Creating first user as admin: alice` |
| Blocked | `Blocked login attempt from muted user: bob` |

### ActivityPub Events

| Event | Example |
|-------|---------|
| Activity | `Received Follow from https://...` |
| Delivery | `Delivered Accept to https://...` |
| Error | `Failed to deliver to inbox: timeout` |

### HTTP Events

Gin framework logs:

```
[GIN] 2024/01/15 - 10:30:00 | 200 | 12.5ms | 192.168.1.1 | GET "/u/alice"
```

---

## Fallback Behavior

### Journald Unavailable

If journald is requested but unavailable:

```
Warning: Journald not available on this system; using standard logging
```

Falls back to stderr with timestamps.

### Non-Linux Systems

```
Warning: Journald logging is not supported on this operating system
Falling back to standard logging (stdout/stderr)
```

---

## Docker Logging

### Standard Docker

Logs to container stdout, captured by Docker:

```bash
docker logs stegodon
docker-compose logs -f stegodon
```

### Log Drivers

Docker supports various log drivers:
- `json-file` (default)
- `syslog`
- `journald`

```yaml
services:
  stegodon:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

---

## Systemd Service

### Example Unit File

```ini
[Unit]
Description=Stegodon SSH-first fediverse blog
After=network.target

[Service]
Type=simple
User=stegodon
Group=stegodon
ExecStart=/usr/local/bin/stegodon
Restart=on-failure
Environment=STEGODON_WITH_JOURNALD=true

[Install]
WantedBy=multi-user.target
```

### Log Access

With journald enabled:

```bash
journalctl -u stegodon -f
```

---

## Log Rotation

### Standard Logging

Redirect to file with logrotate:

```
/var/log/stegodon.log {
    daily
    rotate 7
    compress
    missingok
    notifempty
}
```

### Journald

Journald handles rotation automatically via:
- `/etc/systemd/journald.conf`
- `SystemMaxUse`, `SystemMaxFileSize`

---

## Source Files

- `util/logger_linux.go` - Linux journald implementation
- `util/logger_other.go` - Non-Linux stub
- `main.go` - SetupLogging call
- `util/config.go` - WithJournald configuration
