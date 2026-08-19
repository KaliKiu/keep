package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

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
	if userID == senderID {
		return true, nil
	}

	if userID != receiverID {
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
