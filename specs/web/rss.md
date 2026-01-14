# RSS Feeds

This document specifies RSS feed generation for user posts.

---

## Overview

Stegodon provides RSS feeds for:
- All local posts (public timeline)
- Individual user posts
- Single post items

Feeds use the [gorilla/feeds](https://github.com/gorilla/feeds) library.

---

## Endpoints

### All Posts Feed

**Route:** `GET /feed`

Returns RSS feed of all local posts (excluding replies).

### User Posts Feed

**Route:** `GET /feed?username=alice`

Returns RSS feed of a specific user's posts.

### Single Item Feed

**Route:** `GET /feed/:id`

Returns RSS feed containing a single post.

---

## Feed Structure

### Channel Metadata

```xml
<rss version="2.0">
  <channel>
    <title>Stegodon Notes - alice</title>
    <link>https://example.com/feed?username=alice</link>
    <description>rss feed for testing stegodon</description>
    <author>alice@stegodon</author>
    <pubDate>Mon, 15 Jan 2024 10:30:00 UTC</pubDate>
  </channel>
</rss>
```

### Item Structure

```xml
<item>
  <guid>550e8400-e29b-41d4-a716-446655440000</guid>
  <title>Jan 15, 2024 10:30</title>
  <link>https://example.com/feed/550e8400-e29b-41d4-a716-446655440000</link>
  <description><![CDATA[<p>Hello world! Check out <a href="https://example.com">this link</a></p>]]></description>
  <author>alice@stegodon</author>
  <pubDate>Mon, 15 Jan 2024 10:30:00 UTC</pubDate>
</item>
```

---

## Implementation

### GetRSS Function

```go
func GetRSS(conf *util.AppConfig, username string) (string, error) {
    var notes *[]domain.Note
    var title string

    if username != "" {
        // User-specific feed
        err, notes = db.GetDB().ReadNotesByUsername(username)
        title = fmt.Sprintf("Stegodon Notes - %s", username)
    } else {
        // All posts feed
        err, notes = db.GetDB().ReadAllNotes()
        title = "All Stegodon Notes"
    }

    feed := &feeds.Feed{
        Title:       title,
        Link:        &feeds.Link{Href: buildURL(conf, "/feed")},
        Description: "rss feed for testing stegodon",
        Author:      &feeds.Author{Name: createdBy, Email: email},
        Created:     time.Now(),
    }

    for _, note := range *notes {
        // Skip replies
        if note.InReplyToURI != "" {
            continue
        }

        // Convert Markdown links to HTML
        contentHTML := util.MarkdownLinksToHTML(note.Message)

        feedItems = append(feedItems, &feeds.Item{
            Id:      note.Id.String(),
            Title:   note.CreatedAt.Format(util.DateTimeFormat()),
            Link:    &feeds.Link{Href: buildURL(conf, fmt.Sprintf("/feed/%s", note.Id))},
            Content: contentHTML,
            Author:  &feeds.Author{Name: note.CreatedBy, Email: email},
            Created: note.CreatedAt,
        })
    }

    feed.Items = feedItems
    return feed.ToRss()
}
```

### GetRSSItem Function

```go
func GetRSSItem(conf *util.AppConfig, id uuid.UUID) (string, error) {
    err, note := db.GetDB().ReadNoteId(id)
    if err != nil || note == nil {
        return "", errors.New("error retrieving note by id")
    }

    // Convert Markdown links to HTML
    contentHTML := util.MarkdownLinksToHTML(note.Message)

    feed := &feeds.Feed{
        Title: "Single Stegodon Note",
        Link:  &feeds.Link{Href: buildURL(conf, fmt.Sprintf("/feed/%s", note.Id))},
        // ...
    }

    feed.Items = []*feeds.Item{{
        Id:      note.Id.String(),
        Title:   note.CreatedAt.Format(util.DateTimeFormat()),
        Link:    &feeds.Link{Href: url},
        Content: contentHTML,
        Author:  &feeds.Author{Name: note.CreatedBy, Email: email},
        Created: note.CreatedAt,
    }}

    return feed.ToRss()
}
```

---

## URL Building

### buildURL Function

```go
func buildURL(conf *util.AppConfig, path string) string {
    if conf.Conf.WithAp && conf.Conf.SslDomain != "" {
        return fmt.Sprintf("https://%s%s", conf.Conf.SslDomain, path)
    }
    return fmt.Sprintf("http://%s:%d%s", conf.Conf.Host, conf.Conf.HttpPort, path)
}
```

| Mode | Example URL |
|------|-------------|
| ActivityPub enabled | `https://example.com/feed` |
| Local only | `http://127.0.0.1:9999/feed` |

---

## Content Processing

### Markdown Link Conversion

Markdown links are converted to HTML for feed content:

```go
contentHTML := util.MarkdownLinksToHTML(note.Message)
```

| Input | Output |
|-------|--------|
| `Check [this](https://example.com)` | `Check <a href="https://example.com">this</a>` |

---

## Reply Filtering

Replies are excluded from feed listings:

```go
for _, note := range *notes {
    if note.InReplyToURI != "" {
        continue  // Skip replies
    }
    // Add to feed...
}
```

---

## Feed Fields

### Channel Fields

| Field | Source |
|-------|--------|
| Title | `"Stegodon Notes - {username}"` or `"All Stegodon Notes"` |
| Link | Feed URL with username parameter |
| Description | `"rss feed for testing stegodon"` |
| Author | Username or `"everyone"` |
| Created | Current time |

### Item Fields

| Field | Source |
|-------|--------|
| Id (GUID) | Note UUID as string |
| Title | Formatted creation timestamp |
| Link | Individual item feed URL |
| Content | HTML-converted message |
| Author | Note creator username |
| Created | Note creation timestamp |

---

## Email Format

Author email is constructed as:

```go
email := fmt.Sprintf("%s@stegodon", username)
```

Example: `alice@stegodon`

---

## Date Format

Item titles use the application's datetime format:

```go
note.CreatedAt.Format(util.DateTimeFormat())
```

Example: `Jan 15, 2024 10:30`

---

## Content-Type

```
Content-Type: application/xml; charset=utf-8
```

---

## Error Handling

| Error | Response |
|-------|----------|
| Username not found | Empty feed (no items) |
| Note ID not found | `errors.New("error retrieving note by id")` |
| Database error | `errors.New("error retrieving notes")` |

---

## Library

Uses [gorilla/feeds](https://github.com/gorilla/feeds):

```go
import "github.com/gorilla/feeds"

feed := &feeds.Feed{...}
rssString, err := feed.ToRss()
```

---

## Source Files

- `web/rss.go` - RSS generation functions
- `web/router.go` - Route registration
- `util/text.go` - Markdown link conversion
