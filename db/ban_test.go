package db

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBanOperations(t *testing.T) {
	// Create test database
	testDB := setupTestDB(t)
	defer testDB.db.Close()

	// Create bans table for testing
	_, err := testDB.db.Exec(sqlCreateBansTable)
	if err != nil {
		t.Fatalf("Failed to create bans table: %v", err)
	}
	_, err = testDB.db.Exec(sqlCreateBansIndices)
	if err != nil {
		t.Fatalf("Failed to create bans indices: %v", err)
	}

	t.Run("CreateBan creates a ban record", func(t *testing.T) {
		id := uuid.New().String()
		username := "testuser"
		ipAddress := "192.168.1.100"
		publicKeyHash := "testhash123"
		reason := "Test ban reason"

		err := testDB.CreateBan(id, username, ipAddress, publicKeyHash, reason)
		if err != nil {
			t.Fatalf("Failed to create ban: %v", err)
		}

		// Verify ban was created
		err, bans := testDB.ReadAllBans()
		if err != nil {
			t.Fatalf("Failed to read bans: %v", err)
		}
		if bans == nil || len(*bans) != 1 {
			t.Fatal("Expected 1 ban record")
		}

		ban := (*bans)[0]
		if ban.Id != id {
			t.Errorf("Expected ID %s, got %s", id, ban.Id)
		}
		if ban.Username != username {
			t.Errorf("Expected username %s, got %s", username, ban.Username)
		}
		if ban.IPAddress != ipAddress {
			t.Errorf("Expected IP %s, got %s", ipAddress, ban.IPAddress)
		}
		if ban.PublicKeyHash != publicKeyHash {
			t.Errorf("Expected public key hash %s, got %s", publicKeyHash, ban.PublicKeyHash)
		}
		if ban.Reason != reason {
			t.Errorf("Expected reason %s, got %s", reason, ban.Reason)
		}
	})

	t.Run("IsIPBanned detects banned IP", func(t *testing.T) {
		bannedIP := "10.0.0.1"
		notBannedIP := "10.0.0.2"

		// Create a ban with the IP
		err := testDB.CreateBan(uuid.New().String(), "user1", bannedIP, "key1", "Test")
		if err != nil {
			t.Fatalf("Failed to create ban: %v", err)
		}

		// Check banned IP
		if !testDB.IsIPBanned(bannedIP) {
			t.Error("Expected IP to be banned")
		}

		// Check non-banned IP
		if testDB.IsIPBanned(notBannedIP) {
			t.Error("Expected IP to not be banned")
		}
	})

	t.Run("IsPublicKeyBanned detects banned public key", func(t *testing.T) {
		bannedKey := "bannedkeyhash123"
		notBannedKey := "notbannedkeyhash456"

		// Create a ban with the key
		err := testDB.CreateBan(uuid.New().String(), "user2", "1.2.3.4", bannedKey, "Test")
		if err != nil {
			t.Fatalf("Failed to create ban: %v", err)
		}

		// Check banned key
		if !testDB.IsPublicKeyBanned(bannedKey) {
			t.Error("Expected public key to be banned")
		}

		// Check non-banned key
		if testDB.IsPublicKeyBanned(notBannedKey) {
			t.Error("Expected public key to not be banned")
		}
	})

	t.Run("DeleteBan removes ban record", func(t *testing.T) {
		id := uuid.New().String()
		err := testDB.CreateBan(id, "user3", "5.6.7.8", "key3", "Test")
		if err != nil {
			t.Fatalf("Failed to create ban: %v", err)
		}

		// Verify it exists
		err, bans := testDB.ReadAllBans()
		if err != nil || bans == nil {
			t.Fatal("Failed to read bans")
		}
		initialCount := len(*bans)

		// Delete it
		err = testDB.DeleteBan(id)
		if err != nil {
			t.Fatalf("Failed to delete ban: %v", err)
		}

		// Verify it's gone
		err, bans = testDB.ReadAllBans()
		if err != nil {
			t.Fatalf("Failed to read bans after delete: %v", err)
		}
		if bans != nil && len(*bans) >= initialCount {
			t.Error("Ban was not deleted")
		}
	})

	t.Run("ReadAllBans returns bans in chronological order", func(t *testing.T) {
		// Create multiple bans with slight delays
		id1 := uuid.New().String()
		id2 := uuid.New().String()
		id3 := uuid.New().String()

		testDB.CreateBan(id1, "user_a", "1.1.1.1", "key_a", "First")
		time.Sleep(10 * time.Millisecond)
		testDB.CreateBan(id2, "user_b", "2.2.2.2", "key_b", "Second")
		time.Sleep(10 * time.Millisecond)
		testDB.CreateBan(id3, "user_c", "3.3.3.3", "key_c", "Third")

		err, bans := testDB.ReadAllBans()
		if err != nil {
			t.Fatalf("Failed to read bans: %v", err)
		}
		if bans == nil || len(*bans) < 3 {
			t.Fatal("Expected at least 3 bans")
		}

		// Should be in reverse chronological order (newest first)
		// Last created should be first in the list
		found := false
		for i, ban := range *bans {
			if ban.Id == id3 {
				if i > 0 {
					t.Error("Newest ban should be first in list")
				}
				found = true
				break
			}
		}
		if !found {
			t.Error("Could not find newest ban in list")
		}
	})

	t.Run("Empty IP address is handled correctly", func(t *testing.T) {
		// Create ban with empty IP (relying only on public key)
		id := uuid.New().String()
		publicKey := "onlypubkey"
		err := testDB.CreateBan(id, "user_noip", "", publicKey, "No IP ban")
		if err != nil {
			t.Fatalf("Failed to create ban with empty IP: %v", err)
		}

		// Should still be able to ban by public key
		if !testDB.IsPublicKeyBanned(publicKey) {
			t.Error("Expected public key to be banned")
		}

		// A different IP should not be banned
		if testDB.IsIPBanned("8.8.8.8") {
			t.Error("Unrelated IP should not be considered banned")
		}
	})
}
