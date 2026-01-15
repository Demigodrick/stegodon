package domain

import (
	"time"

	"github.com/google/uuid"
)

// InfoBox represents a customizable information box shown on the web index page
type InfoBox struct {
	Id        uuid.UUID `json:"id"`
	Title     string    `json:"title"`      // Title of the info box (supports HTML for icons)
	Content   string    `json:"content"`    // Content in markdown format
	OrderNum  int       `json:"order_num"`  // Display order (lower numbers first)
	Enabled   bool      `json:"enabled"`    // Whether this box is shown
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
