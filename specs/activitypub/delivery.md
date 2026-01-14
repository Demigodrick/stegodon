# Delivery Queue

This document specifies the background delivery queue system for reliable ActivityPub activity delivery.

---

## Overview

The delivery queue provides reliable activity delivery to remote servers. It features:
- Background worker with periodic polling
- Persistent queue stored in SQLite
- Exponential backoff for failed deliveries
- Maximum retry limit before abandoning
- Graceful shutdown support

---

## Architecture

```
Activity Created (Create, Update, Delete, etc.)
      │
      ▼
Collect Target Inboxes
      │
      ├── Followers' inboxes
      ├── Mentioned users' inboxes
      ├── Parent author inbox (replies)
      └── Active relay inboxes
            │
            ▼
Enqueue Delivery Items (one per inbox)
      │
      ▼
Background Worker (10s interval)
      │
      ▼
Process Pending Deliveries
      │
      ├── Success → Delete from queue
      └── Failure → Retry with backoff
            │
            ├── Attempts < 10 → Schedule retry
            └── Attempts >= 10 → Delete (give up)
```

---

## Configuration

```go
const (
    pollInterval    = 10 * time.Second  // How often worker checks queue
    batchSize       = 50                // Max items processed per cycle
    maxAttempts     = 10                // Max retries before abandoning
    requestTimeout  = 10 * time.Second  // HTTP request timeout
)
```

---

## DeliveryQueueItem Entity

```go
type DeliveryQueueItem struct {
    Id           uuid.UUID
    InboxURI     string     // Target inbox URL
    ActivityJSON string     // Serialized activity
    Attempts     int        // Number of delivery attempts
    NextRetryAt  time.Time  // When to retry next
    CreatedAt    time.Time  // When item was queued
}
```

---

## Exponential Backoff

Retry delays increase exponentially with each failed attempt:

```go
backoffMinutes := []int{1, 5, 15, 60, 240, 1440}[min(item.Attempts-1, 5)]
item.NextRetryAt = time.Now().Add(time.Duration(backoffMinutes) * time.Minute)
```

| Attempt | Retry Delay | Cumulative Wait |
|---------|-------------|-----------------|
| 1 | 1 minute | 1 minute |
| 2 | 5 minutes | 6 minutes |
| 3 | 15 minutes | 21 minutes |
| 4 | 1 hour | ~1.5 hours |
| 5 | 4 hours | ~5.5 hours |
| 6+ | 24 hours | ~29.5 hours |

After 10 failed attempts, the delivery is abandoned.

---

## Worker Lifecycle

### Starting the Worker

```go
func StartDeliveryWorker(conf *util.AppConfig) func() {
    log.Println("Starting ActivityPub delivery worker...")

    ticker := time.NewTicker(10 * time.Second)
    stop := make(chan struct{})

    go func() {
        for {
            select {
            case <-ticker.C:
                processDeliveryQueue(conf)
            case <-stop:
                ticker.Stop()
                log.Println("ActivityPub delivery worker stopped")
                return
            }
        }
    }()

    // Return stop function for graceful shutdown
    return func() {
        close(stop)
    }
}
```

### Graceful Shutdown

```go
// In app shutdown handler
stopDeliveryWorker := StartDeliveryWorker(conf)
defer stopDeliveryWorker()

// On SIGTERM/SIGINT
stopDeliveryWorker()  // Stops the ticker, waits for current delivery to finish
```

---

## Queue Processing

### Process Loop

```go
func processDeliveryQueueWithDeps(conf *util.AppConfig, deps *DeliveryDeps) {
    database := deps.Database

    // Get pending deliveries (max 50 at a time)
    err, items := database.ReadPendingDeliveries(50)
    if err != nil {
        log.Printf("DeliveryWorker: Failed to read queue: %v", err)
        return
    }

    if items == nil || len(*items) == 0 {
        return  // Nothing to process
    }

    log.Printf("DeliveryWorker: Processing %d pending deliveries", len(*items))

    for _, item := range *items {
        if err := deliverActivityWithDeps(&item, conf, deps); err != nil {
            handleDeliveryFailure(&item, err, database)
        } else {
            handleDeliverySuccess(&item, database)
        }
    }
}
```

### Success Handling

```go
func handleDeliverySuccess(item *domain.DeliveryQueueItem, database Database) {
    log.Printf("DeliveryWorker: Successfully delivered to %s", item.InboxURI)
    database.DeleteDelivery(item.Id)
}
```

### Failure Handling

```go
func handleDeliveryFailure(item *domain.DeliveryQueueItem, err error, database Database) {
    item.Attempts++
    backoffMinutes := []int{1, 5, 15, 60, 240, 1440}[min(item.Attempts-1, 5)]
    item.NextRetryAt = time.Now().Add(time.Duration(backoffMinutes) * time.Minute)

    if item.Attempts >= 10 {
        // Give up after 10 attempts
        log.Printf("DeliveryWorker: Giving up on delivery to %s after %d attempts",
            item.InboxURI, item.Attempts)
        database.DeleteDelivery(item.Id)
    } else {
        log.Printf("DeliveryWorker: Delivery to %s failed (attempt %d), retry in %dm: %v",
            item.InboxURI, item.Attempts, backoffMinutes, err)
        database.UpdateDeliveryAttempt(item.Id, item.Attempts, item.NextRetryAt)
    }
}
```

---

## Delivery Execution

### Request Construction

```go
func deliverActivityWithDeps(item *domain.DeliveryQueueItem, conf *util.AppConfig, deps *DeliveryDeps) error {
    // Parse the activity JSON
    var activity map[string]any
    if err := json.Unmarshal([]byte(item.ActivityJSON), &activity); err != nil {
        return fmt.Errorf("failed to parse activity JSON: %w", err)
    }

    // Extract actor from activity
    actor, ok := activity["actor"].(string)
    if !ok {
        return fmt.Errorf("activity missing actor field")
    }

    // Extract username from actor URI
    // actor format: "https://example.com/users/alice"
    parts := strings.Split(actor, "/")
    username := parts[len(parts)-1]

    // Get local account for signing
    database := deps.Database
    err, localAccount := database.ReadAccByUsername(username)
    if err != nil {
        return fmt.Errorf("failed to get local account: %w", err)
    }

    // Parse private key
    privateKey, err := ParsePrivateKey(localAccount.WebPrivateKey)
    if err != nil {
        return fmt.Errorf("failed to parse private key: %w", err)
    }

    // Calculate digest for HTTP signature
    activityBytes := []byte(item.ActivityJSON)
    hash := sha256.Sum256(activityBytes)
    digest := "SHA-256=" + base64.StdEncoding.EncodeToString(hash[:])

    // Create HTTP request
    req, err := http.NewRequest("POST", item.InboxURI, bytes.NewReader(activityBytes))
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }

    // Set headers
    req.Header.Set("Content-Type", "application/activity+json")
    req.Header.Set("Accept", "application/activity+json")
    req.Header.Set("User-Agent", "stegodon/1.0 ActivityPub")
    req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
    req.Header.Set("Host", req.URL.Host)
    req.Header.Set("Digest", digest)

    // Sign request
    keyID := fmt.Sprintf("https://%s/users/%s#main-key", conf.Conf.SslDomain, username)
    if err := SignRequest(req, privateKey, keyID); err != nil {
        return fmt.Errorf("failed to sign request: %w", err)
    }

    // Send request
    resp, err := deps.HTTPClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return fmt.Errorf("remote server returned status: %d", resp.StatusCode)
    }

    return nil
}
```

---

## Enqueueing Activities

### From SendCreate

```go
// Collect unique inboxes
inboxes := make(map[string]bool)

// Add followers
for _, follower := range *followers {
    if !follower.IsLocal {
        inboxes[remoteActor.InboxURI] = true
    }
}

// Add relays
for _, relay := range *relays {
    inboxes[relay.InboxURI] = true
}

// Queue delivery to each inbox
for inboxURI := range inboxes {
    queueItem := &domain.DeliveryQueueItem{
        Id:           uuid.New(),
        InboxURI:     inboxURI,
        ActivityJSON: mustMarshal(create),
        Attempts:     0,
        NextRetryAt:  time.Now(),
        CreatedAt:    time.Now(),
    }

    if err := database.EnqueueDelivery(queueItem); err != nil {
        log.Printf("Failed to queue delivery to %s: %v", inboxURI, err)
    }
}
```

---

## Database Operations

### Schema

```sql
CREATE TABLE IF NOT EXISTS delivery_queue (
    id TEXT PRIMARY KEY,
    inbox_uri TEXT NOT NULL,
    activity_json TEXT NOT NULL,
    attempts INTEGER DEFAULT 0,
    next_retry_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE INDEX idx_delivery_queue_next_retry ON delivery_queue(next_retry_at);
```

### Interface

```go
type Database interface {
    // Queue operations
    EnqueueDelivery(item *domain.DeliveryQueueItem) error
    ReadPendingDeliveries(limit int) (error, *[]domain.DeliveryQueueItem)
    UpdateDeliveryAttempt(id uuid.UUID, attempts int, nextRetry time.Time) error
    DeleteDelivery(id uuid.UUID) error
}
```

### ReadPendingDeliveries Query

```sql
SELECT id, inbox_uri, activity_json, attempts, next_retry_at, created_at
FROM delivery_queue
WHERE next_retry_at <= ?
ORDER BY next_retry_at ASC
LIMIT ?
```

---

## Dependency Injection

```go
type DeliveryDeps struct {
    Database   Database
    HTTPClient HTTPClient
}

// Production
func processDeliveryQueue(conf *util.AppConfig) {
    deps := &DeliveryDeps{
        Database:   NewDBWrapper(),
        HTTPClient: defaultHTTPClient,
    }
    processDeliveryQueueWithDeps(conf, deps)
}

// Testing
func processDeliveryQueueWithDeps(conf *util.AppConfig, deps *DeliveryDeps) {
    // ...
}
```

---

## Error Scenarios

| Error | Behavior |
|-------|----------|
| Network timeout | Retry with backoff |
| Connection refused | Retry with backoff |
| HTTP 4xx | Retry with backoff (may indicate temporary issue) |
| HTTP 5xx | Retry with backoff (server error) |
| Invalid activity JSON | Logged, retried (shouldn't happen) |
| Missing local account | Logged, retried (shouldn't happen) |
| Private key parse error | Logged, retried (shouldn't happen) |

---

## Monitoring

### Log Messages

```
DeliveryWorker: Processing 5 pending deliveries
DeliveryWorker: Successfully delivered to https://mastodon.social/inbox
DeliveryWorker: Delivery to https://example.com/inbox failed (attempt 3), retry in 15m: connection refused
DeliveryWorker: Giving up on delivery to https://dead.server/inbox after 10 attempts
```

### Queue Status

The queue can be monitored via:
- Database query: `SELECT COUNT(*) FROM delivery_queue`
- Database query: `SELECT * FROM delivery_queue WHERE attempts > 0` (failed items)

---

## Performance Considerations

1. **Batch Size**: Processing 50 items per cycle prevents long-running cycles
2. **Poll Interval**: 10 seconds balances responsiveness with CPU usage
3. **Indexed Query**: `next_retry_at` index enables efficient pending item lookup
4. **Inbox Deduplication**: Map-based deduplication prevents duplicate queue entries

---

## Source Files

- `activitypub/delivery.go` - Background worker and delivery logic
- `activitypub/outbox.go` - Activity enqueueing
- `activitypub/httpsig.go` - Request signing
- `activitypub/deps.go` - Database and HTTP client interfaces
- `domain/delivery_queue.go` - DeliveryQueueItem entity
- `db/db.go` - Queue database operations
