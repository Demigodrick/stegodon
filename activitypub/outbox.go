package activitypub

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

// SendActivity sends an activity to a remote inbox.
// This is the production wrapper that uses the default HTTP client.
func SendActivity(activity any, inboxURI string, localAccount *domain.Account, conf *util.AppConfig) error {
	return SendActivityWithDeps(activity, inboxURI, localAccount, conf, defaultHTTPClient)
}

// SendActivityWithDeps sends an activity to a remote inbox.
// This version accepts dependencies for testing.
func SendActivityWithDeps(activity any, inboxURI string, localAccount *domain.Account, conf *util.AppConfig, client HTTPClient) error {
	// Marshal activity to JSON
	activityJSON, err := json.Marshal(activity)
	if err != nil {
		return fmt.Errorf("failed to marshal activity: %w", err)
	}

	// Calculate digest for HTTP signature
	hash := sha256.Sum256(activityJSON)
	digest := "SHA-256=" + base64.StdEncoding.EncodeToString(hash[:])

	// Create HTTP request
	req, err := http.NewRequest("POST", inboxURI, bytes.NewReader(activityJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/activity+json")
	req.Header.Set("Accept", "application/activity+json")
	req.Header.Set("User-Agent", "stegodon/1.0 ActivityPub")
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("Digest", digest)

	// Parse private key for signing
	privateKey, err := ParsePrivateKey(localAccount.WebPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Sign request
	keyID := fmt.Sprintf("https://%s/users/%s#main-key", conf.Conf.SslDomain, localAccount.Username)
	if err := SignRequest(req, privateKey, keyID); err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("remote server returned status: %d", resp.StatusCode)
	}

	log.Printf("Outbox: Sent %T to %s (status: %d)", activity, inboxURI, resp.StatusCode)
	return nil
}

// SendAccept sends an Accept activity in response to a Follow.
// This is the production wrapper that uses the default HTTP client.
func SendAccept(localAccount *domain.Account, remoteActor *domain.RemoteAccount, followID string, conf *util.AppConfig) error {
	return SendAcceptWithDeps(localAccount, remoteActor, followID, conf, defaultHTTPClient)
}

// SendAcceptWithDeps sends an Accept activity in response to a Follow.
// This version accepts dependencies for testing.
func SendAcceptWithDeps(localAccount *domain.Account, remoteActor *domain.RemoteAccount, followID string, conf *util.AppConfig, client HTTPClient) error {
	acceptID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)

	accept := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       acceptID,
		"type":     "Accept",
		"actor":    actorURI,
		"object": map[string]any{
			"id":     followID,
			"type":   "Follow",
			"actor":  remoteActor.ActorURI,
			"object": actorURI,
		},
	}

	return SendActivityWithDeps(accept, remoteActor.InboxURI, localAccount, conf, client)
}

// SendCreate sends a Create activity for a new note.
// This is the production wrapper that uses the default database.
func SendCreate(note *domain.Note, localAccount *domain.Account, conf *util.AppConfig) error {
	return SendCreateWithDeps(note, localAccount, conf, NewDBWrapper())
}

// SendCreateWithDeps sends a Create activity for a new note.
// This version accepts dependencies for testing.
func SendCreateWithDeps(note *domain.Note, localAccount *domain.Account, conf *util.AppConfig, database Database) error {
	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)
	noteURI := fmt.Sprintf("https://%s/notes/%s", conf.Conf.SslDomain, note.Id.String())
	createID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())

	// Convert Markdown links to HTML for ActivityPub content
	contentHTML := util.MarkdownLinksToHTML(note.Message)

	create := map[string]any{
		"@context":  "https://www.w3.org/ns/activitystreams",
		"id":        createID,
		"type":      "Create",
		"actor":     actorURI,
		"published": note.CreatedAt.Format(time.RFC3339),
		"to": []string{
			"https://www.w3.org/ns/activitystreams#Public",
		},
		"cc": []string{
			fmt.Sprintf("https://%s/users/%s/followers", conf.Conf.SslDomain, localAccount.Username),
		},
		"object": map[string]any{
			"id":           noteURI,
			"type":         "Note",
			"attributedTo": actorURI,
			"content":      contentHTML,
			"mediaType":    "text/html",
			"published":    note.CreatedAt.Format(time.RFC3339),
			"to": []string{
				"https://www.w3.org/ns/activitystreams#Public",
			},
			"cc": []string{
				fmt.Sprintf("https://%s/users/%s/followers", conf.Conf.SslDomain, localAccount.Username),
			},
		},
	}

	// Get all followers and queue delivery to their inboxes
	err, followers := database.ReadFollowersByAccountId(localAccount.Id)
	if err != nil {
		log.Printf("Outbox: Failed to get followers: %v", err)
		return nil // Don't fail if we can't get followers
	}

	if followers == nil || len(*followers) == 0 {
		log.Printf("Outbox: No followers to deliver to")
		return nil
	}

	// Queue delivery to each follower's inbox
	for _, follower := range *followers {
		// AccountId is the follower (remote actor we need to deliver to)
		err, remoteActor := database.ReadRemoteAccountById(follower.AccountId)
		if err != nil {
			log.Printf("Outbox: Failed to get remote actor %s: %v", follower.AccountId, err)
			continue
		}

		// Queue for delivery
		queueItem := &domain.DeliveryQueueItem{
			Id:           uuid.New(),
			InboxURI:     remoteActor.InboxURI,
			ActivityJSON: mustMarshal(create),
			Attempts:     0,
			NextRetryAt:  time.Now(),
			CreatedAt:    time.Now(),
		}

		if err := database.EnqueueDelivery(queueItem); err != nil {
			log.Printf("Outbox: Failed to queue delivery to %s: %v", remoteActor.InboxURI, err)
		}
	}

	log.Printf("Outbox: Queued Create activity for note %s to %d followers", note.Id, len(*followers))
	return nil
}

// SendUpdate sends an Update activity to all followers when a note is edited.
// This is the production wrapper that uses the default database.
func SendUpdate(note *domain.Note, localAccount *domain.Account, conf *util.AppConfig) error {
	return SendUpdateWithDeps(note, localAccount, conf, NewDBWrapper())
}

// SendUpdateWithDeps sends an Update activity to all followers when a note is edited.
// This version accepts dependencies for testing.
func SendUpdateWithDeps(note *domain.Note, localAccount *domain.Account, conf *util.AppConfig, database Database) error {
	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)
	noteURI := fmt.Sprintf("https://%s/notes/%s", conf.Conf.SslDomain, note.Id.String())
	updateID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())

	// Use EditedAt if available, otherwise use CreatedAt
	updatedTime := note.CreatedAt
	if note.EditedAt != nil {
		updatedTime = *note.EditedAt
	}

	// Convert Markdown links to HTML for ActivityPub content
	contentHTML := util.MarkdownLinksToHTML(note.Message)

	update := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       updateID,
		"type":     "Update",
		"actor":    actorURI,
		"to": []string{
			"https://www.w3.org/ns/activitystreams#Public",
		},
		"cc": []string{
			fmt.Sprintf("https://%s/users/%s/followers", conf.Conf.SslDomain, localAccount.Username),
		},
		"object": map[string]any{
			"id":           noteURI,
			"type":         "Note",
			"attributedTo": actorURI,
			"content":      contentHTML,
			"mediaType":    "text/html",
			"published":    note.CreatedAt.Format(time.RFC3339),
			"updated":      updatedTime.Format(time.RFC3339),
			"to": []string{
				"https://www.w3.org/ns/activitystreams#Public",
			},
			"cc": []string{
				fmt.Sprintf("https://%s/users/%s/followers", conf.Conf.SslDomain, localAccount.Username),
			},
		},
	}

	// Get all followers and queue delivery to their inboxes
	err, followers := database.ReadFollowersByAccountId(localAccount.Id)
	if err != nil {
		log.Printf("Outbox: Failed to get followers for Update: %v", err)
		return nil
	}

	if followers == nil || len(*followers) == 0 {
		log.Printf("Outbox: No followers to deliver Update to")
		return nil
	}

	// Queue delivery to each follower's inbox
	for _, follower := range *followers {
		err, remoteActor := database.ReadRemoteAccountById(follower.AccountId)
		if err != nil {
			log.Printf("Outbox: Failed to get remote actor %s: %v", follower.AccountId, err)
			continue
		}

		queueItem := &domain.DeliveryQueueItem{
			Id:           uuid.New(),
			InboxURI:     remoteActor.InboxURI,
			ActivityJSON: mustMarshal(update),
			Attempts:     0,
			NextRetryAt:  time.Now(),
			CreatedAt:    time.Now(),
		}

		if err := database.EnqueueDelivery(queueItem); err != nil {
			log.Printf("Outbox: Failed to queue Update delivery to %s: %v", remoteActor.InboxURI, err)
		}
	}

	log.Printf("Outbox: Queued Update activity for note %s to %d followers", note.Id, len(*followers))
	return nil
}

// SendDelete sends a Delete activity to all followers when a note is deleted.
// This is the production wrapper that uses the default database.
func SendDelete(noteId uuid.UUID, localAccount *domain.Account, conf *util.AppConfig) error {
	return SendDeleteWithDeps(noteId, localAccount, conf, NewDBWrapper())
}

// SendDeleteWithDeps sends a Delete activity to all followers when a note is deleted.
// This version accepts dependencies for testing.
func SendDeleteWithDeps(noteId uuid.UUID, localAccount *domain.Account, conf *util.AppConfig, database Database) error {
	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)
	noteURI := fmt.Sprintf("https://%s/notes/%s", conf.Conf.SslDomain, noteId.String())
	deleteID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())

	deleteActivity := map[string]any{
		"@context":  "https://www.w3.org/ns/activitystreams",
		"id":        deleteID,
		"type":      "Delete",
		"actor":     actorURI,
		"published": time.Now().Format(time.RFC3339),
		"to": []string{
			"https://www.w3.org/ns/activitystreams#Public",
		},
		"cc": []string{
			fmt.Sprintf("https://%s/users/%s/followers", conf.Conf.SslDomain, localAccount.Username),
		},
		"object": noteURI,
	}

	// Get all followers and queue delivery to their inboxes
	err, followers := database.ReadFollowersByAccountId(localAccount.Id)
	if err != nil {
		log.Printf("Outbox: Failed to get followers for Delete: %v", err)
		return nil
	}

	if followers == nil || len(*followers) == 0 {
		log.Printf("Outbox: No followers to deliver Delete to")
		return nil
	}

	// Queue delivery to each follower's inbox
	for _, follower := range *followers {
		err, remoteActor := database.ReadRemoteAccountById(follower.AccountId)
		if err != nil {
			log.Printf("Outbox: Failed to get remote actor %s: %v", follower.AccountId, err)
			continue
		}

		queueItem := &domain.DeliveryQueueItem{
			Id:           uuid.New(),
			InboxURI:     remoteActor.InboxURI,
			ActivityJSON: mustMarshal(deleteActivity),
			Attempts:     0,
			NextRetryAt:  time.Now(),
			CreatedAt:    time.Now(),
		}

		if err := database.EnqueueDelivery(queueItem); err != nil {
			log.Printf("Outbox: Failed to queue Delete delivery to %s: %v", remoteActor.InboxURI, err)
		}
	}

	log.Printf("Outbox: Queued Delete activity for note %s to %d followers", noteId, len(*followers))
	return nil
}

// SendFollow sends a Follow activity to a remote actor.
// This is the production wrapper that uses the default HTTP client and database.
func SendFollow(localAccount *domain.Account, remoteActorURI string, conf *util.AppConfig) error {
	return SendFollowWithDeps(localAccount, remoteActorURI, conf, defaultHTTPClient, NewDBWrapper())
}

// SendFollowWithDeps sends a Follow activity to a remote actor.
// This version accepts dependencies for testing.
func SendFollowWithDeps(localAccount *domain.Account, remoteActorURI string, conf *util.AppConfig, client HTTPClient, database Database) error {
	// Fetch remote actor
	remoteActor, err := GetOrFetchActorWithDeps(remoteActorURI, client, database)
	if err != nil {
		return fmt.Errorf("failed to fetch remote actor: %w", err)
	}

	// Check if trying to follow yourself
	if remoteActor.Domain == conf.Conf.SslDomain && remoteActor.Username == localAccount.Username {
		log.Printf("SendFollow: User %s attempted to follow themselves", localAccount.Username)
		return fmt.Errorf("self-follow not allowed on stegodon for now")
	}

	// Check if already following this user
	err, existingFollow := database.ReadFollowByAccountIds(localAccount.Id, remoteActor.Id)
	if err != sql.ErrNoRows && err != nil {
		// Database error (not "not found")
		log.Printf("SendFollow: Error checking existing follow: %v", err)
		return fmt.Errorf("failed to check existing follow: %w", err)
	}
	if existingFollow != nil {
		// Follow relationship already exists - check if accepted
		if existingFollow.Accepted {
			// Already following and accepted
			log.Printf("SendFollow: User %s is already following %s@%s (accepted)", localAccount.Username, remoteActor.Username, remoteActor.Domain)
			return fmt.Errorf("already following %s@%s", remoteActor.Username, remoteActor.Domain)
		} else {
			// Follow exists but pending acceptance
			log.Printf("SendFollow: User %s has pending follow request to %s@%s", localAccount.Username, remoteActor.Username, remoteActor.Domain)
			return fmt.Errorf("follow pending %s@%s", remoteActor.Username, remoteActor.Domain)
		}
	}

	// Not following yet, create the follow
	followID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)

	follow := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       followID,
		"type":     "Follow",
		"actor":    actorURI,
		"object":   remoteActorURI,
	}

	// Store follow relationship as pending
	followRecord := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       localAccount.Id,
		TargetAccountId: remoteActor.Id,
		URI:             followID,
		Accepted:        false, // Pending until Accept received
		CreatedAt:       time.Now(),
	}

	if err := database.CreateFollow(followRecord); err != nil {
		return fmt.Errorf("failed to store follow: %w", err)
	}

	// Send Follow activity
	return SendActivityWithDeps(follow, remoteActor.InboxURI, localAccount, conf, client)
}

// SendUndo sends an Undo activity for a Follow (i.e., unfollow).
// This is the production wrapper that uses the default HTTP client.
func SendUndo(localAccount *domain.Account, follow *domain.Follow, remoteActor *domain.RemoteAccount, conf *util.AppConfig) error {
	return SendUndoWithDeps(localAccount, follow, remoteActor, conf, defaultHTTPClient)
}

// SendUndoWithDeps sends an Undo activity for a Follow (i.e., unfollow).
// This version accepts dependencies for testing.
func SendUndoWithDeps(localAccount *domain.Account, follow *domain.Follow, remoteActor *domain.RemoteAccount, conf *util.AppConfig, client HTTPClient) error {
	undoID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)

	// Create Undo activity with embedded Follow object
	undo := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       undoID,
		"type":     "Undo",
		"actor":    actorURI,
		"object": map[string]any{
			"id":     follow.URI,
			"type":   "Follow",
			"actor":  actorURI,
			"object": remoteActor.ActorURI,
		},
	}

	log.Printf("Outbox: Sending Undo (unfollow) from %s to %s@%s", localAccount.Username, remoteActor.Username, remoteActor.Domain)
	return SendActivityWithDeps(undo, remoteActor.InboxURI, localAccount, conf, client)
}

// mustMarshal marshals v to JSON, panicking on error
func mustMarshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal: %v", err))
	}
	return string(b)
}
