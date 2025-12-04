package activitypub

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

// defaultHTTPClient is the default HTTP client for production use
var defaultHTTPClient HTTPClient = NewDefaultHTTPClient(10 * time.Second)

// ActorResponse represents the JSON structure of an ActivityPub actor
type ActorResponse struct {
	Context           any    `json:"@context"`
	ID                string `json:"id"`
	Type              string `json:"type"`
	PreferredUsername string `json:"preferredUsername"`
	Name              string `json:"name"`
	Summary           string `json:"summary"`
	Inbox             string `json:"inbox"`
	Outbox            string `json:"outbox"`
	Icon              struct {
		Type      string `json:"type"`
		MediaType string `json:"mediaType"`
		URL       string `json:"url"`
	} `json:"icon"`
	PublicKey struct {
		ID           string `json:"id"`
		Owner        string `json:"owner"`
		PublicKeyPem string `json:"publicKeyPem"`
	} `json:"publicKey"`
}

// FetchRemoteActor fetches an actor from a remote server and stores in cache.
// This is the production wrapper that uses the default HTTP client and database.
func FetchRemoteActor(actorURI string) (*domain.RemoteAccount, error) {
	return FetchRemoteActorWithDeps(actorURI, defaultHTTPClient, NewDBWrapper())
}

// FetchRemoteActorWithDeps fetches an actor from a remote server and stores in cache.
// This version accepts dependencies for testing.
func FetchRemoteActorWithDeps(actorURI string, client HTTPClient, database Database) (*domain.RemoteAccount, error) {
	// Create HTTP request with Accept: application/activity+json
	req, err := http.NewRequest("GET", actorURI, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/activity+json")
	req.Header.Set("User-Agent", "stegodon/1.0 ActivityPub")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("actor fetch failed with status: %d", resp.StatusCode)
	}

	// Parse ActivityPub actor JSON
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var actor ActorResponse
	if err := json.Unmarshal(body, &actor); err != nil {
		return nil, fmt.Errorf("failed to parse actor JSON: %w", err)
	}

	// Validate required fields
	if actor.ID == "" || actor.Inbox == "" || actor.PublicKey.PublicKeyPem == "" {
		return nil, fmt.Errorf("actor missing required fields")
	}

	// Extract domain from actor URI
	domainName, err := extractDomain(actor.ID)
	if err != nil {
		return nil, err
	}

	// Check if remote account already exists
	err, existingAcc := database.ReadRemoteAccountByURI(actor.ID)

	var remoteAcc *domain.RemoteAccount
	if err == nil && existingAcc != nil {
		// Account exists - reuse the ID and update
		remoteAcc = &domain.RemoteAccount{
			Id:            existingAcc.Id, // Reuse existing ID
			Username:      actor.PreferredUsername,
			Domain:        domainName,
			ActorURI:      actor.ID,
			DisplayName:   actor.Name,
			Summary:       actor.Summary,
			InboxURI:      actor.Inbox,
			OutboxURI:     actor.Outbox,
			PublicKeyPem:  actor.PublicKey.PublicKeyPem,
			AvatarURL:     actor.Icon.URL,
			LastFetchedAt: time.Now(),
		}
		err = database.UpdateRemoteAccount(remoteAcc)
		if err != nil {
			return nil, fmt.Errorf("failed to update remote account: %w", err)
		}
	} else {
		// Account doesn't exist - create new
		remoteAcc = &domain.RemoteAccount{
			Id:            uuid.New(),
			Username:      actor.PreferredUsername,
			Domain:        domainName,
			ActorURI:      actor.ID,
			DisplayName:   actor.Name,
			Summary:       actor.Summary,
			InboxURI:      actor.Inbox,
			OutboxURI:     actor.Outbox,
			PublicKeyPem:  actor.PublicKey.PublicKeyPem,
			AvatarURL:     actor.Icon.URL,
			LastFetchedAt: time.Now(),
		}
		err = database.CreateRemoteAccount(remoteAcc)
		if err != nil {
			return nil, fmt.Errorf("failed to create remote account: %w", err)
		}
	}

	return remoteAcc, nil
}

// GetOrFetchActor returns actor from cache or fetches if not cached/stale.
// This is the production wrapper that uses the default HTTP client and database.
func GetOrFetchActor(actorURI string) (*domain.RemoteAccount, error) {
	return GetOrFetchActorWithDeps(actorURI, defaultHTTPClient, NewDBWrapper())
}

// GetOrFetchActorWithDeps returns actor from cache or fetches if not cached/stale.
// This version accepts dependencies for testing.
func GetOrFetchActorWithDeps(actorURI string, client HTTPClient, database Database) (*domain.RemoteAccount, error) {
	// Check cache first
	err, cached := database.ReadRemoteAccountByURI(actorURI)
	if err == nil && cached != nil {
		// Check if cache is fresh (< 24 hours)
		if time.Since(cached.LastFetchedAt) < 24*time.Hour {
			return cached, nil
		}
	}

	// Fetch fresh data
	return FetchRemoteActorWithDeps(actorURI, client, database)
}

// extractDomain extracts the domain from an actor URI
// Example: "https://mastodon.social/users/alice" -> "mastodon.social"
func extractDomain(actorURI string) (string, error) {
	parsed, err := url.Parse(actorURI)
	if err != nil {
		return "", fmt.Errorf("invalid actor URI: %w", err)
	}

	return parsed.Host, nil
}

// extractUsername extracts username from various URI formats
// Examples:
// - "https://example.com/users/alice" -> "alice"
// - "https://example.com/@alice" -> "alice"
func extractUsername(uri string) string {
	parts := strings.Split(uri, "/")
	if len(parts) > 0 {
		username := parts[len(parts)-1]
		// Remove @ prefix if present
		return strings.TrimPrefix(username, "@")
	}
	return ""
}
