package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Taskam-Tracker/Tasktam-CLI/internal/api"
)

func (a *app) projectsCommand() *cobra.Command {
	var archived bool
	cmd := &cobra.Command{
		Use:     "projects",
		Aliases: []string{"p", "project"},
		Short:   "Your projects and the ones shared with you",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.requireLogin(); err != nil {
				return err
			}
			projects, err := a.client.Projects(cmd.Context(), archived)
			if err != nil {
				return err
			}
			if a.json {
				return a.printJSON(projects)
			}
			if len(projects) == 0 {
				a.println(a.style.Dim("No projects yet. Add a task with #Name to create one."))
				return nil
			}
			width := 1
			for _, p := range projects {
				width = max(width, len(strconv.Itoa(p.ID)))
			}
			for _, p := range projects {
				name := p.Name
				if p.Icon != "" {
					name = p.Icon + " " + name
				}
				var meta []string
				meta = append(meta, fmt.Sprintf("%d open", p.OpenCount))
				switch {
				case p.Role != "" && p.Role != "owner" && p.Owner != nil:
					meta = append(meta, fmt.Sprintf("%s · shared by %s", p.Role, p.Owner.Name))
				case p.MemberCount > 0:
					meta = append(meta, fmt.Sprintf("shared with %d", p.MemberCount))
				}
				if p.IsArchived {
					meta = append(meta, "archived")
				}
				a.println(fmt.Sprintf("%*d  %s  %s", width, p.ID, a.style.Bold(name), a.style.Dim(strings.Join(meta, " · "))))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&archived, "archived", false, "include archived projects")
	cmd.AddCommand(a.membersCommand(), a.shareCommand(), a.unshareCommand(), a.leaveCommand())
	return cmd
}

func (a *app) printMembers(members []api.Member) error {
	if a.json {
		return a.printJSON(members)
	}
	for _, m := range members {
		a.println(fmt.Sprintf("%-8s %s %s", m.Role, a.style.Bold(m.User.Name), a.style.Dim("<"+m.User.Email+">")))
	}
	return nil
}

func (a *app) membersCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "members <project>",
		Short: "Who a project is shared with",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireLogin(); err != nil {
				return err
			}
			p, err := a.project(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			members, err := a.client.Members(cmd.Context(), p.ID)
			if err != nil {
				return err
			}
			return a.printMembers(members)
		},
	}
}

func (a *app) shareCommand() *cobra.Command {
	var role string
	cmd := &cobra.Command{
		Use:   "share <project> <email>",
		Short: "Share a project with someone who has a Taskam account (or change their role)",
		Long: `Share a project you own. Editors add, edit, complete and assign tasks and comment; viewers only read.
Running it again for an existing member changes their role.`,
		Example: `  taskam projects share Launch sara@example.com
  taskam projects share Launch ali@example.com --role viewer`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireLogin(); err != nil {
				return err
			}
			if role != "editor" && role != "viewer" {
				return fmt.Errorf("--role must be editor or viewer")
			}
			ctx := cmd.Context()
			p, err := a.project(ctx, args[0])
			if err != nil {
				return err
			}
			members, err := a.client.Members(ctx, p.ID)
			if err != nil {
				return err
			}
			for _, m := range members {
				if strings.EqualFold(m.User.Email, args[1]) {
					if m.Role == "owner" {
						return fmt.Errorf("%s owns %s", m.User.Email, p.Name)
					}
					members, err = a.client.UpdateMember(ctx, p.ID, m.User.ID, role)
					if err != nil {
						return err
					}
					return a.printMembers(members)
				}
			}
			members, err = a.client.AddMember(ctx, p.ID, args[1], role)
			if err != nil {
				return err
			}
			return a.printMembers(members)
		},
	}
	cmd.Flags().StringVar(&role, "role", "editor", "editor or viewer")
	return cmd
}

func (a *app) unshareCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "unshare <project> <email>",
		Short: "Remove someone from a project you own (their assignments are cleared)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireLogin(); err != nil {
				return err
			}
			ctx := cmd.Context()
			p, err := a.project(ctx, args[0])
			if err != nil {
				return err
			}
			members, err := a.client.Members(ctx, p.ID)
			if err != nil {
				return err
			}
			for _, m := range members {
				if strings.EqualFold(m.User.Email, args[1]) && m.Role != "owner" {
					if err := a.client.RemoveMember(ctx, p.ID, m.User.ID); err != nil {
						return err
					}
					a.println(a.style.Green("✓"), "Removed", m.User.Email, "from", p.Name)
					return nil
				}
			}
			return fmt.Errorf("%s isn't a member of %s", args[1], p.Name)
		},
	}
}

func (a *app) leaveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "leave <project>",
		Short: "Leave a project someone shared with you",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireLogin(); err != nil {
				return err
			}
			p, err := a.project(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if err := a.client.Leave(cmd.Context(), p.ID); err != nil {
				return err
			}
			a.println(a.style.Green("✓"), "Left", p.Name)
			return nil
		},
	}
}

func (a *app) openCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "open [task id | today | upcoming | calendar | settings]",
		Short: "Open Taskam (or a task) in your browser",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			path := "/today"
			if len(args) == 1 {
				if id, err := strconv.Atoi(strings.TrimPrefix(args[0], "#")); err == nil {
					path = "/tasks/" + strconv.Itoa(id)
				} else {
					switch args[0] {
					case "today", "upcoming", "calendar", "settings", "inbox", "assistant":
						path = "/" + args[0]
					default:
						return fmt.Errorf("unknown page %q", args[0])
					}
				}
			}
			url := a.siteLink(path)
			if !openBrowser(url) {
				a.println(url)
			}
			return nil
		},
	}
}
