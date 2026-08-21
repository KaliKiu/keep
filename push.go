package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	webpush "github.com/SherClockHolmes/webpush-go"
)

func SendPushNotification(userID int, title, body, url string) error {
	publicKey := os.Getenv("VAPID_PUBLIC_KEY")
	privateKey := os.Getenv("VAPID_PRIVATE_KEY")
	subscriber := os.Getenv("VAPID_SUBSCRIBER")

	if publicKey == "" || privateKey == "" {
		return fmt.Errorf("VAPID keys are not configured")
	}

	if subscriber == "" {
		return fmt.Errorf("VAPID_SUBSCRIBER is not configured")
	}

	payload, err := json.Marshal(map[string]string{
		"title": title,
		"body":  body,
		"url":   url,
	})
	if err != nil {
		return err
	}

	rows, err := db.Query(`
		SELECT endpoint, p256dh, auth
		FROM push_subscriptions
		WHERE user_id = ?
	`, userID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var endpoint string
		var p256dh string
		var auth string

		if err := rows.Scan(&endpoint, &p256dh, &auth); err != nil {
			return err
		}

		subscription := &webpush.Subscription{
			Endpoint: endpoint,
			Keys: webpush.Keys{
				P256dh: p256dh,
				Auth:   auth,
			},
		}

		response, err := webpush.SendNotification(
			payload,
			subscription,
			&webpush.Options{
				Subscriber:      subscriber,
				VAPIDPublicKey:  publicKey,
				VAPIDPrivateKey: privateKey,
				TTL:             60,
			},
		)

		if err != nil {
			log.Printf("Push send failed: %v", err)
			continue
		}

		io.Copy(io.Discard, response.Body)
		response.Body.Close()

		log.Printf("Push response for user %d: %s", userID, response.Status)

		// Subscription no longer exists on that device.
		if response.StatusCode == http.StatusGone ||
			response.StatusCode == http.StatusNotFound {

			_, _ = db.Exec(`
				DELETE FROM push_subscriptions
				WHERE endpoint = ?
			`, endpoint)

			log.Printf("Removed expired push subscription")
		}
	}

	return rows.Err()
}
