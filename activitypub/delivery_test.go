package activitypub

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

// TestDeliverActivityWithDeps_Success tests successful activity delivery
func TestDeliverActivityWithDeps_Success(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	// Generate keypair for signing
	keypair, _ := GenerateTestKeyPair()

	// Create local account
	localAccount := &domain.Account{
		Id:            uuid.New(),
		Username:      "alice",
		WebPrivateKey: keypair.PrivatePEM,
		WebPublicKey:  keypair.PublicPEM,
	}
	mockDB.AddAccount(localAccount)

	// Setup mock HTTP to accept the delivery
	mockHTTP.SetResponse("https://remote.example.com/inbox", 202, []byte(""))

	deps := &DeliveryDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	activityJSON := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://local.example.com/activities/123",
		"type": "Create",
		"actor": "https://local.example.com/users/alice",
		"object": {
			"id": "https://local.example.com/notes/456",
			"type": "Note",
			"content": "Hello world"
		}
	}`

	item := &domain.DeliveryQueueItem{
		Id:           uuid.New(),
		InboxURI:     "https://remote.example.com/inbox",
		ActivityJSON: activityJSON,
		Attempts:     0,
		NextRetryAt:  time.Now(),
		CreatedAt:    time.Now(),
	}

	err := deliverActivityWithDeps(item, conf, deps)
	if err != nil {
		t.Errorf("Expected successful delivery, got error: %v", err)
	}

	// Verify HTTP request was made
	if len(mockHTTP.Requests) == 0 {
		t.Fatal("Expected HTTP request to be made")
	}

	req := mockHTTP.Requests[len(mockHTTP.Requests)-1]

	if req.Method != "POST" {
		t.Errorf("Expected POST method, got %s", req.Method)
	}

	if req.URL.String() != "https://remote.example.com/inbox" {
		t.Errorf("Expected URL https://remote.example.com/inbox, got %s", req.URL.String())
	}

	// Verify headers
	if req.Header.Get("Content-Type") != "application/activity+json" {
		t.Errorf("Expected Content-Type application/activity+json, got %s", req.Header.Get("Content-Type"))
	}

	if req.Header.Get("Signature") == "" {
		t.Error("Expected Signature header to be set")
	}

	if req.Header.Get("Digest") == "" {
		t.Error("Expected Digest header to be set")
	}
}

// TestDeliverActivityWithDeps_InvalidJSON tests delivery with invalid JSON
func TestDeliverActivityWithDeps_InvalidJSON(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	deps := &DeliveryDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	item := &domain.DeliveryQueueItem{
		Id:           uuid.New(),
		InboxURI:     "https://remote.example.com/inbox",
		ActivityJSON: "invalid json",
		Attempts:     0,
		NextRetryAt:  time.Now(),
		CreatedAt:    time.Now(),
	}

	err := deliverActivityWithDeps(item, conf, deps)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "failed to parse activity JSON") {
		t.Errorf("Expected JSON parse error, got: %v", err)
	}
}

// TestDeliverActivityWithDeps_MissingActor tests delivery with missing actor field
func TestDeliverActivityWithDeps_MissingActor(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	deps := &DeliveryDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	activityJSON := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://local.example.com/activities/123",
		"type": "Create"
	}`

	item := &domain.DeliveryQueueItem{
		Id:           uuid.New(),
		InboxURI:     "https://remote.example.com/inbox",
		ActivityJSON: activityJSON,
		Attempts:     0,
		NextRetryAt:  time.Now(),
		CreatedAt:    time.Now(),
	}

	err := deliverActivityWithDeps(item, conf, deps)
	if err == nil {
		t.Error("Expected error for missing actor")
	}

	if !strings.Contains(err.Error(), "activity missing actor field") {
		t.Errorf("Expected missing actor error, got: %v", err)
	}
}

// TestDeliverActivityWithDeps_InvalidActorURI tests delivery with invalid actor URI
func TestDeliverActivityWithDeps_InvalidActorURI(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	deps := &DeliveryDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	activityJSON := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://local.example.com/activities/123",
		"type": "Create",
		"actor": "x"
	}`

	item := &domain.DeliveryQueueItem{
		Id:           uuid.New(),
		InboxURI:     "https://remote.example.com/inbox",
		ActivityJSON: activityJSON,
		Attempts:     0,
		NextRetryAt:  time.Now(),
		CreatedAt:    time.Now(),
	}

	err := deliverActivityWithDeps(item, conf, deps)
	if err == nil {
		t.Error("Expected error for invalid actor URI")
	}

	if !strings.Contains(err.Error(), "invalid actor URI") {
		t.Errorf("Expected invalid actor URI error, got: %v", err)
	}
}

// TestDeliverActivityWithDeps_AccountNotFound tests delivery when local account not found
func TestDeliverActivityWithDeps_AccountNotFound(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	// Don't add any account to the mock DB

	deps := &DeliveryDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	activityJSON := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://local.example.com/activities/123",
		"type": "Create",
		"actor": "https://local.example.com/users/alice"
	}`

	item := &domain.DeliveryQueueItem{
		Id:           uuid.New(),
		InboxURI:     "https://remote.example.com/inbox",
		ActivityJSON: activityJSON,
		Attempts:     0,
		NextRetryAt:  time.Now(),
		CreatedAt:    time.Now(),
	}

	err := deliverActivityWithDeps(item, conf, deps)
	if err == nil {
		t.Error("Expected error when account not found")
	}

	if !strings.Contains(err.Error(), "failed to get local account") {
		t.Errorf("Expected account not found error, got: %v", err)
	}
}

// TestDeliverActivityWithDeps_RemoteServerError tests delivery when remote server returns error
func TestDeliverActivityWithDeps_RemoteServerError(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	keypair, _ := GenerateTestKeyPair()

	localAccount := &domain.Account{
		Id:            uuid.New(),
		Username:      "alice",
		WebPrivateKey: keypair.PrivatePEM,
		WebPublicKey:  keypair.PublicPEM,
	}
	mockDB.AddAccount(localAccount)

	// Setup mock HTTP to return 500 error
	mockHTTP.SetResponse("https://remote.example.com/inbox", 500, []byte("Internal Server Error"))

	deps := &DeliveryDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	activityJSON := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://local.example.com/activities/123",
		"type": "Create",
		"actor": "https://local.example.com/users/alice"
	}`

	item := &domain.DeliveryQueueItem{
		Id:           uuid.New(),
		InboxURI:     "https://remote.example.com/inbox",
		ActivityJSON: activityJSON,
		Attempts:     0,
		NextRetryAt:  time.Now(),
		CreatedAt:    time.Now(),
	}

	err := deliverActivityWithDeps(item, conf, deps)
	if err == nil {
		t.Error("Expected error when remote server returns 500")
	}

	if !strings.Contains(err.Error(), "remote server returned status: 500") {
		t.Errorf("Expected remote server error, got: %v", err)
	}
}

// TestProcessDeliveryQueueWithDeps_EmptyQueue tests processing an empty queue
func TestProcessDeliveryQueueWithDeps_EmptyQueue(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	deps := &DeliveryDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	// Process empty queue - should not panic or error
	processDeliveryQueueWithDeps(conf, deps)

	// No HTTP requests should have been made
	if len(mockHTTP.Requests) != 0 {
		t.Error("Expected no HTTP requests for empty queue")
	}
}

// TestProcessDeliveryQueueWithDeps_SuccessfulDelivery tests successful queue processing
func TestProcessDeliveryQueueWithDeps_SuccessfulDelivery(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	keypair, _ := GenerateTestKeyPair()

	localAccount := &domain.Account{
		Id:            uuid.New(),
		Username:      "alice",
		WebPrivateKey: keypair.PrivatePEM,
		WebPublicKey:  keypair.PublicPEM,
	}
	mockDB.AddAccount(localAccount)

	// Setup mock HTTP to accept delivery
	mockHTTP.SetResponse("https://remote.example.com/inbox", 202, []byte(""))

	deps := &DeliveryDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	activityJSON := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://local.example.com/activities/123",
		"type": "Create",
		"actor": "https://local.example.com/users/alice"
	}`

	// Add item to delivery queue
	item := &domain.DeliveryQueueItem{
		Id:           uuid.New(),
		InboxURI:     "https://remote.example.com/inbox",
		ActivityJSON: activityJSON,
		Attempts:     0,
		NextRetryAt:  time.Now().Add(-1 * time.Minute), // Ready for delivery
		CreatedAt:    time.Now(),
	}
	mockDB.AddDeliveryQueueItem(item)

	// Process queue
	processDeliveryQueueWithDeps(conf, deps)

	// Verify item was removed from queue after successful delivery
	if len(mockDB.DeliveryQueue) != 0 {
		t.Errorf("Expected delivery queue to be empty after successful delivery, got %d items", len(mockDB.DeliveryQueue))
	}
}

// TestProcessDeliveryQueueWithDeps_FailedDeliveryRetry tests retry logic for failed deliveries
func TestProcessDeliveryQueueWithDeps_FailedDeliveryRetry(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	keypair, _ := GenerateTestKeyPair()

	localAccount := &domain.Account{
		Id:            uuid.New(),
		Username:      "alice",
		WebPrivateKey: keypair.PrivatePEM,
		WebPublicKey:  keypair.PublicPEM,
	}
	mockDB.AddAccount(localAccount)

	// Setup mock HTTP to return error
	mockHTTP.SetResponse("https://remote.example.com/inbox", 500, []byte("Error"))

	deps := &DeliveryDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	activityJSON := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://local.example.com/activities/123",
		"type": "Create",
		"actor": "https://local.example.com/users/alice"
	}`

	itemID := uuid.New()
	item := &domain.DeliveryQueueItem{
		Id:           itemID,
		InboxURI:     "https://remote.example.com/inbox",
		ActivityJSON: activityJSON,
		Attempts:     0,
		NextRetryAt:  time.Now().Add(-1 * time.Minute),
		CreatedAt:    time.Now(),
	}
	mockDB.AddDeliveryQueueItem(item)

	// Process queue
	processDeliveryQueueWithDeps(conf, deps)

	// Verify item is still in queue with incremented attempts
	if len(mockDB.DeliveryQueue) != 1 {
		t.Fatalf("Expected 1 item in queue after failed delivery, got %d", len(mockDB.DeliveryQueue))
	}

	updatedItem := mockDB.DeliveryQueue[itemID]
	if updatedItem.Attempts != 1 {
		t.Errorf("Expected 1 attempt after failed delivery, got %d", updatedItem.Attempts)
	}

	// NextRetryAt should be in the future (1 minute backoff for first failure)
	if !updatedItem.NextRetryAt.After(time.Now()) {
		t.Error("Expected NextRetryAt to be in the future after failed delivery")
	}
}

// TestProcessDeliveryQueueWithDeps_MaxRetriesExceeded tests giving up after max retries
func TestProcessDeliveryQueueWithDeps_MaxRetriesExceeded(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	keypair, _ := GenerateTestKeyPair()

	localAccount := &domain.Account{
		Id:            uuid.New(),
		Username:      "alice",
		WebPrivateKey: keypair.PrivatePEM,
		WebPublicKey:  keypair.PublicPEM,
	}
	mockDB.AddAccount(localAccount)

	// Setup mock HTTP to return error
	mockHTTP.SetResponse("https://remote.example.com/inbox", 500, []byte("Error"))

	deps := &DeliveryDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	activityJSON := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://local.example.com/activities/123",
		"type": "Create",
		"actor": "https://local.example.com/users/alice"
	}`

	// Item already at 9 attempts (next failure will be 10th and final)
	item := &domain.DeliveryQueueItem{
		Id:           uuid.New(),
		InboxURI:     "https://remote.example.com/inbox",
		ActivityJSON: activityJSON,
		Attempts:     9,
		NextRetryAt:  time.Now().Add(-1 * time.Minute),
		CreatedAt:    time.Now(),
	}
	mockDB.AddDeliveryQueueItem(item)

	// Process queue
	processDeliveryQueueWithDeps(conf, deps)

	// Verify item was removed from queue after max retries
	if len(mockDB.DeliveryQueue) != 0 {
		t.Errorf("Expected delivery queue to be empty after max retries exceeded, got %d items", len(mockDB.DeliveryQueue))
	}
}

// TestProcessDeliveryQueueWithDeps_DatabaseError tests handling of database errors
func TestProcessDeliveryQueueWithDeps_DatabaseError(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	// Force database error
	mockDB.SetForceError(errors.New("database error"))

	deps := &DeliveryDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	// Process queue - should handle error gracefully without panicking
	processDeliveryQueueWithDeps(conf, deps)

	// No HTTP requests should have been made
	if len(mockHTTP.Requests) != 0 {
		t.Error("Expected no HTTP requests when database error occurs")
	}
}

// TestMin tests the min helper function
func TestMin(t *testing.T) {
	tests := []struct {
		a, b, expected int
	}{
		{1, 2, 1},
		{2, 1, 1},
		{5, 5, 5},
		{0, 10, 0},
		{-1, 1, -1},
	}

	for _, tt := range tests {
		result := min(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("min(%d, %d) = %d, expected %d", tt.a, tt.b, result, tt.expected)
		}
	}
}
