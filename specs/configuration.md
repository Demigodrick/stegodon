# Configuration System

This document specifies the stegodon configuration system, including file-based configuration, environment variable overrides, and default values.

---

## Overview

Configuration is loaded in the following priority order (highest first):

1. **Environment variables** - Override all other settings
2. **Config file** - `config.yaml` in local or user directory
3. **Embedded defaults** - Compiled into the binary

---

## Configuration Structure

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
        MaxChars        int    `yaml:"maxChars"`
    }
}
```

---

## Configuration Options

### Network Settings

| Option | YAML Key | Env Variable | Default | Description |
|--------|----------|--------------|---------|-------------|
| Host | `host` | `STEGODON_HOST` | `127.0.0.1` | Server bind IP address |
| SSH Port | `sshPort` | `STEGODON_SSHPORT` | `23232` | SSH server port |
| HTTP Port | `httpPort` | `STEGODON_HTTPPORT` | `9999` | HTTP server port |
| SSL Domain | `sslDomain` | `STEGODON_SSLDOMAIN` | `example.com` | Public domain for ActivityPub |

### Feature Flags

| Option | YAML Key | Env Variable | Default | Description |
|--------|----------|--------------|---------|-------------|
| ActivityPub | `withAp` | `STEGODON_WITH_AP` | `false` | Enable ActivityPub federation |
| Single User | `single` | `STEGODON_SINGLE` | `false` | Limit to one registered user |
| Closed | `closed` | `STEGODON_CLOSED` | `false` | Disable new registrations |

### Content Settings

| Option | YAML Key | Env Variable | Default | Description |
|--------|----------|--------------|---------|-------------|
| Max Characters | `maxChars` | `STEGODON_MAX_CHARS` | `150` | Maximum visible characters per note (1-300) |

### Server Info

| Option | YAML Key | Env Variable | Default | Description |
|--------|----------|--------------|---------|-------------|
| Node Description | `nodeDescription` | `STEGODON_NODE_DESCRIPTION` | (empty) | Server description for NodeInfo |

### Debugging

| Option | YAML Key | Env Variable | Default | Description |
|--------|----------|--------------|---------|-------------|
| Journald | `withJournald` | `STEGODON_WITH_JOURNALD` | `false` | Use systemd journald logging |
| Pprof | `withPprof` | `STEGODON_WITH_PPROF` | `false` | Enable pprof on :6060 |

---

## Config File Format

### Default Configuration

```yaml
conf:
  host: 127.0.0.1
  sshPort: 23232
  httpPort: 9999
  sslDomain: example.com
  withAp: false
  single: false
  closed: false
  maxChars: 150
```

### Production Example

```yaml
conf:
  host: 0.0.0.0
  sshPort: 23232
  httpPort: 9999
  sslDomain: stegodon.example.com
  withAp: true
  single: false
  closed: false
  nodeDescription: "A stegodon instance for our community"
  maxChars: 300  # Allow longer posts
```

---

## File Locations

Configuration files are searched in order:

1. `./config.yaml` - Current working directory
2. `~/.config/stegodon/config.yaml` - User config directory

### Resolution Logic

```go
configPath := ResolveFilePath(ConfigFileName)
```

The `ResolveFilePath` function:
1. Checks current directory first
2. Falls back to user config directory
3. Uses `$XDG_CONFIG_HOME/stegodon` if set, otherwise `~/.config/stegodon`

### Auto-Creation

If no config file exists, the embedded default is written to the user config directory:

```
Config file not found at ./config.yaml, using embedded defaults
Created default config file at ~/.config/stegodon/config.yaml
```

---

## Environment Variables

All environment variables use the `STEGODON_` prefix.

### String Variables

```bash
export STEGODON_HOST="0.0.0.0"
export STEGODON_SSLDOMAIN="stegodon.example.com"
export STEGODON_NODE_DESCRIPTION="My stegodon instance"
```

### Integer Variables

```bash
export STEGODON_SSHPORT="23232"
export STEGODON_HTTPPORT="9999"
export STEGODON_MAX_CHARS="200"  # Note character limit (1-300)
```

Parse errors are logged but don't prevent startup:

```
Error parsing STEGODON_SSHPORT: strconv.Atoi: parsing "abc": invalid syntax
```

### MaxChars Validation

The `STEGODON_MAX_CHARS` variable has special validation:

```go
if v > 300 {
    log.Printf("STEGODON_MAX_CHARS value %d exceeds maximum of 300, capping at 300", v)
    c.Conf.MaxChars = 300
} else if v < 1 {
    log.Printf("STEGODON_MAX_CHARS value %d is less than minimum of 1, setting to default 150", v)
    c.Conf.MaxChars = 150
}
```

Values outside 1-300 are clamped to valid range.

### Boolean Variables

Boolean variables are only `true` when set to the exact string `"true"`:

```bash
export STEGODON_WITH_AP="true"      # Enabled
export STEGODON_SINGLE="true"       # Enabled
export STEGODON_CLOSED="true"       # Enabled
export STEGODON_WITH_JOURNALD="true"
export STEGODON_WITH_PPROF="true"
```

Any other value (including empty, `"false"`, `"1"`, `"yes"`) keeps the YAML default.

---

## Loading Process

```
util.ReadConf()
    │
    ├── Resolve config file path
    │       ├── Check ./config.yaml
    │       └── Check ~/.config/stegodon/config.yaml
    │
    ├── Read config file
    │       ├── If exists: parse YAML
    │       └── If not: use embedded defaults, create user config
    │
    └── Apply environment overrides
            ├── STEGODON_HOST
            ├── STEGODON_SSHPORT
            ├── STEGODON_HTTPPORT
            ├── STEGODON_SSLDOMAIN
            ├── STEGODON_WITH_AP
            ├── STEGODON_SINGLE
            ├── STEGODON_CLOSED
            ├── STEGODON_NODE_DESCRIPTION
            ├── STEGODON_WITH_JOURNALD
            ├── STEGODON_WITH_PPROF
            └── STEGODON_MAX_CHARS (with 1-300 validation)
```

---

## Registration Modes

Three mutually-aware registration modes:

| Mode | `single` | `closed` | Behavior |
|------|----------|----------|----------|
| Open | `false` | `false` | Anyone can register |
| Single-user | `true` | `false` | Only one user allowed |
| Closed | `false` | `true` | No new registrations |
| Both | `true` | `true` | First user only, then closed |

### Single-User Mode

- Checked in `middleware.AuthMiddleware`
- Counts existing accounts before allowing registration
- Returns message: "This blog is in single-user mode..."

### Closed Registration

- Checked in `middleware.AuthMiddleware`
- Rejects all new SSH connections without existing account
- Returns message: "Registration is closed..."

---

## ActivityPub Mode

When `withAp: true`:

1. **Delivery Worker Started** - Background goroutine processes queue
2. **Federation Endpoints Active** - Inbox/outbox accept activities
3. **HTTP Signatures Enabled** - Outgoing requests are signed
4. **SSL Domain Required** - Must be valid public domain

### Local Testing

```bash
# Using ngrok
STEGODON_WITH_AP=true STEGODON_SSLDOMAIN=your-domain.ngrok-free.app ./stegodon
```

---

## Pprof Profiling

When `withPprof: true`:

- Server starts on `localhost:6060`
- Access at `http://localhost:6060/debug/pprof/`
- Standard Go pprof endpoints available

```bash
# Enable pprof
STEGODON_WITH_PPROF=true ./stegodon

# Or in config.yaml
conf:
  withPprof: true
```

---

## Logging Output

Configuration is logged at startup:

```
stegodon v1.4.3
Configuration:
{
  "Conf": {
    "Host": "127.0.0.1",
    "SshPort": 23232,
    "HttpPort": 9999,
    "SslDomain": "example.com",
    "WithAp": false,
    "Single": false,
    "Closed": false,
    "NodeDescription": "",
    "WithJournald": false,
    "WithPprof": false,
    "MaxChars": 150
  }
}
```

---

## Embedded Defaults

Default configuration is embedded at compile time:

```go
//go:embed config_default.yaml
var embeddedConfig []byte
```

This ensures the binary is self-contained and can run without any config file.

---

## Source Files

- `util/config.go` - Configuration loading and parsing
- `util/config_default.yaml` - Embedded default configuration
- `util/path.go` - File path resolution utilities
