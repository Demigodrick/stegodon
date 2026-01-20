package middleware

import (
	"testing"
)

// TestExtractIPFromRemoteAddr tests IP extraction logic used in auth middleware
func TestExtractIPFromRemoteAddr(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		expectedIP string
	}{
		{
			name:       "IPv4 with port",
			remoteAddr: "192.168.1.100:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name:       "IPv4 without port",
			remoteAddr: "192.168.1.100",
			expectedIP: "192.168.1.100",
		},
		{
			name:       "IPv6 with port (brackets stripped)",
			remoteAddr: "[::1]:12345",
			expectedIP: "::1",
		},
		{
			name:       "IPv6 without port",
			remoteAddr: "::1",
			expectedIP: "::1",
		},
		{
			name:       "localhost with port",
			remoteAddr: "127.0.0.1:22",
			expectedIP: "127.0.0.1",
		},
		{
			name:       "full IPv6 with port",
			remoteAddr: "[2001:db8::1]:22",
			expectedIP: "2001:db8::1",
		},
		{
			name:       "full IPv6 without port",
			remoteAddr: "2001:db8::1",
			expectedIP: "2001:db8::1",
		},
		{
			name:       "bracketed IPv6 without port",
			remoteAddr: "[2001:db8::1]",
			expectedIP: "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := extractIP(tt.remoteAddr)
			if ip != tt.expectedIP {
				t.Errorf("extractIP(%q) = %q, want %q", tt.remoteAddr, ip, tt.expectedIP)
			}
		})
	}
}

// TestBanCheckOrder documents the expected order of ban checks in middleware
func TestBanCheckOrder(t *testing.T) {
	// This test documents the expected behavior of the auth middleware
	// The actual middleware can't be easily unit tested without mocking SSH sessions,
	// but we can document the expected order of operations:
	//
	// 1. Extract IP from remote address
	// 2. Check if IP is banned (IsIPBanned) - blocks with "Your IP address is banned" message
	// 3. Check if public key is banned (IsPublicKeyBanned) - blocks with generic ban message
	// 4. Look up account by session
	// 5. If account found:
	//    a. Check if account.Banned is true - blocks with generic ban message
	//    b. Check if account.Muted is true - blocks with muted message
	//    c. Update account's LastIP
	// 6. If account not found, create new account (if registration open)
	//    a. Update new account's LastIP
	//
	// The order ensures:
	// - IP bans are checked first (fastest, no DB account lookup needed)
	// - Public key bans are checked second (fast, just a hash lookup)
	// - Account-level bans are checked after account lookup
	// - Muted status is separate from banned status

	t.Log("Ban check order documented - see test comments for details")
}
