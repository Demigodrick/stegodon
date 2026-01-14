# Auto-Refresh

This document specifies timeline auto-refresh patterns and goroutine lifecycle management.

---

## Overview

Timeline views auto-refresh periodically to show new posts:
- **Home timeline** - Refreshes every 10 seconds
- **Thread view** - Refreshes on data changes
- Uses `isActive` flag to prevent goroutine leaks

---

## Refresh Configuration

### Constants

```go
const (
    // TimelineRefreshSeconds is the interval for auto-refreshing timeline views
    TimelineRefreshSeconds = 10

    // HomeTimelinePostLimit is the maximum number of posts to load
    HomeTimelinePostLimit = 50
)
```

---

## Active State Pattern

### Model Structure

```go
type Model struct {
    // ... other fields
    isActive bool // Track if this view is currently visible
}
```

### Initialization

```go
func InitialModel(...) Model {
    return Model{
        isActive: false, // Start inactive
        // ...
    }
}
```

### Init Command

```go
func (m Model) Init() tea.Cmd {
    // Don't start any commands here - model starts inactive
    // ActivateViewMsg handler will load data and start ticker
    return nil
}
```

---

## View Activation

### ActivateViewMsg

```go
case common.ActivateViewMsg:
    m.isActive = true
    // Reset scroll position
    m.Selected = 0
    m.Offset = 0
    // Load data first, tick will be scheduled when data arrives
    return m, loadHomePosts(m.AccountId)
```

### DeactivateViewMsg

```go
case common.DeactivateViewMsg:
    m.isActive = false
    return m, nil
```

---

## Ticker Pattern

### Tick Message

```go
type refreshTickMsg struct{}
```

### Tick Command

```go
func tickRefresh() tea.Cmd {
    return tea.Tick(common.TimelineRefreshSeconds*time.Second, func(t time.Time) tea.Msg {
        return refreshTickMsg{}
    })
}
```

### Tick Handler

```go
case refreshTickMsg:
    // Only schedule next refresh if view is still active
    if m.isActive {
        return m, loadHomePosts(m.AccountId)
    }
    // View is inactive, stop the ticker chain
    return m, nil
```

---

## Data Loading Flow

### Load Command

```go
func loadHomePosts(accountId uuid.UUID) tea.Cmd {
    return func() tea.Msg {
        database := db.GetDB()
        err, posts := database.ReadHomeTimelinePosts(accountId, common.HomeTimelinePostLimit)
        if err != nil {
            log.Printf("Failed to load home timeline: %v", err)
            return postsLoadedMsg{posts: []domain.HomePost{}}
        }
        return postsLoadedMsg{posts: *posts}
    }
}
```

### Data Loaded Handler

```go
case postsLoadedMsg:
    m.Posts = msg.posts
    // Keep selection within bounds
    if m.Selected >= len(m.Posts) {
        m.Selected = max(0, len(m.Posts)-1)
    }
    m.Offset = m.Selected

    // Schedule next tick AFTER data loads (only if still active)
    if m.isActive {
        return m, tickRefresh()
    }
    return m, nil
```

---

## Refresh Sequence

### Complete Flow

```
1. User navigates to home timeline
2. SuperTUI sends ActivateViewMsg
3. Model sets isActive = true
4. Model returns loadHomePosts() command
5. Data loads, postsLoadedMsg received
6. Model updates Posts
7. Model returns tickRefresh() command (if isActive)
8. After 10 seconds, refreshTickMsg received
9. If isActive, goto step 4; else stop
10. User navigates away
11. SuperTUI sends DeactivateViewMsg
12. Model sets isActive = false
13. Next refreshTickMsg does nothing (breaks chain)
```

### Diagram

```
          ActivateViewMsg
                │
                ▼
         isActive = true
                │
                ▼
        loadHomePosts() ◄────────┐
                │                │
                ▼                │
        postsLoadedMsg           │
                │                │
          ┌─────┴─────┐          │
          │ isActive? │          │
          └─────┬─────┘          │
                │                │
         true   │   false        │
                ▼                │
        tickRefresh()            │
                │                │
          (10 seconds)           │
                │                │
                ▼                │
        refreshTickMsg           │
                │                │
          ┌─────┴─────┐          │
          │ isActive? │          │
          └─────┬─────┘          │
                │                │
         true   │   false        │
                │     │          │
                │     ▼          │
                │   (stop)       │
                │                │
                └────────────────┘

     DeactivateViewMsg
            │
            ▼
     isActive = false
            │
            ▼
     (chain breaks at next tick)
```

---

## Manual Refresh

### UpdateNoteList

When notes are created/updated, force refresh:

```go
case common.SessionState:
    if msg == common.UpdateNoteList {
        return m, loadHomePosts(m.AccountId)
    }
    return m, nil
```

---

## Goroutine Safety

### Why isActive Matters

Without `isActive` check:
- User navigates to home timeline → ticker starts
- User navigates to settings → ticker continues
- User returns to home timeline → second ticker starts
- Result: Multiple overlapping tickers (goroutine leak)

With `isActive` check:
- User navigates away → `isActive = false`
- Next tick sees `isActive = false` → chain breaks
- User returns → fresh ticker starts
- Result: Single ticker at a time

### Best Practices

| Do | Don't |
|----|-------|
| Start ticker on ActivateViewMsg | Start ticker in Init() |
| Check isActive before scheduling | Always schedule next tick |
| Use single ticker chain | Use go func() with for loop |
| Break chain when inactive | Use explicit goroutine stop |

---

## Thread View Refresh

### On Like/Update

Thread view refreshes when notes change:

```go
case common.SessionState:
    if msg == common.UpdateNoteList && m.isActive && m.ParentURI != "" {
        // Store selection to restore after reload
        m.pendingSelection = m.Selected
        m.pendingOffset = m.Offset
        // Reload thread
        return m, loadThread(m.ParentURI)
    }
    return m, nil
```

### Selection Preservation

```go
case threadLoadedMsg:
    m.ParentPost = msg.parent
    m.Replies = msg.replies
    // Restore selection if pending
    if m.pendingSelection != -2 {
        if m.pendingSelection >= -1 && m.pendingSelection < len(m.Replies) {
            m.Selected = m.pendingSelection
            m.Offset = m.pendingOffset
        }
        m.pendingSelection = -2 // Clear pending
    }
```

---

## Views Using Auto-Refresh

| View | Auto-Refresh | Trigger |
|------|--------------|---------|
| Home Timeline | Yes (10s) | Ticker |
| My Posts | Yes (10s) | Ticker |
| Thread View | On change | UpdateNoteList |
| Followers | No | Manual |
| Following | No | Manual |
| Notifications | Yes (10s) | Ticker |

---

## Source Files

- `ui/hometimeline/hometimeline.go` - Primary auto-refresh implementation
- `ui/myposts/myposts.go` - Similar pattern
- `ui/threadview/threadview.go` - On-change refresh
- `ui/common/layout.go` - `TimelineRefreshSeconds` constant
- `ui/common/commands.go` - `ActivateViewMsg`, `DeactivateViewMsg`
