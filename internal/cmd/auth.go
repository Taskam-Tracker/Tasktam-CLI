package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Taskam-Tracker/Tasktam-CLI/internal/api"
	"github.com/Taskam-Tracker/Tasktam-CLI/internal/auth"
)

func (a *app) loginCommand() *cobra.Command {
	var token string
	var noBrowser bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Connect this terminal to your Taskam account",
		Long: `Connect this terminal. A short code is shown here; approve it in Taskam in your browser (any sign-in
works, including Google and GitHub). The CLI gets its own token, listed — and revocable — in
Taskam → Settings → API & MCP.

With --token, use a personal access token you created in Settings instead.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			previous := a.client.Token
			if token == "" {
				verifier, challenge, err := auth.PKCE()
				if err != nil {
					return err
				}
				anon := api.New(a.cfg.APIURL, "", a.version)
				host, _ := os.Hostname()
				start, err := anon.DeviceStart(ctx, challenge, strings.TrimSuffix(host, ".local"))
				if err != nil {
					return err
				}
				fmt.Fprintf(a.errOut, "To connect this terminal, open:\n\n  %s\n\nand confirm the code  %s\n\n",
					a.style.Bold(start.VerificationURIComplete), a.style.Bold(a.style.Violet(start.UserCode)))
				if !noBrowser && a.openBrowser(start.VerificationURIComplete) {
					fmt.Fprintln(a.errOut, a.style.Dim("Opened your browser."))
				}
				fmt.Fprintln(a.errOut, a.style.Dim("Waiting for approval… (Ctrl-C to cancel)"))
				token, err = auth.Wait(ctx, anon, start, verifier, auth.Sleep)
				if err != nil {
					return err
				}
			}
			token = strings.TrimSpace(token)
			if !strings.HasPrefix(token, "tk_") {
				return fmt.Errorf("that doesn't look like a Taskam token (they start with tk_)")
			}
			a.client.Token = token
			me, err := a.client.Me(ctx)
			if err != nil {
				return fmt.Errorf("the token doesn't work: %w", err)
			}
			where, err := a.cfg.StoreToken(token)
			if err != nil {
				return err
			}
			// Replacing an earlier CLI login: revoke its token so credentials don't pile up.
			if previous != "" && previous != token && os.Getenv("TASKAM_TOKEN") == "" {
				_ = a.revoke(cmd, previous)
			}
			if a.json {
				return a.printJSON(me)
			}
			a.println(a.style.Green("✓"), "Logged in as", a.style.Bold(me.Name), a.style.Dim("<"+me.Email+">"))
			a.println(a.style.Dim("Token saved in " + where + "."))
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "use an existing personal access token (tk_…)")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "don't open the browser; just print the link")
	return cmd
}

// revoke deletes the server-side token matching raw (by its hint) when it is a CLI token.
func (a *app) revoke(cmd *cobra.Command, raw string) error {
	client := api.New(a.cfg.APIURL, raw, a.version)
	tokens, err := client.Tokens(cmd.Context())
	if err != nil {
		return err
	}
	for _, t := range tokens {
		if t.Client == "cli" && len(raw) >= len(t.Hint) && strings.HasSuffix(raw, t.Hint) {
			return client.RevokeToken(cmd.Context(), t.ID)
		}
	}
	return nil
}

func (a *app) logoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Sign this terminal out and revoke its token",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if os.Getenv("TASKAM_TOKEN") != "" {
				return fmt.Errorf("TASKAM_TOKEN is set in your environment: unset it to sign out")
			}
			if a.client.Token == "" {
				a.println("Not logged in.")
				return nil
			}
			if err := a.revoke(cmd, a.client.Token); err != nil && !api.IsStatus(err, 401) {
				fmt.Fprintln(a.errOut, a.style.Yellow("Couldn't revoke the token on the server ("+err.Error()+
					"); revoke it in Taskam → Settings → API & MCP."))
			}
			if err := a.cfg.ClearToken(); err != nil {
				return err
			}
			a.println(a.style.Green("✓"), "Logged out.")
			return nil
		},
	}
}

func (a *app) whoamiCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the signed-in account",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.requireLogin(); err != nil {
				return err
			}
			me, err := a.user(cmd.Context())
			if err != nil {
				return err
			}
			if a.json {
				return a.printJSON(me)
			}
			calendar := "Gregorian"
			if me.CalendarSystem == "jalali" {
				calendar = "Jalali"
			}
			a.println(a.style.Bold(me.Name), a.style.Dim("<"+me.Email+">"))
			a.println(a.style.Dim(fmt.Sprintf("%s · %s calendar · %s", a.cfg.APIURL, calendar, me.Timezone)))
			return nil
		},
	}
}
