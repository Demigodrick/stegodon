# NodeInfo

This document specifies the NodeInfo implementation for server metadata and statistics.

---

## Overview

NodeInfo is a standardized way of exposing server metadata. Federation-aware tools and services use NodeInfo to:
- Identify the server software and version
- Get usage statistics (users, posts)
- Check registration status
- Display instance information

---

## Endpoints

### Well-Known NodeInfo

**Route:** `GET /.well-known/nodeinfo`

Returns discovery document pointing to NodeInfo 2.0 and 2.1 endpoints.

```json
{
    "links": [
        {
            "rel": "http://nodeinfo.diaspora.software/ns/schema/2.0",
            "href": "https://example.com/nodeinfo/2.0"
        },
        {
            "rel": "http://nodeinfo.diaspora.software/ns/schema/2.1",
            "href": "https://example.com/nodeinfo/2.1"
        }
    ]
}
```

### NodeInfo 2.0

**Route:** `GET /nodeinfo/2.0`

Returns full server statistics and metadata.

### NodeInfo 2.1

**Route:** `GET /nodeinfo/2.1`

Returns server statistics with additional software metadata (repository, homepage).

---

## NodeInfo 2.0 Response

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

---

## NodeInfo 2.1 Response

```json
{
    "version": "2.1",
    "software": {
        "name": "stegodon",
        "version": "1.6.0",
        "repository": "https://github.com/deemkeen/stegodon",
        "homepage": "https://stegodon.social"
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

### NodeInfo 2.1 Additional Fields

| Field | Type | Description |
|-------|------|-------------|
| `software.repository` | string | URL to source code repository |
| `software.homepage` | string | URL to software homepage |

---

## Data Structures

### NodeInfo20

```go
type NodeInfo20 struct {
    Version           string           `json:"version"`
    Software          NodeInfoSoftware `json:"software"`
    Protocols         []string         `json:"protocols"`
    Services          NodeInfoServices `json:"services"`
    OpenRegistrations bool             `json:"openRegistrations"`
    Usage             NodeInfoUsage    `json:"usage"`
    Metadata          NodeInfoMetadata `json:"metadata"`
}
```

### NodeInfoSoftware

```go
type NodeInfoSoftware struct {
    Name    string `json:"name"`
    Version string `json:"version"`
}
```

### NodeInfoServices

```go
type NodeInfoServices struct {
    Inbound  []string `json:"inbound"`
    Outbound []string `json:"outbound"`
}
```

### NodeInfoUsage

```go
type NodeInfoUsage struct {
    Users      NodeInfoUsers `json:"users"`
    LocalPosts int           `json:"localPosts"`
}

type NodeInfoUsers struct {
    Total          int `json:"total"`
    ActiveMonth    int `json:"activeMonth"`
    ActiveHalfyear int `json:"activeHalfyear"`
}
```

### NodeInfoMetadata

```go
type NodeInfoMetadata struct {
    NodeName        string `json:"nodeName"`
    NodeDescription string `json:"nodeDescription"`
}
```

---

## Statistics Queries

### Database Queries

```go
// Total registered users
totalUsers, err := database.CountAccounts()

// Total local posts
localPosts, err := database.CountLocalPosts()

// Users who posted in last 30 days
activeMonth, err := database.CountActiveUsersMonth()

// Users who posted in last 180 days
activeHalfyear, err := database.CountActiveUsersHalfYear()
```

### SQL Queries

```sql
-- Total users
SELECT COUNT(*) FROM accounts

-- Total posts
SELECT COUNT(*) FROM notes

-- Active users (month)
SELECT COUNT(DISTINCT user_id) FROM notes
WHERE created_at >= datetime('now', '-30 days')

-- Active users (half year)
SELECT COUNT(DISTINCT user_id) FROM notes
WHERE created_at >= datetime('now', '-180 days')
```

---

## Registration Status

### Open Registration Logic

```go
openRegistrations := !conf.Conf.Closed
if conf.Conf.Single && totalUsers >= 1 {
    openRegistrations = false
}
```

| Mode | CLOSED | SINGLE | Users | openRegistrations |
|------|--------|--------|-------|-------------------|
| Normal | false | false | any | true |
| Closed | true | any | any | false |
| Single (empty) | false | true | 0 | true |
| Single (full) | false | true | 1+ | false |

---

## Node Description

### Custom Description

Set via environment variable:
```bash
STEGODON_NODE_DESCRIPTION="My personal blog"
```

### Default Description

If not set:
```
A SSH-first federated microblog
```

---

## Implementation

### GetWellKnownNodeInfo

```go
func GetWellKnownNodeInfo(conf *util.AppConfig) string {
    wellKnown := WellKnownNodeInfo{
        Links: []NodeInfoLink{
            {
                Rel:  "http://nodeinfo.diaspora.software/ns/schema/2.0",
                Href: "https://" + conf.Conf.SslDomain + "/nodeinfo/2.0",
            },
            {
                Rel:  "http://nodeinfo.diaspora.software/ns/schema/2.1",
                Href: "https://" + conf.Conf.SslDomain + "/nodeinfo/2.1",
            },
        },
    }
    jsonBytes, _ := json.Marshal(wellKnown)
    return string(jsonBytes)
}
```

### GetNodeInfo20

```go
func GetNodeInfo20(conf *util.AppConfig) string {
    database := db.GetDB()

    // Gather statistics
    totalUsers, _ := database.CountAccounts()
    localPosts, _ := database.CountLocalPosts()
    activeMonth, _ := database.CountActiveUsersMonth()
    activeHalfyear, _ := database.CountActiveUsersHalfYear()

    // Determine registration status
    openRegistrations := !conf.Conf.Closed
    if conf.Conf.Single && totalUsers >= 1 {
        openRegistrations = false
    }

    // Get description
    nodeDescription := conf.Conf.NodeDescription
    if nodeDescription == "" {
        nodeDescription = "A SSH-first federated microblog"
    }

    // Build JSON response with proper field order
    return fmt.Sprintf(`{
        "version": "2.0",
        "software": {"name": "stegodon", "version": "%s"},
        ...
    }`, util.GetVersion(), ...)
}
```

---

## Content-Type

| Endpoint | Content-Type |
|----------|--------------|
| `/.well-known/nodeinfo` | `application/json; charset=utf-8` |
| `/nodeinfo/2.0` | `application/json; profile="http://nodeinfo.diaspora.software/ns/schema/2.0#"; charset=utf-8` |
| `/nodeinfo/2.1` | `application/json; profile="http://nodeinfo.diaspora.software/ns/schema/2.1#"; charset=utf-8` |

---

## Error Handling

Statistics queries fail gracefully:

```go
totalUsers, err := database.CountAccounts()
if err != nil {
    log.Printf("Failed to count accounts: %v", err)
    totalUsers = 0  // Default to 0 on error
}
```

---

## Caching

NodeInfo responses are generated fresh on each request. No caching is implemented as statistics change frequently.

---

## Standards Compliance

- Implements NodeInfo 2.0 and 2.1 schemas
- Schema URLs:
  - `http://nodeinfo.diaspora.software/ns/schema/2.0`
  - `http://nodeinfo.diaspora.software/ns/schema/2.1`
- Documentation: https://nodeinfo.diaspora.software/
- NodeInfo 2.1 adds `repository` and `homepage` fields to `software` object

---

## Source Files

- `web/nodeinfo.go` - NodeInfo handlers and data structures
- `web/router.go` - Route registration
- `db/db.go` - Statistics queries
