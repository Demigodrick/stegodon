package db

import (
	"testing"
	"time"

	"github.com/deemkeen/stegodon/domain"
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

	t.Run("CleanupExpiredIPBans clears old IP addresses", func(t *testing.T) {
		// Create a ban with an old timestamp (more than 60 days ago)
		id := uuid.New().String()
		oldIP := "192.168.99.99"
		publicKey := "oldbankey"

		// Insert directly with an old timestamp
		_, err := testDB.db.Exec(
			`INSERT INTO bans(id, username, ip_address, public_key_hash, reason, banned_at) VALUES (?, ?, ?, ?, ?, datetime('now', '-61 days'))`,
			id, "olduser", oldIP, publicKey, "Old ban",
		)
		if err != nil {
			t.Fatalf("Failed to create old ban: %v", err)
		}

		// Verify the IP is NOT banned (because it's too old)
		if testDB.IsIPBanned(oldIP) {
			t.Error("Old IP should not be considered banned (>60 days)")
		}

		// But the public key should still be banned
		if !testDB.IsPublicKeyBanned(publicKey) {
			t.Error("Public key should still be banned regardless of age")
		}

		// Run cleanup
		affected, err := testDB.CleanupExpiredIPBans()
		if err != nil {
			t.Fatalf("Cleanup failed: %v", err)
		}
		if affected < 1 {
			t.Error("Expected at least 1 expired IP to be cleaned up")
		}

		// Verify the ban still exists but IP is cleared
		err, bans := testDB.ReadAllBans()
		if err != nil {
			t.Fatalf("Failed to read bans: %v", err)
		}

		found := false
		for _, ban := range *bans {
			if ban.Id == id {
				found = true
				if ban.IPAddress != "" {
					t.Errorf("Expected IP to be cleared, got %s", ban.IPAddress)
				}
				// Public key should still be there
				if ban.PublicKeyHash != publicKey {
					t.Errorf("Public key should be preserved, got %s", ban.PublicKeyHash)
				}
				break
			}
		}
		if !found {
			t.Error("Ban record should still exist after cleanup")
		}
	})

	t.Run("Recent IP bans are not cleaned up", func(t *testing.T) {
		// Create a recent ban
		id := uuid.New().String()
		recentIP := "192.168.50.50"
		publicKey := "recentbankey"

		err := testDB.CreateBan(id, "recentuser", recentIP, publicKey, "Recent ban")
		if err != nil {
			t.Fatalf("Failed to create recent ban: %v", err)
		}

		// Verify the IP is banned
		if !testDB.IsIPBanned(recentIP) {
			t.Error("Recent IP should be banned")
		}

		// Run cleanup
		testDB.CleanupExpiredIPBans()

		// IP should still be banned
		if !testDB.IsIPBanned(recentIP) {
			t.Error("Recent IP should still be banned after cleanup")
		}
	})
}

func TestBanAccount(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.db.Close()

	// Create a test account
	accountId := uuid.New()
	createBanTestAccount(t, testDB, accountId, "bantest", "banhash", "banpubkey", "banprivkey")

	t.Run("BanAccount sets banned flag to true", func(t *testing.T) {
		// Verify account is not banned initially
		err, acc := testDB.ReadAccById(accountId)
		if err != nil {
			t.Fatalf("Failed to read account: %v", err)
		}
		if acc.Banned {
			t.Error("Account should not be banned initially")
		}

		// Ban the account
		err = testDB.BanAccount(accountId)
		if err != nil {
			t.Fatalf("Failed to ban account: %v", err)
		}

		// Verify account is now banned
		err, acc = testDB.ReadAccById(accountId)
		if err != nil {
			t.Fatalf("Failed to read account after ban: %v", err)
		}
		if !acc.Banned {
			t.Error("Account should be banned after BanAccount")
		}
	})

	t.Run("UnbanAccount clears banned flag", func(t *testing.T) {
		// Account should still be banned from previous test
		err, acc := testDB.ReadAccById(accountId)
		if err != nil {
			t.Fatalf("Failed to read account: %v", err)
		}
		if !acc.Banned {
			t.Error("Account should be banned before unban test")
		}

		// Unban the account
		err = testDB.UnbanAccount(accountId)
		if err != nil {
			t.Fatalf("Failed to unban account: %v", err)
		}

		// Verify account is no longer banned
		err, acc = testDB.ReadAccById(accountId)
		if err != nil {
			t.Fatalf("Failed to read account after unban: %v", err)
		}
		if acc.Banned {
			t.Error("Account should not be banned after UnbanAccount")
		}
	})

	t.Run("BanAccount on non-existent account does not error", func(t *testing.T) {
		// Banning a non-existent account should not return an error
		// (SQL UPDATE on non-existent row just affects 0 rows)
		nonExistentId := uuid.New()
		err := testDB.BanAccount(nonExistentId)
		if err != nil {
			t.Errorf("BanAccount on non-existent account should not error, got: %v", err)
		}
	})
}

func TestUpdateAccountLastIP(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.db.Close()

	t.Run("UpdateAccountLastIP sets IP address", func(t *testing.T) {
		// Create a test account
		accountId := uuid.New()
		createBanTestAccount(t, testDB, accountId, "iptest1", "iphash1", "ippubkey1", "ipprivkey1")

		// Verify last_ip is empty initially
		err, acc := testDB.ReadAccById(accountId)
		if err != nil {
			t.Fatalf("Failed to read account: %v", err)
		}
		if acc.LastIP != "" {
			t.Errorf("LastIP should be empty initially, got: %s", acc.LastIP)
		}

		// Update the IP
		testIP := "192.168.1.100"
		err = testDB.UpdateAccountLastIP(accountId, testIP)
		if err != nil {
			t.Fatalf("Failed to update last IP: %v", err)
		}

		// Verify IP was updated
		err, acc = testDB.ReadAccById(accountId)
		if err != nil {
			t.Fatalf("Failed to read account after IP update: %v", err)
		}
		if acc.LastIP != testIP {
			t.Errorf("Expected LastIP %s, got %s", testIP, acc.LastIP)
		}
	})

	t.Run("UpdateAccountLastIP overwrites previous IP", func(t *testing.T) {
		// Create a test account
		accountId := uuid.New()
		createBanTestAccount(t, testDB, accountId, "iptest2", "iphash2", "ippubkey2", "ipprivkey2")

		// Set initial IP
		firstIP := "10.0.0.1"
		err := testDB.UpdateAccountLastIP(accountId, firstIP)
		if err != nil {
			t.Fatalf("Failed to set first IP: %v", err)
		}

		// Update to new IP
		secondIP := "10.0.0.2"
		err = testDB.UpdateAccountLastIP(accountId, secondIP)
		if err != nil {
			t.Fatalf("Failed to update to second IP: %v", err)
		}

		// Verify only the latest IP is stored
		err, acc := testDB.ReadAccById(accountId)
		if err != nil {
			t.Fatalf("Failed to read account: %v", err)
		}
		if acc.LastIP != secondIP {
			t.Errorf("Expected LastIP %s, got %s", secondIP, acc.LastIP)
		}
	})

	t.Run("UpdateAccountLastIPByPkHash sets IP by public key hash", func(t *testing.T) {
		// Create a test account
		accountId := uuid.New()
		pkHash := "uniquepkhash123"
		createBanTestAccount(t, testDB, accountId, "iptest3", pkHash, "ippubkey3", "ipprivkey3")

		// Update IP by public key hash
		testIP := "172.16.0.50"
		err := testDB.UpdateAccountLastIPByPkHash(pkHash, testIP)
		if err != nil {
			t.Fatalf("Failed to update IP by pk hash: %v", err)
		}

		// Verify IP was updated
		err, acc := testDB.ReadAccById(accountId)
		if err != nil {
			t.Fatalf("Failed to read account: %v", err)
		}
		if acc.LastIP != testIP {
			t.Errorf("Expected LastIP %s, got %s", testIP, acc.LastIP)
		}
	})
}

func TestAccountBannedFieldInQueries(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.db.Close()

	t.Run("ReadAllAccounts includes banned field", func(t *testing.T) {
		// Create two accounts - one banned, one not
		id1 := uuid.New()
		id2 := uuid.New()
		createBanTestAccount(t, testDB, id1, "readall1", "hash1", "pub1", "priv1")
		createBanTestAccount(t, testDB, id2, "readall2", "hash2", "pub2", "priv2")

		// Mark first_time_login = 0 so they show up in ReadAllAccounts
		testDB.db.Exec(`UPDATE accounts SET first_time_login = 0 WHERE id = ?`, id1.String())
		testDB.db.Exec(`UPDATE accounts SET first_time_login = 0 WHERE id = ?`, id2.String())

		// Ban the first account
		testDB.BanAccount(id1)

		// Read all accounts
		err, accounts := testDB.ReadAllAccounts()
		if err != nil {
			t.Fatalf("ReadAllAccounts failed: %v", err)
		}
		if accounts == nil || len(*accounts) < 2 {
			t.Fatal("Expected at least 2 accounts")
		}

		// Verify banned status is correctly read
		var foundBanned, foundNotBanned bool
		for _, acc := range *accounts {
			if acc.Id == id1 {
				if !acc.Banned {
					t.Error("Account id1 should be banned")
				}
				foundBanned = true
			}
			if acc.Id == id2 {
				if acc.Banned {
					t.Error("Account id2 should not be banned")
				}
				foundNotBanned = true
			}
		}
		if !foundBanned || !foundNotBanned {
			t.Error("Did not find both test accounts in results")
		}
	})

	t.Run("ReadAllAccountsAdmin includes banned field", func(t *testing.T) {
		// Create a banned account
		id := uuid.New()
		createBanTestAccount(t, testDB, id, "adminread", "adminhash", "adminpub", "adminpriv")
		testDB.BanAccount(id)

		// Read all accounts (admin view)
		err, accounts := testDB.ReadAllAccountsAdmin()
		if err != nil {
			t.Fatalf("ReadAllAccountsAdmin failed: %v", err)
		}

		// Find our account and verify banned status
		var found bool
		for _, acc := range *accounts {
			if acc.Id == id {
				if !acc.Banned {
					t.Error("Account should be banned in admin view")
				}
				found = true
				break
			}
		}
		if !found {
			t.Error("Did not find test account in admin results")
		}
	})

	t.Run("ReadAccByUsername includes banned and last_ip fields", func(t *testing.T) {
		// Create and configure a test account
		id := uuid.New()
		username := "usernametest"
		createBanTestAccount(t, testDB, id, username, "usernamehash", "usernamepub", "usernamepriv")
		testDB.BanAccount(id)
		testDB.UpdateAccountLastIP(id, "8.8.8.8")

		// Read by username
		err, acc := testDB.ReadAccByUsername(username)
		if err != nil {
			t.Fatalf("ReadAccByUsername failed: %v", err)
		}
		if !acc.Banned {
			t.Error("Account should be banned")
		}
		if acc.LastIP != "8.8.8.8" {
			t.Errorf("Expected LastIP 8.8.8.8, got %s", acc.LastIP)
		}
	})
}

// createBanTestAccount is a helper to create test accounts with all required fields
func createBanTestAccount(t *testing.T, db *DB, id uuid.UUID, username, pkHash, webPubKey, webPrivKey string) {
	_, err := db.db.Exec(`
		INSERT INTO accounts (id, username, publickey, created_at, first_time_login, web_public_key, web_private_key)
		VALUES (?, ?, ?, ?, 1, ?, ?)
	`, id.String(), username, pkHash, time.Now(), webPubKey, webPrivKey)
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}
}

// Ensure domain.Account is used (for import)
var _ = domain.Account{}
