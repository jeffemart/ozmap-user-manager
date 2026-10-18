package ozmap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/ozmap-user-manager/internal/config"
)

func TestListProjectsPaginatesAndCreateUserSendsProjects(t *testing.T) {
	var createRequest CreateUserRequest
	projectIDs := make([]string, 62)
	for i := range projectIDs {
		projectIDs[i] = fmt.Sprintf("project-%d", i+1)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/token":
			_, _ = fmt.Fprint(w, `{"accessToken":"test-token"}`)
		case "/api/v2/projects":
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("unexpected Authorization header %q", r.Header.Get("Authorization"))
			}
			if r.Method != http.MethodGet {
				t.Errorf("projects method = %s, want GET", r.Method)
			}
			if r.URL.Query().Get("limit") != "25" {
				t.Errorf("projects limit = %q, want 25", r.URL.Query().Get("limit"))
			}
			page := 1
			if pageValue := r.URL.Query().Get("page"); pageValue != "" {
				if _, err := fmt.Sscanf(pageValue, "%d", &page); err != nil {
					t.Errorf("invalid project page %q", pageValue)
					http.Error(w, "invalid page", http.StatusBadRequest)
					return
				}
			}
			start := (page - 1) * 25
			if r.URL.Query().Get("start") != "" {
				var requestedStart int
				if _, err := fmt.Sscanf(r.URL.Query().Get("start"), "%d", &requestedStart); err != nil || requestedStart != start {
					t.Errorf("projects start = %q, want %d", r.URL.Query().Get("start"), start)
					http.Error(w, "invalid start", http.StatusBadRequest)
					return
				}
			}
			if start >= len(projectIDs) {
				t.Errorf("unexpected project pagination start %d", start)
				http.Error(w, "unexpected start", http.StatusBadRequest)
				return
			}
			end := start + 25
			if end > len(projectIDs) {
				end = len(projectIDs)
			}
			rows := make([]Project, 0, end-start)
			for _, id := range projectIDs[start:end] {
				rows = append(rows, Project{ID: id})
			}
			if err := json.NewEncoder(w).Encode(struct {
				Total       int       `json:"total"`
				Count       int       `json:"count"`
				Rows        []Project `json:"rows"`
				Start       int       `json:"start"`
				Limit       int       `json:"limit"`
				HasNextPage bool      `json:"hasNextPage"`
				NextURL     string    `json:"nextUrl"`
			}{
				Total:       len(projectIDs),
				Count:       len(rows),
				Rows:        rows,
				Start:       start,
				Limit:       25,
				HasNextPage: end < len(projectIDs),
				NextURL: func() string {
					if end < len(projectIDs) {
						return fmt.Sprintf("/api/v2/projects?start=%d&limit=25&page=%d", end, page+1)
					}
					return ""
				}(),
			}); err != nil {
				t.Errorf("encode projects page: %v", err)
			}
		case "/api/v2/users":
			createRequest = CreateUserRequest{}
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("unexpected Authorization header %q", r.Header.Get("Authorization"))
			}
			if r.Method != http.MethodPost {
				t.Errorf("users method = %s, want POST", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&createRequest); err != nil {
				t.Errorf("decode user request: %v", err)
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			_, _ = fmt.Fprint(w, `{"username":"test-user"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(config.Config{OZMapBaseURL: server.URL})
	projects, err := client.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) != len(projectIDs) {
		t.Fatalf("ListProjects() returned %d projects, want %d", len(projects), len(projectIDs))
	}
	for i, want := range projectIDs {
		if projects[i].ID != want {
			t.Errorf("ListProjects()[%d].ID = %q, want %q", i, projects[i].ID, want)
		}
	}

	wantProjects := make([]ProjectRole, 0, len(projectIDs))
	for _, id := range projectIDs {
		wantProjects = append(wantProjects, ProjectRole{Project: id, Role: "role-1"})
	}
	_, err = client.CreateUser(context.Background(), CreateUserRequest{
		Email:    "user@example.com",
		Username: "test-user",
		Role:     "role-1",
		Projects: wantProjects,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if len(createRequest.Projects) != len(wantProjects) {
		t.Fatalf("CreateUser() projects = %#v, want %#v", createRequest.Projects, wantProjects)
	}
	for i, want := range wantProjects {
		if createRequest.Projects[i] != want {
			t.Errorf("CreateUser() project[%d] = %#v, want %#v", i, createRequest.Projects[i], want)
		}
	}

	wantGroups := []ProjectGroup{{Group: "group-1"}, {Group: "group-2"}}
	_, err = client.CreateUser(context.Background(), CreateUserRequest{
		Email:         "group-user@example.com",
		Username:      "group-user",
		Role:          "role-1",
		ProjectGroups: wantGroups,
	})
	if err != nil {
		t.Fatalf("CreateUser() with project groups error = %v", err)
	}
	if len(createRequest.ProjectGroups) != len(wantGroups) {
		t.Fatalf("CreateUser() projectGroups = %#v, want %#v", createRequest.ProjectGroups, wantGroups)
	}
	for i, want := range wantGroups {
		if createRequest.ProjectGroups[i] != want {
			t.Errorf("CreateUser() projectGroup[%d] = %#v, want %#v", i, createRequest.ProjectGroups[i], want)
		}
	}
	if len(createRequest.Projects) != 0 {
		t.Errorf("CreateUser() projects = %#v, want no project associations for group user", createRequest.Projects)
	}
}

func TestListProjectsFailsWhenPageContainsNoNewProjects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/token":
			_, _ = fmt.Fprint(w, `{"accessToken":"test-token"}`)
		case "/api/v2/projects":
			if r.URL.Query().Get("page") == "2" {
				_, _ = fmt.Fprint(w, `{"total":10,"count":5,"rows":[{"id":"project-1"},{"id":"project-2"},{"id":"project-3"},{"id":"project-4"},{"id":"project-5"}],"start":5,"limit":5,"hasNextPage":true,"nextUrl":"/api/v2/projects?start=5&limit=5&page=2"}`)
				return
			}

			_, _ = fmt.Fprint(w, `{"total":10,"count":5,"rows":[{"id":"project-1"},{"id":"project-2"},{"id":"project-3"},{"id":"project-4"},{"id":"project-5"}],"start":0,"limit":5,"hasNextPage":true,"nextUrl":"/api/v2/projects?start=5&limit=5&page=2"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(config.Config{OZMapBaseURL: server.URL})
	_, err := client.ListProjects(context.Background())
	if err == nil {
		t.Fatal("ListProjects() error = nil, want pagination no-progress error")
	}
}

func TestCreateUserRequestWithProjectGroupsIncludesEmptyProjects(t *testing.T) {
	body, err := json.Marshal(CreateUserRequest{
		Username:      "test-user",
		Projects:      []ProjectRole{},
		ProjectGroups: []ProjectGroup{{Group: "group-1"}},
	})
	if err != nil {
		t.Fatalf("marshal CreateUserRequest: %v", err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal request payload: %v", err)
	}
	if got := string(payload["projects"]); got != "[]" {
		t.Errorf("projects payload = %q, want []", got)
	}
	if got := string(payload["projectGroups"]); got != `[{"group":"group-1"}]` {
		t.Errorf("projectGroups payload = %q, want group association", got)
	}
}
