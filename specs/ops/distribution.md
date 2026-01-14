# Single Binary Distribution

This document specifies embedded assets and single binary distribution.

---

## Overview

Stegodon compiles to a single binary with all assets embedded:
- HTML templates
- Static files (CSS, images)
- Default configuration
- Version information

No external files required at runtime.

---

## Embedded Assets

### Go embed Directive

```go
import "embed"
```

Assets are embedded at compile time using `//go:embed` directives.

---

## Template Embedding

### Location

```
web/templates/*.html
```

### Embed Directive

```go
//go:embed templates/*.html
var embeddedTemplates embed.FS
```

### Templates

| File | Purpose |
|------|---------|
| `index.html` | Home timeline |
| `profile.html` | User profile |
| `post.html` | Single post view |
| `tag.html` | Hashtag feed |

### Usage

```go
tmpl, err := template.ParseFS(embeddedTemplates, "templates/*.html")
g.SetHTMLTemplate(tmpl)
```

---

## Static Asset Embedding

### Logo

```go
//go:embed static/stegologo.png
var embeddedLogo []byte
```

Served at `/static/stegologo.png` with 24-hour cache.

### Stylesheet

```go
//go:embed static/style.css
var embeddedCSS []byte
```

Served at `/static/style.css`.

### Serving

```go
g.GET("/static/stegologo.png", func(c *gin.Context) {
    c.Header("Content-Type", "image/png")
    c.Header("Cache-Control", "public, max-age=86400")
    c.Data(200, "image/png", embeddedLogo)
})

g.GET("/static/style.css", func(c *gin.Context) {
    c.Header("Content-Type", "text/css; charset=utf-8")
    c.Data(200, "text/css; charset=utf-8", embeddedCSS)
})
```

---

## Configuration Embedding

### Default Config

```go
//go:embed config_default.yaml
var embeddedConfig []byte
```

### Fallback Behavior

```go
func ReadConf() (*AppConfig, error) {
    buf, err := os.ReadFile(configPath)
    if err != nil {
        // Use embedded defaults
        buf = embeddedConfig

        // Create config file for user
        os.WriteFile(userConfigPath, embeddedConfig, 0644)
    }
    // Parse and apply env overrides
}
```

---

## Version Embedding

### Version File

```
util/version.txt
```

Contains version string (e.g., `1.4.3`).

### Embed Directive

```go
//go:embed version.txt
var embeddedVersion string
```

### Access Functions

```go
func GetVersion() string {
    return strings.TrimSpace(embeddedVersion)
}

func GetNameAndVersion() string {
    return fmt.Sprintf("%s / %s", Name, GetVersion())
}
```

---

## Build Process

### Standard Build

```bash
go build -o stegodon .
```

### Optimized Build

```bash
go build -ldflags="-s -w" -o stegodon .
```

| Flag | Effect |
|------|--------|
| `-s` | Omit symbol table |
| `-w` | Omit DWARF debug info |

### Cross-Compilation

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o stegodon-linux-amd64

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o stegodon-linux-arm64

# macOS AMD64
GOOS=darwin GOARCH=amd64 go build -o stegodon-darwin-amd64

# macOS ARM64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o stegodon-darwin-arm64
```

### CGO-Free Build

```bash
CGO_ENABLED=0 go build -o stegodon .
```

Pure Go build with no C dependencies (including SQLite via `modernc.org/sqlite`).

---

## Binary Contents

### Embedded at Compile Time

| Category | Files |
|----------|-------|
| Templates | 4 HTML files |
| Static | 1 CSS, 1 PNG |
| Config | 1 YAML |
| Version | 1 text file |

### Runtime Generated

| File | Location | Content |
|------|----------|---------|
| Database | `~/.config/stegodon/database.db` | SQLite |
| SSH Key | `~/.config/stegodon/.ssh/stegodonhostkey` | Host key |
| Config | `~/.config/stegodon/config.yaml` | User config |

---

## File Path Resolution

### Priority Order

1. Current directory (`./`)
2. User config directory (`~/.config/stegodon/`)

### ResolveFilePath Function

```go
func ResolveFilePath(filename string) string {
    // Check current directory first
    if _, err := os.Stat(filename); err == nil {
        return filename
    }

    // Check user config directory
    configDir, err := GetConfigDir()
    if err == nil {
        userPath := filepath.Join(configDir, filename)
        if _, err := os.Stat(userPath); err == nil {
            return userPath
        }
    }

    // Default to user config directory
    return filepath.Join(configDir, filename)
}
```

---

## Distribution Methods

### Direct Download

Single binary per platform:
- `stegodon-linux-amd64`
- `stegodon-linux-arm64`
- `stegodon-darwin-amd64`
- `stegodon-darwin-arm64`

### Docker

Pre-built images at `ghcr.io/deemkeen/stegodon:latest`.

### Source Build

```bash
git clone https://github.com/deemkeen/stegodon
cd stegodon
go build
./stegodon
```

---

## Runtime Requirements

### None Required

The binary is self-contained. No external dependencies needed.

### Optional

| Item | Purpose |
|------|---------|
| Config file | Custom configuration |
| Reverse proxy | HTTPS for ActivityPub |

---

## First Run Behavior

### Auto-Generated Files

1. **SSH Host Key** - Generated on first SSH connection
2. **Database** - Created on first run
3. **Config File** - Created from embedded defaults

### Directory Creation

```go
configDir, _ := GetConfigDir()
os.MkdirAll(configDir, 0755)
```

Creates `~/.config/stegodon/` if it doesn't exist.

---

## Upgrade Path

### Binary Replacement

1. Stop stegodon
2. Replace binary
3. Start stegodon

Database migrations run automatically on startup.

### Data Preservation

All data in `~/.config/stegodon/` is preserved across binary updates:
- Database with user data
- SSH host key (consistent fingerprint)
- Configuration

---

## Size Optimization

### Techniques

| Technique | Size Reduction |
|-----------|---------------|
| `-ldflags="-s -w"` | ~30% |
| Minimal templates | Embedded size |
| Single CSS file | No frameworks |

### Typical Binary Size

~15-20 MB (platform dependent)

---

## Source Files

- `web/router.go` - Template/asset embedding
- `util/config.go` - Config embedding
- `util/util.go` - Version embedding
- `util/paths.go` - Path resolution
- `Dockerfile` - Build configuration
