# WebFinger Protocol

This document specifies the WebFinger implementation for user discovery via `acct:` URIs.

---

## Overview

WebFinger enables federated user discovery. When a Mastodon user searches for `@alice@example.com`, their server queries `https://example.com/.well-known/webfinger?resource=acct:alice@example.com` to find Alice's ActivityPub actor URL.

---

## Endpoint

**Route:** `GET /.well-known/webfinger`

**Query Parameter:** `resource` - The `acct:` URI to look up

### Example Request

```
GET /.well-known/webfinger?resource=acct:alice@example.com
Accept: application/jrd+json
```

### Response Format

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

### Response Fields

| Field | Description |
|-------|-------------|
| `subject` | The original `acct:` URI from the request |
| `links[].rel` | Relationship type (`self` for the actor) |
| `links[].type` | Media type (`application/activity+json`) |
| `links[].href` | The ActivityPub actor URI |

---

## Implementation

### GetWebfinger Function

```go
func GetWebfinger(user string, conf *util.AppConfig) (error, string) {
    // 1. Look up user in database
    err, acc := db.GetDB().ReadAccByUsername(user)
    if err != nil {
        return err, GetWebFingerNotFound()
    }

    // 2. Build WebFinger response
    return nil, fmt.Sprintf(`{
        "subject": "acct:%s@%s",
        "links": [
            {
                "rel": "self",
                "type": "application/activity+json",
                "href": "https://%s/users/%s"
            }
        ]
    }`, acc.Username, conf.Conf.SslDomain,
        conf.Conf.SslDomain, acc.Username)
}
```

### Error Response (404)

```json
{"detail": "Not Found"}
```

---

## Outgoing WebFinger Resolution

Stegodon also performs WebFinger lookups when following remote users.

### ResolveWebFinger Function

```go
func ResolveWebFinger(username, domain string) (string, error) {
    // Build WebFinger URL
    webfingerURL := fmt.Sprintf("https://%s/.well-known/webfinger?resource=acct:%s@%s",
        domain, username, domain)

    // Create request with appropriate headers
    req, err := http.NewRequest("GET", webfingerURL, nil)
    req.Header.Set("Accept", "application/jrd+json")
    req.Header.Set("User-Agent", "stegodon/1.0 ActivityPub")

    // Execute with timeout
    client := &http.Client{Timeout: 5 * time.Second}
    resp, err := client.Do(req)

    // Parse response and find ActivityPub actor
    for _, link := range result.Links {
        if link.Rel == "self" {
            if link.Type == "application/activity+json" ||
               link.Type == "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\"" {
                return link.Href, nil
            }
        }
    }
    return "", fmt.Errorf("no ActivityPub actor found")
}
```

### WebFinger Response Structure

```go
type WebFingerResponse struct {
    Subject string `json:"subject"`
    Links   []struct {
        Rel  string `json:"rel"`
        Type string `json:"type"`
        Href string `json:"href"`
    } `json:"links"`
}
```

---

## HTTP Client Configuration

| Setting | Value |
|---------|-------|
| Timeout | 5 seconds |
| Accept Header | `application/jrd+json` |
| User-Agent | `stegodon/1.0 ActivityPub` |

---

## Content-Type Support

### Inbound (Serving)

Returns `application/json; charset=utf-8`

### Outbound (Resolution)

Accepts both ActivityPub content types:
- `application/activity+json`
- `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`

---

## Usage Flow

### Following a Remote User

1. User enters `@alice@mastodon.social` in TUI
2. Parse username (`alice`) and domain (`mastodon.social`)
3. Call `ResolveWebFinger("alice", "mastodon.social")`
4. Returns `https://mastodon.social/users/alice`
5. Fetch actor profile from returned URL
6. Create Follow activity

### Being Discovered

1. Remote user searches `@bob@stegodon.example.com`
2. Remote server queries `/.well-known/webfinger?resource=acct:bob@stegodon.example.com`
3. Stegodon returns actor URL `https://stegodon.example.com/users/bob`
4. Remote server fetches actor profile
5. Follow activity sent to inbox

---

## Error Handling

| Error | HTTP Status | Response |
|-------|-------------|----------|
| User not found | 404 | `{"detail": "Not Found"}` |
| Invalid resource format | 400 | (empty) |
| Database error | 500 | (empty) |

---

## Security Considerations

- Only responds for local users (no open redirector)
- Uses HTTPS for all outbound requests
- Validates response structure before using
- Times out after 5 seconds to prevent hanging

---

## Source Files

- `web/webfinger.go` - WebFinger handlers and resolution
- `web/router.go` - Route registration
