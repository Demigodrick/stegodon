# Note Limits

This document specifies character limits for notes.

---

## Overview

Stegodon has a two-tier character limit system:
- **Visible limit**: Configurable 1-300 characters (default: 150)
- **Database limit**: 1000 characters (includes markdown/URLs)

---

## Visible Character Limit

### Configuration

The visible character limit is configurable via environment variable or config file:

```go
// maxLetters is the maximum number of visible characters allowed in a note
// This value is loaded from configuration (with STEGODON_MAX_CHARS env var support)
var maxLetters int
```

### Configuration Options

| Method | Setting | Default |
|--------|---------|---------|
| Environment | `STEGODON_MAX_CHARS` | 150 |
| Config file | `maxChars` | 150 |

### Limits

| Constraint | Value |
|------------|-------|
| Minimum | 1 |
| Maximum | 300 |
| Default | 150 |

### Validation Logic

```go
if envMaxChars != "" {
    v, err := strconv.Atoi(envMaxChars)
    if err != nil {
        log.Printf("Error parsing STEGODON_MAX_CHARS: %v", err)
    } else {
        if v > 300 {
            log.Printf("STEGODON_MAX_CHARS value %d exceeds maximum of 300, capping at 300", v)
            c.Conf.MaxChars = 300
        } else if v < 1 {
            log.Printf("STEGODON_MAX_CHARS value %d is less than minimum of 1, setting to default 150", v)
            c.Conf.MaxChars = 150
        } else {
            c.Conf.MaxChars = v
        }
    }
}
```

### Purpose

The configurable visible limit:
- Encourages concise posts (default 150)
- Can be increased up to 300 for longer-form content
- Visible character count excludes URL portions of markdown links

### Counting Logic

```go
func CountVisibleChars(text string) int {
    // Strip ANSI escapes
    stripped := ansiEscapeRegex.ReplaceAllString(text, "")
    // Replace markdown links with just text
    result := markdownLinkRegex.ReplaceAllString(stripped, "$1")
    // Count Unicode runes
    return utf8.RuneCountInString(result)
}
```

### What Counts

| Content | Counted |
|---------|---------|
| Plain text | Yes |
| Markdown link text `[text](url)` | Only "text" |
| Hashtags | Yes |
| Mentions | Yes |
| Emojis | Yes (as runes) |
| ANSI escape codes | No |

### Examples

| Input | Visible Count |
|-------|---------------|
| `hello world` | 11 |
| `café` | 4 |
| `[Link](https://example.com/very/long/path)` | 4 |
| `Check [this](url) out` | 14 |

---

## Database Character Limit

### Constant

```go
const MaxNoteDBLength = 1000
```

### Purpose

The 1000-character database limit:
- Accommodates full markdown syntax
- Allows multiple long URLs in links
- Prevents database bloat

### Validation

```go
func ValidateNoteLength(text string) error {
    const maxDBLength = 1000

    if len(text) > maxDBLength {
        return fmt.Errorf("Note too long (max %d characters including links)", maxDBLength)
    }
    return nil
}
```

### What Counts

| Content | Counted |
|---------|---------|
| All text | Yes |
| Markdown syntax | Yes |
| URLs in links | Yes |
| Whitespace | Yes |

---

## Initialization

The limit is loaded from configuration at model initialization:

```go
func InitialNote(contentWidth int, userId uuid.UUID) Model {
    // Load configuration to get max characters setting
    if conf, err := util.ReadConf(); err == nil {
        maxLetters = conf.Conf.MaxChars
    } else {
        // Fallback to default if config can't be read
        maxLetters = 150
    }
    // ...
}
```

---

## Validation Flow

### On Save (Ctrl+S)

```go
case tea.KeyCtrlS:
    rawValue := m.Textarea.Value()

    // 1. Check not empty
    if len(strings.TrimSpace(rawValue)) == 0 {
        m.Error = "Cannot save an empty note"
        return m, nil
    }

    // 2. Validate visible characters ≤ maxChars
    visibleChars := util.CountVisibleChars(rawValue)
    if visibleChars > maxLetters {
        m.Error = fmt.Sprintf("Note too long (%d visible characters, max %d)", visibleChars, maxLetters)
        return m, nil
    }

    // 3. Validate full text ≤ 1000 (before normalization)
    if err := util.ValidateNoteLength(rawValue); err != nil {
        m.Error = err.Error()
        return m, nil
    }

    // 4. Normalize and save
    value := util.NormalizeInput(rawValue)
```

---

## TUI Character Counter

### Display

```
characters left: 142

post message: ctrl+s
```

### Calculation

```go
func (m Model) CharCount() int {
    visibleChars := util.CountVisibleChars(m.Textarea.Value())
    return maxLetters - visibleChars
}
```

### Counter Behavior

The counter shows remaining characters based on the configured `maxChars`:

| Scenario (maxChars=150) | Display |
|-------------------------|---------|
| Empty note | `characters left: 150` |
| 100 chars typed | `characters left: 50` |
| At limit | `characters left: 0` |
| Over limit | `characters left: -10` |

| Scenario (maxChars=300) | Display |
|-------------------------|---------|
| Empty note | `characters left: 300` |
| 100 chars typed | `characters left: 200` |
| At limit | `characters left: 0` |

Negative values indicate over-limit (save will fail).

---

## Textarea Configuration

### Setup

```go
ti := textarea.New()
ti.Placeholder = "enter your message"
ti.CharLimit = common.MaxNoteDBLength // 1000 - allows typing up to DB limit
ti.ShowLineNumbers = false
ti.SetWidth(common.TextInputDefaultWidth)
```

### Width

```go
const TextInputDefaultWidth = 30
```

Fixed width textarea regardless of terminal size.

---

## Error Messages

### Empty Note

```
Cannot save an empty note
```

### Visible Limit Exceeded

```
Note too long (165 visible characters, max 150)
```

Note: The `max` value shown reflects the configured `maxChars` setting (1-300).

### Database Limit Exceeded

```
Note too long (max 1000 characters including links)
```

---

## URL Auto-Conversion

### Behavior

When a URL is pasted alone, it auto-converts to markdown:

```go
currentValue := m.Textarea.Value()
if util.IsURL(strings.TrimSpace(currentValue)) {
    url := strings.TrimSpace(currentValue)
    markdown := fmt.Sprintf("[Link](%s)", url)
    m.Textarea.SetValue(markdown)
}
```

### Example

| Pasted | Result |
|--------|--------|
| `https://example.com/page` | `[Link](https://example.com/page)` |

### Benefit

- URL `https://example.com/page` = 26 visible chars
- Markdown `[Link](https://example.com/page)` = 4 visible chars

---

## Link Indicator

### Display

When markdown links are detected:

```
✓ 1 markdown link detected
✓ 2 markdown links detected
```

### Implementation

```go
if linkCount := util.GetMarkdownLinkCount(m.Textarea.Value()); linkCount > 0 {
    linkStyle := lipgloss.NewStyle().
        Foreground(lipgloss.Color(common.COLOR_SUCCESS)).
        PaddingLeft(5)
    plural := ""
    if linkCount > 1 {
        plural = "s"
    }
    linkIndicator = "\n" + linkStyle.Render(fmt.Sprintf("✓ %d markdown link%s detected", linkCount, plural))
}
```

---

## Content Truncation

### Display Truncation

```go
const MaxContentTruncateWidth = 150
```

Long content is truncated in timeline views:

```go
util.TruncateVisibleLength(content, common.MaxContentTruncateWidth)
```

### Truncation Output

```
This is a very long post that will be truncated...
```

Ellipsis (`...`) added at truncation point.

---

## Source Files

- `ui/writenote/writenote.go` - `maxLetters` variable, validation
- `ui/common/layout.go` - `MaxNoteDBLength`, `MaxContentTruncateWidth`
- `util/util.go` - `CountVisibleChars()`, `ValidateNoteLength()`
- `util/config.go` - `MaxChars` configuration field
- `util/config_default.yaml` - Default `maxChars: 150`
