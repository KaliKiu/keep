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
	err := db.QueryRow("SELECT id, username, friend_code, bio, status, pfp_path FROM users WHERE username = ?", username).
		Scan(&u.ID, &u.Username, &u.FriendCode, &u.Bio, &u.Status, &u.PfpPath)
	return u, err
}

func GetUserByID(id int) (User, error) {
	var u User
	err := db.QueryRow("SELECT id, username, friend_code, bio, status, pfp_path FROM users WHERE id = ?", id).
		Scan(&u.ID, &u.Username, &u.FriendCode, &u.Bio, &u.Status, &u.PfpPath)
	return u, err
}

func CanUserAccessUpload(imagePath string, userID int) (bool, error) {
	if imagePath == "" || userID == 0 {
		return false, nil
	}

	var profileOwnerID int
	err := db.QueryRow("SELECT id FROM users WHERE pfp_path = ?", imagePath).Scan(&profileOwnerID)
	if err == nil {
		if profileOwnerID == userID {
			return true, nil
		}

		var linked bool
		err = db.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM partnerships
				WHERE (user1_id = ? AND user2_id = ?)
				   OR (user1_id = ? AND user2_id = ?)
			)`, userID, profileOwnerID, profileOwnerID, userID).Scan(&linked)
		return linked, err
	}
	if err != sql.ErrNoRows {
		return false, err
	}

	var senderID, receiverID int
	var unlockType string
	var unlockAt sql.NullTime
	var senderReady, receiverReady bool
	err = db.QueryRow(`
		SELECT root.sender_id, root.receiver_id, root.unlock_type, root.unlock_at,
		       root.sender_ready, root.receiver_ready
		FROM letters asset
		JOIN letters root ON root.id = COALESCE(asset.parent_id, asset.id)
		WHERE asset.image_path = ? AND asset.image_path != ''
		LIMIT 1`, imagePath).Scan(
		&senderID, &receiverID, &unlockType, &unlockAt, &senderReady, &receiverReady,
	)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if userID != senderID && userID != receiverID {
		return false, nil
	}

	switch unlockType {
	case "instant":
		return true, nil
	case "date", "random":
		return unlockAt.Valid && time.Now().After(unlockAt.Time), nil
	case "mutual_ready":
		return senderReady && receiverReady, nil
	default:
		return false, nil
	}
}

func UpdateUserProfile(userID int, bio, status, pfpPath string) error {
	if pfpPath != "" {
		_, err := db.Exec("UPDATE users SET bio = ?, status = ?, pfp_path = ? WHERE id = ?", bio, status, pfpPath, userID)
		return err
	}
	_, err := db.Exec("UPDATE users SET bio = ?, status = ? WHERE id = ?", bio, status, userID)
	return err
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

func CreateLetter(senderID, receiverID int, title, content, emoji, unlockType string, unlockAt *time.Time, parentID *int, imagePath string) error {
	if emoji == "" {
		emoji = "💌"
	}
	_, err := db.Exec(
		"INSERT INTO letters (sender_id, receiver_id, title, content, emoji, unlock_type, unlock_at, parent_id, image_path) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		senderID, receiverID, title, content, emoji, unlockType, unlockAt, parentID, imagePath,
	)
	return err
}

func GetVaultLetters(userID int) ([]Letter, error) {
	query := `
	SELECT id, sender_id, receiver_id, title, content, emoji, unlock_type, unlock_at, sender_ready, receiver_ready, is_read, created_at, read_at, image_path
	FROM letters 
	WHERE (sender_id = ? OR receiver_id = ?) AND parent_id IS NULL
	ORDER BY created_at DESC`

	rows, err := db.Query(query, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var letters []Letter
	for rows.Next() {
		var l Letter
		var unlockAt, readAt sql.NullTime
		err := rows.Scan(&l.ID, &l.SenderID, &l.ReceiverID, &l.Title, &l.Content, &l.Emoji, &l.UnlockType, &unlockAt, &l.SenderReady, &l.ReceiverReady, &l.IsRead, &l.CreatedAt, &readAt, &l.ImagePath)
		if err != nil {
			return nil, err
		}
		if unlockAt.Valid {
			l.UnlockAt = &unlockAt.Time
		}
		if readAt.Valid {
			l.ReadAt = &readAt.Time
		}

		l.IsSender = (l.SenderID == userID)
		l.IsUnlocked = false
		if l.UnlockType == "instant" {
			l.IsUnlocked = true
		} else if (l.UnlockType == "date" || l.UnlockType == "random") && l.UnlockAt != nil {
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
	var unlockAt, readAt sql.NullTime
	query := `
	SELECT id, sender_id, receiver_id, title, content, emoji, unlock_type, unlock_at, sender_ready, receiver_ready, is_read, created_at, read_at, image_path
	FROM letters 
	WHERE id = ? AND (sender_id = ? OR receiver_id = ?)`

	err := db.QueryRow(query, letterID, userID, userID).Scan(
		&l.ID, &l.SenderID, &l.ReceiverID, &l.Title, &l.Content, &l.Emoji,
		&l.UnlockType, &unlockAt, &l.SenderReady, &l.ReceiverReady, &l.IsRead, &l.CreatedAt, &readAt, &l.ImagePath,
	)
	if err != nil {
		return l, err
	}
	if unlockAt.Valid {
		l.UnlockAt = &unlockAt.Time
	}
	if readAt.Valid {
		l.ReadAt = &readAt.Time
	}

	l.IsSender = (l.SenderID == userID)

	if l.IsSender {
		l.CurrentUserReady = l.SenderReady
		l.PartnerReady = l.ReceiverReady
	} else {
		l.CurrentUserReady = l.ReceiverReady
		l.PartnerReady = l.SenderReady
	}

	l.IsUnlocked = false
	if l.UnlockType == "instant" {
		l.IsUnlocked = true
	} else if (l.UnlockType == "date" || l.UnlockType == "random") && l.UnlockAt != nil {
		if time.Now().After(*l.UnlockAt) {
			l.IsUnlocked = true
		}
	} else if l.UnlockType == "mutual_ready" {
		if l.SenderReady && l.ReceiverReady {
			l.IsUnlocked = true
		}
	}

	// Fetch threads (replies)
	rows, _ := db.Query("SELECT id, sender_id, receiver_id, title, content, emoji, created_at, image_path FROM letters WHERE parent_id = ? ORDER BY created_at ASC", letterID)
	defer rows.Close()
	for rows.Next() {
		var reply Letter
		rows.Scan(&reply.ID, &reply.SenderID, &reply.ReceiverID, &reply.Title, &reply.Content, &reply.Emoji, &reply.CreatedAt, &reply.ImagePath)
		reply.IsSender = (reply.SenderID == userID)
		reply.IsUnlocked = true
		l.Replies = append(l.Replies, reply)
	}

	return l, nil
}

func ToggleReadyStatus(letterID, userID int) error {
	var senderID int
	err := db.QueryRow("SELECT sender_id FROM letters WHERE id = ?", letterID).Scan(&senderID)
	if err != nil {
		return err
	}
	if senderID == userID {
		_, err = db.Exec("UPDATE letters SET sender_ready = NOT sender_ready WHERE id = ?", letterID)
	} else {
		_, err = db.Exec("UPDATE letters SET receiver_ready = NOT receiver_ready WHERE id = ?", letterID)
	}
	return err
}

func MarkLetterAsRead(letterID int) error {
	// Captures exactly when they opened it!
	_, err := db.Exec("UPDATE letters SET is_read = 1, read_at = ? WHERE id = ?", time.Now(), letterID)
	return err
}
