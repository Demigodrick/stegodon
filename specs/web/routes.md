# HTTP Routes

This document specifies all HTTP routes, rate limiting, and request handling for the web server.

---

## Overview

Stegodon's HTTP server uses [Gin](https://github.com/gin-gonic/gin) framework with:
- Embedded static assets and templates
- GZIP compression
- Per-IP rate limiting
- Request body size limits for ActivityPub

---

## Server Configuration

```go
g := gin.New()
g.Use(gin.Logger(), gin.Recovery())
g.Use(gzip.Gzip(gzip.DefaultCompression))
```

| Middleware | Purpose |
|------------|---------|
| `gin.Logger()` | Request logging |
| `gin.Recovery()` | Panic recovery |
| `gzip.Gzip()` | Response compression |

---

## Rate Limiting

### Global Rate Limiter

Applied to all routes:

```go
globalLimiter := NewRateLimiter(rate.Limit(10), 20)
g.Use(RateLimitMiddleware(globalLimiter))
```

| Setting | Value |
|---------|-------|
| Rate | 10 requests/second |
| Burst | 20 requests |

### ActivityPub Rate Limiter

Stricter limits for federation endpoints:

```go
apLimiter := NewRateLimiter(rate.Limit(5), 10)
```

| Setting | Value |
|---------|-------|
| Rate | 5 requests/second |
| Burst | 10 requests |

### Rate Limiter Implementation

Per-IP token bucket with automatic cleanup:

```go
type RateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.RWMutex
    rate     rate.Limit
    burst    int
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    limiter, exists := rl.limiters[ip]
    if !exists {
        limiter = rate.NewLimiter(rl.rate, rl.burst)
        rl.limiters[ip] = limiter
    }
    return limiter
}
```

**Cleanup**: Every 5 minutes, if map exceeds 10,000 entries, it's reset.

---

## Request Body Limits

ActivityPub endpoints have a 1MB body limit:

```go
maxBodySize := MaxBytesMiddleware(1 * 1024 * 1024)  // 1MB
```

Returns HTTP 413 if exceeded.

---

## Route Summary

### Web UI Routes

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/` | `HandleIndex` | Home timeline |
| GET | `/u/:username` | `HandleProfile` | User profile |
| GET | `/u/:username/:noteid` | `HandleSinglePost` | Single post view |
| GET | `/@:username` | (redirect) | Mastodon-style → `/u/username` |
| GET | `/tags/:tag` | `HandleTagFeed` | Hashtag feed |

### Static Assets

| Method | Path | Description |
|--------|------|-------------|
| GET | `/static/stegologo.png` | Logo image (24h cache) |
| GET | `/static/style.css` | Stylesheet |

### RSS Feeds

| Method | Path | Description |
|--------|------|-------------|
| GET | `/feed` | RSS feed (optional `?username=`) |
| GET | `/feed/:id` | Single item RSS |

### ActivityPub Routes (when `WithAp=true`)

| Method | Path | Handler | Rate Limit |
|--------|------|---------|------------|
| GET | `/notes/:id` | Note object | Global |
| GET | `/users/:actor` | Actor profile | Global |
| POST | `/inbox` | Shared inbox | AP (5/sec) |
| POST | `/users/:actor/inbox` | User inbox | AP (5/sec) |
| GET | `/users/:actor/outbox` | User outbox | Global |
| GET | `/users/:actor/followers` | Followers collection | Global |
| GET | `/users/:actor/following` | Following collection | Global |

### Discovery Routes (when `WithAp=true`)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/.well-known/webfinger` | `GetWebfinger` | User discovery |
| GET | `/.well-known/nodeinfo` | `GetWellKnownNodeInfo` | NodeInfo links |
| GET | `/nodeinfo/2.0` | `GetNodeInfo20` | Server statistics |

---

## Embedded Assets

Assets are embedded using Go's `embed` package:

```go
//go:embed templates/*.html
var embeddedTemplates embed.FS

//go:embed static/stegologo.png
var embeddedLogo []byte

//go:embed static/style.css
var embeddedCSS []byte
```

### Template Loading

```go
tmpl, err := template.ParseFS(embeddedTemplates, "templates/*.html")
g.SetHTMLTemplate(tmpl)
```

### Static Asset Caching

```go
g.GET("/static/stegologo.png", func(c *gin.Context) {
    c.Header("Content-Type", "image/png")
    c.Header("Cache-Control", "public, max-age=86400")  // 24 hours
    c.Data(200, "image/png", embeddedLogo)
})
```

---

## Content-Type Headers

| Route Type | Content-Type |
|------------|--------------|
| HTML pages | `text/html; charset=utf-8` |
| ActivityPub | `application/activity+json; charset=utf-8` |
| WebFinger | `application/json; charset=utf-8` |
| RSS | `application/xml; charset=utf-8` |
| CSS | `text/css; charset=utf-8` |
| PNG | `image/png` |

---

## Error Responses

### Rate Limit Exceeded

```json
{
    "error": "Rate limit exceeded. Please try again later."
}
```

HTTP Status: 429 Too Many Requests

### Request Too Large

```json
{
    "error": "Request body too large"
}
```

HTTP Status: 413 Request Entity Too Large

### Not Found

HTTP 404 with appropriate error page or JSON.

---

## URL Redirects

### Mastodon-Style URLs

```go
g.GET("/@:username", func(c *gin.Context) {
    username := c.Param("username")
    c.Redirect(301, "/u/"+username)
})
```

Redirects `/@alice` to `/u/alice` for compatibility.

---

## Content Negotiation

### Actor Endpoint (`/users/:actor`)

The `/users/:actor` endpoint uses content negotiation to serve both ActivityPub clients and web browsers. This fixes federation compatibility with Lemmy (GitHub issue #36).

**Problem**: Lemmy uses the ActivityPub `id` field (`/users/username`) for redirects instead of the `url` field (`/u/username`).

**Solution**: Content negotiation based on `Accept` header.

```go
g.GET("/users/:actor", func(c *gin.Context) {
    actorName := c.Param("actor")
    accept := c.GetHeader("Accept")

    // Browser requests redirect to human-readable profile
    if IsHTMLRequest(accept) {
        c.Redirect(302, "/u/"+actorName)
        return
    }

    // ActivityPub clients get JSON
    c.Header("Content-Type", "application/activity+json; charset=utf-8")
    err, actor := GetActor(actorName, conf)
    // ...
})
```

### IsHTMLRequest Helper

Determines if a request is from a browser (HTML) vs ActivityPub client:

```go
func IsHTMLRequest(accept string) bool {
    // Empty or wildcard = browser
    if accept == "" || accept == "*/*" {
        return true
    }

    // ActivityPub content types = NOT browser
    if strings.Contains(accept, "application/activity+json") ||
        strings.Contains(accept, "application/ld+json") ||
        strings.Contains(accept, "application/json") {
        return false
    }

    // HTML content type = browser
    if strings.Contains(accept, "text/html") {
        return true
    }

    // Default: treat as browser
    return true
}
```

### Accept Header Examples

| Accept Header | Request Type | Response |
|---------------|--------------|----------|
| (empty) | Browser | 302 → `/u/username` |
| `*/*` | Browser | 302 → `/u/username` |
| `text/html` | Browser | 302 → `/u/username` |
| `text/html,application/xhtml+xml,...` | Browser | 302 → `/u/username` |
| `application/activity+json` | ActivityPub | JSON actor |
| `application/ld+json` | ActivityPub | JSON actor |
| `application/json` | ActivityPub | JSON actor |

---

## Conditional Route Registration

ActivityPub routes are only registered when federation is enabled:

```go
if conf.Conf.WithAp {
    // ActivityPub endpoints
    g.GET("/users/:actor", ...)
    g.POST("/inbox", ...)
    g.GET("/.well-known/webfinger", ...)
    // ...
}
```

---

## Log Output

Router initialization logs:

```
Initializing HTTP router on port 9999
```

Request logging via `gin.Logger()`:

```
[GIN] 2024/01/15 - 10:30:00 | 200 |   12.5ms | 192.168.1.1 | GET "/u/alice"
```

---

## Source Files

- `web/router.go` - Route definitions
- `web/middleware.go` - Rate limiting, body limits
- `web/ui.go` - Web UI handlers
- `web/actor.go` - ActivityPub actor handlers
- `web/webfinger.go` - WebFinger handlers
- `web/nodeinfo.go` - NodeInfo handlers
- `web/outbox.go` - Outbox handlers
- `web/rss.go` - RSS handlers
