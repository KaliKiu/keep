package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	sessions   = make(map[string]string)
	sessionsMu sync.RWMutex
)

func renderTemplate(
	w http.ResponseWriter,
	tmplName string,
	data interface{},
	lang Language,
) {
	tmpl, err := template.New(tmplName).
		Funcs(template.FuncMap{
			"T": func(key string) string {
				return translate(lang, key)
			},
		}).
		ParseFiles("templates/" + tmplName)

	if err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, data)
}

func HandleLogin(w http.ResponseWriter, r *http.Request) {
	var errorMsg string
	if r.Method == http.MethodPost {
		r.ParseForm()
		username := r.FormValue("username")
		password := r.FormValue("password")

		if CheckUserCredentials(username, password) {
			sessionToken, err := generateSessionToken()
			if err != nil {
				log.Printf("failed generating session token: %v", err)
				http.Error(w, "Failed to create session", http.StatusInternalServerError)
				return
			}
			storeSession(sessionToken, username)
			http.SetCookie(w, &http.Cookie{
				Name:     "keep_session",
				Value:    sessionToken,
				Path:     "/",
				Expires:  time.Now().Add(7 * 24 * time.Hour),
				MaxAge:   7 * 24 * 60 * 60,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				Secure:   r.TLS != nil,
			})
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		errorMsg = "Invalid username or password."
	}
	renderTemplate(w, "login.html", map[string]string{"Error": errorMsg}, LanguageEN)
}

func generateSessionToken() (string, error) {
	bytes := make([]byte, 32)

	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
func storeSession(token, username string) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	sessions[token] = username
}

func getSessionUsername(token string) (string, bool) {
	sessionsMu.RLock()
	defer sessionsMu.RUnlock()

	username, exists := sessions[token]
	return username, exists
}

func deleteSession(token string) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	delete(sessions, token)
}

func HandleRegister(w http.ResponseWriter, r *http.Request) {
	var errorMsg string
	if r.Method == http.MethodPost {
		r.ParseForm()
		username := r.FormValue("username")
		password := r.FormValue("password")
		confirm := r.FormValue("confirm_password")

		if password != confirm {
			errorMsg = "Passwords do not match."
		} else {
			err := RegisterUser(username, password)
			if err != nil {
				errorMsg = "Username already taken."
			} else {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
		}
	}
	renderTemplate(w, "register.html", map[string]string{"Error": errorMsg}, LanguageEN)
}

func HandleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("keep_session")
	if err == nil {
		deleteSession(cookie.Value)
		http.SetCookie(w, &http.Cookie{Name: "keep_session", Value: "", Path: "/", MaxAge: -1})
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func HandleHome(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		log.Printf("home access denied: invalid or missing session")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	partnerInfo, err := GetPartnershipInfo(user.ID)
	if err != nil {
		log.Printf("failed loading partnership for user %d: %v", user.ID, err)
		http.Error(w, "Failed to load account data", http.StatusInternalServerError)
		return
	}

	var partnerProfile User

	if partnerInfo.HasPartner {
		partnerProfile, err = GetUserByID(partnerInfo.PartnerID)
		if err != nil {
			log.Printf(
				"failed loading partner profile for user %d: %v",
				user.ID,
				err,
			)

			http.Error(w, "Failed to load account data", http.StatusInternalServerError)
			return
		}
	}

	letters, err := GetVaultLetters(user.ID)
	if err != nil {
		log.Printf("failed loading letters for user %d: %v", user.ID, err)
		http.Error(w, "Failed to load letters", http.StatusInternalServerError)
		return
	}

	tab := r.URL.Query().Get("tab")

	switch tab {
	case "":
		tab = "inbox"

	case "inbox", "history_rx", "history_tx", "write", "profile":
		// valid

	default:
		tab = "inbox"
	}

	data := map[string]interface{}{
		"User":           user,
		"Partner":        partnerInfo,
		"PartnerProfile": partnerProfile,
		"Letters":        letters,
		"Tab":            tab,
		"Error":          r.URL.Query().Get("err"),
	}

	renderTemplate(
		w,
		"home.html",
		data,
		user.LanguagePreference,
	)
}

func HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}

	cookie, err := r.Cookie("keep_session")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	username, exists := getSessionUsername(cookie.Value)
	if !exists {
		http.NotFound(w, r)
		return
	}
	user, err := GetUserByUsername(username)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	filename := strings.TrimPrefix(r.URL.Path, "/uploads/")
	if filename == "" || filepath.Base(filename) != filename {
		http.NotFound(w, r)
		return
	}

	allowed, err := CanUserAccessUpload(r.URL.Path, user.ID)
	if err != nil || !allowed {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, filepath.Join("uploads", filename))
}

func HandleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	cookie, _ := r.Cookie("keep_session")
	username, _ := getSessionUsername(cookie.Value)
	user, _ := GetUserByUsername(username)

	r.ParseMultipartForm(10 << 20)
	bio := r.FormValue("bio")
	status := r.FormValue("status")
	language := Language(r.FormValue("language"))
	switch language {
	case LanguageEN, LanguageDE, LanguageZHTW:
		// valid
	default:
		language = LanguageEN
	}

	var imagePath string
	file, header, err := r.FormFile("pfp")
	if err == nil {
		defer file.Close()
		os.MkdirAll("uploads", os.ModePerm)
		filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), header.Filename)
		out, _ := os.Create("uploads/" + filename)
		defer out.Close()
		io.Copy(out, file)
		imagePath = "/uploads/" + filename
	}

	UpdateUserProfile(user.ID, bio, status, imagePath, language)
	http.Redirect(w, r, "/?tab=profile", http.StatusSeeOther)
}

func HandleAddPartner(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	cookie, _ := r.Cookie("keep_session")
	username, _ := getSessionUsername(cookie.Value)
	user, _ := GetUserByUsername(username)

	friendCode := r.FormValue("friend_code")
	err := ProcessFriendCode(user.ID, friendCode)
	if err != nil {
		http.Redirect(w, r, "/?tab=profile&err="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/?tab=profile", http.StatusSeeOther)
}

func HandleRemovePartner(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	user, ok := authenticatedUser(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := RemovePartnership(user.ID); err != nil {
		log.Printf(
			"failed removing partnership for user %d: %v",
			user.ID,
			err,
		)

		http.Error(
			w,
			"Failed to remove partnership",
			http.StatusInternalServerError,
		)
		return
	}

	http.Redirect(w, r, "/?tab=profile", http.StatusSeeOther)
}

func HandleNotificationSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, ok := authenticatedUser(r)
	if !ok {
		log.Printf("push subscription rejected: unauthenticated request")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var sub PushSubscription

	decoder := json.NewDecoder(
		http.MaxBytesReader(w, r.Body, 16<<10),
	)

	if err := decoder.Decode(&sub); err != nil {
		log.Printf(
			"push subscription rejected for user %d: invalid payload",
			user.ID,
		)

		http.Error(
			w,
			"Invalid subscription",
			http.StatusBadRequest,
		)
		return
	}

	if sub.Endpoint == "" ||
		sub.Keys.P256DH == "" ||
		sub.Keys.Auth == "" {

		log.Printf(
			"push subscription rejected for user %d: incomplete payload",
			user.ID,
		)

		http.Error(
			w,
			"Incomplete subscription",
			http.StatusBadRequest,
		)
		return
	}

	if _, err := db.Exec(`
		INSERT INTO push_subscriptions (
			user_id,
			endpoint,
			p256dh,
			auth
		)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(endpoint) DO UPDATE SET
			user_id = excluded.user_id,
			p256dh = excluded.p256dh,
			auth = excluded.auth
	`,
		user.ID,
		sub.Endpoint,
		sub.Keys.P256DH,
		sub.Keys.Auth,
	); err != nil {

		log.Printf(
			"failed saving push subscription for user %d: %v",
			user.ID,
			err,
		)

		http.Error(
			w,
			"Failed to save subscription",
			http.StatusInternalServerError,
		)
		return
	}

	log.Printf(
		"push subscription saved for user %d",
		user.ID,
	)

	w.WriteHeader(http.StatusNoContent)
}

func HandleNotificationUnsubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, ok := authenticatedUser(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var sub PushSubscription

	if err := json.NewDecoder(
		http.MaxBytesReader(w, r.Body, 16<<10),
	).Decode(&sub); err != nil {
		http.Error(w, "Invalid subscription", http.StatusBadRequest)
		return
	}

	if sub.Endpoint == "" {
		http.Error(w, "Invalid subscription", http.StatusBadRequest)
		return
	}

	_, err := db.Exec(`
		DELETE FROM push_subscriptions
		WHERE endpoint = ?
		  AND user_id = ?
	`,
		sub.Endpoint,
		user.ID,
	)

	if err != nil {
		log.Printf(
			"failed removing push subscription for user %d: %v",
			user.ID,
			err,
		)

		http.Error(
			w,
			"Failed to remove subscription",
			http.StatusInternalServerError,
		)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func HandleNotificationTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, ok := authenticatedUser(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := SendPushNotification(
		user.ID,
		"keep. 🌻",
		"Web Push is working!",
		"/?tab=inbox",
	); err != nil {
		log.Printf("test push failed for user %d: %v", user.ID, err)
		http.Error(w, "Failed to send test notification", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func HandleServiceWorker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Cache-Control", "no-cache")

	http.ServeFile(w, r, "static/service-worker.js")
}

func authenticatedUser(r *http.Request) (User, bool) {
	cookie, err := r.Cookie("keep_session")
	if err != nil {
		return User{}, false
	}

	username, exists := getSessionUsername(cookie.Value)
	if !exists {
		return User{}, false
	}

	user, err := GetUserByUsername(username)
	if err != nil {
		return User{}, false
	}

	return user, true
}
