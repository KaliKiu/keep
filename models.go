package main

import "time"

type Language string

const (
	LanguageEN   Language = "en"
	LanguageDE   Language = "de"
	LanguageZHTW Language = "zh-TW"
)

type User struct {
	ID                 int
	Username           string
	FriendCode         string
	Bio                string
	Status             string
	PfpPath            string
	LanguagePreference Language
}

type PartnershipInfo struct {
	HasPartner  bool
	IsPending   bool
	IsSender    bool
	PartnerID   int
	PartnerName string
	PartnerCode string
}

type Letter struct {
	ID            int
	SenderID      int
	ReceiverID    int
	RequestID     string
	Title         string
	Content       string
	Emoji         string
	UnlockType    string
	UnlockAt      *time.Time
	SenderReady   bool
	ReceiverReady bool
	IsRead        bool
	CreatedAt     time.Time
	ReadAt        *time.Time
	ParentID      *int
	ImagePath     string

	// Computed fields just for the UI
	IsUnlocked          bool
	IsSender            bool
	Replies             []Letter // For threads
	CurrentUserReady    bool     // Mutual ready UI
	PartnerReady        bool     // Mutual ready UI
	LatestReplyUsername string
	LatestReplyRead     bool
}

type PushSubscription struct {
	Endpoint string `json:"endpoint"`

	Keys struct {
		P256DH string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

//struct for each html data to be passed to the template
