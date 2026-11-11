package api

import "time"

// Person is a user as other people see them (assignees, project members).
type Person struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// User is the signed-in account (GET /me/).
type User struct {
	ID                int    `json:"id"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	PreferredLanguage string `json:"preferred_language"`
	CalendarSystem    string `json:"calendar_system"` // "jalali" or "gregorian"
	Timezone          string `json:"timezone"`
	WeekStart         int    `json:"week_start"` // 0 = Monday … 6 = Sunday
}

// Location is the user's time zone, falling back to the machine's.
func (u *User) Location() *time.Location {
	if u != nil && u.Timezone != "" {
		if loc, err := time.LoadLocation(u.Timezone); err == nil {
			return loc
		}
	}
	return time.Local
}

type Task struct {
	ID           int        `json:"id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Status       string     `json:"status"`
	Priority     string     `json:"priority"`
	Project      *int       `json:"project"`
	Parent       *int       `json:"parent"`
	Labels       []int      `json:"labels"`
	DueAt        *time.Time `json:"due_at"`
	DueHasTime   bool       `json:"due_has_time"`
	Recurrence   string     `json:"recurrence"`
	EstimateMins *int       `json:"estimate_minutes"`
	CompletedAt  *time.Time `json:"completed_at"`
	SubtaskCount int        `json:"subtask_count"`
	SubtaskDone  int        `json:"subtask_done"`
	Assignee     *Person    `json:"assignee"`
	CreatedBy    *Person    `json:"created_by"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Closed reports whether the task is done or cancelled.
func (t Task) Closed() bool { return t.Status == "done" || t.Status == "cancelled" }

type Project struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Color       string  `json:"color"`
	Icon        string  `json:"icon"`
	IsArchived  bool    `json:"is_archived"`
	IsFavorite  bool    `json:"is_favorite"`
	OpenCount   int     `json:"open_count"`
	DoneCount   int     `json:"done_count"`
	MemberCount int     `json:"member_count"`
	Role        string  `json:"role"` // owner, editor, viewer
	Owner       *Person `json:"owner"`
}

type Label struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Color   string `json:"color"`
	Project *int   `json:"project"`
}

type Member struct {
	User    Person    `json:"user"`
	Role    string    `json:"role"`
	AddedAt time.Time `json:"added_at"`
}

type Activity struct {
	ID        int            `json:"id"`
	Kind      string         `json:"kind"` // created, updated, status, comment
	Body      string         `json:"body"`
	Changes   map[string]any `json:"changes"`
	Via       string         `json:"via"`
	ActorName *string        `json:"actor_name"`
	CreatedAt time.Time      `json:"created_at"`
}

type Stats struct {
	Open           int `json:"open"`
	Overdue        int `json:"overdue"`
	DueToday       int `json:"due_today"`
	P0Open         int `json:"p0_open"`
	Inbox          int `json:"inbox"`
	CompletedToday int `json:"completed_today"`
	CompletedWeek  int `json:"completed_week"`
}

type Event struct {
	ID           int       `json:"id"`
	Title        string    `json:"title"`
	StartAt      time.Time `json:"start_at"`
	EndAt        time.Time `json:"end_at"`
	AllDay       bool      `json:"all_day"`
	Location     string    `json:"location"`
	MeetingURL   string    `json:"meeting_url"`
	CalendarName string    `json:"calendar_name"`
}

// QuickParse is what quick add understood from a line of text (preview mode).
type QuickParse struct {
	Title      string   `json:"title"`
	Project    *string  `json:"project"`
	Labels     []string `json:"labels"`
	Priority   *string  `json:"priority"`
	DueDate    *string  `json:"due_date"`
	DueTime    *string  `json:"due_time"`
	Recurrence string   `json:"recurrence"`
}

type APIToken struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Client string `json:"client"`
	Hint   string `json:"hint"`
}

// DeviceStart is the reply to POST /auth/device/start/.
type DeviceStart struct {
	PairID                  string `json:"pair_id"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	Interval                int    `json:"interval"`
	ExpiresIn               int    `json:"expires_in"`
}
