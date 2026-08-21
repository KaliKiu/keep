package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	webpush "github.com/SherClockHolmes/webpush-go"
)

type pushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
}

func SendPushNotification(userID int, title, body, url string) error {
	publicKey := os.Getenv("VAPID_PUBLIC_KEY")
	privateKey := os.Getenv("VAPID_PRIVATE_KEY")
	subscriber := os.Getenv("VAPID_SUBSCRIBER")

	if publicKey == "" || privateKey == "" || subscriber == "" {
		return errors.New("web push is not configured")
	}

	payload, err := json.Marshal(pushPayload{
		Title: title,
		Body:  body,
		URL:   url,
	})
	if err != nil {
		return fmt.Errorf("encode push payload: %w", err)
	}

	rows, err := db.Query(`
		SELECT endpoint, p256dh, auth
		FROM push_subscriptions
		WHERE user_id = ?
	`, userID)
	if err != nil {
		return fmt.Errorf("query push subscriptions: %w", err)
	}
	defer rows.Close()

	var (
		sentCount   int
		failedCount int
	)

	for rows.Next() {
		var (
			endpoint string
			p256dh   string
			auth     string
		)

		if err := rows.Scan(&endpoint, &p256dh, &auth); err != nil {
			return fmt.Errorf("scan push subscription: %w", err)
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
			failedCount++
			log.Printf("push delivery failed for user %d: %v", userID, err)
			continue
		}

		_, _ = io.Copy(io.Discard, response.Body)
		response.Body.Close()

		switch response.StatusCode {
		case http.StatusCreated,
			http.StatusOK,
			http.StatusAccepted,
			http.StatusNoContent:

			sentCount++

		case http.StatusGone,
			http.StatusNotFound:

			if err := deletePushSubscription(endpoint); err != nil {
				log.Printf(
					"failed removing expired subscription for user %d: %v",
					userID,
					err,
				)
			}

		default:
			failedCount++

			log.Printf(
				"push service returned status %d for user %d",
				response.StatusCode,
				userID,
			)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate push subscriptions: %w", err)
	}

	if sentCount == 0 && failedCount > 0 {
		return fmt.Errorf("all push deliveries failed")
	}

	log.Printf(
		"push delivery finished for user %d: %d sent, %d failed",
		userID,
		sentCount,
		failedCount,
	)

	return nil
}

func deletePushSubscription(endpoint string) error {
	_, err := db.Exec(`
		DELETE FROM push_subscriptions
		WHERE endpoint = ?
	`, endpoint)

	return err
}
