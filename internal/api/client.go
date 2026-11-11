// Package api is a small typed client for the Taskam REST API (/api/v1).
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Error is a non-2xx reply. Code is Taskam's stable machine-readable code ("task_not_found", "forbidden", …).
type Error struct {
	Status int
	Code   string
	Detail string
	Fields map[string][]string
}

func (e *Error) Error() string {
	msg := e.Detail
	if msg == "" || msg == "Invalid input." {
		var parts []string
		for field, problems := range e.Fields {
			for _, p := range problems {
				if field == "non_field_errors" {
					parts = append(parts, p)
				} else {
					parts = append(parts, field+": "+p)
				}
			}
		}
		if len(parts) > 0 {
			msg = strings.Join(parts, "; ")
		}
	}
	if msg == "" {
		msg = http.StatusText(e.Status)
	}
	return msg
}

// IsCode reports whether err is an API error with the given code.
func IsCode(err error, code string) bool {
	var apiErr *Error
	return errors.As(err, &apiErr) && apiErr.Code == code
}

// IsStatus reports whether err is an API error with the given HTTP status.
func IsStatus(err error, status int) bool {
	var apiErr *Error
	return errors.As(err, &apiErr) && apiErr.Status == status
}

type Client struct {
	BaseURL   string // e.g. https://api-taskam.moeein.me
	Token     string // personal access token (tk_…); empty for anonymous calls
	UserAgent string
	HTTP      *http.Client
}

func New(baseURL, token, version string) *Client {
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		Token:     token,
		UserAgent: "taskam-cli/" + version,
		HTTP:      &http.Client{Timeout: 30 * time.Second},
	}
}

// Do sends a request to /api/v1{path} and decodes a JSON reply into out (when non-nil).
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	endpoint := c.BaseURL + "/api/v1" + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("can't reach Taskam at %s: %w", c.BaseURL, err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return err
	}
	if cleaned, err := cleanJSON(data); err == nil {
		data = cleaned
	}
	if res.StatusCode >= 400 {
		apiErr := &Error{Status: res.StatusCode}
		var payload struct {
			Error  string              `json:"error"`
			Detail string              `json:"detail"`
			Fields map[string][]string `json:"fields"`
		}
		if json.Unmarshal(data, &payload) == nil {
			apiErr.Code, apiErr.Detail, apiErr.Fields = payload.Error, payload.Detail, payload.Fields
		}
		return apiErr
	}
	if out == nil || res.StatusCode == http.StatusNoContent || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("unexpected reply from %s: %w", path, err)
	}
	return nil
}

// --- account -------------------------------------------------------------------------------------------

func (c *Client) Me(ctx context.Context) (*User, error) {
	var u User
	return &u, c.Do(ctx, http.MethodGet, "/me/", nil, nil, &u)
}

func (c *Client) Tokens(ctx context.Context) ([]APIToken, error) {
	var tokens []APIToken
	return tokens, c.Do(ctx, http.MethodGet, "/me/api-tokens/", nil, nil, &tokens)
}

func (c *Client) RevokeToken(ctx context.Context, id int) error {
	return c.Do(ctx, http.MethodDelete, "/me/api-tokens/"+strconv.Itoa(id)+"/", nil, nil, nil)
}

func (c *Client) DeviceStart(ctx context.Context, challenge, name string) (*DeviceStart, error) {
	var start DeviceStart
	body := map[string]string{"challenge": challenge, "name": name, "client": "cli"}
	return &start, c.Do(ctx, http.MethodPost, "/auth/device/start/", nil, body, &start)
}

// DeviceToken polls for the token; it fails with codes authorization_pending, slow_down, access_denied or
// expired_token until the login is approved.
func (c *Client) DeviceToken(ctx context.Context, pairID, verifier string) (string, error) {
	var reply struct {
		Token string `json:"token"`
	}
	body := map[string]string{"pair_id": pairID, "verifier": verifier}
	err := c.Do(ctx, http.MethodPost, "/auth/device/token/", nil, body, &reply)
	return reply.Token, err
}

// --- tasks ---------------------------------------------------------------------------------------------

// TaskFilter mirrors GET /tasks/ query parameters. Empty fields are omitted.
type TaskFilter struct {
	Query, Project, Labels, Priority, Status, Due, Parent, Assignee, Ordering string
	IncludeClosed                                                             bool
	Limit                                                                     int
}

func (f TaskFilter) values() url.Values {
	v := url.Values{}
	set := func(k, val string) {
		if val != "" {
			v.Set(k, val)
		}
	}
	set("q", f.Query)
	set("project", f.Project)
	set("labels", f.Labels)
	set("priority", f.Priority)
	set("status", f.Status)
	set("due", f.Due)
	set("parent", f.Parent)
	set("assignee", f.Assignee)
	set("ordering", f.Ordering)
	if f.IncludeClosed {
		v.Set("include_closed", "true")
	}
	if f.Limit > 0 {
		v.Set("limit", strconv.Itoa(f.Limit))
	}
	return v
}

func (c *Client) Tasks(ctx context.Context, f TaskFilter) ([]Task, error) {
	var tasks []Task
	return tasks, c.Do(ctx, http.MethodGet, "/tasks/", f.values(), nil, &tasks)
}

func (c *Client) Task(ctx context.Context, id int) (*Task, error) {
	var t Task
	return &t, c.Do(ctx, http.MethodGet, taskPath(id, ""), nil, nil, &t)
}

func (c *Client) QuickAdd(ctx context.Context, text, project string) (*Task, error) {
	var t Task
	body := map[string]any{"text": text}
	if project != "" {
		body["project"] = project
	}
	return &t, c.Do(ctx, http.MethodPost, "/tasks/quick-add/", nil, body, &t)
}

func (c *Client) PreviewQuickAdd(ctx context.Context, text string) (*QuickParse, error) {
	var p QuickParse
	return &p, c.Do(ctx, http.MethodPost, "/tasks/quick-add/", nil, map[string]any{"text": text, "preview": true}, &p)
}

// UpdateTask PATCHes the given fields (title, description, status, priority, project, due_at, due_has_time,
// assignee, …). A nil value clears a field.
func (c *Client) UpdateTask(ctx context.Context, id int, fields map[string]any) (*Task, error) {
	var t Task
	return &t, c.Do(ctx, http.MethodPatch, taskPath(id, ""), nil, fields, &t)
}

func (c *Client) Complete(ctx context.Context, id int) (*Task, error) {
	var t Task
	return &t, c.Do(ctx, http.MethodPost, taskPath(id, "complete/"), nil, nil, &t)
}

// Delete moves a task and its subtasks to the trash (restorable for 30 days).
func (c *Client) Delete(ctx context.Context, id int) error {
	return c.Do(ctx, http.MethodDelete, taskPath(id, ""), nil, nil, nil)
}

func (c *Client) Restore(ctx context.Context, id int) (*Task, error) {
	var t Task
	return &t, c.Do(ctx, http.MethodPost, taskPath(id, "restore/"), nil, nil, &t)
}

func (c *Client) Activity(ctx context.Context, id int) ([]Activity, error) {
	var items []Activity
	return items, c.Do(ctx, http.MethodGet, taskPath(id, "activity/"), nil, nil, &items)
}

func (c *Client) Comment(ctx context.Context, id int, body string) error {
	return c.Do(ctx, http.MethodPost, taskPath(id, "activity/"), nil, map[string]string{"body": body}, nil)
}

func (c *Client) Stats(ctx context.Context) (*Stats, error) {
	var s Stats
	return &s, c.Do(ctx, http.MethodGet, "/tasks/stats/", nil, nil, &s)
}

func taskPath(id int, suffix string) string {
	return "/tasks/" + strconv.Itoa(id) + "/" + suffix
}

// --- projects, labels, events --------------------------------------------------------------------------

func (c *Client) Projects(ctx context.Context, archived bool) ([]Project, error) {
	var projects []Project
	var q url.Values
	if archived {
		q = url.Values{"archived": {"1"}}
	}
	return projects, c.Do(ctx, http.MethodGet, "/projects/", q, nil, &projects)
}

func (c *Client) Labels(ctx context.Context) ([]Label, error) {
	var labels []Label
	return labels, c.Do(ctx, http.MethodGet, "/labels/", nil, nil, &labels)
}

func (c *Client) Members(ctx context.Context, projectID int) ([]Member, error) {
	var m []Member
	return m, c.Do(ctx, http.MethodGet, projectPath(projectID, "members/"), nil, nil, &m)
}

func (c *Client) AddMember(ctx context.Context, projectID int, email, role string) ([]Member, error) {
	var m []Member
	body := map[string]string{"email": email, "role": role}
	return m, c.Do(ctx, http.MethodPost, projectPath(projectID, "members/"), nil, body, &m)
}

func (c *Client) UpdateMember(ctx context.Context, projectID, userID int, role string) ([]Member, error) {
	var m []Member
	path := projectPath(projectID, "members/"+strconv.Itoa(userID)+"/")
	return m, c.Do(ctx, http.MethodPatch, path, nil, map[string]string{"role": role}, &m)
}

func (c *Client) RemoveMember(ctx context.Context, projectID, userID int) error {
	return c.Do(ctx, http.MethodDelete, projectPath(projectID, "members/"+strconv.Itoa(userID)+"/"), nil, nil, nil)
}

func (c *Client) Leave(ctx context.Context, projectID int) error {
	return c.Do(ctx, http.MethodPost, projectPath(projectID, "leave/"), nil, map[string]string{}, nil)
}

func (c *Client) Events(ctx context.Context, start, end time.Time) ([]Event, error) {
	var events []Event
	q := url.Values{"start": {start.Format(time.RFC3339)}, "end": {end.Format(time.RFC3339)}}
	return events, c.Do(ctx, http.MethodGet, "/events/", q, nil, &events)
}

func projectPath(id int, suffix string) string {
	return "/projects/" + strconv.Itoa(id) + "/" + suffix
}
