package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Taskam-Tracker/Tasktam-CLI/internal/api"
	"github.com/Taskam-Tracker/Tasktam-CLI/internal/render"
)

// --- lists -----------------------------------------------------------------------------------------------

func (a *app) todayCommand() *cobra.Command {
	var noEvents bool
	cmd := &cobra.Command{
		Use:     "today",
		Aliases: []string{"t"},
		Short:   "Overdue, due today and today's events",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.requireLogin(); err != nil {
				return err
			}
			ctx := cmd.Context()
			rc, err := a.renderContext(ctx)
			if err != nil {
				return err
			}
			tasks, err := a.client.Tasks(ctx, api.TaskFilter{Due: "today", Ordering: "due"})
			if err != nil {
				return err
			}
			var events []api.Event
			if !noEvents {
				start := time.Date(rc.Dates.Now.Year(), rc.Dates.Now.Month(), rc.Dates.Now.Day(), 0, 0, 0, 0, rc.Dates.Loc)
				events, _ = a.client.Events(ctx, start, start.AddDate(0, 0, 1)) // calendars are optional
			}
			if a.json {
				return a.printJSON(map[string]any{"tasks": tasks, "events": events})
			}
			var overdue, today []api.Task
			for _, t := range tasks {
				if t.DueAt != nil && rc.Dates.DaysFromToday(*t.DueAt) < 0 {
					overdue = append(overdue, t)
				} else {
					today = append(today, t)
				}
			}
			a.println(rc.Style.Bold(rc.Dates.Long(rc.Dates.Now)))
			width := render.IDWidth(tasks)
			if len(overdue) > 0 {
				a.println()
				a.println(rc.Heading("Overdue", len(overdue), true))
				a.printTasks(rc, overdue, width)
			}
			a.println()
			a.println(rc.Heading("Today", len(today), false))
			if len(today) == 0 {
				a.println(rc.Style.Dim("  Nothing due today."))
			}
			inDay := rc
			inDay.TimeOnly = true
			a.printTasks(inDay, today, width)
			if len(events) > 0 {
				a.println()
				a.println(rc.Heading("Calendar", len(events), false))
				for _, e := range events {
					a.println("  " + eventLine(rc, e))
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&noEvents, "no-events", false, "don't show calendar events")
	return cmd
}

func eventLine(rc render.Context, e api.Event) string {
	when := "all day     "
	if !e.AllDay {
		when = e.StartAt.In(rc.Dates.Loc).Format("15:04") + "–" + e.EndAt.In(rc.Dates.Loc).Format("15:04")
	}
	line := rc.Style.Dim(when) + "  " + e.Title
	if e.Location != "" {
		line += rc.Style.Dim(" · " + e.Location)
	}
	if strings.HasPrefix(e.MeetingURL, "https://") || strings.HasPrefix(e.MeetingURL, "http://") {
		line += "  " + rc.Style.Blue(e.MeetingURL)
	}
	return line
}

func (a *app) printTasks(rc render.Context, tasks []api.Task, width int) {
	for _, t := range tasks {
		a.println("  " + rc.TaskLine(t, width))
	}
}

func (a *app) upcomingCommand() *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:     "upcoming",
		Aliases: []string{"up"},
		Short:   "What's due after today, day by day",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.requireLogin(); err != nil {
				return err
			}
			ctx := cmd.Context()
			rc, err := a.renderContext(ctx)
			if err != nil {
				return err
			}
			tasks, err := a.client.Tasks(ctx, api.TaskFilter{Due: "upcoming", Ordering: "due", Limit: 500})
			if err != nil {
				return err
			}
			var ahead []api.Task
			for _, t := range tasks {
				if t.DueAt != nil && rc.Dates.DaysFromToday(*t.DueAt) >= 1 && rc.Dates.DaysFromToday(*t.DueAt) <= days {
					ahead = append(ahead, t)
				}
			}
			if a.json {
				return a.printJSON(ahead)
			}
			if len(ahead) == 0 {
				a.println(rc.Style.Dim(fmt.Sprintf("Nothing due in the next %d days.", days)))
				return nil
			}
			width := render.IDWidth(ahead)
			for i := 0; i < len(ahead); {
				day := rc.Dates.DaysFromToday(*ahead[i].DueAt)
				j := i
				for j < len(ahead) && rc.Dates.DaysFromToday(*ahead[j].DueAt) == day {
					j++
				}
				label := rc.Dates.Day(*ahead[i].DueAt)
				if day > 1 && day < 7 {
					label += ", " + rc.Dates.Date(*ahead[i].DueAt)
				}
				if i > 0 {
					a.println()
				}
				a.println(rc.Heading(strings.ToUpper(label[:1])+label[1:], j-i, false))
				inDay := rc
				inDay.TimeOnly = true // the heading already names the day
				a.printTasks(inDay, ahead[i:j], width)
				i = j
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&days, "days", "d", 14, "how many days ahead")
	return cmd
}

func (a *app) listCommand() *cobra.Command {
	var f api.TaskFilter
	var all bool
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List tasks with filters",
		Example: `  taskam ls --project Work
  taskam ls --due week --priority p0,p1
  taskam ls --assignee me --project Launch
  taskam ls --label urgent --all`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f.IncludeClosed = all
			return a.runList(cmd.Context(), f)
		},
	}
	fl := cmd.Flags()
	fl.StringVarP(&f.Project, "project", "p", "", "project name or id (`inbox` for the Inbox)")
	fl.StringVarP(&f.Labels, "label", "l", "", "comma-separated labels; a task must have all")
	fl.StringVar(&f.Priority, "priority", "", "comma-separated: p0,p1,p2,none")
	fl.StringVar(&f.Status, "status", "", "comma-separated: todo,in_progress,blocked,done,cancelled")
	fl.StringVar(&f.Due, "due", "", "overdue | today | tomorrow | week | upcoming | none | any")
	fl.StringVarP(&f.Assignee, "assignee", "a", "", "me | none | any | a teammate's id")
	fl.StringVar(&f.Ordering, "sort", "due", "due | priority | created | updated | title | position")
	fl.IntVarP(&f.Limit, "limit", "n", 100, "maximum number of tasks")
	fl.BoolVar(&all, "all", false, "include done and cancelled tasks")
	return cmd
}

func (a *app) searchCommand() *cobra.Command {
	var all bool
	return withAll(&cobra.Command{
		Use:   "search <text>",
		Short: "Search everything you can see (fuzzy title and notes)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runList(cmd.Context(), api.TaskFilter{
				Query: strings.Join(args, " "), Assignee: "any", IncludeClosed: all, Limit: 50, Ordering: "updated",
			})
		},
	}, &all)
}

func withAll(cmd *cobra.Command, all *bool) *cobra.Command {
	cmd.Flags().BoolVar(all, "all", false, "include done and cancelled tasks")
	return cmd
}

func (a *app) runList(ctx context.Context, f api.TaskFilter) error {
	if err := a.requireLogin(); err != nil {
		return err
	}
	if f.Project != "" && f.Project != "inbox" {
		p, err := a.project(ctx, f.Project)
		if err != nil {
			return err
		}
		f.Project = strconv.Itoa(p.ID)
	}
	tasks, err := a.client.Tasks(ctx, f)
	if err != nil {
		return err
	}
	if a.json {
		return a.printJSON(tasks)
	}
	rc, err := a.renderContext(ctx)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		a.println(rc.Style.Dim("No tasks."))
		return nil
	}
	a.printTasks(rc, tasks, render.IDWidth(tasks))
	a.println(rc.Style.Dim(fmt.Sprintf("\n%d task(s)", len(tasks))))
	return nil
}

// --- single tasks ------------------------------------------------------------------------------------------

func (a *app) addCommand() *cobra.Command {
	var project string
	var preview bool
	cmd := &cobra.Command{
		Use:   "add <text>",
		Short: "Add a task in plain language",
		Long: `Add a task the way you'd write it. Taskam understands, in English and Persian:
  dates and times   tomorrow 5pm · friday · next week · 12 dec · in 3 days
  repetition        every monday · every month
  priority          p0 p1 p2 (or !!! !! !)
  project / labels  #Work @errands  (one-word projects; use --project "Website relaunch" otherwise)`,
		Example: `  taskam add "Call Sara tomorrow 10am #Work p1"
  taskam add Pay rent every month @bills
  taskam add --project "Website relaunch" Review the hero copy friday
  taskam add --preview "Standup every monday 9:30"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireLogin(); err != nil {
				return err
			}
			ctx := cmd.Context()
			text := strings.Join(args, " ")
			if preview {
				parsed, err := a.client.PreviewQuickAdd(ctx, text)
				if err != nil {
					return err
				}
				if a.json {
					return a.printJSON(parsed)
				}
				a.println(a.style.Bold(parsed.Title))
				show := func(label string, value *string) {
					if value != nil && *value != "" {
						a.println(a.style.Dim(fmt.Sprintf("  %-9s", label)) + *value)
					}
				}
				show("due", joinNonEmpty(parsed.DueDate, parsed.DueTime))
				show("priority", parsed.Priority)
				show("project", parsed.Project)
				if len(parsed.Labels) > 0 {
					labels := "@" + strings.Join(parsed.Labels, " @")
					show("labels", &labels)
				}
				show("repeats", &parsed.Recurrence)
				return nil
			}
			if project != "" {
				p, err := a.project(ctx, project)
				if err != nil {
					return err
				}
				project = strconv.Itoa(p.ID)
			}
			task, err := a.client.QuickAdd(ctx, text, project)
			if err != nil {
				return err
			}
			if a.json {
				return a.printJSON(task)
			}
			rc, err := a.renderContext(ctx)
			if err != nil {
				return err
			}
			a.println(rc.Style.Green("+"), rc.TaskLine(*task, 1))
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "add to this project (name or id)")
	cmd.Flags().BoolVar(&preview, "preview", false, "show what Taskam understands, without adding")
	return cmd
}

func joinNonEmpty(parts ...*string) *string {
	var out []string
	for _, p := range parts {
		if p != nil && *p != "" {
			out = append(out, *p)
		}
	}
	joined := strings.Join(out, " ")
	return &joined
}

func (a *app) showCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "A task with its notes, subtasks and recent comments",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireLogin(); err != nil {
				return err
			}
			ids, err := parseIDs(args)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			task, err := a.client.Task(ctx, ids[0])
			if err != nil {
				return notFound(err, ids[0])
			}
			subtasks, err := a.client.Tasks(ctx, api.TaskFilter{Parent: strconv.Itoa(task.ID), IncludeClosed: true})
			if err != nil {
				return err
			}
			activity, err := a.client.Activity(ctx, task.ID)
			if err != nil {
				return err
			}
			var comments []api.Activity
			for _, item := range activity {
				if item.Kind == "comment" && len(comments) < 5 {
					comments = append(comments, item)
				}
			}
			if a.json {
				return a.printJSON(map[string]any{"task": task, "subtasks": subtasks, "comments": comments})
			}
			rc, err := a.renderContext(ctx)
			if err != nil {
				return err
			}
			s := rc.Style
			a.println(s.Bold(task.Title), s.Dim("#"+strconv.Itoa(task.ID)))
			field := func(label, value string) {
				if value != "" {
					a.println(s.Dim(fmt.Sprintf("  %-10s", label)) + value)
				}
			}
			field("status", strings.ReplaceAll(task.Status, "_", " "))
			if task.Priority != "none" {
				field("priority", strings.ToUpper(task.Priority))
			}
			due := rc.Dates.Due(*task)
			if rc.Dates.Overdue(*task) {
				due = s.Red(due + " (overdue)")
			}
			field("due", due)
			if task.Project != nil {
				if p, ok := rc.Projects[*task.Project]; ok {
					field("project", p.Name)
				}
			} else {
				field("project", "Inbox")
			}
			if task.Assignee != nil {
				field("assignee", task.Assignee.Name+" <"+task.Assignee.Email+">")
			}
			if task.CreatedBy != nil && (rc.Me == nil || task.CreatedBy.ID != rc.Me.ID) {
				field("created by", task.CreatedBy.Name)
			}
			field("repeats", task.Recurrence)
			if task.EstimateMins != nil {
				field("estimate", fmt.Sprintf("%dm", *task.EstimateMins))
			}
			if len(task.Labels) > 0 {
				if labels, err := a.client.Labels(ctx); err == nil {
					names := map[int]string{}
					for _, l := range labels {
						names[l.ID] = l.Name
					}
					var tags []string
					for _, id := range task.Labels {
						if n, ok := names[id]; ok {
							tags = append(tags, "@"+n)
						}
					}
					field("labels", strings.Join(tags, " "))
				}
			}
			if strings.TrimSpace(task.Description) != "" {
				a.println()
				for _, line := range strings.Split(strings.TrimSpace(task.Description), "\n") {
					a.println("  " + line)
				}
			}
			if len(subtasks) > 0 {
				a.println()
				a.println(rc.Heading("Subtasks", len(subtasks), false))
				a.printTasks(rc, subtasks, render.IDWidth(subtasks))
			}
			if len(comments) > 0 {
				a.println()
				a.println(rc.Heading("Comments", len(comments), false))
				for _, c := range comments {
					who := "someone"
					if c.ActorName != nil {
						who = *c.ActorName
					}
					a.println("  " + s.Bold(who) + s.Dim(" · "+rc.Dates.Day(c.CreatedAt)))
					for _, line := range strings.Split(strings.TrimSpace(c.Body), "\n") {
						a.println("    " + line)
					}
				}
			}
			a.println()
			a.println(s.Dim(a.siteLink("/tasks/" + strconv.Itoa(task.ID))))
			return nil
		},
	}
}

// eachTask runs fn for every id, reporting per-task failures and returning an error if any failed.
func (a *app) eachTask(ctx context.Context, args []string, fn func(id int) (string, error)) error {
	if err := a.requireLogin(); err != nil {
		return err
	}
	ids, err := parseIDs(args)
	if err != nil {
		return err
	}
	failed := 0
	for _, id := range ids {
		msg, err := fn(id)
		err = notFound(err, id)
		if err != nil {
			failed++
			fmt.Fprintf(a.errOut, "%s %d: %s\n", a.style.Red("✗"), id, err)
			continue
		}
		if msg != "" && !a.json {
			a.println(msg)
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d failed", failed, len(ids))
	}
	return nil
}

func (a *app) doneCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "done <id>...",
		Aliases: []string{"complete", "x"},
		Short:   "Complete tasks (recurring ones get their next occurrence)",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return a.eachTask(ctx, args, func(id int) (string, error) {
				t, err := a.client.Complete(ctx, id)
				if err != nil {
					return "", err
				}
				return a.style.Green("✓ ") + t.Title, nil
			})
		},
	}
}

func (a *app) reopenCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "reopen <id>...",
		Short: "Mark tasks as not done",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return a.eachTask(ctx, args, func(id int) (string, error) {
				t, err := a.client.UpdateTask(ctx, id, map[string]any{"status": "todo"})
				if err != nil {
					return "", err
				}
				return a.style.Yellow("○ ") + t.Title, nil
			})
		},
	}
}

func (a *app) removeCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "rm <id>...",
		Aliases: []string{"delete"},
		Short:   "Move tasks (and their subtasks) to the trash",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return a.eachTask(ctx, args, func(id int) (string, error) {
				if err := a.client.Delete(ctx, id); err != nil {
					return "", err
				}
				return a.style.Dim(fmt.Sprintf("Moved #%d to the trash (undo: taskam restore %d)", id, id)), nil
			})
		},
	}
}

func (a *app) restoreCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "restore <id>...",
		Short: "Bring tasks back from the trash",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return a.eachTask(ctx, args, func(id int) (string, error) {
				t, err := a.client.Restore(ctx, id)
				if err != nil {
					return "", err
				}
				return a.style.Green("↺ ") + t.Title, nil
			})
		},
	}
}

func (a *app) commentCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "comment <id> <text>",
		Short: "Add a comment to a task",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			body := strings.Join(args[1:], " ")
			return a.eachTask(ctx, args[:1], func(id int) (string, error) {
				return a.style.Green("✓ ") + "Comment added", a.client.Comment(ctx, id, body)
			})
		},
	}
}

func (a *app) assignCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "assign <id> <email|me|none>",
		Short: "Assign a task to a project member (or unassign it)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			var who any = args[1]
			if args[1] == "none" {
				who = nil
			}
			return a.eachTask(ctx, args[:1], func(id int) (string, error) {
				t, err := a.client.UpdateTask(ctx, id, map[string]any{"assignee": who})
				if err != nil {
					return "", err
				}
				if t.Assignee == nil {
					return a.style.Dim("Unassigned ") + t.Title, nil
				}
				return fmt.Sprintf("%s %s → %s", a.style.Green("✓"), t.Title, t.Assignee.Name), nil
			})
		},
	}
}

// --- edit ----------------------------------------------------------------------------------------------------

func (a *app) editCommand() *cobra.Command {
	var title, notes, priority, status, due, project, assignee string
	var estimate int
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Change a task's fields",
		Example: `  taskam edit 42 --due tomorrow --priority p1
  taskam edit 42 --due "2026-12-01 17:00" --project Work
  taskam edit 42 --due none --assignee me`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireLogin(); err != nil {
				return err
			}
			ctx := cmd.Context()
			ids, err := parseIDs(args)
			if err != nil {
				return err
			}
			changed := cmd.Flags().Changed
			fields := map[string]any{}
			if changed("title") {
				fields["title"] = title
			}
			if changed("notes") {
				fields["description"] = notes
			}
			if changed("priority") {
				fields["priority"] = strings.ToLower(priority)
			}
			if changed("status") {
				fields["status"] = strings.ToLower(status)
			}
			if changed("estimate") {
				fields["estimate_minutes"] = estimate
			}
			if changed("assignee") {
				if assignee == "none" {
					fields["assignee"] = nil
				} else {
					fields["assignee"] = assignee
				}
			}
			if changed("project") {
				if strings.EqualFold(project, "inbox") {
					fields["project"] = nil
				} else {
					p, err := a.project(ctx, project)
					if err != nil {
						return err
					}
					fields["project"] = strconv.Itoa(p.ID)
				}
			}
			if changed("due") {
				me, err := a.user(ctx)
				if err != nil {
					return err
				}
				at, hasTime, err := ParseDue(due, time.Now().In(me.Location()))
				if err != nil {
					return err
				}
				if at == nil {
					fields["due_at"] = nil
				} else {
					fields["due_at"] = at.Format(time.RFC3339)
					fields["due_has_time"] = hasTime
				}
			}
			if len(fields) == 0 {
				return fmt.Errorf("nothing to change: pass at least one flag (see `taskam edit --help`)")
			}
			task, err := a.client.UpdateTask(ctx, ids[0], fields)
			if err != nil {
				return err
			}
			if a.json {
				return a.printJSON(task)
			}
			rc, err := a.renderContext(ctx)
			if err != nil {
				return err
			}
			a.println(rc.Style.Green("✓"), rc.TaskLine(*task, 1))
			return nil
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&title, "title", "", "new title")
	fl.StringVar(&notes, "notes", "", "replace the notes (Markdown)")
	fl.StringVar(&priority, "priority", "", "p0 | p1 | p2 | none")
	fl.StringVar(&status, "status", "", "todo | in_progress | blocked | done | cancelled")
	fl.StringVar(&due, "due", "", "today | tomorrow | friday | +3d | 2026-12-01 | \"2026-12-01 17:00\" | 17:00 | none")
	fl.StringVar(&project, "project", "", "project name or id, or inbox")
	fl.StringVar(&assignee, "assignee", "", "a member's email, me, or none")
	fl.IntVar(&estimate, "estimate", 0, "estimate in minutes")
	return cmd
}

// notFound turns a 404 into a message naming the task (it may exist but not be visible to this account).
func notFound(err error, id int) error {
	if api.IsStatus(err, 404) {
		return fmt.Errorf("no task #%d (or it isn't shared with you)", id)
	}
	return err
}

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// ParseDue reads a deadline relative to now (in the user's zone). Whole days end at 23:59, like Taskam's own.
// It returns (nil, false, nil) for "none".
func ParseDue(input string, now time.Time) (*time.Time, bool, error) {
	s := strings.ToLower(strings.TrimSpace(input))
	loc := now.Location()
	endOf := func(t time.Time) *time.Time {
		v := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 0, 0, loc)
		return &v
	}
	at := func(day time.Time, clock string) (*time.Time, error) {
		hm, err := time.Parse("15:04", clock)
		if err != nil {
			return nil, fmt.Errorf("%q isn't a time (use HH:MM)", clock)
		}
		v := time.Date(day.Year(), day.Month(), day.Day(), hm.Hour(), hm.Minute(), 0, 0, loc)
		return &v, nil
	}
	switch {
	case s == "none" || s == "clear" || s == "":
		return nil, false, nil
	case s == "today":
		return endOf(now), false, nil
	case s == "tomorrow":
		return endOf(now.AddDate(0, 0, 1)), false, nil
	case strings.HasPrefix(s, "+") && strings.HasSuffix(s, "d"):
		n, err := strconv.Atoi(s[1 : len(s)-1])
		if err != nil || n < 0 {
			return nil, false, fmt.Errorf("%q: use +Nd, e.g. +3d", input)
		}
		return endOf(now.AddDate(0, 0, n)), false, nil
	}
	if len(s) >= 3 {
		if wd, ok := weekdays[s[:3]]; ok && strings.HasPrefix(weekdayName(wd), s) {
			ahead := (int(wd) - int(now.Weekday()) + 7) % 7
			if ahead == 0 {
				ahead = 7 // "friday" on a Friday means next Friday
			}
			return endOf(now.AddDate(0, 0, ahead)), false, nil
		}
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04", s, loc); err == nil {
		return &t, true, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, loc); err == nil {
		return endOf(t), false, nil
	}
	if strings.Contains(s, ":") {
		v, err := at(now, s)
		return v, err == nil, err
	}
	return nil, false, fmt.Errorf("can't read the date %q (try today, tomorrow, friday, +3d, 2026-12-01 or \"2026-12-01 17:00\")", input)
}

func weekdayName(wd time.Weekday) string { return strings.ToLower(wd.String()) }
