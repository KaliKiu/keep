package main

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

func CreateLetter(senderID, receiverID int, title, content, emoji, unlockType string, unlockAt *time.Time, parentID *int, imagePath string, latestReplyUsername string) error {
	if emoji == "" {
		emoji = "💌"
	}
	_, err := db.Exec(
		"INSERT INTO letters (sender_id, receiver_id, title, content, emoji, unlock_type, unlock_at, parent_id, image_path, latest_reply_user_name) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		senderID, receiverID, title, content, emoji, unlockType, unlockAt, parentID, imagePath, latestReplyUsername,
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

func MarkReplyLetterAsRead(letterID int) error {
	_, err := db.Exec("UPDATE letters SET latest_reply_read = 1 WHERE id = ?", letterID)
	return err
}
