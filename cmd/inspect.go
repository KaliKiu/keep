package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	// Open the database directly (relative to where you run it, or point to ../keep.db if running from cmd/)
	db, err := sql.Open("sqlite", "keep.db")
	if err != nil {
		log.Fatal("Failed to open db:", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, username, password_hash FROM users")
	if err != nil {
		log.Fatal("Failed to query users (make sure keep.db exists!):", err)
	}
	defer rows.Close()

	fmt.Println("========================================")
	fmt.Println("       KEEP. DATABASE INSPECTOR 🌻      ")
	fmt.Println("========================================")

	count := 0
	for rows.Next() {
		var id int
		var username, hash string
		rows.Scan(&id, &username, &hash)

		fmt.Printf("ID:       %d\n", id)
		fmt.Printf("Username: %s\n", username)
		fmt.Printf("Hash:     %s...\n", hash[:15])
		fmt.Println("----------------------------------------")
		count++
	}

	fmt.Printf("Total registered accounts: %d\n", count)
	fmt.Println("========================================")
}
