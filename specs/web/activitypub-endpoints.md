# ActivityPub Endpoints

This document specifies the ActivityPub endpoints for actor profiles, inbox, outbox, and collections.

---

## Overview

ActivityPub endpoints enable federation with the fediverse. All endpoints:
- Require `WithAp=true` configuration
- Return `application/activity+json` content type
- Use HTTPS with valid SSL certificates

---

## Actor Endpoint

**Route:** `GET /users/:actor`

Returns the ActivityPub Actor object for a user.

### Response Format

```json
{
    "@context": [
        "https://www.w3.org/ns/activitystreams",
        "https://w3id.org/security/v1"
    ],
    "id": "https://example.com/users/alice",
    "type": "Person",
    "preferredUsername": "alice",
    "name": "Alice",
    "summary": "Bio text here",
    "inbox": "https://example.com/users/alice/inbox",
    "outbox": "https://example.com/users/alice/outbox",
    "followers": "https://example.com/users/alice/followers",
    "following": "https://example.com/users/alice/following",
    "url": "https://example.com/u/alice",
    "manuallyApprovesFollowers": false,
    "discoverable": true,
    "icon": {
        "type": "Image",
        "mediaType": "image/png",
        "url": "https://example.com/static/stegologo.png"
    },
    "endpoints": {
        "sharedInbox": "https://example.com/inbox"
    },
    "publicKey": {
        "id": "https://example.com/users/alice#main-key",
        "owner": "https://example.com/users/alice",
        "publicKeyPem": "-----BEGIN PUBLIC KEY-----..."
    }
}
```

### URI Patterns

| Field | Pattern |
|-------|---------|
| Actor ID | `https://{domain}/users/{username}` |
| Inbox | `https://{domain}/users/{username}/inbox` |
| Outbox | `https://{domain}/users/{username}/outbox` |
| Followers | `https://{domain}/users/{username}/followers` |
| Following | `https://{domain}/users/{username}/following` |
| Shared Inbox | `https://{domain}/inbox` |
| Public Key ID | `https://{domain}/users/{username}#main-key` |
| Profile URL | `https://{domain}/u/{username}` |

---

## Inbox Endpoints

### Shared Inbox

**Route:** `POST /inbox`

Receives activities addressed to any local user. Used by relays and for efficiency.

```go
g.POST("/inbox", RateLimitMiddleware(apLimiter), maxBodySize, func(c *gin.Context) {
    // 1. Read request body
    // 2. Parse activity JSON
    // 3. Determine target username from:
    //    - "to" field
    //    - "cc" field (followers collections)
    //    - "object" field (for Follow activities)
    //    - Actor's followers (for Create/Update from followed users)
    //    - Any local user (for relay content)
    // 4. Route to HandleInbox with target username
})
```

### User Inbox

**Route:** `POST /users/:actor/inbox`

Receives activities addressed to a specific user.

```go
g.POST("/users/:actor/inbox", RateLimitMiddleware(apLimiter), maxBodySize, func(c *gin.Context) {
    actor := c.Param("actor")
    activitypub.HandleInbox(c.Writer, c.Request, actor, conf)
})
```

### Inbox Rate Limiting

| Setting | Value |
|---------|-------|
| Rate | 5 requests/second |
| Burst | 10 requests |
| Max Body | 1MB |

---

## Outbox Endpoint

**Route:** `GET /users/:actor/outbox`

Returns a paginated collection of a user's public posts.

### Collection Response (no page parameter)

```json
{
    "@context": "https://www.w3.org/ns/activitystreams",
    "id": "https://example.com/users/alice/outbox",
    "type": "OrderedCollection",
    "totalItems": 42,
    "first": "https://example.com/users/alice/outbox?page=1"
}
```

### Collection Page Response (?page=1)

```json
{
    "@context": [
        "https://www.w3.org/ns/activitystreams",
        {"Hashtag": "as:Hashtag"}
    ],
    "id": "https://example.com/users/alice/outbox?page=1",
    "type": "OrderedCollectionPage",
    "partOf": "https://example.com/users/alice/outbox",
    "orderedItems": [
        {
            "id": "https://example.com/notes/{uuid}#activity",
            "type": "Create",
            "actor": "https://example.com/users/alice",
            "published": "2024-01-15T10:30:00Z",
            "to": ["https://www.w3.org/ns/activitystreams#Public"],
            "cc": ["https://example.com/users/alice/followers"],
            "object": {
                "id": "https://example.com/notes/{uuid}",
                "type": "Note",
                "attributedTo": "https://example.com/users/alice",
                "content": "<p>Hello world!</p>",
                "mediaType": "text/html",
                "published": "2024-01-15T10:30:00Z",
                "url": "https://example.com/u/alice/{uuid}",
                "to": ["https://www.w3.org/ns/activitystreams#Public"],
                "cc": ["https://example.com/users/alice/followers"]
            }
        }
    ],
    "next": "https://example.com/users/alice/outbox?page=2"
}
```

### Pagination

| Parameter | Default | Items Per Page |
|-----------|---------|----------------|
| `page` | 0 (collection) | 20 |

---

## Note Object Endpoint

**Route:** `GET /notes/:id`

Returns a single Note as an ActivityPub object.

### Response Format

```json
{
    "@context": "https://www.w3.org/ns/activitystreams",
    "id": "https://example.com/notes/{uuid}",
    "type": "Note",
    "attributedTo": "https://example.com/users/alice",
    "content": "<p>Hello @bob@mastodon.social! Check out #fediverse</p>",
    "mediaType": "text/html",
    "published": "2024-01-15T10:30:00Z",
    "url": "https://example.com/u/alice/{uuid}",
    "to": ["https://www.w3.org/ns/activitystreams#Public"],
    "cc": [
        "https://example.com/users/alice/followers",
        "https://mastodon.social/users/bob"
    ],
    "tag": [
        {
            "type": "Hashtag",
            "href": "https://example.com/tags/fediverse",
            "name": "#fediverse"
        },
        {
            "type": "Mention",
            "href": "https://mastodon.social/users/bob",
            "name": "@bob@mastodon.social"
        }
    ]
}
```

### Optional Fields

| Field | Present When |
|-------|--------------|
| `updated` | Note was edited |
| `tag` | Has hashtags or mentions |

---

## Followers Collection

**Route:** `GET /users/:actor/followers`

Returns followers as an OrderedCollection.

### Collection Response

```json
{
    "@context": "https://www.w3.org/ns/activitystreams",
    "id": "https://example.com/users/alice/followers",
    "type": "OrderedCollection",
    "totalItems": 5,
    "first": "https://example.com/users/alice/followers?page=1"
}
```

### Collection Page Response (?page=1)

```json
{
    "@context": "https://www.w3.org/ns/activitystreams",
    "id": "https://example.com/users/alice/followers?page=1",
    "type": "OrderedCollectionPage",
    "partOf": "https://example.com/users/alice/followers",
    "orderedItems": [
        "https://mastodon.social/users/bob",
        "https://example.com/users/charlie"
    ],
    "totalItems": 5
}
```

---

## Following Collection

**Route:** `GET /users/:actor/following`

Returns accounts the user follows as an OrderedCollection.

### Response Format

Same structure as followers collection.

---

## WebFinger Endpoint

**Route:** `GET /.well-known/webfinger`

Enables user discovery via `acct:` URI.

### Request

```
GET /.well-known/webfinger?resource=acct:alice@example.com
```

### Response

```json
{
    "subject": "acct:alice@example.com",
    "links": [
        {
            "rel": "self",
            "type": "application/activity+json",
            "href": "https://example.com/users/alice"
        }
    ]
}
```

### Error Response (404)

```json
{"detail": "Not Found"}
```

---

## NodeInfo Endpoints

### Well-Known NodeInfo

**Route:** `GET /.well-known/nodeinfo`

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

```json
{
    "version": "2.0",
    "software": {
        "name": "stegodon",
        "version": "1.6.0"
    },
    "protocols": ["activitypub"],
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

### NodeInfo 2.1

**Route:** `GET /nodeinfo/2.1`

NodeInfo 2.1 adds `repository` and `homepage` fields to the `software` object.

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

## Content-Type Handling

All ActivityPub endpoints set:

```go
c.Header("Content-Type", "application/activity+json; charset=utf-8")
```

---

## Shared Inbox Routing

The shared inbox determines target username by checking:

1. **"to" field**: Direct addressing
2. **"cc" field**: Followers collections (`/users/{username}/followers`)
3. **"object" field**: Follow activity targets
4. **Actor's followers**: For Create/Update from followed users
5. **Any local user**: For relay content

```go
// Helper to extract username from URI
extractUsername := func(uri string) string {
    if strings.Contains(uri, conf.Conf.SslDomain) && strings.Contains(uri, "/users/") {
        // Extract username from https://domain/users/username
    }
    return ""
}
```

---

## Error Handling

| Error | HTTP Status | Response |
|-------|-------------|----------|
| User not found | 404 | `{}` |
| Invalid note ID | 404 | `{"error": "Invalid note ID"}` |
| Note not found | 404 | `{"error": "Note not found"}` |
| Parse error | 400 | (empty) |

---

## Source Files

- `web/router.go` - Route definitions
- `web/actor.go` - Actor, Note, Collection handlers
- `web/outbox.go` - Outbox handlers
- `web/webfinger.go` - WebFinger handlers
- `web/nodeinfo.go` - NodeInfo handlers
- `web/inbox.go` - Inbox routing (shared inbox)
- `activitypub/inbox.go` - Activity processing
