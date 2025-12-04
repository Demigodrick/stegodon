package activitypub

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

// TestKeyPair holds a test RSA key pair
type TestKeyPair struct {
	PrivateKey    *rsa.PrivateKey
	PrivatePEM    string
	PublicPEM     string
	PublicKeyPKIX string
}

// GenerateTestKeyPair creates a test RSA key pair
func GenerateTestKeyPair() (*TestKeyPair, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	// Encode private key to PKCS#8 PEM
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Encode public key to PKIX PEM
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return &TestKeyPair{
		PrivateKey:    privateKey,
		PrivatePEM:    string(privatePEM),
		PublicPEM:     string(publicPEM),
		PublicKeyPKIX: string(publicPEM),
	}, nil
}

// MockActivityPubServer creates a mock ActivityPub server for testing
type MockActivityPubServer struct {
	Server           *httptest.Server
	ReceivedRequests []ReceivedRequest
	ActorResponse    *ActorResponse
	WebFingerHandler func(w http.ResponseWriter, r *http.Request)
	ActorHandler     func(w http.ResponseWriter, r *http.Request)
	InboxHandler     func(w http.ResponseWriter, r *http.Request)
}

// ReceivedRequest stores details of received HTTP requests
type ReceivedRequest struct {
	Method      string
	Path        string
	Headers     http.Header
	Body        []byte
	ContentType string
}

// NewMockActivityPubServer creates a new mock server
func NewMockActivityPubServer() *MockActivityPubServer {
	mock := &MockActivityPubServer{
		ReceivedRequests: []ReceivedRequest{},
	}

	mux := http.NewServeMux()

	// WebFinger endpoint
	mux.HandleFunc("/.well-known/webfinger", func(w http.ResponseWriter, r *http.Request) {
		if mock.WebFingerHandler != nil {
			mock.WebFingerHandler(w, r)
			return
		}
		// Default WebFinger response
		resource := r.URL.Query().Get("resource")
		if resource == "" {
			http.Error(w, "resource required", http.StatusBadRequest)
			return
		}

		response := map[string]any{
			"subject": resource,
			"links": []map[string]string{
				{
					"rel":  "self",
					"type": "application/activity+json",
					"href": mock.Server.URL + "/users/testuser",
				},
			},
		}

		w.Header().Set("Content-Type", "application/jrd+json")
		json.NewEncoder(w).Encode(response)
	})

	// Actor endpoint
	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		if mock.ActorHandler != nil {
			mock.ActorHandler(w, r)
			return
		}

		if mock.ActorResponse != nil {
			w.Header().Set("Content-Type", "application/activity+json")
			json.NewEncoder(w).Encode(mock.ActorResponse)
			return
		}

		http.NotFound(w, r)
	})

	// Inbox endpoint
	mux.HandleFunc("/inbox", func(w http.ResponseWriter, r *http.Request) {
		mock.recordRequest(r)
		if mock.InboxHandler != nil {
			mock.InboxHandler(w, r)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})

	mock.Server = httptest.NewServer(mux)
	return mock
}

func (m *MockActivityPubServer) recordRequest(r *http.Request) {
	body := make([]byte, 0)
	if r.Body != nil {
		body, _ = readRequestBody(r)
	}

	m.ReceivedRequests = append(m.ReceivedRequests, ReceivedRequest{
		Method:      r.Method,
		Path:        r.URL.Path,
		Headers:     r.Header.Clone(),
		Body:        body,
		ContentType: r.Header.Get("Content-Type"),
	})
}

func readRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	body := make([]byte, r.ContentLength)
	r.Body.Read(body)
	return body, nil
}

// Close shuts down the mock server
func (m *MockActivityPubServer) Close() {
	if m.Server != nil {
		m.Server.Close()
	}
}

// SetActorResponse sets the actor response for the mock server
func (m *MockActivityPubServer) SetActorResponse(actor *ActorResponse) {
	m.ActorResponse = actor
}

// CreateTestActorResponse creates a test actor response
func CreateTestActorResponse(serverURL, username string, publicKeyPEM string) *ActorResponse {
	return &ActorResponse{
		Context:           "https://www.w3.org/ns/activitystreams",
		ID:                serverURL + "/users/" + username,
		Type:              "Person",
		PreferredUsername: username,
		Name:              "Test User " + username,
		Summary:           "A test user",
		Inbox:             serverURL + "/users/" + username + "/inbox",
		Outbox:            serverURL + "/users/" + username + "/outbox",
		PublicKey: struct {
			ID           string `json:"id"`
			Owner        string `json:"owner"`
			PublicKeyPem string `json:"publicKeyPem"`
		}{
			ID:           serverURL + "/users/" + username + "#main-key",
			Owner:        serverURL + "/users/" + username,
			PublicKeyPem: publicKeyPEM,
		},
	}
}

// CreateTestAccount creates a test domain.Account
func CreateTestAccount(username string, keypair *TestKeyPair) *domain.Account {
	return &domain.Account{
		Id:             uuid.New(),
		Username:       username,
		Publickey:      "testhash123",
		CreatedAt:      time.Now(),
		FirstTimeLogin: domain.FALSE,
		WebPublicKey:   keypair.PublicPEM,
		WebPrivateKey:  keypair.PrivatePEM,
		DisplayName:    "Test " + username,
		Summary:        "Test account",
	}
}

// CreateTestRemoteAccount creates a test domain.RemoteAccount
func CreateTestRemoteAccount(serverURL, username, publicKeyPEM string) *domain.RemoteAccount {
	return &domain.RemoteAccount{
		Id:            uuid.New(),
		Username:      username,
		Domain:        extractDomainFromURL(serverURL),
		ActorURI:      serverURL + "/users/" + username,
		DisplayName:   "Remote " + username,
		Summary:       "A remote test account",
		InboxURI:      serverURL + "/users/" + username + "/inbox",
		OutboxURI:     serverURL + "/users/" + username + "/outbox",
		PublicKeyPem:  publicKeyPEM,
		LastFetchedAt: time.Now(),
	}
}

func extractDomainFromURL(serverURL string) string {
	// Remove protocol prefix
	domain := serverURL
	if len(domain) > 8 && domain[:8] == "https://" {
		domain = domain[8:]
	} else if len(domain) > 7 && domain[:7] == "http://" {
		domain = domain[7:]
	}
	return domain
}

// CreateTestFollowActivity creates a test Follow activity JSON
func CreateTestFollowActivity(actorURI, objectURI string) string {
	activity := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       actorURI + "/activities/" + uuid.New().String(),
		"type":     "Follow",
		"actor":    actorURI,
		"object":   objectURI,
	}
	bytes, _ := json.Marshal(activity)
	return string(bytes)
}

// CreateTestAcceptActivity creates a test Accept activity JSON
func CreateTestAcceptActivity(actorURI, followActivityURI string) string {
	activity := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       actorURI + "/activities/" + uuid.New().String(),
		"type":     "Accept",
		"actor":    actorURI,
		"object":   followActivityURI,
	}
	bytes, _ := json.Marshal(activity)
	return string(bytes)
}

// CreateTestCreateActivity creates a test Create activity with Note
func CreateTestCreateActivity(actorURI, content string) string {
	noteID := actorURI + "/notes/" + uuid.New().String()
	activity := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       actorURI + "/activities/" + uuid.New().String(),
		"type":     "Create",
		"actor":    actorURI,
		"object": map[string]any{
			"id":           noteID,
			"type":         "Note",
			"content":      content,
			"published":    time.Now().UTC().Format(time.RFC3339),
			"attributedTo": actorURI,
			"to":           []string{"https://www.w3.org/ns/activitystreams#Public"},
		},
	}
	bytes, _ := json.Marshal(activity)
	return string(bytes)
}

// CreateTestUndoActivity creates a test Undo activity
func CreateTestUndoActivity(actorURI string, undoneActivity map[string]any) string {
	activity := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       actorURI + "/activities/" + uuid.New().String(),
		"type":     "Undo",
		"actor":    actorURI,
		"object":   undoneActivity,
	}
	bytes, _ := json.Marshal(activity)
	return string(bytes)
}

// CreateTestLikeActivity creates a test Like activity
func CreateTestLikeActivity(actorURI, objectURI string) string {
	activity := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       actorURI + "/activities/" + uuid.New().String(),
		"type":     "Like",
		"actor":    actorURI,
		"object":   objectURI,
	}
	bytes, _ := json.Marshal(activity)
	return string(bytes)
}

// CreateTestDeleteActivity creates a test Delete activity
func CreateTestDeleteActivity(actorURI, objectURI string) string {
	activity := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       actorURI + "/activities/" + uuid.New().String(),
		"type":     "Delete",
		"actor":    actorURI,
		"object":   objectURI,
	}
	bytes, _ := json.Marshal(activity)
	return string(bytes)
}

// CreateTestUpdateActivity creates a test Update activity
func CreateTestUpdateActivity(actorURI string, updatedObject map[string]any) string {
	activity := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       actorURI + "/activities/" + uuid.New().String(),
		"type":     "Update",
		"actor":    actorURI,
		"object":   updatedObject,
	}
	bytes, _ := json.Marshal(activity)
	return string(bytes)
}

// ValidateActivityJSON validates that a JSON string is a valid ActivityPub activity
func ValidateActivityJSON(jsonStr string) (map[string]any, error) {
	var activity map[string]any
	err := json.Unmarshal([]byte(jsonStr), &activity)
	if err != nil {
		return nil, err
	}

	// Check required fields
	if _, ok := activity["type"]; !ok {
		return nil, &ValidationError{Field: "type", Message: "missing required field"}
	}
	if _, ok := activity["actor"]; !ok {
		return nil, &ValidationError{Field: "actor", Message: "missing required field"}
	}

	return activity, nil
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// MockHTTPClient is a mock HTTP client for testing
type MockHTTPClient struct {
	// Responses maps request URL to response
	Responses map[string]*http.Response
	// Errors maps request URL to error
	Errors map[string]error
	// Requests stores all received requests
	Requests []*http.Request
	// DefaultResponse is returned when no specific response is configured
	DefaultResponse *http.Response
	// DefaultError is returned when no specific error is configured
	DefaultError error
}

// NewMockHTTPClient creates a new mock HTTP client
func NewMockHTTPClient() *MockHTTPClient {
	return &MockHTTPClient{
		Responses: make(map[string]*http.Response),
		Errors:    make(map[string]error),
		Requests:  []*http.Request{},
	}
}

// Do implements the HTTPClient interface
func (c *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	c.Requests = append(c.Requests, req)

	url := req.URL.String()

	if err, ok := c.Errors[url]; ok {
		return nil, err
	}

	if resp, ok := c.Responses[url]; ok {
		return resp, nil
	}

	if c.DefaultError != nil {
		return nil, c.DefaultError
	}

	if c.DefaultResponse != nil {
		return c.DefaultResponse, nil
	}

	// Return 404 by default
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(bytes.NewReader([]byte("not found"))),
	}, nil
}

// SetResponse sets a response for a specific URL
func (c *MockHTTPClient) SetResponse(url string, statusCode int, body []byte) {
	c.Responses[url] = &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     make(http.Header),
	}
}

// SetJSONResponse sets a JSON response for a specific URL
func (c *MockHTTPClient) SetJSONResponse(url string, statusCode int, data any) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	c.Responses[url] = &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
	return nil
}

// SetError sets an error for a specific URL
func (c *MockHTTPClient) SetError(url string, err error) {
	c.Errors[url] = err
}

// Ensure MockHTTPClient implements HTTPClient interface
var _ HTTPClient = (*MockHTTPClient)(nil)
