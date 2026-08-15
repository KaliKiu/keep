package main

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func setupUploadAccessTestDB(t *testing.T) {
	t.Helper()

	testDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}

	statements := []string{
		`CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			username TEXT,
			password_hash TEXT,
			friend_code TEXT,
			bio TEXT,
			status TEXT,
			pfp_path TEXT
		)`,
		`CREATE TABLE partnerships (
			id INTEGER PRIMARY KEY,
			user1_id INTEGER,
			user2_id INTEGER,
			status TEXT
		)`,
		`CREATE TABLE letters (
			id INTEGER PRIMARY KEY,
			sender_id INTEGER,
			receiver_id INTEGER,
			unlock_type TEXT,
			unlock_at DATETIME,
			sender_ready BOOLEAN DEFAULT 0,
			receiver_ready BOOLEAN DEFAULT 0,
			parent_id INTEGER,
			image_path TEXT
		)`,
	}
	for _, statement := range statements {
		if _, err := testDB.Exec(statement); err != nil {
			t.Fatalf("create test schema: %v", err)
		}
	}

	previousDB := db
	db = testDB
	t.Cleanup(func() {
		db = previousDB
		testDB.Close()
	})
}

func TestCanUserAccessUpload(t *testing.T) {
	setupUploadAccessTestDB(t)

	for _, user := range []struct {
		id       int
		username string
		pfpPath  string
	}{
		{1, "alice", "/uploads/alice.jpg"},
		{2, "bob", ""},
		{3, "mallory", ""},
	} {
		_, err := db.Exec(
			"INSERT INTO users (id, username, pfp_path) VALUES (?, ?, ?)",
			user.id, user.username, user.pfpPath,
		)
		if err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	if _, err := db.Exec("INSERT INTO partnerships (user1_id, user2_id, status) VALUES (1, 2, 'active')"); err != nil {
		t.Fatalf("insert partnership: %v", err)
	}

	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	letters := []struct {
		id            int
		unlockType    string
		unlockAt      *time.Time
		senderReady   bool
		receiverReady bool
		parentID      *int
		imagePath     string
	}{
		{1, "instant", nil, false, false, nil, "/uploads/instant.jpg"},
		{2, "date", &future, false, false, nil, "/uploads/locked.jpg"},
		{3, "date", &past, false, false, nil, "/uploads/unlocked.jpg"},
		{4, "mutual_ready", nil, true, false, nil, "/uploads/not-ready.jpg"},
		{5, "instant", nil, false, false, intPointer(1), "/uploads/reply.jpg"},
	}
	for _, letter := range letters {
		_, err := db.Exec(`
			INSERT INTO letters (
				id, sender_id, receiver_id, unlock_type, unlock_at,
				sender_ready, receiver_ready, parent_id, image_path
			) VALUES (?, 1, 2, ?, ?, ?, ?, ?, ?)`,
			letter.id, letter.unlockType, letter.unlockAt, letter.senderReady,
			letter.receiverReady, letter.parentID, letter.imagePath,
		)
		if err != nil {
			t.Fatalf("insert letter %d: %v", letter.id, err)
		}
	}

	tests := []struct {
		name      string
		imagePath string
		userID    int
		want      bool
	}{
		{"profile owner", "/uploads/alice.jpg", 1, true},
		{"profile partner", "/uploads/alice.jpg", 2, true},
		{"profile outsider", "/uploads/alice.jpg", 3, false},
		{"instant sender", "/uploads/instant.jpg", 1, true},
		{"instant receiver", "/uploads/instant.jpg", 2, true},
		{"instant outsider", "/uploads/instant.jpg", 3, false},
		{"future date", "/uploads/locked.jpg", 2, false},
		{"past date", "/uploads/unlocked.jpg", 2, true},
		{"mutual not ready", "/uploads/not-ready.jpg", 2, false},
		{"reply follows parent access", "/uploads/reply.jpg", 2, true},
		{"unknown upload", "/uploads/missing.jpg", 1, false},
		{"missing user", "/uploads/instant.jpg", 0, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := CanUserAccessUpload(test.imagePath, test.userID)
			if err != nil {
				t.Fatalf("CanUserAccessUpload returned an error: %v", err)
			}
			if got != test.want {
				t.Fatalf("CanUserAccessUpload() = %v, want %v", got, test.want)
			}
		})
	}
}

func intPointer(value int) *int {
	return &value
}
