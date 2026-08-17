package main

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

func HandleWriteLetter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	cookie, _ := r.Cookie("keep_session")
	username, _ := sessions[cookie.Value]
	user, _ := GetUserByUsername(username)
	partnerInfo, _ := GetPartnershipInfo(user.ID)

	if !partnerInfo.HasPartner || partnerInfo.IsPending {
		http.Redirect(w, r, "/?err=no_active_partner", http.StatusSeeOther)
		return
	}

	r.ParseMultipartForm(10 << 20)
	title := r.FormValue("title")
	content := r.FormValue("content")
	unlockType := r.FormValue("unlock_type")

	var unlockAt *time.Time
	if unlockType == "date" {
		dateStr := r.FormValue("unlock_date")
		parsedTime, err := time.Parse("2006-01-02T15:04", dateStr)
		if err == nil {
			unlockAt = &parsedTime
		}
	} else if unlockType == "random" {
		daysStr := r.FormValue("random_days")
		days, _ := strconv.Atoi(daysStr)
		if days == 0 {
			days = 7
		}
		randomSeconds := rand.Int63n(int64(days * 24 * 60 * 60))
		t := time.Now().Add(time.Duration(randomSeconds) * time.Second)
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
		os.MkdirAll("uploads", os.ModePerm)
		filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), header.Filename)
		out, _ := os.Create("uploads/" + filename)
		defer out.Close()
		io.Copy(out, file)
		imagePath = "/uploads/" + filename
	}

	CreateLetter(user.ID, partnerInfo.PartnerID, title, content, emoji, unlockType, unlockAt, nil, imagePath, "")
	http.Redirect(w, r, "/?tab=history_tx", http.StatusSeeOther)
}

func HandleViewLetter(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("keep_session")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	username, _ := sessions[cookie.Value]
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

	if !letter.IsUnlocked && letter.UnlockType != "mutual_ready" {
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
	renderTemplate(w, "letter.html", data)
}

func HandleReadyLetter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	cookie, _ := r.Cookie("keep_session")
	username, _ := sessions[cookie.Value]
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
	cookie, _ := r.Cookie("keep_session")
	username, _ := sessions[cookie.Value]
	user, _ := GetUserByUsername(username)
	partnerInfo, _ := GetPartnershipInfo(user.ID)

	r.ParseMultipartForm(10 << 20)
	parentID, _ := strconv.Atoi(r.FormValue("parent_id"))
	content := r.FormValue("content")
	emoji := "💬"

	var imagePath string
	file, header, err := r.FormFile("image")
	if err == nil {
		defer file.Close()
		os.MkdirAll("uploads", os.ModePerm)
		filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), header.Filename)
		out, _ := os.Create("uploads/" + filename)
		defer out.Close()
		io.Copy(out, file)
		imagePath = "/uploads/" + filename
	}

	CreateLetter(user.ID, partnerInfo.PartnerID, "Reply", content, emoji, "instant", nil, &parentID, imagePath, user.Username)
	http.Redirect(w, r, "/letter/view?id="+r.FormValue("parent_id"), http.StatusSeeOther)
}
