package main

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func InitDB(dbPath string) {
	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}

	queryUsers := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		username TEXT UNIQUE, 
		password_hash TEXT,
		friend_code TEXT UNIQUE,
		bio TEXT DEFAULT 'Just setting up my keep.',
		status TEXT DEFAULT '🌻',
		pfp_path TEXT DEFAULT '',
		language_preference TEXT NOT NULL DEFAULT 'en'
	);`
	db.Exec(queryUsers)

	queryPartners := `
	CREATE TABLE IF NOT EXISTS partnerships (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user1_id INTEGER,
		user2_id INTEGER,
		status TEXT,
		UNIQUE(user1_id, user2_id)
	);`
	db.Exec(queryPartners)

	queryLetters := `
	CREATE TABLE IF NOT EXISTS letters (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sender_id INTEGER,
		receiver_id INTEGER,
		request_id TEXT DEFAULT NULL,
		title TEXT,
		content TEXT,
		emoji TEXT DEFAULT '💌',
		unlock_type TEXT,      
		unlock_at DATETIME,    
		sender_ready BOOLEAN DEFAULT 0,
		receiver_ready BOOLEAN DEFAULT 0,
		is_read BOOLEAN DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		read_at DATETIME DEFAULT NULL,
		parent_id INTEGER DEFAULT NULL,
		image_path TEXT DEFAULT '',
		latest_reply_user_name TEXT DEFAULT NULL,
		latest_reply_read BOOLEAN DEFAULT 0,

		FOREIGN KEY(sender_id) REFERENCES users(id),
		FOREIGN KEY(receiver_id) REFERENCES users(id),
		FOREIGN KEY(parent_id) REFERENCES letters(id)
	);`
	db.Exec(queryLetters)

	queryPushSubscriptions := `
	CREATE TABLE IF NOT EXISTS push_subscriptions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		endpoint TEXT NOT NULL UNIQUE,
		p256dh TEXT NOT NULL,
		auth TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	_, err = db.Exec(queryPushSubscriptions)

	if err != nil {
		log.Fatal("Failed creating push_subscriptions table:", err)
	}

	// Migrations for existing databases
	db.Exec("ALTER TABLE letters ADD COLUMN is_read BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE letters ADD COLUMN emoji TEXT DEFAULT '💌'")
	db.Exec("ALTER TABLE letters ADD COLUMN parent_id INTEGER DEFAULT NULL")
	db.Exec("ALTER TABLE letters ADD COLUMN image_path TEXT DEFAULT ''")
	db.Exec("ALTER TABLE letters ADD COLUMN read_at DATETIME DEFAULT NULL")
	db.Exec("ALTER TABLE letters ADD COLUMN latest_reply_user_name TEXT DEFAULT NULL")
	db.Exec("ALTER TABLE letters ADD COLUMN latest_reply_read BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE letters ADD COLUMN request_id TEXT DEFAULT NULL")

	db.Exec("ALTER TABLE users ADD COLUMN bio TEXT DEFAULT 'Just setting up my keep.'")
	db.Exec("ALTER TABLE users ADD COLUMN status TEXT DEFAULT '🌻'")
	db.Exec("ALTER TABLE users ADD COLUMN pfp_path TEXT DEFAULT ''")
	db.Exec("ALTER TABLE users ADD COLUMN language_preference TEXT NOT NULL DEFAULT 'en'")

	_, err = db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_letters_sender_request
		ON letters(sender_id, request_id)
	`)
	if err != nil {
		log.Fatal("Failed creating request ID index:", err)
	}
}
