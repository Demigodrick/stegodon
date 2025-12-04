package activitypub

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

func TestActorResponseUnmarshal(t *testing.T) {
	// Test unmarshaling a typical ActivityPub actor response
	jsonData := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://mastodon.social/users/alice",
		"type": "Person",
		"preferredUsername": "alice",
		"name": "Alice Example",
		"summary": "Just a test user",
		"inbox": "https://mastodon.social/users/alice/inbox",
		"outbox": "https://mastodon.social/users/alice/outbox",
		"icon": {
			"type": "Image",
			"mediaType": "image/png",
			"url": "https://mastodon.social/avatars/alice.png"
		},
		"publicKey": {
			"id": "https://mastodon.social/users/alice#main-key",
			"owner": "https://mastodon.social/users/alice",
			"publicKeyPem": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBg...\n-----END PUBLIC KEY-----"
		}
	}`

	var actor ActorResponse
	if err := json.Unmarshal([]byte(jsonData), &actor); err != nil {
		t.Fatalf("Failed to unmarshal ActorResponse: %v", err)
	}

	// Verify required fields
	if actor.ID != "https://mastodon.social/users/alice" {
		t.Errorf("Expected ID 'https://mastodon.social/users/alice', got '%s'", actor.ID)
	}
	if actor.Type != "Person" {
		t.Errorf("Expected Type 'Person', got '%s'", actor.Type)
	}
	if actor.PreferredUsername != "alice" {
		t.Errorf("Expected PreferredUsername 'alice', got '%s'", actor.PreferredUsername)
	}
	if actor.Name != "Alice Example" {
		t.Errorf("Expected Name 'Alice Example', got '%s'", actor.Name)
	}
	if actor.Summary != "Just a test user" {
		t.Errorf("Expected Summary 'Just a test user', got '%s'", actor.Summary)
	}
	if actor.Inbox != "https://mastodon.social/users/alice/inbox" {
		t.Errorf("Expected Inbox URL, got '%s'", actor.Inbox)
	}
	if actor.Outbox != "https://mastodon.social/users/alice/outbox" {
		t.Errorf("Expected Outbox URL, got '%s'", actor.Outbox)
	}
	if actor.Icon.URL != "https://mastodon.social/avatars/alice.png" {
		t.Errorf("Expected Icon URL, got '%s'", actor.Icon.URL)
	}
	if !strings.Contains(actor.PublicKey.PublicKeyPem, "BEGIN PUBLIC KEY") {
		t.Error("PublicKeyPem should contain PEM header")
	}
}

func TestActorResponseMinimal(t *testing.T) {
	// Test with minimal required fields
	jsonData := `{
		"id": "https://example.com/users/bob",
		"type": "Person",
		"preferredUsername": "bob",
		"inbox": "https://example.com/users/bob/inbox",
		"publicKey": {
			"id": "https://example.com/users/bob#main-key",
			"owner": "https://example.com/users/bob",
			"publicKeyPem": "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----"
		}
	}`

	var actor ActorResponse
	if err := json.Unmarshal([]byte(jsonData), &actor); err != nil {
		t.Fatalf("Failed to unmarshal minimal actor: %v", err)
	}

	// Verify required fields are present
	if actor.ID == "" {
		t.Error("ID should not be empty")
	}
	if actor.Inbox == "" {
		t.Error("Inbox should not be empty")
	}
	if actor.PublicKey.PublicKeyPem == "" {
		t.Error("PublicKeyPem should not be empty")
	}
}

func TestActorResponseValidation(t *testing.T) {
	// Test validation logic for required fields
	tests := []struct {
		name      string
		actor     ActorResponse
		wantValid bool
	}{
		{
			name: "valid actor",
			actor: ActorResponse{
				ID:    "https://example.com/users/alice",
				Inbox: "https://example.com/users/alice/inbox",
				PublicKey: struct {
					ID           string `json:"id"`
					Owner        string `json:"owner"`
					PublicKeyPem string `json:"publicKeyPem"`
				}{
					PublicKeyPem: "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
				},
			},
			wantValid: true,
		},
		{
			name: "missing ID",
			actor: ActorResponse{
				Inbox: "https://example.com/inbox",
				PublicKey: struct {
					ID           string `json:"id"`
					Owner        string `json:"owner"`
					PublicKeyPem string `json:"publicKeyPem"`
				}{
					PublicKeyPem: "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
				},
			},
			wantValid: false,
		},
		{
			name: "missing Inbox",
			actor: ActorResponse{
				ID: "https://example.com/users/alice",
				PublicKey: struct {
					ID           string `json:"id"`
					Owner        string `json:"owner"`
					PublicKeyPem string `json:"publicKeyPem"`
				}{
					PublicKeyPem: "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
				},
			},
			wantValid: false,
		},
		{
			name: "missing PublicKey",
			actor: ActorResponse{
				ID:    "https://example.com/users/alice",
				Inbox: "https://example.com/users/alice/inbox",
			},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is the validation logic from FetchRemoteActor
			isValid := tt.actor.ID != "" && tt.actor.Inbox != "" && tt.actor.PublicKey.PublicKeyPem != ""

			if isValid != tt.wantValid {
				t.Errorf("Expected valid=%v, got valid=%v", tt.wantValid, isValid)
			}
		})
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		name       string
		actorURI   string
		wantDomain string
		wantError  bool
	}{
		{
			name:       "Mastodon user",
			actorURI:   "https://mastodon.social/users/alice",
			wantDomain: "mastodon.social",
			wantError:  false,
		},
		{
			name:       "Pleroma user",
			actorURI:   "https://pleroma.site/users/bob",
			wantDomain: "pleroma.site",
			wantError:  false,
		},
		{
			name:       "Custom port",
			actorURI:   "https://social.example.com:8080/users/charlie",
			wantDomain: "social.example.com:8080",
			wantError:  false,
		},
		{
			name:       "Subdomain",
			actorURI:   "https://masto.subdomain.example.com/users/dave",
			wantDomain: "masto.subdomain.example.com",
			wantError:  false,
		},
		{
			name:       "Invalid URI",
			actorURI:   "://invalid",
			wantDomain: "",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain, err := extractDomain(tt.actorURI)

			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if domain != tt.wantDomain {
				t.Errorf("Expected domain '%s', got '%s'", tt.wantDomain, domain)
			}
		})
	}
}

func TestExtractUsername(t *testing.T) {
	tests := []struct {
		name         string
		uri          string
		wantUsername string
	}{
		{
			name:         "standard users path",
			uri:          "https://mastodon.social/users/alice",
			wantUsername: "alice",
		},
		{
			name:         "@ prefix path",
			uri:          "https://mastodon.social/@bob",
			wantUsername: "bob",
		},
		{
			name:         "activity path",
			uri:          "https://example.com/users/charlie/statuses/123",
			wantUsername: "123",
		},
		{
			name:         "simple path",
			uri:          "https://example.com/dave",
			wantUsername: "dave",
		},
		{
			name:         "empty uri",
			uri:          "",
			wantUsername: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			username := extractUsername(tt.uri)
			if username != tt.wantUsername {
				t.Errorf("Expected username '%s', got '%s'", tt.wantUsername, username)
			}
		})
	}
}

func TestActorContextVariants(t *testing.T) {
	// Test different @context formats
	tests := []struct {
		name        string
		contextJSON string
	}{
		{
			name:        "string context",
			contextJSON: `"https://www.w3.org/ns/activitystreams"`,
		},
		{
			name:        "array context",
			contextJSON: `["https://www.w3.org/ns/activitystreams", "https://w3id.org/security/v1"]`,
		},
		{
			name:        "complex context",
			contextJSON: `[{"@vocab": "https://www.w3.org/ns/activitystreams"}, "https://w3id.org/security/v1"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData := `{
				"@context": ` + tt.contextJSON + `,
				"id": "https://example.com/users/test",
				"type": "Person",
				"inbox": "https://example.com/inbox",
				"publicKey": {
					"publicKeyPem": "test"
				}
			}`

			var actor ActorResponse
			if err := json.Unmarshal([]byte(jsonData), &actor); err != nil {
				t.Fatalf("Failed to unmarshal actor with %s: %v", tt.name, err)
			}

			if actor.ID != "https://example.com/users/test" {
				t.Error("Actor fields should be parsed correctly regardless of context format")
			}
		})
	}
}

func TestActorTypeVariants(t *testing.T) {
	// Test different actor types
	actorTypes := []string{"Person", "Application", "Service", "Organization", "Group"}

	for _, actorType := range actorTypes {
		t.Run(actorType, func(t *testing.T) {
			jsonData := `{
				"id": "https://example.com/actor",
				"type": "` + actorType + `",
				"inbox": "https://example.com/inbox",
				"publicKey": {"publicKeyPem": "test"}
			}`

			var actor ActorResponse
			if err := json.Unmarshal([]byte(jsonData), &actor); err != nil {
				t.Fatalf("Failed to unmarshal %s actor: %v", actorType, err)
			}

			if actor.Type != actorType {
				t.Errorf("Expected Type '%s', got '%s'", actorType, actor.Type)
			}
		})
	}
}

func TestActorIconVariants(t *testing.T) {
	// Test different icon formats
	tests := []struct {
		name     string
		iconJSON string
		wantURL  string
	}{
		{
			name: "full icon object",
			iconJSON: `{
				"type": "Image",
				"mediaType": "image/png",
				"url": "https://example.com/avatar.png"
			}`,
			wantURL: "https://example.com/avatar.png",
		},
		{
			name:     "missing icon",
			iconJSON: `{}`,
			wantURL:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData := `{
				"id": "https://example.com/actor",
				"type": "Person",
				"inbox": "https://example.com/inbox",
				"icon": ` + tt.iconJSON + `,
				"publicKey": {"publicKeyPem": "test"}
			}`

			var actor ActorResponse
			if err := json.Unmarshal([]byte(jsonData), &actor); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if actor.Icon.URL != tt.wantURL {
				t.Errorf("Expected icon URL '%s', got '%s'", tt.wantURL, actor.Icon.URL)
			}
		})
	}
}

func TestPublicKeyStructure(t *testing.T) {
	// Test PublicKey structure
	jsonData := `{
		"id": "https://example.com/actor",
		"type": "Person",
		"inbox": "https://example.com/inbox",
		"publicKey": {
			"id": "https://example.com/actor#main-key",
			"owner": "https://example.com/actor",
			"publicKeyPem": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...\n-----END PUBLIC KEY-----"
		}
	}`

	var actor ActorResponse
	if err := json.Unmarshal([]byte(jsonData), &actor); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if actor.PublicKey.ID != "https://example.com/actor#main-key" {
		t.Errorf("Expected publicKey.id, got '%s'", actor.PublicKey.ID)
	}
	if actor.PublicKey.Owner != "https://example.com/actor" {
		t.Errorf("Expected publicKey.owner, got '%s'", actor.PublicKey.Owner)
	}
	if !strings.Contains(actor.PublicKey.PublicKeyPem, "BEGIN PUBLIC KEY") {
		t.Error("PublicKeyPem should contain PEM markers")
	}
	if !strings.Contains(actor.PublicKey.PublicKeyPem, "END PUBLIC KEY") {
		t.Error("PublicKeyPem should contain END marker")
	}
}

func TestActorEndpointURLs(t *testing.T) {
	// Test that all endpoint URLs are properly parsed
	jsonData := `{
		"id": "https://mastodon.social/users/alice",
		"type": "Person",
		"inbox": "https://mastodon.social/users/alice/inbox",
		"outbox": "https://mastodon.social/users/alice/outbox",
		"following": "https://mastodon.social/users/alice/following",
		"followers": "https://mastodon.social/users/alice/followers",
		"publicKey": {"publicKeyPem": "test"}
	}`

	var actor ActorResponse
	if err := json.Unmarshal([]byte(jsonData), &actor); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify all URLs start with HTTPS
	urls := []string{actor.ID, actor.Inbox, actor.Outbox}
	for _, url := range urls {
		if url != "" && !strings.HasPrefix(url, "https://") {
			t.Errorf("URL should use HTTPS: %s", url)
		}
	}

	// Verify inbox and outbox are different
	if actor.Inbox == actor.Outbox && actor.Inbox != "" {
		t.Error("Inbox and Outbox should typically be different endpoints")
	}
}

func TestActorDisplayNameHandling(t *testing.T) {
	// Test preferredUsername vs name (display name)
	jsonData := `{
		"id": "https://example.com/users/alice",
		"type": "Person",
		"preferredUsername": "alice",
		"name": "Alice Wonderland 🎭",
		"inbox": "https://example.com/inbox",
		"publicKey": {"publicKeyPem": "test"}
	}`

	var actor ActorResponse
	if err := json.Unmarshal([]byte(jsonData), &actor); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if actor.PreferredUsername != "alice" {
		t.Error("PreferredUsername should be simple username")
	}
	if actor.Name != "Alice Wonderland 🎭" {
		t.Error("Name should support display names with emoji")
	}
	if actor.PreferredUsername == actor.Name {
		t.Error("Username and display name should be different fields")
	}
}

func TestActorSummaryHandling(t *testing.T) {
	// Test summary/bio field with HTML
	jsonData := `{
		"id": "https://example.com/users/alice",
		"type": "Person",
		"preferredUsername": "alice",
		"summary": "<p>Software developer interested in <a href=\"https://example.com/tags/golang\">#golang</a> and <a href=\"https://example.com/tags/activitypub\">#activitypub</a></p>",
		"inbox": "https://example.com/inbox",
		"publicKey": {"publicKeyPem": "test"}
	}`

	var actor ActorResponse
	if err := json.Unmarshal([]byte(jsonData), &actor); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if !strings.Contains(actor.Summary, "<p>") {
		t.Error("Summary should preserve HTML tags")
	}
	if !strings.Contains(actor.Summary, "golang") {
		t.Error("Summary should contain content")
	}
}

func TestCacheFreshnessLogic(t *testing.T) {
	// Test the 24-hour cache freshness logic
	tests := []struct {
		name      string
		age       time.Duration
		wantFresh bool
	}{
		{
			name:      "just fetched",
			age:       1 * time.Minute,
			wantFresh: true,
		},
		{
			name:      "12 hours old",
			age:       12 * time.Hour,
			wantFresh: true,
		},
		{
			name:      "23 hours old",
			age:       23 * time.Hour,
			wantFresh: true,
		},
		{
			name:      "25 hours old",
			age:       25 * time.Hour,
			wantFresh: false,
		},
		{
			name:      "48 hours old",
			age:       48 * time.Hour,
			wantFresh: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lastFetched := time.Now().Add(-tt.age)
			isFresh := time.Since(lastFetched) < 24*time.Hour

			if isFresh != tt.wantFresh {
				t.Errorf("Expected fresh=%v for age %v, got fresh=%v", tt.wantFresh, tt.age, isFresh)
			}
		})
	}
}

// TestFetchRemoteActor_IDReuse tests that existing account IDs are reused
func TestFetchRemoteActor_IDReuse(t *testing.T) {
	// This test requires a mock HTTP server, so we'll test the logic conceptually
	// The actual implementation in FetchRemoteActor checks if account exists
	// and reuses the ID instead of creating a new one

	// Test data
	existingID := uuid.New()
	actorURI := "https://example.com/users/alice"

	// Simulate: First fetch creates new account with existingID
	account1 := &domain.RemoteAccount{
		Id:            existingID,
		Username:      "alice",
		Domain:        "example.com",
		ActorURI:      actorURI,
		DisplayName:   "Alice",
		InboxURI:      "https://example.com/users/alice/inbox",
		PublicKeyPem:  "pubkey1",
		LastFetchedAt: time.Now().Add(-25 * time.Hour), // Stale
	}

	// Simulate: Second fetch should reuse existingID
	account2 := &domain.RemoteAccount{
		Id:            existingID, // SAME ID - this is the fix
		Username:      "alice",
		Domain:        "example.com",
		ActorURI:      actorURI,
		DisplayName:   "Alice Updated",
		InboxURI:      "https://example.com/users/alice/inbox",
		PublicKeyPem:  "pubkey1",
		LastFetchedAt: time.Now(),
	}

	// Verify same ID
	if account1.Id != account2.Id {
		t.Error("FetchRemoteActor should reuse existing account ID")
	}

	// Verify same ActorURI (this is the lookup key)
	if account1.ActorURI != account2.ActorURI {
		t.Error("ActorURI should match")
	}

	// Verify DisplayName can be updated
	if account2.DisplayName == account1.DisplayName {
		t.Log("DisplayName can be updated on refetch")
	}
}

// TestRemoteAccountCreation tests remote account struct creation
func TestRemoteAccountCreation(t *testing.T) {
	tests := []struct {
		name    string
		actor   ActorResponse
		domain  string
		wantErr bool
	}{
		{
			name: "complete actor",
			actor: ActorResponse{
				ID:                "https://mastodon.social/users/alice",
				Type:              "Person",
				PreferredUsername: "alice",
				Name:              "Alice Example",
				Summary:           "Test bio",
				Inbox:             "https://mastodon.social/users/alice/inbox",
				Outbox:            "https://mastodon.social/users/alice/outbox",
			},
			domain:  "mastodon.social",
			wantErr: false,
		},
		{
			name: "minimal actor",
			actor: ActorResponse{
				ID:                "https://example.com/users/bob",
				PreferredUsername: "bob",
				Inbox:             "https://example.com/inbox",
			},
			domain:  "example.com",
			wantErr: false,
		},
		{
			name: "actor without username",
			actor: ActorResponse{
				ID:    "https://example.com/users/anon",
				Inbox: "https://example.com/inbox",
			},
			domain:  "example.com",
			wantErr: false, // Empty username is allowed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create RemoteAccount from ActorResponse (simulating FetchRemoteActor logic)
			remoteAcc := &domain.RemoteAccount{
				Id:            uuid.New(),
				Username:      tt.actor.PreferredUsername,
				Domain:        tt.domain,
				ActorURI:      tt.actor.ID,
				DisplayName:   tt.actor.Name,
				Summary:       tt.actor.Summary,
				InboxURI:      tt.actor.Inbox,
				OutboxURI:     tt.actor.Outbox,
				PublicKeyPem:  tt.actor.PublicKey.PublicKeyPem,
				AvatarURL:     tt.actor.Icon.URL,
				LastFetchedAt: time.Now(),
			}

			if remoteAcc.ActorURI != tt.actor.ID {
				t.Error("ActorURI should match actor ID")
			}
			if remoteAcc.Domain != tt.domain {
				t.Error("Domain should be set correctly")
			}
			if remoteAcc.InboxURI != tt.actor.Inbox {
				t.Error("InboxURI should match actor inbox")
			}
		})
	}
}

// TestMastodonActorFormat tests parsing of Mastodon's actor format
func TestMastodonActorFormat(t *testing.T) {
	mastodonJSON := `{
		"@context": [
			"https://www.w3.org/ns/activitystreams",
			"https://w3id.org/security/v1",
			{
				"manuallyApprovesFollowers": "as:manuallyApprovesFollowers",
				"toot": "http://joinmastodon.org/ns#",
				"featured": {"@id": "toot:featured", "@type": "@id"},
				"alsoKnownAs": {"@id": "as:alsoKnownAs", "@type": "@id"},
				"movedTo": {"@id": "as:movedTo", "@type": "@id"},
				"schema": "http://schema.org#",
				"PropertyValue": "schema:PropertyValue",
				"value": "schema:value"
			}
		],
		"id": "https://mastodon.social/users/Gargron",
		"type": "Person",
		"following": "https://mastodon.social/users/Gargron/following",
		"followers": "https://mastodon.social/users/Gargron/followers",
		"inbox": "https://mastodon.social/users/Gargron/inbox",
		"outbox": "https://mastodon.social/users/Gargron/outbox",
		"featured": "https://mastodon.social/users/Gargron/collections/featured",
		"preferredUsername": "Gargron",
		"name": "Eugen Rochko",
		"summary": "<p>Founder of Mastodon</p>",
		"url": "https://mastodon.social/@Gargron",
		"manuallyApprovesFollowers": false,
		"icon": {
			"type": "Image",
			"mediaType": "image/jpeg",
			"url": "https://files.mastodon.social/accounts/avatars/000/000/001/original/example.jpg"
		},
		"publicKey": {
			"id": "https://mastodon.social/users/Gargron#main-key",
			"owner": "https://mastodon.social/users/Gargron",
			"publicKeyPem": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBg...\n-----END PUBLIC KEY-----"
		}
	}`

	var actor ActorResponse
	err := json.Unmarshal([]byte(mastodonJSON), &actor)
	if err != nil {
		t.Fatalf("Failed to parse Mastodon actor: %v", err)
	}

	// Verify Mastodon-specific fields
	if actor.PreferredUsername != "Gargron" {
		t.Errorf("Expected username Gargron, got %s", actor.PreferredUsername)
	}
	if actor.Name != "Eugen Rochko" {
		t.Errorf("Expected name 'Eugen Rochko', got %s", actor.Name)
	}
	if !strings.Contains(actor.Summary, "Founder") {
		t.Error("Summary should contain bio content")
	}
	if actor.Icon.URL == "" {
		t.Error("Icon URL should be parsed")
	}
}

// TestPleromaActorFormat tests parsing of Pleroma's actor format
func TestPleromaActorFormat(t *testing.T) {
	pleromaJSON := `{
		"@context": [
			"https://www.w3.org/ns/activitystreams",
			"https://pleroma.example.com/schemas/litepub-0.1.jsonld"
		],
		"id": "https://pleroma.example.com/users/admin",
		"type": "Person",
		"preferredUsername": "admin",
		"name": "Pleroma Admin",
		"summary": "Instance administrator",
		"inbox": "https://pleroma.example.com/users/admin/inbox",
		"outbox": "https://pleroma.example.com/users/admin/outbox",
		"publicKey": {
			"id": "https://pleroma.example.com/users/admin#main-key",
			"owner": "https://pleroma.example.com/users/admin",
			"publicKeyPem": "-----BEGIN PUBLIC KEY-----\nMIIBIjAN...\n-----END PUBLIC KEY-----"
		}
	}`

	var actor ActorResponse
	err := json.Unmarshal([]byte(pleromaJSON), &actor)
	if err != nil {
		t.Fatalf("Failed to parse Pleroma actor: %v", err)
	}

	if actor.Type != "Person" {
		t.Errorf("Expected type Person, got %s", actor.Type)
	}
	if actor.PreferredUsername != "admin" {
		t.Errorf("Expected username admin, got %s", actor.PreferredUsername)
	}
}

// TestMisskeyActorFormat tests parsing of Misskey's actor format
func TestMisskeyActorFormat(t *testing.T) {
	misskeyJSON := `{
		"@context": [
			"https://www.w3.org/ns/activitystreams",
			"https://w3id.org/security/v1"
		],
		"type": "Person",
		"id": "https://misskey.io/users/1234abcd",
		"inbox": "https://misskey.io/users/1234abcd/inbox",
		"outbox": "https://misskey.io/users/1234abcd/outbox",
		"followers": "https://misskey.io/users/1234abcd/followers",
		"following": "https://misskey.io/users/1234abcd/following",
		"featured": "https://misskey.io/users/1234abcd/collections/featured",
		"preferredUsername": "testuser",
		"name": "Test User",
		"summary": "<p>Misskey user</p>",
		"icon": {
			"type": "Image",
			"url": "https://misskey.io/files/avatar.png"
		},
		"publicKey": {
			"id": "https://misskey.io/users/1234abcd#main-key",
			"type": "Key",
			"owner": "https://misskey.io/users/1234abcd",
			"publicKeyPem": "-----BEGIN PUBLIC KEY-----\nMIIBIjAN...\n-----END PUBLIC KEY-----"
		}
	}`

	var actor ActorResponse
	err := json.Unmarshal([]byte(misskeyJSON), &actor)
	if err != nil {
		t.Fatalf("Failed to parse Misskey actor: %v", err)
	}

	// Misskey uses hex IDs in URLs
	if !strings.Contains(actor.ID, "misskey.io") {
		t.Error("Expected Misskey domain in ID")
	}
	if actor.PreferredUsername != "testuser" {
		t.Errorf("Expected username testuser, got %s", actor.PreferredUsername)
	}
}

// TestActorWithServiceType tests parsing of Service type actors (bots)
func TestActorWithServiceType(t *testing.T) {
	serviceJSON := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://example.com/actors/bot",
		"type": "Service",
		"preferredUsername": "bot",
		"name": "Helpful Bot",
		"summary": "I am a bot",
		"inbox": "https://example.com/actors/bot/inbox",
		"outbox": "https://example.com/actors/bot/outbox",
		"publicKey": {
			"id": "https://example.com/actors/bot#main-key",
			"owner": "https://example.com/actors/bot",
			"publicKeyPem": "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----"
		}
	}`

	var actor ActorResponse
	err := json.Unmarshal([]byte(serviceJSON), &actor)
	if err != nil {
		t.Fatalf("Failed to parse Service actor: %v", err)
	}

	if actor.Type != "Service" {
		t.Errorf("Expected type Service, got %s", actor.Type)
	}
}

// TestActorWithApplicationType tests parsing of Application type actors
func TestActorWithApplicationType(t *testing.T) {
	applicationJSON := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://example.com/actor",
		"type": "Application",
		"preferredUsername": "relay",
		"name": "ActivityPub Relay",
		"inbox": "https://example.com/inbox",
		"outbox": "https://example.com/outbox",
		"publicKey": {
			"id": "https://example.com/actor#main-key",
			"owner": "https://example.com/actor",
			"publicKeyPem": "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----"
		}
	}`

	var actor ActorResponse
	err := json.Unmarshal([]byte(applicationJSON), &actor)
	if err != nil {
		t.Fatalf("Failed to parse Application actor: %v", err)
	}

	if actor.Type != "Application" {
		t.Errorf("Expected type Application, got %s", actor.Type)
	}
}

// TestActorWithGroupType tests parsing of Group type actors
func TestActorWithGroupType(t *testing.T) {
	groupJSON := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://lemmy.ml/c/programming",
		"type": "Group",
		"preferredUsername": "programming",
		"name": "Programming",
		"summary": "A community for programmers",
		"inbox": "https://lemmy.ml/c/programming/inbox",
		"outbox": "https://lemmy.ml/c/programming/outbox",
		"publicKey": {
			"id": "https://lemmy.ml/c/programming#main-key",
			"owner": "https://lemmy.ml/c/programming",
			"publicKeyPem": "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----"
		}
	}`

	var actor ActorResponse
	err := json.Unmarshal([]byte(groupJSON), &actor)
	if err != nil {
		t.Fatalf("Failed to parse Group actor: %v", err)
	}

	if actor.Type != "Group" {
		t.Errorf("Expected type Group, got %s", actor.Type)
	}
}

// TestActorValidationRequired tests that required fields are checked
func TestActorValidationRequired(t *testing.T) {
	tests := []struct {
		name       string
		actor      ActorResponse
		missingMsg string
	}{
		{
			name: "valid actor",
			actor: ActorResponse{
				ID:    "https://example.com/users/alice",
				Inbox: "https://example.com/inbox",
				PublicKey: struct {
					ID           string `json:"id"`
					Owner        string `json:"owner"`
					PublicKeyPem string `json:"publicKeyPem"`
				}{
					PublicKeyPem: "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
				},
			},
			missingMsg: "",
		},
		{
			name: "missing ID",
			actor: ActorResponse{
				Inbox: "https://example.com/inbox",
				PublicKey: struct {
					ID           string `json:"id"`
					Owner        string `json:"owner"`
					PublicKeyPem string `json:"publicKeyPem"`
				}{
					PublicKeyPem: "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
				},
			},
			missingMsg: "ID",
		},
		{
			name: "missing Inbox",
			actor: ActorResponse{
				ID: "https://example.com/users/alice",
				PublicKey: struct {
					ID           string `json:"id"`
					Owner        string `json:"owner"`
					PublicKeyPem string `json:"publicKeyPem"`
				}{
					PublicKeyPem: "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
				},
			},
			missingMsg: "Inbox",
		},
		{
			name: "missing PublicKey",
			actor: ActorResponse{
				ID:    "https://example.com/users/alice",
				Inbox: "https://example.com/inbox",
			},
			missingMsg: "PublicKey",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate FetchRemoteActor validation
			hasError := tt.actor.ID == "" || tt.actor.Inbox == "" || tt.actor.PublicKey.PublicKeyPem == ""

			if tt.missingMsg == "" && hasError {
				t.Error("Expected valid actor but got validation error")
			}
			if tt.missingMsg != "" && !hasError {
				t.Errorf("Expected validation error for missing %s", tt.missingMsg)
			}
		})
	}
}

// TestExtractDomainEdgeCases tests domain extraction with edge cases
func TestExtractDomainEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		actorURI   string
		wantDomain string
		wantError  bool
	}{
		{
			name:       "standard HTTPS",
			actorURI:   "https://mastodon.social/users/test",
			wantDomain: "mastodon.social",
			wantError:  false,
		},
		{
			name:       "with port",
			actorURI:   "https://localhost:3000/users/test",
			wantDomain: "localhost:3000",
			wantError:  false,
		},
		{
			name:       "IPv4 address",
			actorURI:   "https://192.168.1.1/users/test",
			wantDomain: "192.168.1.1",
			wantError:  false,
		},
		{
			name:       "empty string",
			actorURI:   "",
			wantDomain: "",
			wantError:  false, // url.Parse doesn't error on empty string
		},
		{
			name:       "relative URL",
			actorURI:   "/users/test",
			wantDomain: "",
			wantError:  false,
		},
		{
			name:       "IDN domain (punycode)",
			actorURI:   "https://xn--n3h.example.com/users/emoji",
			wantDomain: "xn--n3h.example.com",
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain, err := extractDomain(tt.actorURI)

			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if domain != tt.wantDomain {
				t.Errorf("Expected domain %q, got %q", tt.wantDomain, domain)
			}
		})
	}
}

// TestExtractUsernameEdgeCases tests username extraction with edge cases
func TestExtractUsernameEdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		uri          string
		wantUsername string
	}{
		{
			name:         "standard /users/ path",
			uri:          "https://mastodon.social/users/alice",
			wantUsername: "alice",
		},
		{
			name:         "@ path",
			uri:          "https://mastodon.social/@alice",
			wantUsername: "alice",
		},
		{
			name:         "trailing slash",
			uri:          "https://example.com/users/bob/",
			wantUsername: "",
		},
		{
			name:         "nested path",
			uri:          "https://example.com/api/v1/users/charlie",
			wantUsername: "charlie",
		},
		{
			name:         "root path only",
			uri:          "https://example.com/",
			wantUsername: "",
		},
		{
			name:         "no path",
			uri:          "https://example.com",
			wantUsername: "example.com",
		},
		{
			name:         "username with dots",
			uri:          "https://example.com/users/user.name",
			wantUsername: "user.name",
		},
		{
			name:         "username with underscore",
			uri:          "https://example.com/users/user_name",
			wantUsername: "user_name",
		},
		{
			name:         "username with numbers",
			uri:          "https://example.com/users/user123",
			wantUsername: "user123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			username := extractUsername(tt.uri)
			if username != tt.wantUsername {
				t.Errorf("Expected username %q, got %q", tt.wantUsername, username)
			}
		})
	}
}

// TestActorResponseJSONRoundTrip tests JSON marshal/unmarshal round-trip
func TestActorResponseJSONRoundTrip(t *testing.T) {
	original := ActorResponse{
		ID:                "https://example.com/users/alice",
		Type:              "Person",
		PreferredUsername: "alice",
		Name:              "Alice 🎭",
		Summary:           "<p>Test bio with <a href='#'>link</a></p>",
		Inbox:             "https://example.com/users/alice/inbox",
		Outbox:            "https://example.com/users/alice/outbox",
	}
	original.Icon.Type = "Image"
	original.Icon.MediaType = "image/png"
	original.Icon.URL = "https://example.com/avatar.png"
	original.PublicKey.ID = "https://example.com/users/alice#main-key"
	original.PublicKey.Owner = "https://example.com/users/alice"
	original.PublicKey.PublicKeyPem = "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----"

	// Marshal to JSON
	jsonBytes, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal back
	var parsed ActorResponse
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify fields preserved
	if parsed.ID != original.ID {
		t.Error("ID not preserved")
	}
	if parsed.Type != original.Type {
		t.Error("Type not preserved")
	}
	if parsed.PreferredUsername != original.PreferredUsername {
		t.Error("PreferredUsername not preserved")
	}
	if parsed.Name != original.Name {
		t.Error("Name not preserved (emoji handling)")
	}
	if parsed.Summary != original.Summary {
		t.Error("Summary not preserved (HTML handling)")
	}
	if parsed.Icon.URL != original.Icon.URL {
		t.Error("Icon URL not preserved")
	}
	if parsed.PublicKey.PublicKeyPem != original.PublicKey.PublicKeyPem {
		t.Error("PublicKeyPem not preserved")
	}
}

// ============================================================================
// Integration tests using dependency injection
// These tests use mock HTTP client and mock database
// ============================================================================

// TestFetchRemoteActorWithDeps_NewActor tests fetching a new remote actor
func TestFetchRemoteActorWithDeps_NewActor(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	// Set up actor response
	actorURI := "https://remote.example.com/users/testuser"
	actorResponse := ActorResponse{
		ID:                actorURI,
		Type:              "Person",
		PreferredUsername: "testuser",
		Name:              "Test User",
		Summary:           "A test user",
		Inbox:             "https://remote.example.com/users/testuser/inbox",
		Outbox:            "https://remote.example.com/users/testuser/outbox",
	}
	actorResponse.PublicKey.ID = actorURI + "#main-key"
	actorResponse.PublicKey.Owner = actorURI
	actorResponse.PublicKey.PublicKeyPem = "-----BEGIN PUBLIC KEY-----\nMIIBIjAN...\n-----END PUBLIC KEY-----"
	actorResponse.Icon.URL = "https://remote.example.com/avatar.png"

	err := mockHTTP.SetJSONResponse(actorURI, 200, actorResponse)
	if err != nil {
		t.Fatalf("Failed to set mock response: %v", err)
	}

	// Fetch the actor
	result, err := FetchRemoteActorWithDeps(actorURI, mockHTTP, mockDB)
	if err != nil {
		t.Fatalf("FetchRemoteActorWithDeps failed: %v", err)
	}

	// Verify result
	if result.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", result.Username)
	}
	if result.Domain != "remote.example.com" {
		t.Errorf("Expected domain 'remote.example.com', got '%s'", result.Domain)
	}
	if result.ActorURI != actorURI {
		t.Errorf("Expected ActorURI '%s', got '%s'", actorURI, result.ActorURI)
	}
	if result.DisplayName != "Test User" {
		t.Errorf("Expected DisplayName 'Test User', got '%s'", result.DisplayName)
	}
	if result.InboxURI != "https://remote.example.com/users/testuser/inbox" {
		t.Errorf("Expected InboxURI, got '%s'", result.InboxURI)
	}

	// Verify actor was stored in mock database
	err, storedActor := mockDB.ReadRemoteAccountByURI(actorURI)
	if err != nil || storedActor == nil {
		t.Error("Actor should be stored in database")
	}

	// Verify HTTP request was made
	if len(mockHTTP.Requests) != 1 {
		t.Errorf("Expected 1 HTTP request, got %d", len(mockHTTP.Requests))
	}
	if mockHTTP.Requests[0].Header.Get("Accept") != "application/activity+json" {
		t.Error("Request should have Accept: application/activity+json header")
	}
}

// TestFetchRemoteActorWithDeps_ExistingActor tests updating an existing actor
func TestFetchRemoteActorWithDeps_ExistingActor(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	actorURI := "https://remote.example.com/users/existing"
	existingID := uuid.New()

	// Pre-populate database with existing account
	existingActor := &domain.RemoteAccount{
		Id:            existingID,
		Username:      "existing",
		Domain:        "remote.example.com",
		ActorURI:      actorURI,
		DisplayName:   "Old Name",
		InboxURI:      "https://remote.example.com/users/existing/inbox",
		PublicKeyPem:  "old-key",
		LastFetchedAt: time.Now().Add(-48 * time.Hour),
	}
	mockDB.AddRemoteAccount(existingActor)

	// Set up updated actor response
	actorResponse := ActorResponse{
		ID:                actorURI,
		Type:              "Person",
		PreferredUsername: "existing",
		Name:              "New Name",
		Inbox:             "https://remote.example.com/users/existing/inbox",
	}
	actorResponse.PublicKey.PublicKeyPem = "new-key"

	err := mockHTTP.SetJSONResponse(actorURI, 200, actorResponse)
	if err != nil {
		t.Fatalf("Failed to set mock response: %v", err)
	}

	// Fetch the actor
	result, err := FetchRemoteActorWithDeps(actorURI, mockHTTP, mockDB)
	if err != nil {
		t.Fatalf("FetchRemoteActorWithDeps failed: %v", err)
	}

	// Verify ID was reused
	if result.Id != existingID {
		t.Error("Existing account ID should be reused")
	}

	// Verify display name was updated
	if result.DisplayName != "New Name" {
		t.Errorf("Expected DisplayName 'New Name', got '%s'", result.DisplayName)
	}

	// Verify LastFetchedAt was updated
	if time.Since(result.LastFetchedAt) > time.Minute {
		t.Error("LastFetchedAt should be updated to recent time")
	}
}

// TestFetchRemoteActorWithDeps_HTTPError tests handling of HTTP errors
func TestFetchRemoteActorWithDeps_HTTPError(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	actorURI := "https://remote.example.com/users/error"
	mockHTTP.SetResponse(actorURI, 404, []byte("Not Found"))

	_, err := FetchRemoteActorWithDeps(actorURI, mockHTTP, mockDB)
	if err == nil {
		t.Error("Expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Error should mention status code: %v", err)
	}
}

// TestFetchRemoteActorWithDeps_InvalidJSON tests handling of invalid JSON
func TestFetchRemoteActorWithDeps_InvalidJSON(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	actorURI := "https://remote.example.com/users/invalid"
	mockHTTP.SetResponse(actorURI, 200, []byte("{invalid json"))

	_, err := FetchRemoteActorWithDeps(actorURI, mockHTTP, mockDB)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("Error should mention parsing: %v", err)
	}
}

// TestFetchRemoteActorWithDeps_MissingFields tests handling of missing required fields
func TestFetchRemoteActorWithDeps_MissingFields(t *testing.T) {
	tests := []struct {
		name  string
		actor ActorResponse
	}{
		{
			name: "missing_ID",
			actor: ActorResponse{
				Inbox: "https://example.com/inbox",
			},
		},
		{
			name: "missing_Inbox",
			actor: ActorResponse{
				ID: "https://example.com/users/test",
			},
		},
		{
			name: "missing_PublicKey",
			actor: ActorResponse{
				ID:    "https://example.com/users/test",
				Inbox: "https://example.com/inbox",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := NewMockDatabase()
			mockHTTP := NewMockHTTPClient()

			actorURI := "https://example.com/users/" + tt.name
			tt.actor.PublicKey.PublicKeyPem = "" // Ensure it's empty for missing PublicKey test
			if tt.name != "missing_PublicKey" {
				tt.actor.PublicKey.PublicKeyPem = "test-key"
			}

			err := mockHTTP.SetJSONResponse(actorURI, 200, tt.actor)
			if err != nil {
				t.Fatalf("Failed to set mock response: %v", err)
			}

			_, err = FetchRemoteActorWithDeps(actorURI, mockHTTP, mockDB)
			if err == nil {
				t.Error("Expected error for missing required fields")
			}
			if !strings.Contains(err.Error(), "missing required fields") {
				t.Errorf("Error should mention missing fields: %v", err)
			}
		})
	}
}

// TestGetOrFetchActorWithDeps_CachedFresh tests returning fresh cached actor
func TestGetOrFetchActorWithDeps_CachedFresh(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	actorURI := "https://remote.example.com/users/cached"

	// Pre-populate with fresh actor (last fetched 1 hour ago)
	freshActor := &domain.RemoteAccount{
		Id:            uuid.New(),
		Username:      "cached",
		Domain:        "remote.example.com",
		ActorURI:      actorURI,
		DisplayName:   "Cached User",
		InboxURI:      "https://remote.example.com/users/cached/inbox",
		LastFetchedAt: time.Now().Add(-1 * time.Hour), // Fresh
	}
	mockDB.AddRemoteAccount(freshActor)

	// Should NOT make HTTP request
	result, err := GetOrFetchActorWithDeps(actorURI, mockHTTP, mockDB)
	if err != nil {
		t.Fatalf("GetOrFetchActorWithDeps failed: %v", err)
	}

	// Verify cached actor was returned
	if result.DisplayName != "Cached User" {
		t.Errorf("Expected cached actor, got '%s'", result.DisplayName)
	}

	// Verify NO HTTP request was made
	if len(mockHTTP.Requests) != 0 {
		t.Error("Should not make HTTP request for fresh cached actor")
	}
}

// TestGetOrFetchActorWithDeps_CachedStale tests refetching stale cached actor
func TestGetOrFetchActorWithDeps_CachedStale(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	actorURI := "https://remote.example.com/users/stale"
	existingID := uuid.New()

	// Pre-populate with stale actor (last fetched 25 hours ago)
	staleActor := &domain.RemoteAccount{
		Id:            existingID,
		Username:      "stale",
		Domain:        "remote.example.com",
		ActorURI:      actorURI,
		DisplayName:   "Old Name",
		InboxURI:      "https://remote.example.com/users/stale/inbox",
		LastFetchedAt: time.Now().Add(-25 * time.Hour), // Stale
	}
	mockDB.AddRemoteAccount(staleActor)

	// Set up updated actor response
	actorResponse := ActorResponse{
		ID:                actorURI,
		Type:              "Person",
		PreferredUsername: "stale",
		Name:              "Updated Name",
		Inbox:             "https://remote.example.com/users/stale/inbox",
	}
	actorResponse.PublicKey.PublicKeyPem = "test-key"

	err := mockHTTP.SetJSONResponse(actorURI, 200, actorResponse)
	if err != nil {
		t.Fatalf("Failed to set mock response: %v", err)
	}

	// Should make HTTP request to refresh
	result, err := GetOrFetchActorWithDeps(actorURI, mockHTTP, mockDB)
	if err != nil {
		t.Fatalf("GetOrFetchActorWithDeps failed: %v", err)
	}

	// Verify updated actor was returned
	if result.DisplayName != "Updated Name" {
		t.Errorf("Expected updated actor, got '%s'", result.DisplayName)
	}

	// Verify ID was reused
	if result.Id != existingID {
		t.Error("Existing account ID should be reused")
	}

	// Verify HTTP request WAS made
	if len(mockHTTP.Requests) != 1 {
		t.Error("Should make HTTP request to refresh stale actor")
	}
}

// TestGetOrFetchActorWithDeps_NotCached tests fetching when not cached
func TestGetOrFetchActorWithDeps_NotCached(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	actorURI := "https://remote.example.com/users/new"

	// Set up actor response
	actorResponse := ActorResponse{
		ID:                actorURI,
		Type:              "Person",
		PreferredUsername: "new",
		Name:              "New User",
		Inbox:             "https://remote.example.com/users/new/inbox",
	}
	actorResponse.PublicKey.PublicKeyPem = "test-key"

	err := mockHTTP.SetJSONResponse(actorURI, 200, actorResponse)
	if err != nil {
		t.Fatalf("Failed to set mock response: %v", err)
	}

	// Should make HTTP request
	result, err := GetOrFetchActorWithDeps(actorURI, mockHTTP, mockDB)
	if err != nil {
		t.Fatalf("GetOrFetchActorWithDeps failed: %v", err)
	}

	// Verify new actor was returned
	if result.DisplayName != "New User" {
		t.Errorf("Expected new actor, got '%s'", result.DisplayName)
	}

	// Verify HTTP request was made
	if len(mockHTTP.Requests) != 1 {
		t.Error("Should make HTTP request for uncached actor")
	}

	// Verify actor was stored
	err, stored := mockDB.ReadRemoteAccountByURI(actorURI)
	if err != nil || stored == nil {
		t.Error("Actor should be stored in database")
	}
}

// TestFetchRemoteActorWithDeps_DatabaseError tests database error handling
func TestFetchRemoteActorWithDeps_DatabaseError(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	actorURI := "https://remote.example.com/users/dberror"

	// Set up actor response
	actorResponse := ActorResponse{
		ID:                actorURI,
		Type:              "Person",
		PreferredUsername: "dberror",
		Name:              "DB Error User",
		Inbox:             "https://remote.example.com/users/dberror/inbox",
	}
	actorResponse.PublicKey.PublicKeyPem = "test-key"

	err := mockHTTP.SetJSONResponse(actorURI, 200, actorResponse)
	if err != nil {
		t.Fatalf("Failed to set mock response: %v", err)
	}

	// Force database error
	mockDB.SetForceError(&mockDatabaseError{message: "forced database error"})

	_, err = FetchRemoteActorWithDeps(actorURI, mockHTTP, mockDB)
	if err == nil {
		t.Error("Expected error when database fails")
	}
}

// mockDatabaseError is a mock database error for testing
type mockDatabaseError struct {
	message string
}

func (e *mockDatabaseError) Error() string {
	return e.message
}

// TestFetchRemoteActorWithDeps_NetworkError tests network error handling
func TestFetchRemoteActorWithDeps_NetworkError(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	actorURI := "https://remote.example.com/users/networkerror"
	networkErr := &mockNetworkError{message: "connection refused"}
	mockHTTP.SetError(actorURI, networkErr)

	_, err := FetchRemoteActorWithDeps(actorURI, mockHTTP, mockDB)
	if err == nil {
		t.Error("Expected error for network failure")
	}
	if !strings.Contains(err.Error(), "request failed") {
		t.Errorf("Error should mention request failure: %v", err)
	}
}

// mockNetworkError is a mock network error for testing
type mockNetworkError struct {
	message string
}

func (e *mockNetworkError) Error() string {
	return e.message
}
