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
		pfp_path TEXT DEFAULT ''
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
		FOREIGN KEY(sender_id) REFERENCES users(id),
		FOREIGN KEY(receiver_id) REFERENCES users(id),
		FOREIGN KEY(parent_id) REFERENCES letters(id)
	);`
	db.Exec(queryLetters)

	// Migrations for existing databases
	db.Exec("ALTER TABLE letters ADD COLUMN is_read BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE letters ADD COLUMN emoji TEXT DEFAULT '💌'")
	db.Exec("ALTER TABLE letters ADD COLUMN parent_id INTEGER DEFAULT NULL")
	db.Exec("ALTER TABLE letters ADD COLUMN image_path TEXT DEFAULT ''")
	db.Exec("ALTER TABLE letters ADD COLUMN read_at DATETIME DEFAULT NULL")

	db.Exec("ALTER TABLE users ADD COLUMN bio TEXT DEFAULT 'Just setting up my keep.'")
	db.Exec("ALTER TABLE users ADD COLUMN status TEXT DEFAULT '🌻'")
	db.Exec("ALTER TABLE users ADD COLUMN pfp_path TEXT DEFAULT ''")
}
