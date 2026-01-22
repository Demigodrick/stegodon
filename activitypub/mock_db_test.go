package activitypub

import (
	"database/sql"
	"sync"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

// MockDatabase is an in-memory mock implementation of the Database interface for testing.
// It stores data in maps and provides full CRUD operations without requiring a real database.
type MockDatabase struct {
	mu sync.RWMutex

	// Storage maps
	Accounts        map[uuid.UUID]*domain.Account
	AccountsByUser  map[string]*domain.Account
	RemoteAccounts  map[uuid.UUID]*domain.RemoteAccount
	RemoteByURI     map[string]*domain.RemoteAccount
	RemoteByActor   map[string]*domain.RemoteAccount
	Follows         map[uuid.UUID]*domain.Follow
	FollowsByURI    map[string]*domain.Follow
	Activities      map[uuid.UUID]*domain.Activity
	ActivitiesByObj map[string]*domain.Activity
	ActivitiesByURI map[string]*domain.Activity // Index by ActivityURI
	DeliveryQueue   map[uuid.UUID]*domain.DeliveryQueueItem
	Notes           map[uuid.UUID]*domain.Note
	NotesByURI      map[string]*domain.Note
	Likes           map[uuid.UUID]*domain.Like
	LikesByURI      map[string]*domain.Like
	Boosts          map[uuid.UUID]*domain.Boost
	Relays          map[uuid.UUID]*domain.Relay
	RelaysByURI     map[string]*domain.Relay

	// Error injection for testing error handling
	ForceError error

	// Call tracking for testing
	IncrementReplyCountCalls []string    // URIs passed to IncrementReplyCountByURI
	IncrementLikeCountCalls  []uuid.UUID // Note IDs passed to IncrementLikeCountByNoteId
	IncrementBoostCountCalls []uuid.UUID // Note IDs passed to IncrementBoostCountByNoteId
}

// NewMockDatabase creates a new mock database with initialized maps
func NewMockDatabase() *MockDatabase {
	return &MockDatabase{
		Accounts:        make(map[uuid.UUID]*domain.Account),
		AccountsByUser:  make(map[string]*domain.Account),
		RemoteAccounts:  make(map[uuid.UUID]*domain.RemoteAccount),
		RemoteByURI:     make(map[string]*domain.RemoteAccount),
		RemoteByActor:   make(map[string]*domain.RemoteAccount),
		Follows:         make(map[uuid.UUID]*domain.Follow),
		FollowsByURI:    make(map[string]*domain.Follow),
		Activities:      make(map[uuid.UUID]*domain.Activity),
		ActivitiesByObj: make(map[string]*domain.Activity),
		ActivitiesByURI: make(map[string]*domain.Activity),
		DeliveryQueue:   make(map[uuid.UUID]*domain.DeliveryQueueItem),
		Notes:           make(map[uuid.UUID]*domain.Note),
		NotesByURI:      make(map[string]*domain.Note),
		Likes:           make(map[uuid.UUID]*domain.Like),
		LikesByURI:      make(map[string]*domain.Like),
		Boosts:          make(map[uuid.UUID]*domain.Boost),
		Relays:          make(map[uuid.UUID]*domain.Relay),
		RelaysByURI:     make(map[string]*domain.Relay),
	}
}

// SetForceError sets an error to be returned by all operations
func (m *MockDatabase) SetForceError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ForceError = err
}

// AddAccount adds an account to the mock database
func (m *MockDatabase) AddAccount(acc *domain.Account) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Accounts[acc.Id] = acc
	m.AccountsByUser[acc.Username] = acc
}

// AddRemoteAccount adds a remote account to the mock database
func (m *MockDatabase) AddRemoteAccount(acc *domain.RemoteAccount) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RemoteAccounts[acc.Id] = acc
	m.RemoteByURI[acc.ActorURI] = acc
	m.RemoteByActor[acc.ActorURI] = acc
}

// AddFollow adds a follow relationship to the mock database
func (m *MockDatabase) AddFollow(follow *domain.Follow) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Follows[follow.Id] = follow
	if follow.URI != "" {
		m.FollowsByURI[follow.URI] = follow
	}
}

// AddActivity adds an activity to the mock database
func (m *MockDatabase) AddActivity(activity *domain.Activity) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Activities[activity.Id] = activity
	if activity.ActivityURI != "" {
		m.ActivitiesByURI[activity.ActivityURI] = activity
	}
	if activity.ObjectURI != "" {
		m.ActivitiesByObj[activity.ObjectURI] = activity
	}
}

// AddDeliveryQueueItem adds a delivery queue item to the mock database
func (m *MockDatabase) AddDeliveryQueueItem(item *domain.DeliveryQueueItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeliveryQueue[item.Id] = item
}

// Account operations

func (m *MockDatabase) ReadAccByUsername(username string) (error, *domain.Account) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	acc, ok := m.AccountsByUser[username]
	if !ok {
		return sql.ErrNoRows, nil
	}
	return nil, acc
}

func (m *MockDatabase) ReadAccById(id uuid.UUID) (error, *domain.Account) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	acc, ok := m.Accounts[id]
	if !ok {
		return sql.ErrNoRows, nil
	}
	return nil, acc
}

// Remote account operations

func (m *MockDatabase) ReadRemoteAccountByURI(uri string) (error, *domain.RemoteAccount) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	acc, ok := m.RemoteByURI[uri]
	if !ok {
		return sql.ErrNoRows, nil
	}
	return nil, acc
}

func (m *MockDatabase) ReadRemoteAccountById(id uuid.UUID) (error, *domain.RemoteAccount) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	acc, ok := m.RemoteAccounts[id]
	if !ok {
		return sql.ErrNoRows, nil
	}
	return nil, acc
}

func (m *MockDatabase) ReadRemoteAccountByActorURI(actorURI string) (error, *domain.RemoteAccount) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	acc, ok := m.RemoteByActor[actorURI]
	if !ok {
		return nil, nil
	}
	return nil, acc
}

func (m *MockDatabase) CreateRemoteAccount(acc *domain.RemoteAccount) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	m.RemoteAccounts[acc.Id] = acc
	m.RemoteByURI[acc.ActorURI] = acc
	m.RemoteByActor[acc.ActorURI] = acc
	return nil
}

func (m *MockDatabase) UpdateRemoteAccount(acc *domain.RemoteAccount) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	m.RemoteAccounts[acc.Id] = acc
	m.RemoteByURI[acc.ActorURI] = acc
	m.RemoteByActor[acc.ActorURI] = acc
	return nil
}

func (m *MockDatabase) DeleteRemoteAccount(id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	if acc, ok := m.RemoteAccounts[id]; ok {
		delete(m.RemoteByURI, acc.ActorURI)
		delete(m.RemoteByActor, acc.ActorURI)
	}
	delete(m.RemoteAccounts, id)
	return nil
}

// Follow operations

func (m *MockDatabase) CreateFollow(follow *domain.Follow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	m.Follows[follow.Id] = follow
	if follow.URI != "" {
		m.FollowsByURI[follow.URI] = follow
	}
	return nil
}

func (m *MockDatabase) ReadFollowByURI(uri string) (error, *domain.Follow) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	follow, ok := m.FollowsByURI[uri]
	if !ok {
		return sql.ErrNoRows, nil
	}
	return nil, follow
}

func (m *MockDatabase) ReadFollowByAccountIds(accountId, targetAccountId uuid.UUID) (error, *domain.Follow) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	for _, follow := range m.Follows {
		if follow.AccountId == accountId && follow.TargetAccountId == targetAccountId {
			return nil, follow
		}
	}
	return sql.ErrNoRows, nil
}

func (m *MockDatabase) DeleteFollowByURI(uri string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	if follow, ok := m.FollowsByURI[uri]; ok {
		delete(m.Follows, follow.Id)
	}
	delete(m.FollowsByURI, uri)
	return nil
}

func (m *MockDatabase) AcceptFollowByURI(uri string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	if follow, ok := m.FollowsByURI[uri]; ok {
		follow.Accepted = true
	}
	return nil
}

func (m *MockDatabase) ReadFollowersByAccountId(accountId uuid.UUID) (error, *[]domain.Follow) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	var followers []domain.Follow
	for _, follow := range m.Follows {
		if follow.TargetAccountId == accountId && follow.Accepted {
			followers = append(followers, *follow)
		}
	}
	return nil, &followers
}

func (m *MockDatabase) DeleteFollowsByRemoteAccountId(remoteAccountId uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	for id, follow := range m.Follows {
		if follow.AccountId == remoteAccountId || follow.TargetAccountId == remoteAccountId {
			if follow.URI != "" {
				delete(m.FollowsByURI, follow.URI)
			}
			delete(m.Follows, id)
		}
	}
	return nil
}

// Activity operations

func (m *MockDatabase) CreateActivity(activity *domain.Activity) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	m.Activities[activity.Id] = activity
	if activity.ActivityURI != "" {
		m.ActivitiesByURI[activity.ActivityURI] = activity
	}
	if activity.ObjectURI != "" {
		// Only set if not already present (first activity with this ObjectURI wins)
		// This matches real DB behavior where ReadActivityByObjectURI returns the first match
		if _, exists := m.ActivitiesByObj[activity.ObjectURI]; !exists {
			m.ActivitiesByObj[activity.ObjectURI] = activity
		}
	}
	return nil
}

func (m *MockDatabase) UpdateActivity(activity *domain.Activity) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	m.Activities[activity.Id] = activity
	if activity.ObjectURI != "" {
		m.ActivitiesByObj[activity.ObjectURI] = activity
	}
	return nil
}

func (m *MockDatabase) ReadActivityByURI(uri string) (error, *domain.Activity) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	activity, ok := m.ActivitiesByURI[uri]
	if !ok {
		return nil, nil
	}
	return nil, activity
}

func (m *MockDatabase) ReadActivityByObjectURI(objectURI string) (error, *domain.Activity) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	activity, ok := m.ActivitiesByObj[objectURI]
	if !ok {
		return nil, nil
	}
	return nil, activity
}

func (m *MockDatabase) DeleteActivity(id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	if activity, ok := m.Activities[id]; ok {
		delete(m.ActivitiesByObj, activity.ObjectURI)
		delete(m.ActivitiesByURI, activity.ActivityURI)
	}
	delete(m.Activities, id)
	return nil
}

// Delivery queue operations

func (m *MockDatabase) EnqueueDelivery(item *domain.DeliveryQueueItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	m.DeliveryQueue[item.Id] = item
	return nil
}

func (m *MockDatabase) ReadPendingDeliveries(limit int) (error, *[]domain.DeliveryQueueItem) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	var items []domain.DeliveryQueueItem
	now := time.Now()
	count := 0
	for _, item := range m.DeliveryQueue {
		if item.NextRetryAt.Before(now) || item.NextRetryAt.Equal(now) {
			items = append(items, *item)
			count++
			if count >= limit {
				break
			}
		}
	}
	return nil, &items
}

func (m *MockDatabase) UpdateDeliveryAttempt(id uuid.UUID, attempts int, nextRetry time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	if item, ok := m.DeliveryQueue[id]; ok {
		item.Attempts = attempts
		item.NextRetryAt = nextRetry
	}
	return nil
}

func (m *MockDatabase) DeleteDelivery(id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	delete(m.DeliveryQueue, id)
	return nil
}

// Note operations

func (m *MockDatabase) ReadNoteByURI(objectURI string) (error, *domain.Note) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	note, ok := m.NotesByURI[objectURI]
	if !ok {
		return sql.ErrNoRows, nil
	}
	return nil, note
}

// Mention operations

func (m *MockDatabase) CreateNoteMention(mention *domain.NoteMention) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	// Just store it - we don't need to track mentions in tests unless specifically needed
	return nil
}

// IncrementReplyCountByURI increments the reply count for a note or activity
func (m *MockDatabase) IncrementReplyCountByURI(parentURI string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.IncrementReplyCountCalls = append(m.IncrementReplyCountCalls, parentURI)
	return nil
}

// Like operations

func (m *MockDatabase) CreateLike(like *domain.Like) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	m.Likes[like.Id] = like
	if like.URI != "" {
		m.LikesByURI[like.URI] = like
	}
	return nil
}

func (m *MockDatabase) HasLikeByURI(uri string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return false, m.ForceError
	}
	_, ok := m.LikesByURI[uri]
	return ok, nil
}

func (m *MockDatabase) HasLike(accountId, noteId uuid.UUID) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return false, m.ForceError
	}
	for _, like := range m.Likes {
		if like.AccountId == accountId && like.NoteId == noteId {
			return true, nil
		}
	}
	return false, nil
}

func (m *MockDatabase) ReadLikeByAccountAndNote(accountId, noteId uuid.UUID) (error, *domain.Like) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	for _, like := range m.Likes {
		if like.AccountId == accountId && like.NoteId == noteId {
			return nil, like
		}
	}
	return sql.ErrNoRows, nil
}

func (m *MockDatabase) DeleteLikeByAccountAndNote(accountId, noteId uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	for id, like := range m.Likes {
		if like.AccountId == accountId && like.NoteId == noteId {
			delete(m.LikesByURI, like.URI)
			delete(m.Likes, id)
			break
		}
	}
	return nil
}

func (m *MockDatabase) IncrementLikeCountByNoteId(noteId uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	m.IncrementLikeCountCalls = append(m.IncrementLikeCountCalls, noteId)
	if note, ok := m.Notes[noteId]; ok {
		note.LikeCount++
	}
	return nil
}

func (m *MockDatabase) DecrementLikeCountByNoteId(noteId uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	if note, ok := m.Notes[noteId]; ok {
		if note.LikeCount > 0 {
			note.LikeCount--
		}
	}
	return nil
}

// AddNote adds a note to the mock database
func (m *MockDatabase) AddNote(note *domain.Note) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Notes[note.Id] = note
	if note.ObjectURI != "" {
		m.NotesByURI[note.ObjectURI] = note
	}
}

// Boost operations

func (m *MockDatabase) CreateBoost(boost *domain.Boost) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	m.Boosts[boost.Id] = boost
	return nil
}

func (m *MockDatabase) HasBoost(accountId, noteId uuid.UUID) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return false, m.ForceError
	}
	for _, boost := range m.Boosts {
		if boost.AccountId == accountId && boost.NoteId == noteId {
			return true, nil
		}
	}
	return false, nil
}

func (m *MockDatabase) DeleteBoostByAccountAndNote(accountId, noteId uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	for id, boost := range m.Boosts {
		if boost.AccountId == accountId && boost.NoteId == noteId {
			delete(m.Boosts, id)
			break
		}
	}
	return nil
}

func (m *MockDatabase) IncrementBoostCountByNoteId(noteId uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	m.IncrementBoostCountCalls = append(m.IncrementBoostCountCalls, noteId)
	if note, ok := m.Notes[noteId]; ok {
		note.BoostCount++
	}
	return nil
}

func (m *MockDatabase) DecrementBoostCountByNoteId(noteId uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	if note, ok := m.Notes[noteId]; ok {
		if note.BoostCount > 0 {
			note.BoostCount--
		}
	}
	return nil
}

// Remote boost operations

func (m *MockDatabase) IsRemoteAccountFollowed(remoteAccountId uuid.UUID) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return false, m.ForceError
	}
	// Check if any local account follows this remote account
	for _, follow := range m.Follows {
		if follow.TargetAccountId == remoteAccountId && follow.Accepted {
			// Verify the follower is a local account
			if _, isLocal := m.Accounts[follow.AccountId]; isLocal {
				return true, nil
			}
		}
	}
	return false, nil
}

func (m *MockDatabase) CreateBoostFromRemote(boost *domain.Boost) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	m.Boosts[boost.Id] = boost
	return nil
}

func (m *MockDatabase) HasBoostFromRemote(remoteAccountId uuid.UUID, objectURI string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return false, m.ForceError
	}
	for _, boost := range m.Boosts {
		if boost.RemoteAccountId == remoteAccountId && boost.ObjectURI == objectURI {
			return true, nil
		}
	}
	return false, nil
}

func (m *MockDatabase) DeleteBoostByRemoteAccountAndObjectURI(remoteAccountId uuid.UUID, objectURI string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	for id, boost := range m.Boosts {
		if boost.RemoteAccountId == remoteAccountId && boost.ObjectURI == objectURI {
			delete(m.Boosts, id)
			return nil
		}
	}
	return nil
}

func (m *MockDatabase) DecrementBoostCountByObjectURI(objectURI string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	for _, activity := range m.Activities {
		if activity.ObjectURI == objectURI {
			if activity.BoostCount > 0 {
				activity.BoostCount--
			}
			return nil
		}
	}
	return nil
}

// Relay operations

func (m *MockDatabase) CreateRelay(relay *domain.Relay) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	m.Relays[relay.Id] = relay
	m.RelaysByURI[relay.ActorURI] = relay
	return nil
}

func (m *MockDatabase) ReadActiveRelays() (error, *[]domain.Relay) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	relays := make([]domain.Relay, 0)
	for _, r := range m.Relays {
		if r.Status == "active" {
			relays = append(relays, *r)
		}
	}
	return nil, &relays
}

func (m *MockDatabase) ReadActiveUnpausedRelays() (error, *[]domain.Relay) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	relays := make([]domain.Relay, 0)
	for _, r := range m.Relays {
		if r.Status == "active" && !r.Paused {
			relays = append(relays, *r)
		}
	}
	return nil, &relays
}

func (m *MockDatabase) ReadRelayByActorURI(actorURI string) (error, *domain.Relay) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	if relay, ok := m.RelaysByURI[actorURI]; ok {
		return nil, relay
	}
	return sql.ErrNoRows, nil
}

func (m *MockDatabase) UpdateRelayStatus(id uuid.UUID, status string, acceptedAt *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	return nil
}

func (m *MockDatabase) DeleteRelay(id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	return nil
}

// CreateNotification creates a notification (no-op for mock)
func (m *MockDatabase) CreateNotification(notification *domain.Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	// No-op for now, can be extended if needed for testing
	return nil
}

// Ensure MockDatabase implements Database interface
var _ Database = (*MockDatabase)(nil)
