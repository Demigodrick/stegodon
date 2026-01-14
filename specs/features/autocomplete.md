# Mention Autocomplete

This document specifies the @mention autocomplete functionality in the note composition view.

---

## Overview

Stegodon provides real-time autocomplete suggestions when typing `@` mentions:
- Loads all local and remote accounts
- Filters candidates as user types
- Supports keyboard navigation
- Inserts full `@username@domain` format

---

## Data Structures

### MentionCandidate

```go
type MentionCandidate struct {
    Username string
    Domain   string
    IsLocal  bool
}

func (m MentionCandidate) FullMention() string {
    return fmt.Sprintf("@%s@%s", m.Username, m.Domain)
}

func (m MentionCandidate) DisplayMention() string {
    if m.IsLocal {
        return fmt.Sprintf("@%s", m.Username)
    }
    return fmt.Sprintf("@%s@%s", m.Username, m.Domain)
}
```

### Model Fields

```go
type Model struct {
    // ... other fields
    showAutocomplete       bool               // Popup visible
    autocompleteCandidates []MentionCandidate // All available
    filteredCandidates     []MentionCandidate // Filtered by input
    autocompleteIndex      int                // Selected suggestion
    mentionStartPos        int                // Position of @
    localDomain            string             // For identifying local users
}
```

---

## Configuration

### Max Suggestions

```go
const maxAutocompleteSuggestions = 5
```

Only top 5 matching candidates are shown.

---

## Loading Candidates

### On Initialization

```go
func InitialNote(contentWidth int, userId uuid.UUID) Model {
    // Get local domain
    localDomain := "example.com"
    if conf, err := util.ReadConf(); err == nil {
        localDomain = conf.Conf.SslDomain
    }

    // Load all candidates
    candidates := loadAutocompleteCandidates(localDomain)

    return Model{
        autocompleteCandidates: candidates,
        localDomain:            localDomain,
        // ...
    }
}
```

### Loading Function

```go
func loadAutocompleteCandidates(localDomain string) []MentionCandidate {
    var candidates []MentionCandidate
    database := db.GetDB()

    // Load local accounts
    err, localAccounts := database.ReadAllAccounts()
    if err == nil && localAccounts != nil {
        for _, acc := range *localAccounts {
            candidates = append(candidates, MentionCandidate{
                Username: acc.Username,
                Domain:   localDomain,
                IsLocal:  true,
            })
        }
    }

    // Load remote accounts (cached from federation)
    err, remoteAccounts := database.ReadAllRemoteAccounts()
    if err == nil && remoteAccounts != nil {
        for _, acc := range remoteAccounts {
            candidates = append(candidates, MentionCandidate{
                Username: acc.Username,
                Domain:   acc.Domain,
                IsLocal:  false,
            })
        }
    }

    return candidates
}
```

---

## Trigger Detection

### Finding Current Mention

```go
func (m *Model) findCurrentMention(text string, cursorPos int) (int, string) {
    if cursorPos == 0 || len(text) == 0 {
        return -1, ""
    }

    runes := []rune(text)
    if cursorPos > len(runes) {
        cursorPos = len(runes)
    }

    // Search backwards for @
    for i := cursorPos - 1; i >= 0; i-- {
        r := runes[i]

        // Stop at whitespace
        if r == ' ' || r == '\n' || r == '\t' {
            return -1, ""
        }

        // Found @
        if r == '@' {
            // Check if at start or after whitespace
            if i == 0 || runes[i-1] == ' ' || runes[i-1] == '\n' || runes[i-1] == '\t' {
                query := string(runes[i+1 : cursorPos])
                return i, query
            }
            return -1, ""
        }
    }

    return -1, ""
}
```

### Update Autocomplete State

```go
func (m *Model) updateAutocomplete() {
    value := m.Textarea.Value()
    cursorPos := len([]rune(value)) // Assume cursor at end

    mentionStart, query := m.findCurrentMention(value, cursorPos)

    if mentionStart >= 0 {
        m.mentionStartPos = mentionStart
        m.filterCandidates(query)
        m.showAutocomplete = len(m.filteredCandidates) > 0
        m.autocompleteIndex = 0
    } else {
        m.showAutocomplete = false
        m.mentionStartPos = -1
    }
}
```

---

## Filtering

### Filter Function

```go
func (m *Model) filterCandidates(query string) {
    query = strings.ToLower(query)
    m.filteredCandidates = nil

    for _, candidate := range m.autocompleteCandidates {
        username := strings.ToLower(candidate.Username)
        fullMention := strings.ToLower(candidate.FullMention())

        if strings.HasPrefix(username, query) || strings.Contains(fullMention, query) {
            m.filteredCandidates = append(m.filteredCandidates, candidate)
            if len(m.filteredCandidates) >= maxAutocompleteSuggestions {
                break
            }
        }
    }
}
```

### Matching Rules

| Query | Matches |
|-------|---------|
| `al` | `alice`, `alfred`, `alice@remote.com` |
| `alice@` | `alice@local.com`, `alice@remote.com` |
| `remote` | `bob@remote.com`, `alice@remote.com` |

---

## Keyboard Navigation

### Key Bindings

```go
if m.showAutocomplete {
    switch msg.Type {
    case tea.KeyUp:
        if m.autocompleteIndex > 0 {
            m.autocompleteIndex--
        }
        return m, nil
    case tea.KeyDown:
        if m.autocompleteIndex < len(m.filteredCandidates)-1 {
            m.autocompleteIndex++
        }
        return m, nil
    case tea.KeyTab, tea.KeyEnter:
        if len(m.filteredCandidates) > 0 {
            m.insertAutocompleteSuggestion()
        }
        m.showAutocomplete = false
        return m, nil
    case tea.KeyEsc:
        m.showAutocomplete = false
        return m, nil
    }
}
```

### Key Actions

| Key | Action |
|-----|--------|
| `↑` / `Up` | Previous suggestion |
| `↓` / `Down` | Next suggestion |
| `Tab` / `Enter` | Insert selected |
| `Esc` | Close popup |

---

## Insertion

### Insert Selected Suggestion

```go
func (m *Model) insertAutocompleteSuggestion() {
    if m.autocompleteIndex >= len(m.filteredCandidates) {
        return
    }

    selected := m.filteredCandidates[m.autocompleteIndex]
    value := m.Textarea.Value()

    runes := []rune(value)
    cursorPos := len(runes)

    // Build new text
    before := string(runes[:m.mentionStartPos])
    mention := selected.FullMention() + " " // Add trailing space
    after := ""
    if cursorPos < len(runes) {
        after = string(runes[cursorPos:])
    }

    newValue := before + mention + after
    m.Textarea.SetValue(newValue)
    m.Textarea.CursorEnd()

    m.lettersLeft = m.CharCount()
}
```

### Insertion Format

Always inserts full format: `@username@domain `

| Input | Inserted |
|-------|----------|
| `@al` (select alice local) | `@alice@local.com ` |
| `@bo` (select bob@remote) | `@bob@remote.com ` |

---

## Popup Rendering

### Render Function

```go
func (m Model) renderAutocompletePopup() string {
    if len(m.filteredCandidates) == 0 {
        return ""
    }

    popupStyle := lipgloss.NewStyle().
        PaddingLeft(5).
        PaddingRight(2)

    normalStyle := lipgloss.NewStyle().
        Foreground(lipgloss.Color(common.COLOR_LIGHT))

    selectedStyle := lipgloss.NewStyle().
        Foreground(lipgloss.Color(common.COLOR_BLACK)).
        Background(lipgloss.Color(common.COLOR_BUTTON)).
        Bold(true)

    localBadgeStyle := lipgloss.NewStyle().
        Foreground(lipgloss.Color(common.COLOR_DIM)).
        Italic(true)

    var lines []string
    for i, candidate := range m.filteredCandidates {
        mention := candidate.DisplayMention()
        badge := ""
        if candidate.IsLocal {
            badge = localBadgeStyle.Render(" (local)")
        }

        line := mention + badge
        if i == m.autocompleteIndex {
            line = selectedStyle.Render(mention) + badge
        } else {
            line = normalStyle.Render(mention) + badge
        }
        lines = append(lines, line)
    }

    return popupStyle.Render(strings.Join(lines, "\n"))
}
```

### Visual Output

```
     @alice (local)
     @bob@mastodon.social
   › @carol@remote.server    ← selected (highlighted)
     @dave (local)
```

---

## Help Text

When autocomplete is visible:

```go
if m.showAutocomplete {
    helpText += "\n↑/↓: navigate, enter: select, esc: close"
}
```

---

## Character Count Impact

Mentions count toward visible character limit:

| Mention | Character Count |
|---------|-----------------|
| `@alice@local.com` | 17 |
| `@bob@mastodon.social` | 21 |

---

## Source Files

- `ui/writenote/writenote.go` - Full autocomplete implementation
- `db/db.go` - `ReadAllAccounts()`, `ReadAllRemoteAccounts()`
- `ui/common/styles.go` - Color constants for popup
