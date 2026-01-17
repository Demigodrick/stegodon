package util

import (
	"testing"
)

func TestParseActivityPubURL(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantUsername string
		wantDomain   string
		wantOk       bool
	}{
		// Stegodon format
		{
			name:         "stegodon format with https",
			input:        "https://example.com/u/alice",
			wantUsername: "alice",
			wantDomain:   "example.com",
			wantOk:       true,
		},
		{
			name:         "stegodon format with http",
			input:        "http://example.com/u/bob",
			wantUsername: "bob",
			wantDomain:   "example.com",
			wantOk:       true,
		},
		// Mastodon format
		{
			name:         "mastodon format with @ in path",
			input:        "https://mastodon.social/@alice",
			wantUsername: "alice",
			wantDomain:   "mastodon.social",
			wantOk:       true,
		},
		{
			name:         "mastodon format with @ as separate path",
			input:        "https://mastodon.social/@/alice",
			wantUsername: "alice",
			wantDomain:   "mastodon.social",
			wantOk:       true,
		},
		// Standard ActivityPub users format
		{
			name:         "users format",
			input:        "https://pleroma.site/users/charlie",
			wantUsername: "charlie",
			wantDomain:   "pleroma.site",
			wantOk:       true,
		},
		// With query parameters
		{
			name:         "url with query parameters",
			input:        "https://example.com/u/dave?foo=bar",
			wantUsername: "dave",
			wantDomain:   "example.com",
			wantOk:       true,
		},
		// With fragment
		{
			name:         "url with fragment",
			input:        "https://example.com/u/eve#section",
			wantUsername: "eve",
			wantDomain:   "example.com",
			wantOk:       true,
		},
		// With whitespace
		{
			name:         "url with leading/trailing whitespace",
			input:        "  https://example.com/u/frank  ",
			wantUsername: "frank",
			wantDomain:   "example.com",
			wantOk:       true,
		},
		// Without protocol (should auto-add https://)
		{
			name:         "url without protocol - stegodon format",
			input:        "example.com/u/george",
			wantUsername: "george",
			wantDomain:   "example.com",
			wantOk:       true,
		},
		{
			name:         "url without protocol - mastodon format",
			input:        "mastodon.social/@henry",
			wantUsername: "henry",
			wantDomain:   "mastodon.social",
			wantOk:       true,
		},
		{
			name:         "url without protocol - users format",
			input:        "pleroma.site/users/iris",
			wantUsername: "iris",
			wantDomain:   "pleroma.site",
			wantOk:       true,
		},
		// Invalid cases
		{
			name:         "webfinger format (not a url)",
			input:        "alice@example.com",
			wantUsername: "",
			wantDomain:   "",
			wantOk:       false,
		},
		{
			name:         "invalid path type",
			input:        "https://example.com/invalid/alice",
			wantUsername: "",
			wantDomain:   "",
			wantOk:       false,
		},
		{
			name:         "too short path",
			input:        "https://example.com/u",
			wantUsername: "",
			wantDomain:   "",
			wantOk:       false,
		},
		{
			name:         "empty username",
			input:        "https://example.com/u/",
			wantUsername: "",
			wantDomain:   "",
			wantOk:       false,
		},
		{
			name:         "empty string",
			input:        "",
			wantUsername: "",
			wantDomain:   "",
			wantOk:       false,
		},
		{
			name:         "just protocol",
			input:        "https://",
			wantUsername: "",
			wantDomain:   "",
			wantOk:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUsername, gotDomain, gotOk := ParseActivityPubURL(tt.input)

			if gotOk != tt.wantOk {
				t.Errorf("ParseActivityPubURL() gotOk = %v, wantOk %v", gotOk, tt.wantOk)
			}

			if gotUsername != tt.wantUsername {
				t.Errorf("ParseActivityPubURL() gotUsername = %v, wantUsername %v", gotUsername, tt.wantUsername)
			}

			if gotDomain != tt.wantDomain {
				t.Errorf("ParseActivityPubURL() gotDomain = %v, wantDomain %v", gotDomain, tt.wantDomain)
			}
		})
	}
}
