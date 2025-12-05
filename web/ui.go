package web

import (
	"fmt"
	"html/template"
	"log"
	"strconv"
	"time"

	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type IndexPageData struct {
	Title    string
	Host     string
	SSHPort  int
	Version  string
	Posts    []PostView
	HasPrev  bool
	HasNext  bool
	PrevPage int
	NextPage int
}

type ProfilePageData struct {
	Title      string
	Host       string
	SSHPort    int
	Version    string
	User       UserView
	Posts      []PostView
	TotalPosts int
	HasPrev    bool
	HasNext    bool
	PrevPage   int
	NextPage   int
}

type UserView struct {
	Username    string
	DisplayName string
	Summary     string
	JoinedAgo   string
}

type PostView struct {
	NoteId      string
	Username    string
	Message     string
	MessageHTML template.HTML // HTML-rendered message with clickable links
	TimeAgo     string
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

	totalPosts := len(*notes)

	// Apply pagination
	start := offset
	end := offset + postsPerPage
	if start > totalPosts {
		start = totalPosts
	}
	if end > totalPosts {
		end = totalPosts
	}

	paginatedNotes := (*notes)[start:end]

	// Convert to PostView
	posts := make([]PostView, 0, len(paginatedNotes))
	for _, note := range paginatedNotes {
		// First convert markdown links, then highlight hashtags
		messageHTML := util.MarkdownLinksToHTML(note.Message)
		messageHTML = util.HighlightHashtagsHTML(messageHTML)

		posts = append(posts, PostView{
			NoteId:      note.Id.String(),
			Username:    note.CreatedBy,
			Message:     note.Message,
			MessageHTML: template.HTML(messageHTML),
			TimeAgo:     formatTimeAgo(note.CreatedAt),
		})
	}

	// Use SSLDomain if federation is enabled, otherwise use Host
	host := conf.Conf.Host
	if conf.Conf.WithAp {
		host = conf.Conf.SslDomain
	}

	data := IndexPageData{
		Title:    "Home",
		Host:     host,
		SSHPort:  conf.Conf.SshPort,
		Version:  util.GetVersion(),
		Posts:    posts,
		HasPrev:  page > 1,
		HasNext:  end < totalPosts,
		PrevPage: page - 1,
		NextPage: page + 1,
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

	totalPosts := len(*notes)

	// Apply pagination
	start := offset
	end := offset + postsPerPage
	if start > totalPosts {
		start = totalPosts
	}
	if end > totalPosts {
		end = totalPosts
	}

	paginatedNotes := (*notes)[start:end]

	// Convert to PostView
	posts := make([]PostView, 0, len(paginatedNotes))
	for _, note := range paginatedNotes {
		// First convert markdown links, then highlight hashtags
		messageHTML := util.MarkdownLinksToHTML(note.Message)
		messageHTML = util.HighlightHashtagsHTML(messageHTML)

		posts = append(posts, PostView{
			NoteId:      note.Id.String(),
			Username:    note.CreatedBy,
			Message:     note.Message,
			MessageHTML: template.HTML(messageHTML),
			TimeAgo:     formatTimeAgo(note.CreatedAt),
		})
	}

	// Use SSLDomain if federation is enabled, otherwise use Host
	host := conf.Conf.Host
	if conf.Conf.WithAp {
		host = conf.Conf.SslDomain
	}

	data := ProfilePageData{
		Title:   fmt.Sprintf("@%s", username),
		Host:    host,
		SSHPort: conf.Conf.SshPort,
		Version: util.GetVersion(),
		User: UserView{
			Username:    account.Username,
			DisplayName: account.DisplayName,
			Summary:     account.Summary,
			JoinedAgo:   formatTimeAgo(account.CreatedAt),
		},
		Posts:      posts,
		TotalPosts: totalPosts,
		HasPrev:    page > 1,
		HasNext:    end < totalPosts,
		PrevPage:   page - 1,
		NextPage:   page + 1,
	}

	c.HTML(200, "profile.html", data)
}

type SinglePostPageData struct {
	Title   string
	Host    string
	SSHPort int
	Version string
	Post    PostView
	User    UserView
}

type TagPageData struct {
	Title      string
	Host       string
	SSHPort    int
	Version    string
	Tag        string
	Posts      []PostView
	TotalPosts int
	HasPrev    bool
	HasNext    bool
	PrevPage   int
	NextPage   int
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

	// First convert markdown links, then highlight hashtags
	messageHTML := util.MarkdownLinksToHTML(note.Message)
	messageHTML = util.HighlightHashtagsHTML(messageHTML)

	post := PostView{
		NoteId:      note.Id.String(),
		Username:    note.CreatedBy,
		Message:     note.Message,
		MessageHTML: template.HTML(messageHTML),
		TimeAgo:     formatTimeAgo(note.CreatedAt),
	}

	data := SinglePostPageData{
		Title:   fmt.Sprintf("@%s - %s", username, formatTimeAgo(note.CreatedAt)),
		Host:    host,
		SSHPort: conf.Conf.SshPort,
		Version: util.GetVersion(),
		Post:    post,
		User: UserView{
			Username:    account.Username,
			DisplayName: account.DisplayName,
			Summary:     account.Summary,
			JoinedAgo:   formatTimeAgo(account.CreatedAt),
		},
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
		// First convert markdown links, then highlight hashtags
		messageHTML := util.MarkdownLinksToHTML(note.Message)
		messageHTML = util.HighlightHashtagsHTML(messageHTML)

		posts = append(posts, PostView{
			NoteId:      note.Id.String(),
			Username:    note.CreatedBy,
			Message:     note.Message,
			MessageHTML: template.HTML(messageHTML),
			TimeAgo:     formatTimeAgo(note.CreatedAt),
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

	data := TagPageData{
		Title:      fmt.Sprintf("#%s", tag),
		Host:       host,
		SSHPort:    conf.Conf.SshPort,
		Version:    util.GetVersion(),
		Tag:        tag,
		Posts:      posts,
		TotalPosts: totalPosts,
		HasPrev:    page > 1,
		HasNext:    end < totalPosts,
		PrevPage:   page - 1,
		NextPage:   page + 1,
	}

	c.HTML(200, "tag.html", data)
}
