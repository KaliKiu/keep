package main

import "time"

type User struct {
	ID         int
	Username   string
	FriendCode string
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
	ID               int
	SenderID         int
	ReceiverID       int
	Title            string
	Content          string
	Emoji            string
	UnlockType       string
	UnlockAt         *time.Time
	SenderReady      bool
	ReceiverReady    bool
	IsRead           bool
	CreatedAt        time.Time
	ParentID         *int
	ImagePath        string
	IsUnlocked       bool
	IsSender         bool
	Replies          []Letter
	CurrentUserReady bool
	PartnerReady     bool
}
