package domain

import (
	"fmt"
	"github.com/google/uuid"
	"time"
)

type SaveNote struct {
	UserId       uuid.UUID
	Message      string
	InReplyToURI string // URI of parent post (empty for top-level posts)
}

type Note struct {
	Id        uuid.UUID
	CreatedBy string
	Message   string
	CreatedAt time.Time
	EditedAt  *time.Time // When the note was last edited (nil if never edited)
	// ActivityPub fields
	Visibility     string // "public", "unlisted", "followers", "direct"
	InReplyToURI   string // URI of the note this is replying to
	ObjectURI      string // ActivityPub object URI
	Federated      bool   // Whether to federate this note
	Sensitive      bool   // Contains sensitive content
	ContentWarning string // Content warning text
}

func (note *Note) ToString() string {
	return fmt.Sprintf("\n\tId: %s \n\tCreatedBy: %s \n\tMessage: %s \n\tCreatedAt: %s)", note.Id, note.CreatedBy, note.Message, note.CreatedAt)
}

// HomePost represents a unified post in the home timeline (either local or remote)
type HomePost struct {
	ID         uuid.UUID
	Author     string    // @user (local) or @user@domain (remote)
	Content    string
	Time       time.Time
	ObjectURI  string
	IsLocal    bool      // true = local note, false = remote activity
	NoteID     uuid.UUID // only set for local posts (for editing/deleting)
	ReplyCount int       // number of replies to this post
}
