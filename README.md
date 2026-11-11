# taskam — Taskam from your terminal

```console
$ taskam login
To connect this terminal, open:

  https://taskam.moeein.me/device?code=K3F9-Q2ZD

and confirm the code  K3F9-Q2ZD

✓ Logged in as Moeein <you@example.com>

$ taskam today
Wednesday, 20 Aban 1405

Overdue (1)
  25  ○ P1  Send invoice to Acme  18 Aban

Today (3)
  26  ○ P1  Prepare sprint demo  16:30 · #Launch · @Sara
  27  ○     Call the dentist

Calendar (1)
  10:00–10:45  Team standup  https://meet.google.com/…

$ taskam add "Pay rent every month p1 #Home @bills"
+ 31  ○ P1  Pay rent  today · ↻ · #Home

$ taskam done 26
✓ Prepare sprint demo
```

Dates follow your Taskam setting (Jalali or Gregorian) and time zone.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/Taskam-Tracker/Tasktam-CLI/main/install.sh | sh
# or, with Go 1.24+
go install github.com/Taskam-Tracker/Tasktam-CLI@latest   # installs as Tasktam-CLI; rename to taskam
```

Release archives for macOS, Linux and Windows (amd64/arm64) are on the Releases page, with checksums.

## Commands

| | |
|---|---|
| `taskam login` / `logout` / `whoami` | Connect by approving a code in your browser (works with Google/GitHub sign-in). `--token tk_…` uses a token from Settings → API & MCP. `logout` revokes the token. |
| `taskam today` | Overdue, due today and today's calendar events. |
| `taskam upcoming [-d 14]` | What's due after today, grouped by day. |
| `taskam ls` | Filters: `--project`, `--label`, `--priority p0,p1`, `--status`, `--due week`, `--assignee me`, `--all`, `--sort`, `-n`. |
| `taskam search <text>` | Fuzzy search across everything you can see. |
| `taskam add <text>` | Natural language: `tomorrow 5pm`, `every monday`, `p1`, `#Project`, `@label` (English and Persian). `--project` for multi-word project names, `--preview` to see the parse. |
| `taskam show <id>` | Notes, subtasks and recent comments. |
| `taskam done` / `reopen` / `rm` / `restore` `<id>...` | `rm` moves to the trash (30 days). |
| `taskam edit <id>` | `--title --notes --priority --status --due --project --assignee --estimate`. Due: `today`, `tomorrow`, `friday`, `+3d`, `2026-12-01`, `"2026-12-01 17:00"`, `17:00`, `none`. |
| `taskam assign <id> <email\|me\|none>`, `comment <id> <text>` | Shared projects: only the owner and editors can be assigned. |
| `taskam projects` | Yours and those shared with you; `members`, `share <project> <email> [--role viewer]`, `unshare`, `leave`. |
| `taskam open [id\|today\|calendar…]` | Open the web app. |
| `taskam completion bash\|zsh\|fish\|powershell` | Shell completion. |

Every command accepts `--json` for scripts. Exit codes: `0` ok, `1` error, `2` usage, `3` not logged in.

## Configuration

- **Token**: stored in the OS keychain (Keychain, Credential Manager, Secret Service). Without one (headless
  servers) it goes to the config file, readable only by you. `TASKAM_TOKEN` overrides it, e.g. in CI.
- **Server**: `--api`, `TASKAM_API`, or `api_url` in the config file
  (`taskam` config dir: `~/Library/Application Support/taskam`, `~/.config/taskam`, `%AppData%\taskam`;
  override with `TASKAM_CONFIG_DIR`).
- `NO_COLOR=1` or `--no-color` disables colours; output to a pipe is never coloured.

## Develop

```sh
go test -race ./...
go run . today --api http://localhost:8000
```

Tagging `vX.Y.Z` builds and publishes release archives with GoReleaser.
