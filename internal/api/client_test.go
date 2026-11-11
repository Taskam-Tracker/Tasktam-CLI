package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestErrorsCarryCodeAndFieldMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tk_test" {
			t.Errorf("missing bearer token: %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "validation_error", "detail": "Invalid input.",
			"fields": map[string][]string{"title": {"This field is required."}},
		})
	}))
	defer srv.Close()
	_, err := New(srv.URL, "tk_test", "test").QuickAdd(context.Background(), "", "")
	if !IsCode(err, "validation_error") || !IsStatus(err, 400) {
		t.Fatalf("unexpected error %#v", err)
	}
	if err.Error() != "title: This field is required." {
		t.Errorf("message = %q", err.Error())
	}
}

func TestTaskFilterQuery(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.RawQuery
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()
	f := TaskFilter{Project: "7", Priority: "p0,p1", Due: "week", Assignee: "me", IncludeClosed: true, Limit: 5}
	if _, err := New(srv.URL, "", "test").Tasks(context.Background(), f); err != nil {
		t.Fatal(err)
	}
	if want := "assignee=me&due=week&include_closed=true&limit=5&priority=p0%2Cp1&project=7"; got != want {
		t.Errorf("query = %q, want %q", got, want)
	}
}

func TestNoContentAndUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	if err := New(srv.URL, "tk_x", "test").Delete(context.Background(), 1); err != nil {
		t.Fatalf("204 should succeed: %v", err)
	}
	srv.Close()
	if err := New(srv.URL, "tk_x", "test").Delete(context.Background(), 1); err == nil {
		t.Fatal("expected a connection error")
	}
}

func TestRepliesAreStrippedOfTerminalControls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/tasks/1/" {
			_, _ = w.Write([]byte(`{"id": 1, "title": "Pay \u001b]0;pwned\u0007rent\r", "description": "line 1\n\tline 2\u009b2J"}`))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error": "forbidden", "detail": "no \u001b[2Jaccess"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "tk_test", "test")
	task, err := c.Task(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if task.Title != "Pay ]0;pwnedrent" || task.Description != "line 1\n\tline 22J" {
		t.Errorf("not sanitised: %q / %q", task.Title, task.Description)
	}
	_, err = c.Task(context.Background(), 2)
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Detail != "no [2Jaccess" {
		t.Errorf("error detail not sanitised: %#v", err)
	}
}
