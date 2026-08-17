package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	// Open the database directly
	db, err := sql.Open("sqlite", "keep.db")
	if err != nil {
		log.Fatal("Failed to open db:", err)
	}
	defer db.Close()

	fmt.Println("========================================")
	fmt.Println("       KEEP. DATABASE INSPECTOR 🌻      ")
	fmt.Println("========================================")

	// 1. INSPECT USERS
	userRows, err := db.Query("SELECT id, username, password_hash, friend_code, bio, status FROM users")
	if err != nil {
		log.Fatal("Failed to query users:", err)
	}
	defer userRows.Close()

	fmt.Println("\n--- USERS ---")
	userCount := 0
	for userRows.Next() {
		var id int
		var username, hash, friendCode, bio, status string
		userRows.Scan(&id, &username, &hash, &friendCode, &bio, &status)

		fmt.Printf("ID:         %d\n", id)
		fmt.Printf("Username:   %s\n", username)
		fmt.Printf("FriendCode: %s\n", friendCode)
		fmt.Printf("Status:     %s | Bio: %s\n", status, bio)
		if len(hash) >= 15 {
			fmt.Printf("Hash:       %s...\n", hash[:15])
		}
		fmt.Println("----------------------------------------")
		userCount++
	}
	fmt.Printf("Total registered accounts: %d\n", userCount)

	// 2. INSPECT LETTERS & REPLIES
	letterRows, err := db.Query(`
		SELECT id, sender_id, receiver_id, title, content, emoji, 
		       unlock_type, unlock_at, sender_ready, receiver_ready, 
		       is_read, created_at, read_at, parent_id, image_path 
		FROM letters`)
	if err != nil {
		log.Fatal("Failed to query letters:", err)
	}
	defer letterRows.Close()

	fmt.Println("\n--- LETTERS & REPLIES ---")
	letterCount := 0
	for letterRows.Next() {
		var id, senderID, receiverID int
		var title, content, emoji, unlockType, imagePath, createdAt string
		var senderReady, receiverReady, isRead bool
		var unlockAt, readAt sql.NullTime
		var parentID sql.NullInt64

		err := letterRows.Scan(
			&id, &senderID, &receiverID, &title, &content, &emoji,
			&unlockType, &unlockAt, &senderReady, &receiverReady,
			&isRead, &createdAt, &readAt, &parentID, &imagePath,
		)
		if err != nil {
			log.Println("Error scanning letter:", err)
			continue
		}

		fmt.Printf("Letter ID:  %d %s (Title: %s)\n", id, emoji, title)
		fmt.Printf("From ID:    %d  --->  To ID: %d\n", senderID, receiverID)

		if parentID.Valid {
			fmt.Printf("Type:       💬 Reply to Root Letter ID #%d\n", parentID.Int64)
		} else {
			fmt.Println("Type:       ✉️ Root Letter")
		}

		fmt.Printf("Content:    %s\n", truncate(content, 50))
		fmt.Printf("Seal Type:  %s | Read: %t\n", unlockType, isRead)
		fmt.Printf("Created At: %s\n", createdAt)

		if unlockAt.Valid {
			fmt.Printf("Unlock At:  %s\n", unlockAt.Time.Format("Jan 02, 2006 15:04"))
		}
		if readAt.Valid {
			fmt.Printf("Read At:    %s\n", readAt.Time.Format("Jan 02, 2006 15:04"))
		}
		if imagePath != "" {
			fmt.Printf("Attachment: %s\n", imagePath)
		}
		fmt.Println("----------------------------------------")
		letterCount++
	}
	fmt.Printf("Total letters/replies in db: %d\n", letterCount)
	fmt.Println("========================================")
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
