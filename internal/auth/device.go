// Package auth implements `taskam login`: device authorization (RFC 8628 shape) with PKCE. The CLI shows a
// short code; the user approves it in a signed-in browser; the CLI polls until a personal token is issued.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/Taskam-Tracker/Tasktam-CLI/internal/api"
)

// PKCE returns a random verifier (kept here) and its S256 challenge (sent to the server).
func PKCE() (verifier, challenge string, err error) {
	buf := make([]byte, 48)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

var (
	ErrDenied  = errors.New("the login was denied in the browser")
	ErrExpired = errors.New("the login request expired; run `taskam login` again")
)

// Wait polls until the login is approved and returns the token. sleep is injectable for tests.
func Wait(ctx context.Context, client *api.Client, start *api.DeviceStart, verifier string,
	sleep func(context.Context, time.Duration) error) (string, error) {
	interval := time.Duration(max(start.Interval, 1)) * time.Second
	deadline := time.Now().Add(time.Duration(start.ExpiresIn) * time.Second)
	for time.Now().Before(deadline) {
		if err := sleep(ctx, interval); err != nil {
			return "", err
		}
		token, err := client.DeviceToken(ctx, start.PairID, verifier)
		switch {
		case err == nil:
			return token, nil
		case api.IsCode(err, "authorization_pending"):
		case api.IsCode(err, "slow_down"):
			interval += 2 * time.Second
		case api.IsCode(err, "access_denied"):
			return "", ErrDenied
		case api.IsCode(err, "expired_token"):
			return "", ErrExpired
		case api.IsStatus(err, 429):
			interval *= 2
		default:
			return "", fmt.Errorf("waiting for approval: %w", err)
		}
	}
	return "", ErrExpired
}

// Sleep waits for d or until ctx is cancelled (Ctrl-C).
func Sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
