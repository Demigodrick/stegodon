package web

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
)

// GetOutbox returns an ActivityPub OrderedCollection of a user's public posts
// This allows remote servers to discover posts without following the user
func GetOutbox(actor string, page int, conf *util.AppConfig) (error, string) {
	// Verify the account exists
	err, _ := db.GetDB().ReadAccByUsername(actor)
	if err != nil {
		log.Printf("GetOutbox: User %s not found: %v", actor, err)
		return err, "{}"
	}

	baseURL := fmt.Sprintf("https://%s", conf.Conf.SslDomain)
	outboxURL := fmt.Sprintf("%s/users/%s/outbox", baseURL, actor)

	// If no page parameter, return the collection metadata
	if page == 0 {
		// Count total public posts
		err, notes := db.GetDB().ReadPublicNotesByUsername(actor, 999999, 0)
		if err != nil {
			log.Printf("GetOutbox: Failed to count notes for %s: %v", actor, err)
			return err, "{}"
		}
		totalItems := 0
		if notes != nil {
			totalItems = len(*notes)
		}

		collection := map[string]any{
			"@context":   "https://www.w3.org/ns/activitystreams",
			"id":         outboxURL,
			"type":       "OrderedCollection",
			"totalItems": totalItems,
			"first":      fmt.Sprintf("%s?page=1", outboxURL),
		}

		jsonData, err := json.Marshal(collection)
		if err != nil {
			log.Printf("GetOutbox: Failed to marshal collection: %v", err)
			return err, "{}"
		}
		return nil, string(jsonData)
	}

	// Return a paginated collection page
	return getOutboxPage(actor, page, conf)
}

func getOutboxPage(actor string, page int, conf *util.AppConfig) (error, string) {
	itemsPerPage := 20
	offset := (page - 1) * itemsPerPage

	// Fetch notes for this page
	err, notes := db.GetDB().ReadPublicNotesByUsername(actor, itemsPerPage+1, offset)
	if err != nil {
		log.Printf("GetOutbox: Failed to fetch notes page %d for %s: %v", page, actor, err)
		return err, "{}"
	}

	baseURL := fmt.Sprintf("https://%s", conf.Conf.SslDomain)
	outboxURL := fmt.Sprintf("%s/users/%s/outbox", baseURL, actor)
	pageURL := fmt.Sprintf("%s?page=%d", outboxURL, page)

	// Check if there are more items
	hasMore := false
	items := []any{}
	hasHashtags := false

	if notes != nil {
		// Check if any notes have hashtags
		for _, note := range *notes {
			if len(util.ParseHashtags(note.Message)) > 0 {
				hasHashtags = true
				break
			}
		}

		if len(*notes) > itemsPerPage {
			hasMore = true
			// Trim the extra item
			pageNotes := (*notes)[:itemsPerPage]
			items = makeNoteActivities(pageNotes, actor, conf)
		} else {
			items = makeNoteActivities(*notes, actor, conf)
		}
	}

	// Build context - include Hashtag definition if any notes have hashtags
	var context any
	if hasHashtags {
		context = []any{
			"https://www.w3.org/ns/activitystreams",
			map[string]any{
				"Hashtag": "as:Hashtag",
			},
		}
	} else {
		context = "https://www.w3.org/ns/activitystreams"
	}

	collectionPage := map[string]any{
		"@context":     context,
		"id":           pageURL,
		"type":         "OrderedCollectionPage",
		"partOf":       outboxURL,
		"orderedItems": items,
	}

	// Add next link if there are more pages
	if hasMore {
		collectionPage["next"] = fmt.Sprintf("%s?page=%d", outboxURL, page+1)
	}

	// Add prev link if not first page
	if page > 1 {
		collectionPage["prev"] = fmt.Sprintf("%s?page=%d", outboxURL, page-1)
	}

	jsonData, err := json.Marshal(collectionPage)
	if err != nil {
		log.Printf("GetOutbox: Failed to marshal collection page: %v", err)
		return err, "{}"
	}
	return nil, string(jsonData)
}

// makeNoteActivities converts domain.Note objects to ActivityPub Create activities
func makeNoteActivities(notes []domain.Note, actor string, conf *util.AppConfig) []any {
	activities := make([]any, 0, len(notes))
	baseURL := fmt.Sprintf("https://%s", conf.Conf.SslDomain)
	database := db.GetDB()

	for _, note := range notes {
		// Use object_uri if available, otherwise generate one
		objectURI := note.ObjectURI
		if objectURI == "" {
			objectURI = fmt.Sprintf("%s/notes/%s", baseURL, note.Id.String())
		}

		// Convert Markdown links and raw URLs to HTML for ActivityPub content
		contentHTML := util.MarkdownLinksToHTML(note.Message)
		contentHTML = util.LinkifyRawURLsHTML(contentHTML)
		// Convert hashtags to ActivityPub-compliant HTML links
		contentHTML = util.HashtagsToActivityPubHTML(contentHTML, baseURL)

		// Build cc list - start with followers
		ccList := []string{
			fmt.Sprintf("%s/users/%s/followers", baseURL, actor),
		}

		// Build tag array for hashtags and mentions
		tags := make([]map[string]any, 0)

		// Extract hashtags and add to tag array
		hashtags := util.ParseHashtags(note.Message)
		for _, tag := range hashtags {
			tags = append(tags, map[string]any{
				"type": "Hashtag",
				"href": fmt.Sprintf("%s/tags/%s", baseURL, tag),
				"name": "#" + tag,
			})
		}

		// Extract mentions and try to resolve from stored data
		mentionURIs := make(map[string]string)
		err, storedMentions := database.ReadMentionsByNoteId(note.Id)
		if err == nil && len(storedMentions) > 0 {
			for _, stored := range storedMentions {
				mentionKey := fmt.Sprintf("@%s@%s", stored.MentionedUsername, stored.MentionedDomain)
				mentionURIs[mentionKey] = stored.MentionedActorURI
				ccList = append(ccList, stored.MentionedActorURI)
				tags = append(tags, map[string]any{
					"type": "Mention",
					"href": stored.MentionedActorURI,
					"name": mentionKey,
				})
			}
		} else {
			// Fall back to WebFinger resolution for mentions not in database
			mentions := util.ParseMentions(note.Message)
			for _, mention := range mentions {
				// Skip local mentions
				if strings.EqualFold(mention.Domain, conf.Conf.SslDomain) {
					continue
				}

				// Try to resolve via WebFinger
				resolvedURI, err := ResolveWebFinger(mention.Username, mention.Domain)
				if err != nil {
					continue
				}

				mentionKey := fmt.Sprintf("@%s@%s", mention.Username, mention.Domain)
				mentionURIs[mentionKey] = resolvedURI
				ccList = append(ccList, resolvedURI)
				tags = append(tags, map[string]any{
					"type": "Mention",
					"href": resolvedURI,
					"name": mentionKey,
				})
			}
		}

		// Convert mentions to ActivityPub HTML
		if len(mentionURIs) > 0 {
			contentHTML = util.MentionsToActivityPubHTML(contentHTML, mentionURIs)
		}

		// Build the Note object
		noteObj := map[string]any{
			"id":           objectURI,
			"type":         "Note",
			"attributedTo": fmt.Sprintf("%s/users/%s", baseURL, actor),
			"content":      contentHTML,
			"mediaType":    "text/html",
			"published":    note.CreatedAt.Format("2006-01-02T15:04:05Z"),
			"url":          fmt.Sprintf("%s/u/%s/%s", baseURL, actor, note.Id.String()),
			"to": []string{
				"https://www.w3.org/ns/activitystreams#Public",
			},
			"cc": ccList,
		}

		// Add updated field if note was edited
		if note.EditedAt != nil {
			noteObj["updated"] = note.EditedAt.Format("2006-01-02T15:04:05Z")
		}

		// Add tag array if we have hashtags or mentions
		if len(tags) > 0 {
			noteObj["tag"] = tags
		}

		// Build the Create activity wrapping the Note
		// Use note URI with #activity fragment so activity ID resolves to the note
		activityURI := fmt.Sprintf("%s/notes/%s#activity", baseURL, note.Id.String())
		activity := map[string]any{
			"id":        activityURI,
			"type":      "Create",
			"actor":     fmt.Sprintf("%s/users/%s", baseURL, actor),
			"published": note.CreatedAt.Format("2006-01-02T15:04:05Z"),
			"to": []string{
				"https://www.w3.org/ns/activitystreams#Public",
			},
			"cc":     ccList,
			"object": noteObj,
		}

		activities = append(activities, activity)
	}

	return activities
}

// ParsePageParam extracts the page parameter from a query string
func ParsePageParam(pageStr string) int {
	if pageStr == "" {
		return 0
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 0 {
		return 0
	}
	return page
}
