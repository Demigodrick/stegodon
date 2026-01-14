# Cryptography

This document specifies RSA keypair generation, key format conversion, and hashing utilities.

---

## Overview

Stegodon uses RSA cryptography for:
- ActivityPub HTTP signatures
- SSH public key hashing for account lookup
- Secure random string generation

---

## RSA Keypair Generation

### GeneratePemKeypair Function

```go
func GeneratePemKeypair() *RsaKeyPair {
    bitSize := 4096

    key, err := rsa.GenerateKey(rand.Reader, bitSize)
    if err != nil {
        panic(err)
    }

    pub := key.Public()

    // PKCS#8 format for private key
    pkcs8Bytes, _ := x509.MarshalPKCS8PrivateKey(key)
    keyPEM := pem.EncodeToMemory(&pem.Block{
        Type:  "PRIVATE KEY",
        Bytes: pkcs8Bytes,
    })

    // PKIX format for public key
    pkixBytes, _ := x509.MarshalPKIXPublicKey(pub)
    pubPEM := pem.EncodeToMemory(&pem.Block{
        Type:  "PUBLIC KEY",
        Bytes: pkixBytes,
    })

    return &RsaKeyPair{
        Private: string(keyPEM),
        Public:  string(pubPEM),
    }
}
```

### Key Configuration

| Parameter | Value |
|-----------|-------|
| Algorithm | RSA |
| Key Size | 4096 bits |
| Private Format | PKCS#8 |
| Public Format | PKIX |
| Entropy Source | `crypto/rand` |

### RsaKeyPair Structure

```go
type RsaKeyPair struct {
    Private string  // PEM-encoded private key
    Public  string  // PEM-encoded public key
}
```

---

## Key Format Conversion

### PKCS#1 to PKCS#8 Private Key

```go
func ConvertPrivateKeyToPKCS8(pkcs1PEM string) (string, error) {
    block, _ := pem.Decode([]byte(pkcs1PEM))

    // Already PKCS#8?
    if block.Type == "PRIVATE KEY" {
        return pkcs1PEM, nil
    }

    if block.Type != "RSA PRIVATE KEY" {
        return "", fmt.Errorf("unexpected PEM type: %s", block.Type)
    }

    privateKey, _ := x509.ParsePKCS1PrivateKey(block.Bytes)
    pkcs8Bytes, _ := x509.MarshalPKCS8PrivateKey(privateKey)

    pkcs8PEM := pem.EncodeToMemory(&pem.Block{
        Type:  "PRIVATE KEY",
        Bytes: pkcs8Bytes,
    })

    return string(pkcs8PEM), nil
}
```

### PKCS#1 to PKIX Public Key

```go
func ConvertPublicKeyToPKIX(pkcs1PEM string) (string, error) {
    block, _ := pem.Decode([]byte(pkcs1PEM))

    // Already PKIX?
    if block.Type == "PUBLIC KEY" {
        return pkcs1PEM, nil
    }

    if block.Type != "RSA PUBLIC KEY" {
        return "", fmt.Errorf("unexpected PEM type: %s", block.Type)
    }

    publicKey, _ := x509.ParsePKCS1PublicKey(block.Bytes)
    pkixBytes, _ := x509.MarshalPKIXPublicKey(publicKey)

    pkixPEM := pem.EncodeToMemory(&pem.Block{
        Type:  "PUBLIC KEY",
        Bytes: pkixBytes,
    })

    return string(pkixPEM), nil
}
```

### PEM Block Types

| Format | Private Key Header | Public Key Header |
|--------|-------------------|-------------------|
| PKCS#1 | `RSA PRIVATE KEY` | `RSA PUBLIC KEY` |
| PKCS#8/PKIX | `PRIVATE KEY` | `PUBLIC KEY` |

---

## SSH Public Key Hashing

### PkToHash Function

```go
func PkToHash(pk string) string {
    h := sha256.New()
    h.Write([]byte(pk))
    return hex.EncodeToString(h.Sum(nil))
}
```

### Properties

| Property | Value |
|----------|-------|
| Algorithm | SHA-256 |
| Output | 64 hex characters |
| Input | OpenSSH authorized_keys format |

### Usage

Used for account lookup by SSH public key:

```go
publicKeyString := util.PublicKeyToString(s.PublicKey())
hash := util.PkToHash(publicKeyString)
// Query: SELECT * FROM accounts WHERE publickey = ?
```

---

## SSH Public Key Conversion

### PublicKeyToString

```go
func PublicKeyToString(s ssh.PublicKey) string {
    return strings.TrimSpace(string(gossh.MarshalAuthorizedKey(s)))
}
```

Converts SSH public key to OpenSSH `authorized_keys` format:

```
ssh-rsa AAAAB3NzaC1yc2E... user@host
```

---

## Random String Generation

### RandomString Function

```go
func RandomString(length int) string {
    b := make([]byte, length)
    rand.Read(b)
    return fmt.Sprintf("%x", b)[:length]
}
```

### Properties

| Property | Description |
|----------|-------------|
| Source | `crypto/rand` (cryptographically secure) |
| Output | Hex characters (0-9, a-f) |
| Length | Specified by caller |

### Usage

Used for initial random usernames:

```go
database.CreateAccount(s, util.RandomString(10))
// Creates username like "a3f8b2c901"
```

---

## Key Generation Timeline

When a new user account is created:

```
1. SSH connection received
2. Account not found in database
3. GeneratePemKeypair() called
4. 4096-bit RSA keypair generated
5. Keys stored in accounts table:
   - web_public_key (PKIX format)
   - web_private_key (PKCS#8 format)
6. Used for ActivityPub HTTP signatures
```

---

## Migration Support

### Legacy Key Format Migration

Older accounts may have PKCS#1 format keys. Conversion functions handle:
- Detection of current format
- Transparent conversion when needed
- No change to key material

```go
// Check if already converted
if block.Type == "PRIVATE KEY" {
    return pkcs1PEM, nil  // Already PKCS#8
}
```

---

## Version Embedding

### GetVersion Function

```go
//go:embed version.txt
var embeddedVersion string

func GetVersion() string {
    return strings.TrimSpace(embeddedVersion)
}

func GetNameAndVersion() string {
    return fmt.Sprintf("%s / %s", Name, GetVersion())
}
```

Version is embedded at build time from `version.txt`.

---

## Error Handling

| Function | Error Behavior |
|----------|----------------|
| `GeneratePemKeypair` | Panics on failure |
| `ConvertPrivateKeyToPKCS8` | Returns error |
| `ConvertPublicKeyToPKIX` | Returns error |
| `PkToHash` | Never fails |
| `RandomString` | Never fails |

---

## Security Considerations

### Key Storage

- Private keys stored in database (encrypted at rest recommended)
- SSH public keys stored as SHA-256 hashes (original not stored)

### Entropy

All random operations use `crypto/rand`:
- RSA key generation
- Random string generation

### Key Size

4096-bit RSA provides:
- Strong security margin
- Compatibility with ActivityPub servers
- Future-proof key strength

---

## Source Files

- `util/util.go` - All cryptography functions
- `util/version.txt` - Embedded version string
- `db/db.go` - Key storage and retrieval
