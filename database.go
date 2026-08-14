package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
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
        friend_code TEXT UNIQUE
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
        FOREIGN KEY(sender_id) REFERENCES users(id),
        FOREIGN KEY(receiver_id) REFERENCES users(id)
    );`
	db.Exec(queryLetters)

	// Migrations for existing databases
	_, err = db.Exec("ALTER TABLE letters ADD COLUMN is_read BOOLEAN DEFAULT 0")
	if err == nil {
		log.Println("Migration successful: added 'is_read' column.")
	}

	_, err = db.Exec("ALTER TABLE letters ADD COLUMN emoji TEXT DEFAULT '💌'")
	if err == nil {
		log.Println("Migration successful: added 'emoji' column to letters table.")
	}
}

func generateFriendCode() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func RegisterUser(username, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	friendCode := generateFriendCode()
	_, err = db.Exec("INSERT INTO users (username, password_hash, friend_code) VALUES (?, ?, ?)", username, string(hashedPassword), friendCode)
	return err
}

func CheckUserCredentials(username, password string) bool {
	var storedHash string
	err := db.QueryRow("SELECT password_hash FROM users WHERE username = ?", username).Scan(&storedHash)
	if err != nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) == nil
}

func GetUserByUsername(username string) (User, error) {
	var u User
	err := db.QueryRow("SELECT id, username, friend_code FROM users WHERE username = ?", username).
		Scan(&u.ID, &u.Username, &u.FriendCode)
	return u, err
}

func GetPartnershipInfo(userID int) (PartnershipInfo, error) {
	var info PartnershipInfo
	var u1, u2, partnerID int
	var status, pName, pCode string

	query := `
    SELECT p.user1_id, p.user2_id, p.status, u.id, u.username, u.friend_code 
    FROM partnerships p
    JOIN users u ON u.id = CASE WHEN p.user1_id = ? THEN p.user2_id ELSE p.user1_id END
    WHERE p.user1_id = ? OR p.user2_id = ? LIMIT 1`

	err := db.QueryRow(query, userID, userID, userID).Scan(&u1, &u2, &status, &partnerID, &pName, &pCode)
	if err != nil {
		if err == sql.ErrNoRows {
			return info, nil
		}
		return info, err
	}

	info.HasPartner = true
	info.PartnerID = partnerID
	info.PartnerName = pName
	info.PartnerCode = pCode
	info.IsPending = (status == "pending")
	info.IsSender = (u1 == userID)
	return info, nil
}

func ProcessFriendCode(senderID int, targetCode string) error {
	var targetID int
	err := db.QueryRow("SELECT id FROM users WHERE friend_code = ?", targetCode).Scan(&targetID)
	if err != nil {
		return errors.New("friend code not found")
	}
	if senderID == targetID {
		return errors.New("you cannot add yourself")
	}

	senderInfo, _ := GetPartnershipInfo(senderID)
	targetInfo, _ := GetPartnershipInfo(targetID)

	if senderInfo.HasPartner && senderInfo.IsPending && !senderInfo.IsSender && senderInfo.PartnerCode == targetCode {
		_, err = db.Exec("UPDATE partnerships SET status = 'active' WHERE user1_id = ? AND user2_id = ?", targetID, senderID)
		return err
	}
	if senderInfo.HasPartner {
		return errors.New("you already have a partner or pending request")
	}
	if targetInfo.HasPartner {
		return errors.New("this user already has a partner or pending request")
	}

	_, err = db.Exec("INSERT INTO partnerships (user1_id, user2_id, status) VALUES (?, ?, 'pending')", senderID, targetID)
	return err
}

func RemovePartnership(userID int) error {
	_, err := db.Exec("DELETE FROM partnerships WHERE user1_id = ? OR user2_id = ?", userID, userID)
	return err
}

// CREATE LETTER WITH EMOJI FALLBACK
func CreateLetter(senderID, receiverID int, title, content, emoji, unlockType string, unlockAt *time.Time) error {
	if emoji == "" {
		emoji = "💌"
	}
	_, err := db.Exec(
		"INSERT INTO letters (sender_id, receiver_id, title, content, emoji, unlock_type, unlock_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		senderID, receiverID, title, content, emoji, unlockType, unlockAt,
	)
	return err
}

func GetVaultLetters(userID int) ([]Letter, error) {
	query := `
    SELECT id, sender_id, receiver_id, title, content, emoji, unlock_type, unlock_at, sender_ready, receiver_ready, is_read, created_at 
    FROM letters 
    WHERE sender_id = ? OR receiver_id = ? 
    ORDER BY created_at DESC`

	rows, err := db.Query(query, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var letters []Letter
	for rows.Next() {
		var l Letter
		var unlockAt sql.NullTime
		err := rows.Scan(&l.ID, &l.SenderID, &l.ReceiverID, &l.Title, &l.Content, &l.Emoji, &l.UnlockType, &unlockAt, &l.SenderReady, &l.ReceiverReady, &l.IsRead, &l.CreatedAt)
		if err != nil {
			return nil, err
		}
		if unlockAt.Valid {
			l.UnlockAt = &unlockAt.Time
		}

		l.IsSender = (l.SenderID == userID)
		l.IsUnlocked = false
		if l.UnlockType == "instant" {
			l.IsUnlocked = true
		} else if l.UnlockType == "date" && l.UnlockAt != nil {
			if time.Now().After(*l.UnlockAt) {
				l.IsUnlocked = true
			}
		} else if l.UnlockType == "mutual_ready" {
			if l.SenderReady && l.ReceiverReady {
				l.IsUnlocked = true
			}
		}
		letters = append(letters, l)
	}
	return letters, nil
}

func GetLetterByID(letterID, userID int) (Letter, error) {
	var l Letter
	var unlockAt sql.NullTime
	query := `
    SELECT id, sender_id, receiver_id, title, content, emoji, unlock_type, unlock_at, sender_ready, receiver_ready, is_read, created_at 
    FROM letters 
    WHERE id = ? AND (sender_id = ? OR receiver_id = ?)`

	err := db.QueryRow(query, letterID, userID, userID).Scan(
		&l.ID, &l.SenderID, &l.ReceiverID, &l.Title, &l.Content, &l.Emoji,
		&l.UnlockType, &unlockAt, &l.SenderReady, &l.ReceiverReady, &l.IsRead, &l.CreatedAt,
	)
	if err != nil {
		return l, err
	}
	if unlockAt.Valid {
		l.UnlockAt = &unlockAt.Time
	}

	l.IsSender = (l.SenderID == userID)
	l.IsUnlocked = false
	if l.UnlockType == "instant" {
		l.IsUnlocked = true
	} else if l.UnlockType == "date" && l.UnlockAt != nil {
		if time.Now().After(*l.UnlockAt) {
			l.IsUnlocked = true
		}
	} else if l.UnlockType == "mutual_ready" {
		if l.SenderReady && l.ReceiverReady {
			l.IsUnlocked = true
		}
	}
	return l, nil
}

func MarkLetterAsRead(letterID int) error {
	_, err := db.Exec("UPDATE letters SET is_read = 1 WHERE id = ?", letterID)
	return err
}
