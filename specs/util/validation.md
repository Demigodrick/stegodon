# Validation

This document specifies username, domain, and configuration validation utilities.

---

## Overview

Stegodon validates:
- Usernames for WebFinger/ActivityPub compatibility
- Configuration values from files and environment
- Input normalization for security

---

## Username Validation

### IsValidWebFingerUsername

Validates usernames for WebFinger/ActivityPub requirements.

```go
var webFingerValidCharsRegex = regexp.MustCompile(`^[A-Za-z0-9\-._~!$&'()*+,;=]+$`)

func IsValidWebFingerUsername(username string) (bool, string) {
    if len(username) == 0 {
        return false, "Username must be at least 1 character"
    }

    if !webFingerValidCharsRegex.MatchString(username) {
        return false, "Username contains invalid characters. Only A-Z, a-z, 0-9, and -._~!$&'()*+,;= are allowed"
    }

    for _, r := range username {
        if unicode.IsControl(r) || !unicode.IsPrint(r) {
            return false, "Username contains non-printable characters"
        }
    }

    return true, ""
}
```

### Allowed Characters

WebFinger allows these characters without percent-encoding:

| Category | Characters |
|----------|------------|
| Alphanumeric | `A-Z a-z 0-9` |
| Special | `- . _ ~ ! $ & ' ( ) * + , ; =` |

### Rejected Characters

| Type | Example | Reason |
|------|---------|--------|
| Unicode | `ä`, `字`, `🔥` | Requires percent-encoding |
| Spaces | ` ` | Not in allowed set |
| Control | `\n`, `\t` | Non-printable |
| Other special | `@`, `#`, `/` | Not in allowed set |

### Validation Flow

```
1. Check minimum length (≥1)
2. Check character set (regex)
3. Check for control characters
4. Return (valid, error message)
```

---

## Configuration System

### AppConfig Structure

```go
type AppConfig struct {
    Conf struct {
        Host            string
        SshPort         int    `yaml:"sshPort"`
        HttpPort        int    `yaml:"httpPort"`
        SslDomain       string `yaml:"sslDomain"`
        WithAp          bool   `yaml:"withAp"`
        Single          bool   `yaml:"single"`
        Closed          bool   `yaml:"closed"`
        NodeDescription string `yaml:"nodeDescription"`
        WithJournald    bool   `yaml:"withJournald"`
        WithPprof       bool   `yaml:"withPprof"`
    }
}
```

### Configuration Sources

Priority order (later overrides earlier):

1. **Embedded defaults** (`config_default.yaml`)
2. **Config file** (`config.yaml`)
3. **Environment variables**

---

## Environment Variables

| Variable | Type | Description |
|----------|------|-------------|
| `STEGODON_HOST` | string | Server bind address |
| `STEGODON_SSHPORT` | int | SSH server port |
| `STEGODON_HTTPPORT` | int | HTTP server port |
| `STEGODON_SSLDOMAIN` | string | Public domain for federation |
| `STEGODON_WITH_AP` | bool | Enable ActivityPub |
| `STEGODON_SINGLE` | bool | Single-user mode |
| `STEGODON_CLOSED` | bool | Close registration |
| `STEGODON_NODE_DESCRIPTION` | string | NodeInfo description |
| `STEGODON_WITH_JOURNALD` | bool | Linux journald logging |
| `STEGODON_WITH_PPROF` | bool | Enable pprof profiling |

### Boolean Parsing

Only `"true"` is truthy:

```go
if envWithAp == "true" {
    c.Conf.WithAp = true
}
```

### Integer Parsing

```go
if envSshPort != "" {
    v, err := strconv.Atoi(envSshPort)
    if err != nil {
        log.Printf("Error parsing STEGODON_SSHPORT: %v", err)
    }
    c.Conf.SshPort = v
}
```

Errors are logged but don't prevent startup.

---

## Configuration Loading

### ReadConf Function

```go
func ReadConf() (*AppConfig, error) {
    c := &AppConfig{}

    // Try to resolve config file path
    configPath := ResolveFilePath(ConfigFileName)

    buf, err := os.ReadFile(configPath)
    if err != nil {
        // Use embedded defaults
        log.Printf("Config file not found, using embedded defaults")
        buf = embeddedConfig

        // Create default config file
        configDir, _ := GetConfigDir()
        os.WriteFile(configDir+"/config.yaml", embeddedConfig, 0644)
    }

    // Parse YAML
    yaml.Unmarshal(buf, c)

    // Apply environment overrides
    // ...

    return c, nil
}
```

### Config File Paths

Checked in order:

1. `./config.yaml` (current directory)
2. `~/.config/stegodon/config.yaml` (user config)

### Default Config Creation

If no config file exists, embedded defaults are:
1. Used for current session
2. Written to user config directory

---

## Default Values

From `config_default.yaml`:

| Setting | Default |
|---------|---------|
| `host` | `127.0.0.1` |
| `sshPort` | `23232` |
| `httpPort` | `9999` |
| `sslDomain` | `example.com` |
| `withAp` | `false` |
| `single` | `false` |
| `closed` | `false` |

---

## Input Normalization

### NormalizeInput Function

```go
func NormalizeInput(text string) string {
    normalized := strings.ReplaceAll(text, "\n", " ")
    normalized = html.EscapeString(normalized)
    return normalized
}
```

| Transform | Description |
|-----------|-------------|
| Newlines → spaces | Prevents line injection |
| HTML escape | Prevents XSS |

### HTML Entity Escaping

```go
html.EscapeString(text)
```

| Input | Output |
|-------|--------|
| `<` | `&lt;` |
| `>` | `&gt;` |
| `&` | `&amp;` |
| `"` | `&#34;` |
| `'` | `&#39;` |

---

## Note Length Validation

### ValidateNoteLength

```go
func ValidateNoteLength(text string) error {
    const maxDBLength = 1000

    if len(text) > maxDBLength {
        return fmt.Errorf("Note too long (max %d characters including links)", maxDBLength)
    }
    return nil
}
```

### Length Limits

| Limit | Value | Purpose |
|-------|-------|---------|
| Visible display | 150 chars | TUI display |
| Database storage | 1000 chars | Full text with links |

The 150-char visible limit is enforced in the TUI; the 1000-char limit protects the database.

---

## Error Messages

### Username Validation Errors

| Error | Message |
|-------|---------|
| Empty | "Username must be at least 1 character" |
| Invalid chars | "Username contains invalid characters. Only A-Z, a-z, 0-9, and -._~!$&'()*+,;= are allowed" |
| Control chars | "Username contains non-printable characters" |

### Configuration Errors

| Error | Log Message |
|-------|-------------|
| Missing file | "Config file not found at {path}, using embedded defaults" |
| Parse error | "in config file: {error}" |
| Port parse | "Error parsing STEGODON_SSHPORT: {error}" |

---

## Constants

```go
const Name = "stegodon"
const ConfigFileName = "config.yaml"
```

---

## Source Files

- `util/validation.go` - Username validation
- `util/config.go` - Configuration loading
- `util/util.go` - Input normalization, note validation
- `util/config_default.yaml` - Embedded defaults
- `util/paths.go` - File path resolution
