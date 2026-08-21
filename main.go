package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, relying on system defaults.")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "keep.db"
	}

	InitDB(dbPath)
	LoadTranslations()

	http.HandleFunc("/", HandleHome)
	http.HandleFunc("/login", HandleLogin)
	http.HandleFunc("/register", HandleRegister)
	http.HandleFunc("/logout", HandleLogout)
	http.HandleFunc("/partner/add", HandleAddPartner)
	http.HandleFunc("/partner/remove", HandleRemovePartner)
	http.HandleFunc("/letters/write", HandleWriteLetter)
	http.HandleFunc("/letter/view", HandleViewLetter)
	http.HandleFunc("/letter/ready", HandleReadyLetter)
	http.HandleFunc("/letter/reply", HandleReplyLetter)
	http.HandleFunc("/profile/update", HandleUpdateProfile)
	http.HandleFunc("/notifications/subscribe", HandleNotificationSubscribe)
	http.HandleFunc("/notifications/test", HandleNotificationTest)

	http.HandleFunc("/service-worker.js", HandleServiceWorker)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/uploads/", HandleUpload)

	serverAddress := ":" + port
	fmt.Printf("Server running at http://localhost%s 🌻\n", serverAddress)
	log.Fatal(http.ListenAndServe(serverAddress, nil))
}
