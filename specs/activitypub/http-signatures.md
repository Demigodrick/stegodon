# HTTP Signatures

This document specifies the HTTP signature implementation used for ActivityPub authentication.

---

## Overview

HTTP signatures provide cryptographic authentication for ActivityPub requests. Stegodon uses RSA-SHA256 signatures to:
- Sign outgoing requests to prove authenticity
- Verify incoming requests from remote servers
- Prevent request tampering and replay attacks

---

## Algorithm

**Signature Algorithm:** RSA-SHA256

**Library:** `code.superseriousbusiness.org/httpsig`

---

## Signed Headers

All signed requests include these headers in the signature:

```go
[]string{"(request-target)", "host", "date", "digest"}
```

| Header | Description |
|--------|-------------|
| `(request-target)` | HTTP method and path (e.g., `post /inbox`) |
| `host` | Target server hostname |
| `date` | RFC 1123 formatted timestamp |
| `digest` | SHA-256 hash of request body |

---

## Key ID Format

The `keyId` in signatures uses the format:

```
https://{domain}/users/{username}#main-key
```

Example: `https://example.com/users/alice#main-key`

---

## Signing Requests

### SignRequest Function

```go
func SignRequest(req *http.Request, privateKey *rsa.PrivateKey, keyId string) error {
    signer, _, err := httpsig.NewSigner(
        []httpsig.Algorithm{httpsig.RSA_SHA256},
        httpsig.DigestSha256,
        []string{"(request-target)", "host", "date", "digest"},
        httpsig.Signature,
        0,
    )
    if err != nil {
        return fmt.Errorf("failed to create signer: %w", err)
    }

    return signer.SignRequest(privateKey, keyId, req, nil)
}
```

### Request Setup Before Signing

Before signing, requests must have these headers set:

```go
req.Header.Set("Content-Type", "application/activity+json")
req.Header.Set("Accept", "application/activity+json")
req.Header.Set("User-Agent", "stegodon/1.0 ActivityPub")
req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
req.Header.Set("Host", req.URL.Host)
req.Header.Set("Digest", digest)  // SHA-256 hash of body
```

### Digest Calculation

```go
hash := sha256.Sum256(activityJSON)
digest := "SHA-256=" + base64.StdEncoding.EncodeToString(hash[:])
```

---

## Verifying Requests

### VerifyRequest Function

```go
func VerifyRequest(req *http.Request, publicKeyPem string) (string, error) {
    verifier, err := httpsig.NewVerifier(req)
    if err != nil {
        return "", fmt.Errorf("failed to create verifier: %w", err)
    }

    // Parse public key (supports PKIX and PKCS#1 formats)
    block, _ := pem.Decode([]byte(publicKeyPem))
    if block == nil {
        return "", fmt.Errorf("failed to parse PEM block")
    }

    var rsaPubKey *rsa.PublicKey

    // Try PKIX format first (standard format)
    pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
    if err != nil {
        // Fallback to PKCS#1 format (old stegodon instances)
        pkcs1Key, pkcs1Err := x509.ParsePKCS1PublicKey(block.Bytes)
        if pkcs1Err != nil {
            return "", fmt.Errorf("failed to parse public key")
        }
        rsaPubKey = pkcs1Key
    } else {
        rsaPubKey = pubKey.(*rsa.PublicKey)
    }

    // Verify the signature
    err = verifier.Verify(rsaPubKey, httpsig.RSA_SHA256)
    if err != nil {
        return "", fmt.Errorf("signature verification failed: %w", err)
    }

    // Extract actor URI from keyId
    keyId := verifier.KeyId()
    actorURI := strings.Split(keyId, "#")[0]

    return actorURI, nil
}
```

### Verification Flow

```
Incoming Request
      │
      ▼
Extract Signature Header
      │
      ├── Missing → Return 401 Unauthorized
      └── Present → Continue
            │
            ▼
Extract keyId from Signature
      │
      ▼
Fetch Signer's Actor
      │
      ├── Failed → Return 400 Bad Request
      └── Success → Get public key
            │
            ▼
Restore Request Body (consumed during read)
      │
      ▼
Verify Signature
      │
      ├── Failed → Return 401 Unauthorized
      └── Success → Process Activity
```

---

## Key Formats

### Private Keys

Supports two formats for backwards compatibility:

| Format | PEM Type | Parser |
|--------|----------|--------|
| PKCS#8 (new) | `PRIVATE KEY` | `x509.ParsePKCS8PrivateKey()` |
| PKCS#1 (legacy) | `RSA PRIVATE KEY` | `x509.ParsePKCS1PrivateKey()` |

```go
func ParsePrivateKey(pemString string) (*rsa.PrivateKey, error) {
    block, _ := pem.Decode([]byte(pemString))
    if block == nil {
        return nil, fmt.Errorf("failed to parse PEM block")
    }

    // Try PKCS#8 first (new standard format)
    if block.Type == "PRIVATE KEY" {
        key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
        if err != nil {
            return nil, err
        }
        return key.(*rsa.PrivateKey), nil
    }

    // Fallback to PKCS#1 (old format)
    if block.Type == "RSA PRIVATE KEY" {
        return x509.ParsePKCS1PrivateKey(block.Bytes)
    }

    return nil, fmt.Errorf("unsupported private key type: %s", block.Type)
}
```

### Public Keys

Supports two formats for interoperability:

| Format | Parser |
|--------|--------|
| PKIX (standard) | `x509.ParsePKIXPublicKey()` |
| PKCS#1 (legacy) | `x509.ParsePKCS1PublicKey()` |

```go
func ParsePublicKey(pemString string) (*rsa.PublicKey, error) {
    block, _ := pem.Decode([]byte(pemString))
    if block == nil {
        return nil, fmt.Errorf("failed to parse PEM block")
    }

    // Try PKIX format first
    pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
    if err != nil {
        // Try PKCS#1 format
        return x509.ParsePKCS1PublicKey(block.Bytes)
    }

    return pubKey.(*rsa.PublicKey), nil
}
```

---

## Signature Header Format

The HTTP Signature header follows this format:

```
Signature: keyId="https://example.com/users/alice#main-key",
           algorithm="rsa-sha256",
           headers="(request-target) host date digest",
           signature="base64signature..."
```

### Extracting keyId

```go
func extractKeyIdFromSignature(signature string) string {
    for _, part := range strings.Split(signature, ",") {
        part = strings.TrimSpace(part)
        if strings.HasPrefix(part, "keyId=") {
            value := strings.TrimPrefix(part, "keyId=")
            return strings.Trim(value, "\"")
        }
    }
    return ""
}
```

---

## Signer vs Actor

For relay-forwarded content, the signer may differ from the activity actor:

```go
// Extract signer from signature header
signerKeyId := extractKeyIdFromSignature(signature)
signerActorURI := strings.Split(signerKeyId, "#")[0]

// Check if signer differs from activity actor
if signerActorURI != activity.Actor {
    log.Printf("Activity signed by %s on behalf of %s", signerActorURI, activity.Actor)
    isFromRelay := true
}
```

This distinction is critical for:
- Relay content (relay signs on behalf of original actor)
- Properly attributing content to original authors
- Verifying against the correct public key

---

## Error Handling

| Error | HTTP Status | Description |
|-------|-------------|-------------|
| Missing signature | 401 | `Signature` header not present |
| Invalid signature format | 401 | Could not extract keyId |
| Failed to fetch signer | 400 | Could not retrieve actor for verification |
| Verification failed | 401 | Signature does not match |

---

## Security Considerations

1. **Date Header Validation**: Remote servers may reject requests with stale dates
2. **Digest Verification**: Ensures body wasn't tampered with in transit
3. **Key Rotation**: Actor re-fetch updates cached public keys
4. **HTTPS Only**: All ActivityPub endpoints require HTTPS

---

## Source Files

- `activitypub/httpsig.go` - HTTP signature signing and verification
- `activitypub/inbox.go` - Signature verification on incoming requests
- `activitypub/outbox.go` - Request signing for outgoing activities
- `activitypub/delivery.go` - Request signing for queued deliveries
