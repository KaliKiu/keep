package main

import (
	"html/template"
	"net/http"
	"time"
)

var sessions = make(map[string]string)

// Helper to render template files cleanly
func renderTemplate(w http.ResponseWriter, tmplName string, data interface{}) {
	tmpl, err := template.ParseFiles("templates/" + tmplName)
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

			http.SetCookie(w, &http.Cookie{
				Name:     "keep_session",
				Value:    sessionToken,
				Path:     "/",
				Expires:  time.Now().Add(7 * 24 * time.Hour),
				HttpOnly: true,
			})

			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		errorMsg = "Invalid username or password."
	}

	renderTemplate(w, "login.html", map[string]string{"Error": errorMsg})
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
				errorMsg = "Registration failed: Username likely already taken."
			} else {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
		}
	}

	renderTemplate(w, "register.html", map[string]string{"Error": errorMsg})
}

func HandleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("keep_session")
	if err == nil {
		delete(sessions, cookie.Value)
		http.SetCookie(w, &http.Cookie{
			Name:   "keep_session",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
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

	renderTemplate(w, "home.html", map[string]string{"Username": username})
}
