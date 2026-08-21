package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var sessions = make(map[string]string)

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
			sessionToken := time.Now().Format("20060102150405") + "-" + username
			sessions[sessionToken] = username
			http.SetCookie(w, &http.Cookie{Name: "keep_session", Value: sessionToken, Path: "/", Expires: time.Now().Add(7 * 24 * time.Hour), HttpOnly: true})
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		errorMsg = "Invalid username or password."
	}
	renderTemplate(w, "login.html", map[string]string{"Error": errorMsg}, LanguageEN)
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
		delete(sessions, cookie.Value)
		http.SetCookie(w, &http.Cookie{Name: "keep_session", Value: "", Path: "/", MaxAge: -1})
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func HandleHome(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("keep_session")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	username, exists := sessions[cookie.Value]
	if !exists {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, _ := GetUserByUsername(username)
	partnerInfo, _ := GetPartnershipInfo(user.ID)

	// Fetch Partner's full profile to display it!
	var partnerProfile User
	if partnerInfo.HasPartner {
		partnerProfile, _ = GetUserByID(partnerInfo.PartnerID)
	}

	letters, _ := GetVaultLetters(user.ID)

	tab := r.URL.Query().Get("tab")
	if tab == "" {
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

	renderTemplate(w, "home.html", data, user.LanguagePreference)
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
	username, exists := sessions[cookie.Value]
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
	username, _ := sessions[cookie.Value]
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
	username, _ := sessions[cookie.Value]
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
	cookie, _ := r.Cookie("keep_session")
	username, _ := sessions[cookie.Value]
	user, _ := GetUserByUsername(username)

	RemovePartnership(user.ID)
	http.Redirect(w, r, "/?tab=profile", http.StatusSeeOther)
}

func HandleNotificationSubscribe(w http.ResponseWriter, r *http.Request) {
	fmt.Println("PUSH 1: HandleNotificationSubscribe HIT")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("keep_session")
	if err != nil {
		fmt.Println("PUSH ERROR: keep_session cookie missing:", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	username, exists := sessions[cookie.Value]
	if !exists {
		fmt.Println("PUSH ERROR: session token not found")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := GetUserByUsername(username)
	if err != nil {
		fmt.Println("PUSH ERROR: user lookup:", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	fmt.Println("PUSH 2: authenticated user:", user.ID, user.Username)

	var sub PushSubscription

	err = json.NewDecoder(r.Body).Decode(&sub)
	if err != nil {
		fmt.Println("PUSH ERROR: JSON decode:", err)
		http.Error(w, "Invalid subscription", http.StatusBadRequest)
		return
	}

	fmt.Println("PUSH 3: endpoint:", sub.Endpoint)

	if sub.Endpoint == "" || sub.Keys.P256DH == "" || sub.Keys.Auth == "" {
		fmt.Println("PUSH ERROR: incomplete subscription")
		http.Error(w, "Incomplete subscription", http.StatusBadRequest)
		return
	}

	_, err = db.Exec(`
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
	)

	if err != nil {
		fmt.Println("PUSH ERROR: DB insert:", err)
		http.Error(w, "Failed to save subscription", http.StatusInternalServerError)
		return
	}

	fmt.Println("PUSH 4: SUBSCRIPTION SAVED ✅")

	w.WriteHeader(http.StatusNoContent)
}

func HandleNotificationTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("keep_session")
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	username, exists := sessions[cookie.Value]
	if !exists {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := GetUserByUsername(username)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err = SendPushNotification(
		user.ID,
		"keep. 🌻",
		"Web Push is working!",
		"/?tab=inbox",
	)

	if err != nil {
		fmt.Println("TEST PUSH ERROR:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
