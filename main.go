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

	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("keep_session")
		if err != nil || sessions[cookie.Value] == "" {
			http.Error(w, "Unauthorized - Please log in", http.StatusUnauthorized)
			return
		}
		http.StripPrefix("/static/", http.FileServer(http.Dir("static"))).ServeHTTP(w, r)
	})

	http.HandleFunc("/uploads/", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("keep_session")
		if err != nil || sessions[cookie.Value] == "" {
			http.Error(w, "Unauthorized - Please log in", http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/uploads/" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))).ServeHTTP(w, r)
	})

	serverAddress := ":" + port
	fmt.Printf("Server running at http://localhost%s 🌻\n", serverAddress)
	log.Fatal(http.ListenAndServe(serverAddress, nil))
}
