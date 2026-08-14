package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// Load the .env file (if it fails, it will fall back to system defaults or log a warning)
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, relying on system environment variables or defaults.")
	}

	// Read variables from environment (with fallbacks if missing)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "keep.db"
	}

	// 1. Initialize the database dynamically
	InitDB(dbPath)

	// 2. Register routes
	http.HandleFunc("/", HandleHome)
	http.HandleFunc("/login", HandleLogin)
	http.HandleFunc("/register", HandleRegister)
	http.HandleFunc("/logout", HandleLogout)

	// Serve static CSS files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// 3. Start server using the environment port
	serverAddress := ":" + port
	fmt.Printf("Server running at http://localhost%s 🌻\n", serverAddress)
	log.Fatal(http.ListenAndServe(serverAddress, nil))
}
