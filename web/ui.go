package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/gin-gonic/gin"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/google/uuid"
)

type IndexPageData struct {
	Title         string
	Host          string
	SSHPort       int
	Version       string
	Posts         []PostView
	HasPrev       bool
	HasNext       bool
	PrevPage      int
	NextPage      int
	InfoBoxes     []InfoBoxView
	ServerMessage *domain.ServerMessage
}

type InfoBoxView struct {
	Title       string        // Plain text title (auto-escaped by template)
	TitleHTML   template.HTML // Title rendered as markdown/HTML (for icons/formatting)
	ContentHTML template.HTML // Sanitized HTML content from markdown
}

type ProfilePageData struct {
	Title         string
	Host          string
	SSHPort       int
	Version       string
	User          UserView
	Posts         []PostView
	TotalPosts    int
	HasPrev       bool
	HasNext       bool
	PrevPage      int
	NextPage      int
	InfoBoxes     []InfoBoxView
	ServerMessage *domain.ServerMessage
}

type UserView struct {
	Username      string
	DisplayName   string
	Summary       string
	JoinedAgo     string
	AvatarURL     string
	AvatarVersion int64 // Unix timestamp for cache busting
}

type PostView struct {
	NoteId       string
	Username     string
	UserDomain   string // Domain for remote users (empty for local)
	ProfileURL   string // Full profile URL
	PostURL      string // Permalink to the post (remote object_uri or local path)
	IsRemote     bool   // True if federated user
	Message      string
	MessageHTML  template.HTML // HTML-rendered message with clickable links
	TimeAgo      string
	CreatedAt    time.Time // For chronological sorting
	InReplyToURI string    // URI of parent post if this is a reply
	ReplyCount   int       // Number of replies to this post
	LikeCount    int       // Number of likes on this post
	BoostCount   int       // Number of boosts on this post
	Likers       []string  // Usernames who liked this post
	Boosters     []string  // Usernames who boosted this post
}

// convertMarkdownToHTML converts markdown text to HTML
func convertMarkdownToHTML(md string) string {
	// Create markdown parser with extensions (including strikethrough)
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock | parser.Strikethrough
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse([]byte(md))

	// Create HTML renderer with options
	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	return string(markdown.Render(doc, renderer))
}

func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if duration < 30*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else {
		return t.Format("Jan 2, 2006")
	}
}

// parseActivityContentForWeb extracts content and author info from an activity for web display
// Returns content, username, domain, profileURL
func parseActivityContentForWeb(activity *domain.Activity, database *db.DB) (content, username, userDomain, profileURL string) {
	// Default to actor URI as fallback
	username = activity.ActorURI
	userDomain = ""
	profileURL = activity.ActorURI

	// Try to get better author info from cached remote account
	err, remoteAcc := database.ReadRemoteAccountByActorURI(activity.ActorURI)
	if err == nil && remoteAcc != nil {
		username = remoteAcc.Username
		userDomain = remoteAcc.Domain
		profileURL = fmt.Sprintf("https://%s/@%s", remoteAcc.Domain, remoteAcc.Username)
	} else {
		// Parse username and domain from actor URI as fallback
		// Format: https://domain.com/users/username or https://domain.com/@username
		if strings.Contains(activity.ActorURI, "/users/") {
			parts := strings.Split(activity.ActorURI, "/users/")
			if len(parts) == 2 {
				domainPart := strings.TrimPrefix(parts[0], "https://")
				userDomain = domainPart
				username = parts[1]
				profileURL = fmt.Sprintf("https://%s/@%s", domainPart, parts[1])
			}
		} else if strings.Contains(activity.ActorURI, "/@") {
			parts := strings.Split(activity.ActorURI, "/@")
			if len(parts) == 2 {
				domainPart := strings.TrimPrefix(parts[0], "https://")
				userDomain = domainPart
				username = parts[1]
			}
		}
	}

	// Parse content from raw JSON
	if activity.RawJSON != "" {
		var activityWrapper struct {
			Type   string `json:"type"`
			Object struct {
				ID      string `json:"id"`
				Content string `json:"content"`
			} `json:"object"`
		}

		if err := json.Unmarshal([]byte(activity.RawJSON), &activityWrapper); err == nil {
			content = util.StripHTMLTags(activityWrapper.Object.Content)
		}
	}

	return content, username, userDomain, profileURL
}

// countTotalRepliesForWeb counts both local and remote replies to a note
// When ActivityPub is enabled, it also counts remote activities that reply to this note
func countTotalRepliesForWeb(database *db.DB, noteId uuid.UUID, sslDomain string, withAp bool) int {
	// Count local replies first
	localCount := 0
	if count, err := database.CountRepliesByNoteId(noteId); err == nil {
		localCount = count
	}

	// If ActivityPub is enabled, also count remote replies
	if withAp && sslDomain != "" {
		canonicalURI := fmt.Sprintf("https://%s/notes/%s", sslDomain, noteId.String())
		if remoteCount, err := database.CountActivitiesByInReplyTo(canonicalURI); err == nil {
			localCount += remoteCount
		}
	}

	return localCount
}

// loadServerMessageForWeb returns the server message if web_enabled is true
func loadServerMessageForWeb() *domain.ServerMessage {
	database := db.GetDB()
	err, msg := database.ReadServerMessage()
	if err != nil {
		log.Printf("Failed to load server message for web: %v", err)
		return nil
	}
	// Only return if web display is enabled
	if msg != nil && msg.WebEnabled && msg.Message != "" {
		return msg
	}
	return nil
}

func HandleIndex(c *gin.Context, conf *util.AppConfig) {
	database := db.GetDB()

	// Pagination
	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	postsPerPage := 20
	offset := (page - 1) * postsPerPage

	// Get all notes from all users (local timeline)
	err, notes := database.ReadAllNotes()
	if err != nil {
		log.Printf("Failed to read notes: %v", err)
		c.HTML(500, "base.html", gin.H{"Title": "Error", "Error": "Failed to load timeline"})
		return
	}

	if notes == nil {
		notes = &[]domain.Note{}
	}

	// Filter out replies (posts with InReplyToURI set)
	var topLevelNotes []domain.Note
	for _, note := range *notes {
		if note.InReplyToURI == "" {
			topLevelNotes = append(topLevelNotes, note)
		}
	}

	totalPosts := len(topLevelNotes)

	// Apply pagination
	start := offset
	end := offset + postsPerPage
	if start > totalPosts {
		start = totalPosts
	}
	if end > totalPosts {
		end = totalPosts
	}

	paginatedNotes := topLevelNotes[start:end]

	// Convert to PostView
	posts := make([]PostView, 0, len(paginatedNotes))
	for _, note := range paginatedNotes {
		// First convert markdown links, then highlight hashtags and mentions
		messageHTML := util.MarkdownLinksToHTML(note.Message)
		messageHTML = util.HighlightHashtagsHTML(messageHTML)
		messageHTML = util.HighlightMentionsHTML(messageHTML, conf.Conf.SslDomain)

		// Get reply count for this post (including remote replies when AP is enabled)
		replyCount := countTotalRepliesForWeb(database, note.Id, conf.Conf.SslDomain, conf.Conf.WithAp)

		posts = append(posts, PostView{
			NoteId:      note.Id.String(),
			Username:    note.CreatedBy,
			Message:     note.Message,
			MessageHTML: template.HTML(messageHTML),
			TimeAgo:     formatTimeAgo(note.CreatedAt),
			ReplyCount:  replyCount,
			LikeCount:   note.LikeCount,
			BoostCount:  note.BoostCount,
		})
	}

	// Use SSLDomain if federation is enabled, otherwise use Host
	host := conf.Conf.Host
	if conf.Conf.WithAp {
		host = conf.Conf.SslDomain
	}

	// Load info boxes for the index page
	var infoBoxViews []InfoBoxView
	err, infoBoxes := database.ReadEnabledInfoBoxes()
	if err == nil && infoBoxes != nil {
		for _, box := range *infoBoxes {
			// Replace placeholders first
			content := util.ReplacePlaceholders(box.Content, conf.Conf.SshPort)

			// Convert markdown to HTML
			htmlContent := convertMarkdownToHTML(content)

			// Render title as markdown too - this allows safe HTML/SVG but prevents XSS
			titleHTML := convertMarkdownToHTML(box.Title)

			infoBoxViews = append(infoBoxViews, InfoBoxView{
				Title:       box.Title,                // Plain text fallback
				TitleHTML:   template.HTML(titleHTML), // Markdown-rendered (allows safe HTML/SVG)
				ContentHTML: template.HTML(htmlContent),
			})
		}
	}

	data := IndexPageData{
		Title:         "Home",
		Host:          host,
		SSHPort:       conf.Conf.SshPort,
		Version:       util.GetVersion(),
		Posts:         posts,
		HasPrev:       page > 1,
		HasNext:       end < totalPosts,
		PrevPage:      page - 1,
		NextPage:      page + 1,
		InfoBoxes:     infoBoxViews,
		ServerMessage: loadServerMessageForWeb(),
	}

	c.HTML(200, "index.html", data)
}

func HandleProfile(c *gin.Context, conf *util.AppConfig) {
	username := c.Param("username")
	database := db.GetDB()

	// Get user account
	err, account := database.ReadAccByUsername(username)
	if err != nil {
		log.Printf("User not found: %s", username)
		c.HTML(404, "base.html", gin.H{"Title": "Not Found", "Error": "User not found"})
		return
	}

	// Pagination
	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	postsPerPage := 20
	offset := (page - 1) * postsPerPage

	// Get user's notes
	err, notes := database.ReadNotesByUserId(account.Id)
	if err != nil {
		log.Printf("Failed to read notes for user %s: %v", username, err)
		c.HTML(500, "base.html", gin.H{"Title": "Error", "Error": "Failed to load user posts"})
		return
	}

	if notes == nil {
		notes = &[]domain.Note{}
	}

	// Filter out replies (posts with InReplyToURI set)
	var topLevelNotes []domain.Note
	for _, note := range *notes {
		if note.InReplyToURI == "" {
			topLevelNotes = append(topLevelNotes, note)
		}
	}

	totalPosts := len(topLevelNotes)

	// Apply pagination
	start := offset
	end := offset + postsPerPage
	if start > totalPosts {
		start = totalPosts
	}
	if end > totalPosts {
		end = totalPosts
	}

	paginatedNotes := topLevelNotes[start:end]

	// Convert to PostView
	posts := make([]PostView, 0, len(paginatedNotes))
	for _, note := range paginatedNotes {
		// First convert markdown links, then highlight hashtags and mentions
		messageHTML := util.MarkdownLinksToHTML(note.Message)
		messageHTML = util.HighlightHashtagsHTML(messageHTML)
		messageHTML = util.HighlightMentionsHTML(messageHTML, conf.Conf.SslDomain)

		// Get reply count for this post (including remote replies when AP is enabled)
		replyCount := countTotalRepliesForWeb(database, note.Id, conf.Conf.SslDomain, conf.Conf.WithAp)

		posts = append(posts, PostView{
			NoteId:      note.Id.String(),
			Username:    note.CreatedBy,
			Message:     note.Message,
			MessageHTML: template.HTML(messageHTML),
			TimeAgo:     formatTimeAgo(note.CreatedAt),
			ReplyCount:  replyCount,
			LikeCount:   note.LikeCount,
			BoostCount:  note.BoostCount,
		})
	}

	// Use SSLDomain if federation is enabled, otherwise use Host
	host := conf.Conf.Host
	if conf.Conf.WithAp {
		host = conf.Conf.SslDomain
	}

	// Load info boxes
	var infoBoxViews []InfoBoxView
	err, infoBoxes := database.ReadEnabledInfoBoxes()
	if err == nil && infoBoxes != nil {
		for _, box := range *infoBoxes {
			content := util.ReplacePlaceholders(box.Content, conf.Conf.SshPort)
			htmlContent := convertMarkdownToHTML(content)
			titleHTML := convertMarkdownToHTML(box.Title)
			infoBoxViews = append(infoBoxViews, InfoBoxView{
				Title:       box.Title,
				TitleHTML:   template.HTML(titleHTML),
				ContentHTML: template.HTML(htmlContent),
			})
		}
	}

	data := ProfilePageData{
		Title:   fmt.Sprintf("@%s", username),
		Host:    host,
		SSHPort: conf.Conf.SshPort,
		Version: util.GetVersion(),
		User: UserView{
			Username:      account.Username,
			DisplayName:   account.DisplayName,
			Summary:       account.Summary,
			JoinedAgo:     formatTimeAgo(account.CreatedAt),
			AvatarURL:     account.AvatarURL,
			AvatarVersion: time.Now().Unix(),
		},
		Posts:         posts,
		TotalPosts:    totalPosts,
		HasPrev:       page > 1,
		HasNext:       end < totalPosts,
		PrevPage:      page - 1,
		NextPage:      page + 1,
		InfoBoxes:     infoBoxViews,
		ServerMessage: loadServerMessageForWeb(),
	}

	c.HTML(200, "profile.html", data)
}

type SinglePostPageData struct {
	Title         string
	Host          string
	SSHPort       int
	Version       string
	Post          PostView
	User          UserView
	ParentPost    *PostView  // Parent post if this is a reply (nil if not a reply)
	Replies       []PostView // Replies to this post
	InfoBoxes     []InfoBoxView
	ServerMessage *domain.ServerMessage
}

type TagPageData struct {
	Title         string
	Host          string
	SSHPort       int
	Version       string
	Tag           string
	Posts         []PostView
	TotalPosts    int
	HasPrev       bool
	HasNext       bool
	PrevPage      int
	NextPage      int
	InfoBoxes     []InfoBoxView
	ServerMessage *domain.ServerMessage
}

func HandleSinglePost(c *gin.Context, conf *util.AppConfig) {
	username := c.Param("username")
	noteIdStr := c.Param("noteid")
	database := db.GetDB()

	// Parse note ID
	noteId, err := uuid.Parse(noteIdStr)
	if err != nil {
		log.Printf("Invalid note ID: %s", noteIdStr)
		c.HTML(404, "base.html", gin.H{"Title": "Not Found", "Error": "Post not found"})
		return
	}

	// Get user account
	err, account := database.ReadAccByUsername(username)
	if err != nil {
		log.Printf("User not found: %s", username)
		c.HTML(404, "base.html", gin.H{"Title": "Not Found", "Error": "User not found"})
		return
	}

	// Get the note
	err, note := database.ReadNoteId(noteId)
	if err != nil || note == nil {
		log.Printf("Note not found: %s", noteIdStr)
		c.HTML(404, "base.html", gin.H{"Title": "Not Found", "Error": "Post not found"})
		return
	}

	// Verify the note belongs to this user
	if note.CreatedBy != username {
		log.Printf("Note %s does not belong to user %s", noteIdStr, username)
		c.HTML(404, "base.html", gin.H{"Title": "Not Found", "Error": "Post not found"})
		return
	}

	// Use SSLDomain if federation is enabled, otherwise use Host
	host := conf.Conf.Host
	if conf.Conf.WithAp {
		host = conf.Conf.SslDomain
	}

	// First convert markdown links, then highlight hashtags and mentions
	messageHTML := util.MarkdownLinksToHTML(note.Message)
	messageHTML = util.HighlightHashtagsHTML(messageHTML)
	messageHTML = util.HighlightMentionsHTML(messageHTML, conf.Conf.SslDomain)

	// Get reply count for this post (including remote replies when AP is enabled)
	replyCount := countTotalRepliesForWeb(database, noteId, conf.Conf.SslDomain, conf.Conf.WithAp)

	// Get engagement info (who liked and boosted this post)
	likers, _ := database.ReadLikersInfoByNoteId(noteId)
	boosters, _ := database.ReadBoostersInfoByNoteId(noteId)

	post := PostView{
		NoteId:       note.Id.String(),
		Username:     note.CreatedBy,
		Message:      note.Message,
		MessageHTML:  template.HTML(messageHTML),
		TimeAgo:      formatTimeAgo(note.CreatedAt),
		InReplyToURI: note.InReplyToURI,
		ReplyCount:   replyCount,
		LikeCount:    note.LikeCount,
		BoostCount:   note.BoostCount,
		Likers:       likers,
		Boosters:     boosters,
	}

	// Check if this is a reply and fetch parent post
	var parentPost *PostView
	if note.InReplyToURI != "" {
		// Try to find parent post in local notes
		err, parentNote := database.ReadNoteByURI(note.InReplyToURI)
		if err == nil && parentNote != nil {
			parentMessageHTML := util.MarkdownLinksToHTML(parentNote.Message)
			parentMessageHTML = util.HighlightHashtagsHTML(parentMessageHTML)
			parentMessageHTML = util.HighlightMentionsHTML(parentMessageHTML, conf.Conf.SslDomain)

			// Get reply count for parent post (including remote replies when AP is enabled)
			parentReplyCount := countTotalRepliesForWeb(database, parentNote.Id, conf.Conf.SslDomain, conf.Conf.WithAp)

			parentPost = &PostView{
				NoteId:      parentNote.Id.String(),
				Username:    parentNote.CreatedBy,
				Message:     parentNote.Message,
				MessageHTML: template.HTML(parentMessageHTML),
				TimeAgo:     formatTimeAgo(parentNote.CreatedAt),
				ReplyCount:  parentReplyCount,
				LikeCount:   parentNote.LikeCount,
				BoostCount:  parentNote.BoostCount,
			}
		}
	}

	// Fetch replies to this post
	var replies []PostView
	err, replyNotes := database.ReadRepliesByNoteId(noteId)
	if err == nil && replyNotes != nil {
		for _, replyNote := range *replyNotes {
			replyMessageHTML := util.MarkdownLinksToHTML(replyNote.Message)
			replyMessageHTML = util.HighlightHashtagsHTML(replyMessageHTML)
			replyMessageHTML = util.HighlightMentionsHTML(replyMessageHTML, conf.Conf.SslDomain)

			// Get reply count for this reply (including remote replies when AP is enabled)
			replyReplyCount := countTotalRepliesForWeb(database, replyNote.Id, conf.Conf.SslDomain, conf.Conf.WithAp)

			replies = append(replies, PostView{
				NoteId:      replyNote.Id.String(),
				Username:    replyNote.CreatedBy,
				Message:     replyNote.Message,
				MessageHTML: template.HTML(replyMessageHTML),
				TimeAgo:     formatTimeAgo(replyNote.CreatedAt),
				CreatedAt:   replyNote.CreatedAt,
				ReplyCount:  replyReplyCount,
				LikeCount:   replyNote.LikeCount,
				BoostCount:  replyNote.BoostCount,
				IsRemote:    false,
			})
		}
	}

	// Fetch remote replies from activities table (when ActivityPub is enabled)
	if conf.Conf.WithAp {
		// Build the canonical URI for this note
		canonicalURI := fmt.Sprintf("https://%s/notes/%s", conf.Conf.SslDomain, noteId.String())
		localActorPrefix := fmt.Sprintf("https://%s/users/", conf.Conf.SslDomain)

		err, remoteReplies := database.ReadActivitiesByInReplyTo(canonicalURI)
		if err == nil && remoteReplies != nil {
			for _, activity := range *remoteReplies {
				// Skip if this is from a local user (already shown as local reply)
				if strings.HasPrefix(activity.ActorURI, localActorPrefix) {
					continue
				}

				// Skip if this activity is a duplicate of a local note
				if activity.ObjectURI != "" {
					dupErr, existingNote := database.ReadNoteByURI(activity.ObjectURI)
					if dupErr == nil && existingNote != nil {
						continue
					}
				}

				// Parse content and author info from activity
				replyContent, replyUsername, replyDomain, replyProfileURL := parseActivityContentForWeb(&activity, database)

				// Process content for display
				replyMessageHTML := util.MarkdownLinksToHTML(replyContent)
				replyMessageHTML = util.HighlightHashtagsHTML(replyMessageHTML)
				replyMessageHTML = util.HighlightMentionsHTML(replyMessageHTML, conf.Conf.SslDomain)

				// Get reply count for this remote reply (using object URI)
				replyReplyCount := 0
				if activity.ObjectURI != "" {
					if count, countErr := database.CountRepliesByURI(activity.ObjectURI); countErr == nil {
						replyReplyCount = count
					}
				}

				// Prefer ObjectURL (web UI link) over ObjectURI (ActivityPub id/JSON)
				postURL := activity.ObjectURL
				if postURL == "" {
					postURL = activity.ObjectURI
				}

				replies = append(replies, PostView{
					NoteId:      activity.Id.String(),
					Username:    replyUsername,
					UserDomain:  replyDomain,
					ProfileURL:  replyProfileURL,
					PostURL:     postURL,
					IsRemote:    true,
					Message:     replyContent,
					MessageHTML: template.HTML(replyMessageHTML),
					TimeAgo:     formatTimeAgo(activity.CreatedAt),
					CreatedAt:   activity.CreatedAt,
					ReplyCount:  replyReplyCount,
					LikeCount:   activity.LikeCount,
					BoostCount:  activity.BoostCount,
				})
			}
		}

		// Sort all replies chronologically
		sort.Slice(replies, func(i, j int) bool {
			return replies[i].CreatedAt.Before(replies[j].CreatedAt)
		})
	}

	// Load info boxes
	var infoBoxViews []InfoBoxView
	err, infoBoxes := database.ReadEnabledInfoBoxes()
	if err == nil && infoBoxes != nil {
		for _, box := range *infoBoxes {
			content := util.ReplacePlaceholders(box.Content, conf.Conf.SshPort)
			htmlContent := convertMarkdownToHTML(content)
			titleHTML := convertMarkdownToHTML(box.Title)
			infoBoxViews = append(infoBoxViews, InfoBoxView{
				Title:       box.Title,
				TitleHTML:   template.HTML(titleHTML),
				ContentHTML: template.HTML(htmlContent),
			})
		}
	}

	data := SinglePostPageData{
		Title:   fmt.Sprintf("@%s - %s", username, formatTimeAgo(note.CreatedAt)),
		Host:    host,
		SSHPort: conf.Conf.SshPort,
		Version: util.GetVersion(),
		Post:    post,
		User: UserView{
			Username:      account.Username,
			DisplayName:   account.DisplayName,
			Summary:       account.Summary,
			JoinedAgo:     formatTimeAgo(account.CreatedAt),
			AvatarURL:     account.AvatarURL,
			AvatarVersion: time.Now().Unix(),
		},
		ParentPost:    parentPost,
		Replies:       replies,
		InfoBoxes:     infoBoxViews,
		ServerMessage: loadServerMessageForWeb(),
	}

	c.HTML(200, "post.html", data)
}

func HandleTagFeed(c *gin.Context, conf *util.AppConfig) {
	tag := c.Param("tag")
	database := db.GetDB()

	// Pagination
	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	postsPerPage := 20
	offset := (page - 1) * postsPerPage

	// Get total count for pagination
	totalPosts, err := database.CountNotesByHashtag(tag)
	if err != nil {
		log.Printf("Failed to count notes for hashtag %s: %v", tag, err)
		totalPosts = 0
	}

	// Get notes with this hashtag
	err, notes := database.ReadNotesByHashtag(tag, postsPerPage, offset)
	if err != nil {
		log.Printf("Failed to read notes for hashtag %s: %v", tag, err)
		c.HTML(500, "base.html", gin.H{"Title": "Error", "Error": "Failed to load tagged posts"})
		return
	}

	if notes == nil {
		notes = &[]domain.Note{}
	}

	// Convert to PostView with hashtag-highlighted content
	posts := make([]PostView, 0, len(*notes))
	for _, note := range *notes {
		// First convert markdown links, then highlight hashtags and mentions
		messageHTML := util.MarkdownLinksToHTML(note.Message)
		messageHTML = util.HighlightHashtagsHTML(messageHTML)
		messageHTML = util.HighlightMentionsHTML(messageHTML, conf.Conf.SslDomain)

		// Get reply count for this post (including remote replies when AP is enabled)
		replyCount := countTotalRepliesForWeb(database, note.Id, conf.Conf.SslDomain, conf.Conf.WithAp)

		posts = append(posts, PostView{
			NoteId:      note.Id.String(),
			Username:    note.CreatedBy,
			Message:     note.Message,
			MessageHTML: template.HTML(messageHTML),
			TimeAgo:     formatTimeAgo(note.CreatedAt),
			ReplyCount:  replyCount,
			LikeCount:   note.LikeCount,
			BoostCount:  note.BoostCount,
		})
	}

	// Use SSLDomain if federation is enabled, otherwise use Host
	host := conf.Conf.Host
	if conf.Conf.WithAp {
		host = conf.Conf.SslDomain
	}

	end := offset + postsPerPage
	if end > totalPosts {
		end = totalPosts
	}

	// Load info boxes
	var infoBoxViews []InfoBoxView
	err, infoBoxes := database.ReadEnabledInfoBoxes()
	if err == nil && infoBoxes != nil {
		for _, box := range *infoBoxes {
			content := util.ReplacePlaceholders(box.Content, conf.Conf.SshPort)
			htmlContent := convertMarkdownToHTML(content)
			titleHTML := convertMarkdownToHTML(box.Title)
			infoBoxViews = append(infoBoxViews, InfoBoxView{
				Title:       box.Title,
				TitleHTML:   template.HTML(titleHTML),
				ContentHTML: template.HTML(htmlContent),
			})
		}
	}

	data := TagPageData{
		Title:         fmt.Sprintf("#%s", tag),
		Host:          host,
		SSHPort:       conf.Conf.SshPort,
		Version:       util.GetVersion(),
		Tag:           tag,
		Posts:         posts,
		TotalPosts:    totalPosts,
		HasPrev:       page > 1,
		HasNext:       end < totalPosts,
		PrevPage:      page - 1,
		NextPage:      page + 1,
		InfoBoxes:     infoBoxViews,
		ServerMessage: loadServerMessageForWeb(),
	}

	c.HTML(200, "tag.html", data)
}
