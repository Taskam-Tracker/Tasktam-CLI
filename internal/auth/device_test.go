package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Taskam-Tracker/Tasktam-CLI/internal/api"
)

func TestPKCE(t *testing.T) {
	verifier, challenge, err := PKCE()
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(verifier))
	if challenge != base64.RawURLEncoding.EncodeToString(sum[:]) || len(challenge) != 43 || len(verifier) < 43 {
		t.Fatalf("bad pair %q / %q", verifier, challenge)
	}
}

func fakeServer(t *testing.T, replies []string) *httptest.Server {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["verifier"] != "v" || body["pair_id"] != "p" {
			t.Errorf("unexpected poll body %v", body)
		}
		reply := replies[min(n, len(replies)-1)]
		n++
		if reply == "token" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"token":"tk_new"}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"` + reply + `","detail":"x"}`))
	}))
	return srv
}

func TestWaitPollsUntilApproved(t *testing.T) {
	srv := fakeServer(t, []string{"authorization_pending", "slow_down", "token"})
	defer srv.Close()
	var slept []time.Duration
	sleep := func(_ context.Context, d time.Duration) error { slept = append(slept, d); return nil }
	start := &api.DeviceStart{PairID: "p", Interval: 5, ExpiresIn: 600}
	token, err := Wait(context.Background(), api.New(srv.URL, "", "test"), start, "v", sleep)
	if err != nil || token != "tk_new" {
		t.Fatalf("token=%q err=%v", token, err)
	}
	if want := []time.Duration{5 * time.Second, 5 * time.Second, 7 * time.Second}; len(slept) != 3 || slept[2] != want[2] {
		t.Errorf("intervals = %v, want %v (slow_down adds 2s)", slept, want)
	}
}

func TestWaitStopsWhenDenied(t *testing.T) {
	srv := fakeServer(t, []string{"authorization_pending", "access_denied"})
	defer srv.Close()
	noSleep := func(context.Context, time.Duration) error { return nil }
	_, err := Wait(context.Background(), api.New(srv.URL, "", "test"), &api.DeviceStart{PairID: "p", Interval: 1, ExpiresIn: 60}, "v", noSleep)
	if !errors.Is(err, ErrDenied) {
		t.Fatalf("err = %v, want ErrDenied", err)
	}
}

func TestWaitHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Wait(ctx, api.New("http://unused", "", "test"), &api.DeviceStart{PairID: "p", Interval: 1, ExpiresIn: 60}, "v", Sleep)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
