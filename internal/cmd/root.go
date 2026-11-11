// Package cmd defines the `taskam` commands.
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	neturl "net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/Taskam-Tracker/Tasktam-CLI/internal/api"
	"github.com/Taskam-Tracker/Tasktam-CLI/internal/config"
	"github.com/Taskam-Tracker/Tasktam-CLI/internal/render"
)

// ErrNotLoggedIn makes the process exit with code 3, so scripts can tell "log in first" from other failures.
var ErrNotLoggedIn = errors.New("not logged in: run `taskam login` (or set TASKAM_TOKEN)")

type app struct {
	version string
	cfg     *config.Config
	client  *api.Client
	style   render.Style
	json    bool
	out     io.Writer
	errOut  io.Writer

	me       *api.User
	projects []api.Project
}

// Execute runs the CLI and returns the process exit code.
func Execute(ctx context.Context, version string) int {
	a := &app{version: version, out: os.Stdout, errOut: os.Stderr}
	root := a.rootCommand()
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(a.errOut, a.style.Red("taskam: ")+err.Error())
		switch {
		case errors.Is(err, ErrNotLoggedIn), api.IsStatus(err, 401):
			return 3
		case errors.Is(err, context.Canceled):
			return 130
		case strings.HasPrefix(err.Error(), "unknown command"), strings.Contains(err.Error(), "flag"),
			strings.Contains(err.Error(), "arg(s)"):
			return 2
		}
		return 1
	}
	return 0
}

func (a *app) rootCommand() *cobra.Command {
	var noColor bool
	var apiURL string
	root := &cobra.Command{
		Use:           "taskam",
		Short:         "Taskam from your terminal: tasks, projects and your day",
		Version:       a.version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Long: `Taskam from your terminal.

  taskam login                      connect this terminal (approve it in your browser)
  taskam today                      what's due today, overdue and today's events
  taskam add "Call Sara tomorrow 10am #Work p1"
  taskam done 42                    complete a task
  taskam ls --project Work          list a project's open tasks

Dates follow your Taskam calendar setting (Jalali or Gregorian). Add --json to any command for scripts.`,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if apiURL != "" {
				cfg.APIURL = strings.TrimRight(apiURL, "/")
			}
			a.cfg = cfg
			a.client = api.New(cfg.APIURL, cfg.LoadToken(), a.version)
			a.style = render.Style{On: !noColor && os.Getenv("NO_COLOR") == "" && isTerminal(os.Stdout)}
			return nil
		},
	}
	root.PersistentFlags().BoolVar(&a.json, "json", false, "print machine-readable JSON")
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colours (also NO_COLOR=1)")
	root.PersistentFlags().StringVar(&apiURL, "api", "", "Taskam API address (default from config, TASKAM_API or "+config.DefaultAPI+")")
	root.SetVersionTemplate("taskam {{.Version}}\n")

	root.AddCommand(
		a.loginCommand(), a.logoutCommand(), a.whoamiCommand(),
		a.todayCommand(), a.upcomingCommand(), a.listCommand(), a.searchCommand(),
		a.addCommand(), a.showCommand(), a.doneCommand(), a.reopenCommand(), a.editCommand(),
		a.assignCommand(), a.commentCommand(), a.removeCommand(), a.restoreCommand(),
		a.projectsCommand(), a.openCommand(),
	)
	return root
}

func isTerminal(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }

// --- helpers shared by commands -----------------------------------------------------------------------

func (a *app) requireLogin() error {
	if a.client.Token == "" {
		return ErrNotLoggedIn
	}
	return nil
}

func (a *app) user(ctx context.Context) (*api.User, error) {
	if a.me == nil {
		me, err := a.client.Me(ctx)
		if err != nil {
			return nil, err
		}
		a.me = me
	}
	return a.me, nil
}

func (a *app) allProjects(ctx context.Context) ([]api.Project, error) {
	if a.projects == nil {
		projects, err := a.client.Projects(ctx, false)
		if err != nil {
			return nil, err
		}
		a.projects = projects
	}
	return a.projects, nil
}

// renderContext loads what task lines need: the user (dates, "me") and project names.
func (a *app) renderContext(ctx context.Context) (render.Context, error) {
	me, err := a.user(ctx)
	if err != nil {
		return render.Context{}, err
	}
	projects, err := a.allProjects(ctx)
	if err != nil {
		return render.Context{}, err
	}
	byID := make(map[int]api.Project, len(projects))
	for _, p := range projects {
		byID[p.ID] = p
	}
	return render.Context{Style: a.style, Dates: render.NewDates(me), Me: me, Projects: byID}, nil
}

// project finds a project by id or (case-insensitive) name among those the user can see.
func (a *app) project(ctx context.Context, ref string) (*api.Project, error) {
	projects, err := a.allProjects(ctx)
	if err != nil {
		return nil, err
	}
	id, idErr := strconv.Atoi(ref)
	var matches []api.Project
	for _, p := range projects {
		if (idErr == nil && p.ID == id) || strings.EqualFold(p.Name, ref) {
			matches = append(matches, p)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no project %q (see `taskam projects`)", ref)
	case 1:
		return &matches[0], nil
	}
	for _, p := range matches { // a shared project can share a name with one of yours: yours wins
		if p.Role == "owner" {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("several projects are called %q: use the id (see `taskam projects`)", ref)
}

func (a *app) printJSON(v any) error {
	enc := json.NewEncoder(a.out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func (a *app) println(parts ...any) { fmt.Fprintln(a.out, parts...) }

func parseIDs(args []string) ([]int, error) {
	ids := make([]int, 0, len(args))
	for _, arg := range args {
		id, err := strconv.Atoi(strings.TrimPrefix(arg, "#"))
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("%q is not a task id", arg)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// openBrowser opens url in the default browser; it reports false when it can't (e.g. over SSH). Only pages of
// the configured Taskam site are opened, so a server-supplied link can't hand the OS opener a file path, a
// custom scheme or a flag.
func (a *app) openBrowser(url string) bool {
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("TASKAM_NO_BROWSER") != "" || !a.onSite(url) {
		return false
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return false
		}
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start() == nil
}

// onSite reports whether raw is an http(s) URL on the configured web app's origin.
func (a *app) onSite(raw string) bool {
	u, err := neturl.Parse(raw)
	site, siteErr := neturl.Parse(a.cfg.SiteURL)
	if err != nil || siteErr != nil || u.User != nil {
		return false
	}
	return (u.Scheme == "https" || u.Scheme == "http") && u.Scheme == site.Scheme && strings.EqualFold(u.Host, site.Host)
}

// siteLink is a page of the web app ("/tasks/42").
func (a *app) siteLink(path string) string { return a.cfg.SiteURL + path }
