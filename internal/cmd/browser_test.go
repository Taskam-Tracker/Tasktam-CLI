package cmd

import (
	"testing"

	"github.com/Taskam-Tracker/Tasktam-CLI/internal/config"
)

func TestOnlySitePagesAreOpened(t *testing.T) {
	a := &app{cfg: &config.Config{SiteURL: "https://taskam.moeein.me"}}
	cases := map[string]bool{
		"https://taskam.moeein.me/device?code=ABCD-EFGH": true,
		"https://TASKAM.moeein.me/tasks/4":               true,
		"http://taskam.moeein.me/device":                 false,
		"https://evil.example/device":                    false,
		"https://taskam.moeein.me.evil.example/":         false,
		"https://user@taskam.moeein.me/":                 false,
		"file:///etc/passwd":                             false,
		"javascript:alert(1)":                            false,
		"-a Calculator":                                  false,
		"":                                               false,
	}
	for raw, want := range cases {
		if got := a.onSite(raw); got != want {
			t.Errorf("onSite(%q) = %v, want %v", raw, got, want)
		}
	}
}
