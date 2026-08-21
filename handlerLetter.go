package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func HandleWriteLetter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	user, ok := authenticatedUser(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	partnerInfo, err := GetPartnershipInfo(user.ID)
	if err != nil {
		log.Printf("failed loading partnership for user %d: %v", user.ID, err)
		http.Error(w, "Failed to load partnership", http.StatusInternalServerError)
		return
	}

	if !partnerInfo.HasPartner || partnerInfo.IsPending {
		http.Redirect(w, r, "/?err=no_active_partner", http.StatusSeeOther)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	requestID := r.FormValue("request_id")
	if requestID == "" {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	content := r.FormValue("content")
	unlockType := r.FormValue("unlock_type")

	var unlockAt *time.Time

	if unlockType == "date" {
		dateStr := r.FormValue("unlock_date")

		parsedTime, err := time.Parse("2006-01-02T15:04", dateStr)
		if err != nil {
			http.Error(w, "Invalid unlock date", http.StatusBadRequest)
			return
		}

		offset, err := strconv.Atoi(r.FormValue("tz_offset"))
		if err != nil {
			http.Error(w, "Invalid timezone offset", http.StatusBadRequest)
			return
		}

		parsedTime = parsedTime.Add(time.Duration(offset) * time.Minute)
		unlockAt = &parsedTime
	}

	if unlockType == "random" {
		days, err := strconv.Atoi(r.FormValue("random_days"))
		if err != nil || days <= 0 {
			days = 7
		}

		randomSeconds := rand.Int63n(int64(days * 24 * 60 * 60))
		t := time.Now().UTC().Add(time.Duration(randomSeconds) * time.Second)
		unlockAt = &t
	}

	emoji := r.FormValue("emoji")
	if emoji == "" {
		emoji = "💌"
	}

	var imagePath string

	file, header, err := r.FormFile("image")
	if err == nil {
		defer file.Close()

		if err := os.MkdirAll("uploads", 0755); err != nil {
			log.Printf("failed creating uploads directory: %v", err)
			http.Error(w, "Failed to save upload", http.StatusInternalServerError)
			return
		}

		filename := fmt.Sprintf(
			"%d_%s",
			time.Now().UnixNano(),
			filepath.Base(header.Filename),
		)

		fullPath := filepath.Join("uploads", filename)

		out, err := os.Create(fullPath)
		if err != nil {
			log.Printf("failed creating upload file: %v", err)
			http.Error(w, "Failed to save upload", http.StatusInternalServerError)
			return
		}

		defer out.Close()

		if _, err := io.Copy(out, file); err != nil {
			log.Printf("failed saving upload: %v", err)
			http.Error(w, "Failed to save upload", http.StatusInternalServerError)
			return
		}

		imagePath = "/uploads/" + filename
	}

	created, err := CreateLetter(
		user.ID,
		partnerInfo.PartnerID,
		requestID,
		title,
		content,
		emoji,
		unlockType,
		unlockAt,
		nil,
		imagePath,
		"",
	)

	if err != nil {
		log.Printf("failed creating letter for user %d: %v", user.ID, err)
		http.Error(w, "Failed to send letter", http.StatusInternalServerError)
		return
	}

	if !created {
		http.Redirect(w, r, "/?tab=history_tx", http.StatusSeeOther)
		return
	}

	go func(receiverID int, senderName string) {
		if err := SendPushNotification(
			receiverID,
			"keep. 💌",
			senderName+" sent you a new letter.",
			"/?tab=inbox",
		); err != nil {
			log.Printf(
				"letter push failed for receiver %d: %v",
				receiverID,
				err,
			)
		}
	}(partnerInfo.PartnerID, user.Username)

	http.Redirect(w, r, "/?tab=history_tx", http.StatusSeeOther)
}

func HandleViewLetter(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("keep_session")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	username, _ := getSessionUsername(cookie.Value)
	user, _ := GetUserByUsername(username)

	letterIDStr := r.URL.Query().Get("id")
	letterID, err := strconv.Atoi(letterIDStr)
	if err != nil {
		http.Redirect(w, r, "/?tab=inbox", http.StatusSeeOther)
		return
	}

	letter, err := GetLetterByID(letterID, user.ID)
	if err != nil {
		http.Redirect(w, r, "/?tab=inbox", http.StatusSeeOther)
		return
	}

	if !letter.IsSender && !letter.IsUnlocked && letter.UnlockType != "mutual_ready" {
		http.Redirect(w, r, "/?tab=inbox&err=Letter+is+still+sealed", http.StatusSeeOther)
		return
	}

	// Trigger read receipt the moment they open it!
	if letter.IsUnlocked && !letter.IsSender && !letter.IsRead {
		MarkLetterAsRead(letter.ID)
	}
	if letter.LatestReplyUsername != user.Username && !letter.LatestReplyRead {
		MarkReplyLetterAsRead(letter.ID)
	}

	data := map[string]interface{}{
		"Letter": letter,
	}
	renderTemplate(w, "letter.html", data, user.LanguagePreference)
}

func HandleReadyLetter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	cookie, _ := r.Cookie("keep_session")
	username, _ := getSessionUsername(cookie.Value)
	user, _ := GetUserByUsername(username)

	letterID, _ := strconv.Atoi(r.FormValue("id"))
	ToggleReadyStatus(letterID, user.ID)
	http.Redirect(w, r, "/letter/view?id="+r.FormValue("id"), http.StatusSeeOther)
}

func HandleReplyLetter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	user, ok := authenticatedUser(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	partnerInfo, err := GetPartnershipInfo(user.ID)
	if err != nil {
		log.Printf("failed loading partnership for user %d: %v", user.ID, err)
		http.Error(w, "Failed to load partnership", http.StatusInternalServerError)
		return
	}

	if !partnerInfo.HasPartner || partnerInfo.IsPending {
		http.Error(w, "No active partner", http.StatusBadRequest)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	requestID := r.FormValue("request_id")
	if requestID == "" {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	parentID, err := strconv.Atoi(r.FormValue("parent_id"))
	if err != nil {
		http.Error(w, "Invalid parent letter", http.StatusBadRequest)
		return
	}

	content := r.FormValue("content")
	emoji := "💬"

	var imagePath string

	file, header, err := r.FormFile("image")
	if err == nil {
		defer file.Close()

		if err := os.MkdirAll("uploads", 0755); err != nil {
			log.Printf("failed creating uploads directory: %v", err)
			http.Error(w, "Failed to save upload", http.StatusInternalServerError)
			return
		}

		filename := fmt.Sprintf(
			"%d_%s",
			time.Now().UnixNano(),
			filepath.Base(header.Filename),
		)

		fullPath := filepath.Join("uploads", filename)

		out, err := os.Create(fullPath)
		if err != nil {
			log.Printf("failed creating reply upload: %v", err)
			http.Error(w, "Failed to save upload", http.StatusInternalServerError)
			return
		}

		defer out.Close()

		if _, err := io.Copy(out, file); err != nil {
			log.Printf("failed saving reply upload: %v", err)
			http.Error(w, "Failed to save upload", http.StatusInternalServerError)
			return
		}

		imagePath = "/uploads/" + filename
	}

	created, err := CreateLetter(
		user.ID,
		partnerInfo.PartnerID,
		requestID,
		"Reply",
		content,
		emoji,
		"instant",
		nil,
		&parentID,
		imagePath,
		user.Username,
	)

	if err != nil {
		log.Printf("failed creating reply for user %d: %v", user.ID, err)
		http.Error(w, "Failed to send reply", http.StatusInternalServerError)
		return
	}

	if !created {
		http.Redirect(
			w,
			r,
			"/letter/view?id="+strconv.Itoa(parentID),
			http.StatusSeeOther,
		)
		return
	}

	if err := UpdateParentWithReply(parentID, user.Username); err != nil {
		log.Printf("failed updating parent letter %d: %v", parentID, err)
	}

	go func(receiverID, parentLetterID int, senderName string) {
		if err := SendPushNotification(
			receiverID,
			"keep. 💬",
			senderName+" replied to your letter.",
			"/letter/view?id="+strconv.Itoa(parentLetterID),
		); err != nil {
			log.Printf(
				"reply push failed for receiver %d: %v",
				receiverID,
				err,
			)
		}
	}(partnerInfo.PartnerID, parentID, user.Username)

	http.Redirect(
		w,
		r,
		"/letter/view?id="+strconv.Itoa(parentID),
		http.StatusSeeOther,
	)
}
