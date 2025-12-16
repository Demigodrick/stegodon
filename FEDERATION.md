# ActivityPub Federation in Stegodon

Stegodon implements ActivityPub Server-to-Server (S2S) federation, allowing users to follow and be followed by accounts on other Fediverse servers (Mastodon, Pleroma, etc.). Activities are signed with HTTP Signatures and delivered via a background queue with retry logic.

## Supported Federation Protocols and Standards

- [ActivityPub](https://www.w3.org/TR/activitypub/) (Server-to-Server)
- [WebFinger](https://tools.ietf.org/html/rfc7033)
- [HTTP Signatures](https://datatracker.ietf.org/doc/html/draft-cavage-http-signatures) (RSA-SHA256)
- [NodeInfo 2.0](https://nodeinfo.diaspora.software/)

## Supported Activities

### Receiving (Inbox)

- `Follow(Actor)` - Auto-accepted, creates follower relationship
- `Accept(Follow)` - Confirms outgoing follow requests
- `Undo(Follow)` - Removes follower relationship (authorization verified)
- `Undo(Like)` - Removes like and decrements counter
- `Undo(Announce)` - Removes boost and decrements counter
- `Create(Note)` - Stores posts from followed accounts or relay subscriptions (with `inReplyTo` support)
- `Update(Note)` - Updates stored post content
- `Update(Person)` - Re-fetches and caches actor profile
- `Delete(Note)` - Removes stored post (authorization verified)
- `Delete(Actor)` - Removes actor and all associated follows
- `Like` - Stores like and increments counter on target note
- `Announce` - Stores boost/reblog or relay-forwarded content

### Sending (Outbox)

- `Accept(Follow)` - Sent automatically when receiving Follow
- `Follow(Actor)` - Sent when following a remote user
- `Follow(Public)` - Sent when subscribing to a relay (object is `https://www.w3.org/ns/activitystreams#Public`)
- `Undo(Follow)` - Sent when unfollowing a remote user or unsubscribing from a relay
- `Create(Note)` - Delivered to all followers when posting (includes `inReplyTo` for replies)
- `Update(Note)` - Delivered to all followers when editing
- `Delete(Note)` - Delivered to all followers when deleting
- `Like` - Sent when pressing 'l' on a remote post (TUI)
- `Undo(Like)` - Sent when unliking a previously liked remote post

## Object Types

- `Note` - Primary content type for posts
- `Tombstone` - Received in Delete activities

## Actor Types

- `Person` - User accounts

## Collections

- `/users/:username` - Actor profile
- `/users/:username/inbox` - Actor inbox (POST)
- `/users/:username/outbox` - Actor outbox (GET, paginated)
- `/users/:username/followers` - Followers (OrderedCollection)
- `/users/:username/following` - Following (OrderedCollection)
- `/inbox` - Shared inbox (POST, used by relays)
- `/notes/:id` - Individual note objects

## Discovery

- `/.well-known/webfinger` - WebFinger endpoint (JRD format)
- `/.well-known/nodeinfo` - NodeInfo discovery
- `/nodeinfo/2.0` - NodeInfo 2.0 endpoint

## HTTP Signatures

- Algorithm: `rsa-sha256`
- Signed headers: `(request-target)`, `host`, `date`, `digest`
- Key format: RSA 2048-bit (PKIX/PKCS#8)
- All incoming activities require valid signatures
- Relay-forwarded content: signature verified against the relay's key (signer may differ from activity actor)

## Content

- Outgoing posts use `mediaType: text/html`
- Markdown links are converted to HTML anchor tags
- Hashtags are parsed and included in the `tag` array with type `Hashtag`
- Hashtag HTML format: `<a href="..." class="hashtag" rel="tag">#<span>tag</span></a>`
- Mentions (@username@domain) are parsed and included in the `tag` array with type `Mention`
- Mention HTML format: `<span class="h-card"><a href="..." class="u-url mention">@<span>username</span></a></span>`
- Mentioned actors are added to the `cc` field for delivery
- JSON-LD context includes `Hashtag: as:Hashtag` when hashtags are present
- Incoming content stored as-is in activity JSON
- Incoming mentions are extracted from the `tag` array and stored in `note_mentions` table

## Replies and Threading

- Replies include the `inReplyTo` field pointing to the parent note's URI
- When replying to a remote user, the parent author's inbox is added to the `cc` list
- Replies are stored with their `in_reply_to_uri` in the database for thread reconstruction
- Reply counts are denormalized and recursively updated (includes all nested sub-replies)
- Duplicate detection prevents counting federated copies of local posts twice
- TUI: Press `r` on a post to reply, press `Enter` to view thread, press `l` to like/unlike
- Web: Single post pages show parent context and replies section
- Full thread depth supported with nested reply navigation

## Relay Support

Stegodon supports ActivityPub relays for discovering content beyond direct follows. Relays aggregate and forward posts from across the Fediverse.

### Supported Relay Types

**FediBuzz-style relays** (e.g., relay.fedi.buzz):
- Hashtag-based subscriptions (e.g., `https://relay.fedi.buzz/tag/music`)
- Content wrapped in `Announce` activities
- Multiple tag subscriptions from same relay domain supported

**YUKIMOCHI Activity-Relay** (e.g., relay.toot.yukimochi.jp):
- Full firehose relay
- Raw `Create` activities forwarded directly
- Uses shared inbox (`/inbox`) for delivery
- Follow object must be `https://www.w3.org/ns/activitystreams#Public`

### Relay Management (Admin Only)

Access the relay panel from the admin menu:

| Key | Action |
|-----|--------|
| `a` | Add new relay subscription |
| `d` | Delete/unsubscribe from relay |
| `p` | Pause/resume relay (toggle) |
| `r` | Retry failed subscription |
| `x` | Delete all relay content from timeline |

### Relay States

- **pending** - Follow request sent, waiting for Accept
- **active** - Relay accepted, receiving content
- **paused** - Subscription active but content not saved (logged only)
- **failed** - Subscription failed (can retry)

### Signature Verification for Relays

When a relay forwards content, the HTTP signature is from the relay, not the original author. Stegodon:
1. Extracts the `keyId` from the Signature header
2. Verifies the signature against the relay's public key
3. Identifies relay-forwarded content when signer differs from activity actor
4. Marks such activities with `from_relay=true` in the database

## Notifications

Stegodon includes a real-time notifications system accessible via the TUI (press `n` key). Notifications are generated for the following events:

### Notification Types

- **Like** - When another user (local or remote) likes your post
- **Follow** - When another user (local or remote) follows you
- **Mention** - When you are mentioned in a post (`@username` or `@username@domain`)
- **Reply** - When another user replies to your post

### Notification Behavior

- Notifications appear in real-time with a badge count in the header (e.g., `🦣 username [3]`)
- Badge updates every 30 seconds even when not viewing the notifications screen
- Press `n` to view notifications, `Enter` to acknowledge and delete individual notifications
- Press `a` to delete all notifications at once
- Notifications use an inbox-zero pattern (deleted on acknowledgment, not marked as read)

## Notable Behaviors

- All incoming Follow requests are auto-accepted
- Remote actors are cached for 24 hours
- Delivery queue uses exponential backoff (10 seconds to 24 hours)
- Create activities accepted from: followed accounts, relay subscriptions, or replies to local posts
- Paused relays: content is logged but not stored
- Rate limiting: 5 requests/second for ActivityPub endpoints
- Maximum activity body size: 1MB

## Not Yet Implemented

- `Announce` (boost/reblog) sending
- Direct messages
- Media attachments
- ActivityPub C2S (Client-to-Server)
- Object integrity proofs (FEP-8b32)
- Account migrations
