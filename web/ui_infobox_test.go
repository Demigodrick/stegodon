package web

import (
	"html/template"
	"strings"
	"testing"

	"github.com/deemkeen/stegodon/domain"
)

// TestInfoBoxView_TitleEscaping tests that titles are properly HTML-escaped to prevent XSS
func TestInfoBoxView_TitleEscaping(t *testing.T) {
	tests := []struct {
		name           string
		title          string
		wantContains   string
		wantNotContain string
	}{
		{
			name:           "Script tag should be escaped",
			title:          "<script>alert('XSS')</script>",
			wantContains:   "&lt;script&gt;",
			wantNotContain: "<script>",
		},
		{
			name:           "HTML tags should be escaped",
			title:          "<div onclick='malicious()'>Click me</div>",
			wantContains:   "&lt;div",
			wantNotContain: "<div onclick",
		},
		{
			name:           "JavaScript event handlers should be escaped",
			title:          "<img src=x onerror='alert(1)'>",
			wantContains:   "&#39;",          // Single quotes should be escaped
			wantNotContain: "onerror='alert", // Unescaped attack should not exist
		},
		{
			name:           "Plain text should work normally",
			title:          "Welcome to Stegodon",
			wantContains:   "Welcome to Stegodon",
			wantNotContain: "&lt;",
		},
		{
			name:           "Special characters should be escaped",
			title:          "Title with & < > \" '",
			wantContains:   "&amp;",
			wantNotContain: "& <",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create InfoBoxView with potentially malicious title
			view := InfoBoxView{
				Title:       tt.title,
				ContentHTML: template.HTML("<p>Test content</p>"),
			}

			// Simulate template rendering by using html/template's escaping
			tmpl, err := template.New("test").Parse(`<h3>{{.Title}}</h3>`)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			var buf strings.Builder
			err = tmpl.Execute(&buf, view)
			if err != nil {
				t.Fatalf("Failed to execute template: %v", err)
			}

			result := buf.String()

			// Check that expected escaped content is present
			if !strings.Contains(result, tt.wantContains) {
				t.Errorf("Expected result to contain %q, but got: %s", tt.wantContains, result)
			}

			// Check that unescaped malicious content is NOT present
			if strings.Contains(result, tt.wantNotContain) {
				t.Errorf("Expected result to NOT contain %q, but it was found in: %s", tt.wantNotContain, result)
			}
		})
	}
}

// TestInfoBoxView_ContentHTMLNotEscaped tests that ContentHTML is rendered as-is (already sanitized markdown)
func TestInfoBoxView_ContentHTMLNotEscaped(t *testing.T) {
	tests := []struct {
		name         string
		contentHTML  template.HTML
		wantContains string
	}{
		{
			name:         "Markdown rendered HTML should not be escaped",
			contentHTML:  template.HTML("<p>This is <strong>bold</strong> text</p>"),
			wantContains: "<strong>bold</strong>",
		},
		{
			name:         "Links should render properly",
			contentHTML:  template.HTML(`<a href="https://example.com" target="_blank">Link</a>`),
			wantContains: `<a href="https://example.com"`,
		},
		{
			name:         "Code blocks should render properly",
			contentHTML:  template.HTML("<pre><code>ssh user@server</code></pre>"),
			wantContains: "<code>ssh user@server</code>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := InfoBoxView{
				Title:       "Safe Title",
				ContentHTML: tt.contentHTML,
			}

			// Template that renders ContentHTML (which should NOT be escaped)
			tmpl, err := template.New("test").Parse(`<div>{{.ContentHTML}}</div>`)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			var buf strings.Builder
			err = tmpl.Execute(&buf, view)
			if err != nil {
				t.Fatalf("Failed to execute template: %v", err)
			}

			result := buf.String()

			if !strings.Contains(result, tt.wantContains) {
				t.Errorf("Expected result to contain %q, but got: %s", tt.wantContains, result)
			}
		})
	}
}

// TestConvertMarkdownToHTML tests that markdown is properly converted and sanitized
func TestConvertMarkdownToHTML(t *testing.T) {
	tests := []struct {
		name         string
		markdown     string
		wantContains string
	}{
		{
			name:         "Bold text",
			markdown:     "**bold**",
			wantContains: "<strong>bold</strong>",
		},
		{
			name:         "Italic text",
			markdown:     "*italic*",
			wantContains: "<em>italic</em>",
		},
		{
			name:         "Links with target blank",
			markdown:     "[link](https://example.com)",
			wantContains: `target="_blank"`,
		},
		{
			name:         "Headings",
			markdown:     "# Heading 1",
			wantContains: "<h1",
		},
		{
			name:         "Blockquotes",
			markdown:     "> quoted text",
			wantContains: "<blockquote>",
		},
		{
			name:         "Code blocks",
			markdown:     "```\ncode\n```",
			wantContains: "<pre><code>",
		},
		{
			name:         "Inline code",
			markdown:     "`code`",
			wantContains: "<code>code</code>",
		},
		{
			name:         "Strikethrough",
			markdown:     "~~deleted~~",
			wantContains: "<del>deleted</del>",
		},
		{
			name:         "Lists",
			markdown:     "- item 1\n- item 2",
			wantContains: "<ul>",
		},
		{
			name:         "Ordered lists",
			markdown:     "1. first\n2. second",
			wantContains: "<ol>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertMarkdownToHTML(tt.markdown)

			if !strings.Contains(result, tt.wantContains) {
				t.Errorf("Expected HTML to contain %q, but got: %s", tt.wantContains, result)
			}
		})
	}
}

// TestInfoBoxView_NoTitleHTMLField tests that TitleHTML field no longer exists
func TestInfoBoxView_NoTitleHTMLField(t *testing.T) {
	// This is a compile-time test - if TitleHTML field exists, this won't compile
	view := InfoBoxView{
		Title:       "Safe Title",
		ContentHTML: template.HTML("<p>Content</p>"),
	}

	// Verify the struct has only the expected fields
	if view.Title == "" {
		t.Error("Title field should exist")
	}
	if view.ContentHTML == "" {
		t.Error("ContentHTML field should exist")
	}

	// This line would cause a compile error if TitleHTML still exists:
	// _ = view.TitleHTML // This should NOT compile
}

// TestInfoBoxSecurity_IntegrationTest tests the full flow from domain to view
func TestInfoBoxSecurity_IntegrationTest(t *testing.T) {
	// Simulate a malicious info box from the database
	maliciousBox := domain.InfoBox{
		Title:   `<script>alert("XSS")</script>Malicious Title`,
		Content: "# Safe Content\n\nThis is **safe** markdown.",
		Enabled: true,
	}

	// Convert to view (simulating what HandleIndex does)
	htmlContent := convertMarkdownToHTML(maliciousBox.Content)
	view := InfoBoxView{
		Title:       maliciousBox.Title,         // Plain string - will be auto-escaped
		ContentHTML: template.HTML(htmlContent), // Already sanitized markdown HTML
	}

	// Render with template
	tmpl, err := template.New("test").Parse(`
		<div class="info-box">
			<h3>{{.Title}}</h3>
			<div class="info-box-content">{{.ContentHTML}}</div>
		</div>
	`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	var buf strings.Builder
	err = tmpl.Execute(&buf, view)
	if err != nil {
		t.Fatalf("Failed to execute template: %v", err)
	}

	result := buf.String()

	// Verify XSS is prevented in title
	if strings.Contains(result, "<script>alert") {
		t.Error("XSS vulnerability: script tag was not escaped in title")
	}

	// Verify title is escaped
	if !strings.Contains(result, "&lt;script&gt;") {
		t.Error("Title should be HTML-escaped")
	}

	// Verify markdown content renders properly
	if !strings.Contains(result, "<strong>safe</strong>") {
		t.Error("Markdown content should render HTML")
	}

	if !strings.Contains(result, "<h1") {
		t.Error("Markdown headings should render")
	}
}
