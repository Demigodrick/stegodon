# Monitoring

This document specifies pprof profiling and NodeInfo statistics monitoring.

---

## Overview

Stegodon provides monitoring through:
- **pprof profiling** - Runtime performance analysis
- **NodeInfo statistics** - Server metadata and usage statistics

---

## pprof Profiling

### Configuration

```yaml
withPprof: true
```

Or via environment variable:

```bash
STEGODON_WITH_PPROF=true
```

### Server Setup

```go
import (
    "net/http"
    _ "net/http/pprof"
)

if conf.Conf.WithPprof {
    go func() {
        log.Println("pprof server listening on localhost:6060")
        log.Println("Access profiling at http://localhost:6060/debug/pprof/")
        if err := http.ListenAndServe("localhost:6060", nil); err != nil {
            log.Printf("pprof server error: %v", err)
        }
    }()
}
```

### Properties

| Property | Value |
|----------|-------|
| Address | `localhost:6060` |
| Base URL | `http://localhost:6060/debug/pprof/` |
| Enabled by | `STEGODON_WITH_PPROF=true` |
| Default | Disabled |

### Security

- Binds to `localhost` only
- Not exposed externally
- No authentication required (local only)

---

## pprof Endpoints

### Available Profiles

| Endpoint | Description |
|----------|-------------|
| `/debug/pprof/` | Index page listing all profiles |
| `/debug/pprof/heap` | Heap memory profile |
| `/debug/pprof/goroutine` | Stack traces of all goroutines |
| `/debug/pprof/allocs` | Memory allocation sampling |
| `/debug/pprof/block` | Stack traces of blocked goroutines |
| `/debug/pprof/mutex` | Stack traces of mutex contention |
| `/debug/pprof/profile` | CPU profile (30-second sample by default) |
| `/debug/pprof/trace` | Execution trace |
| `/debug/pprof/threadcreate` | Stack traces of thread creation |

### Usage Examples

```bash
# Access web UI
open http://localhost:6060/debug/pprof/

# CPU profile (30 seconds)
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Heap profile
go tool pprof http://localhost:6060/debug/pprof/heap

# Goroutine count
curl http://localhost:6060/debug/pprof/goroutine?debug=1 | grep "goroutine profile"

# Allocation profile
go tool pprof http://localhost:6060/debug/pprof/allocs

# Block profile
go tool pprof http://localhost:6060/debug/pprof/block

# Mutex profile
go tool pprof http://localhost:6060/debug/pprof/mutex
```

### Interactive Analysis

```bash
# Start interactive pprof session
go tool pprof http://localhost:6060/debug/pprof/heap

# Common commands in pprof:
# top       - Show top functions by memory/CPU
# list foo  - Show source code for function foo
# web       - Generate SVG graph (requires graphviz)
# pdf       - Generate PDF report
```

---

## NodeInfo Statistics

### Overview

NodeInfo 2.0/2.1 provides standardized server metadata for federation discovery.

### Endpoints

| Endpoint | Purpose |
|----------|---------|
| `/.well-known/nodeinfo` | Discovery document (links to 2.0 and 2.1) |
| `/nodeinfo/2.0` | NodeInfo 2.0 schema |
| `/nodeinfo/2.1` | NodeInfo 2.1 schema (adds repository, homepage) |

### Statistics Collection

```go
func GetNodeInfo20(conf *util.AppConfig) string {
    database := db.GetDB()

    // User statistics
    totalUsers, _ := database.CountAccounts()
    activeMonth, _ := database.CountActiveUsersMonth()
    activeHalfyear, _ := database.CountActiveUsersHalfYear()

    // Post statistics
    localPosts, _ := database.CountLocalPosts()

    // Registration status
    openRegistrations := !conf.Conf.Closed
    if conf.Conf.Single && totalUsers >= 1 {
        openRegistrations = false
    }
}
```

### Statistics Queries

| Statistic | Database Query |
|-----------|----------------|
| Total Users | `COUNT(*) FROM accounts` |
| Active (Month) | `COUNT(*) WHERE lastLogin > NOW() - 30 days` |
| Active (Half Year) | `COUNT(*) WHERE lastLogin > NOW() - 180 days` |
| Local Posts | `COUNT(*) FROM notes WHERE isLocal = true` |

### NodeInfo Response Structure

```json
{
  "version": "2.0",
  "software": {
    "name": "stegodon",
    "version": "1.4.3"
  },
  "protocols": ["activitypub"],
  "services": {
    "outbound": [],
    "inbound": []
  },
  "usage": {
    "users": {
      "total": 10,
      "activeMonth": 5,
      "activeHalfyear": 8
    },
    "localPosts": 150
  },
  "openRegistrations": true,
  "metadata": {
    "nodeName": "Stegodon",
    "nodeDescription": "A SSH-first federated microblog"
  }
}
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `version` | string | Always "2.0" |
| `software.name` | string | Always "stegodon" |
| `software.version` | string | From `util/version.txt` |
| `protocols` | array | Always `["activitypub"]` |
| `usage.users.total` | int | Total registered accounts |
| `usage.users.activeMonth` | int | Active in last 30 days |
| `usage.users.activeHalfyear` | int | Active in last 180 days |
| `usage.localPosts` | int | Total local posts |
| `openRegistrations` | bool | Whether new users can register |
| `metadata.nodeName` | string | Always "Stegodon" |
| `metadata.nodeDescription` | string | Custom or default description |

### Registration Status Logic

```go
openRegistrations := !conf.Conf.Closed
if conf.Conf.Single && totalUsers >= 1 {
    openRegistrations = false
}
```

| Mode | `STEGODON_CLOSED` | `STEGODON_SINGLE` | Users | Result |
|------|-------------------|-------------------|-------|--------|
| Open | false | false | any | `true` |
| Closed | true | false | any | `false` |
| Single (empty) | false | true | 0 | `true` |
| Single (occupied) | false | true | ≥1 | `false` |

---

## Metrics Summary

### Runtime Metrics (pprof)

| Category | Metrics |
|----------|---------|
| Memory | Heap size, allocations, GC stats |
| CPU | CPU time per function |
| Goroutines | Count, stack traces |
| Blocking | Mutex contention, channel waits |

### Application Metrics (NodeInfo)

| Category | Metrics |
|----------|---------|
| Users | Total, active monthly, active half-yearly |
| Content | Local post count |
| Status | Registration open/closed |

---

## Monitoring Commands

### Health Check

```bash
# Quick HTTP check
curl -s http://localhost:9999/feed > /dev/null && echo "OK"

# NodeInfo statistics
curl -s http://localhost:9999/nodeinfo/2.0 | jq .usage
```

### Performance Check

```bash
# Goroutine count (watch for leaks)
curl -s http://localhost:6060/debug/pprof/goroutine?debug=1 | head -1

# Memory usage
curl -s http://localhost:6060/debug/pprof/heap?debug=1 | head -20
```

---

## Best Practices

### When to Enable pprof

| Scenario | Enable |
|----------|--------|
| Development | Yes |
| Debugging performance | Yes |
| Production (normal) | No |
| Production (investigating issue) | Temporarily |

### Monitoring Checklist

1. **Goroutine count** - Watch for steady growth (leaks)
2. **Heap size** - Monitor for excessive growth
3. **Active users** - Track engagement via NodeInfo
4. **Local posts** - Monitor content growth

---

## Source Files

- `main.go` - pprof server setup
- `web/nodeinfo.go` - NodeInfo implementation
- `db/database.go` - Statistics queries
- `util/config.go` - `WithPprof` configuration
