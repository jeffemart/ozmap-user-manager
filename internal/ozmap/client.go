package ozmap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/example/ozmap-user-manager/internal/config"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type AccessTokenResponse struct {
	AccessToken string `json:"accessToken"`
	ExpiresAt   int64  `json:"expiresAt"`
	ClientID    string `json:"clientId"`
}
type ProjectRole struct {
	Project string `json:"project"`
	Role    string `json:"role"`
}
type ProjectGroup struct {
	Group string `json:"group"`
}
type Project struct {
	ID string `json:"id"`
}
type CreateUserRequest struct {
	Resources     []string       `json:"resources"`
	Username      string         `json:"username"`
	Password      string         `json:"password"`
	Email         string         `json:"email"`
	Name          string         `json:"name,omitempty"`
	Phone         string         `json:"phone,omitempty"`
	Role          string         `json:"role"`
	Observation   string         `json:"observation,omitempty"`
	Projects      []ProjectRole  `json:"projects"`
	ProjectGroups []ProjectGroup `json:"projectGroups,omitempty"`
	Locale        string         `json:"locale,omitempty"`
}
type CreateUserResponse struct {
	Username             string         `json:"username"`
	Password             string         `json:"password"`
	Email                string         `json:"email"`
	Name                 string         `json:"name"`
	Phone                string         `json:"phone"`
	Role                 string         `json:"role"`
	Status               int            `json:"status"`
	Observation          string         `json:"observation"`
	EmailConfirmed       bool           `json:"email_confirmed"`
	AutoGeneratePassword bool           `json:"autoGeneratePassword"`
	Projects             []ProjectRole  `json:"projects"`
	ProjectGroups        []ProjectGroup `json:"projectGroups"`
}
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string { return fmt.Sprintf("ozmap http %d: %s", e.Status, e.Message) }

type Client struct {
	cfg     config.Config
	http    *http.Client
	mu      sync.Mutex
	token   string
	expires time.Time
}

func NewClient(c config.Config) *Client {
	return &Client{cfg: c, http: &http.Client{Timeout: c.OZMapTimeout}}
}
func (c *Client) GetAccessToken(ctx context.Context) (*AccessTokenResponse, error) {
	body, _ := json.Marshal(map[string]string{"clientId": c.cfg.OZMapClientID, "clientSecret": c.cfg.OZMapClientSecret})
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.OZMapBaseURL, "/")+"/api/v2/auth/token", bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.OZMapAuthorization != "" {
		req.Header.Set("Authorization", c.cfg.OZMapAuthorization)
	}
	resp, e := c.http.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{resp.StatusCode, safeAPIMessage(b)}
	}
	var out AccessTokenResponse
	if e := json.Unmarshal(b, &out); e != nil || out.AccessToken == "" {
		return nil, fmt.Errorf("ozmap authentication returned invalid response")
	}
	return &out, nil
}
func (c *Client) tokenFor(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.expires) {
		return c.token, nil
	}
	x, e := c.GetAccessToken(ctx)
	if e != nil {
		return "", e
	}
	c.token = x.AccessToken
	c.expires = time.Now().Add(5 * time.Minute)
	if x.ExpiresAt > time.Now().Unix() {
		c.expires = time.Unix(x.ExpiresAt, 0).Add(-30 * time.Second)
	}
	return c.token, nil
}
func (c *Client) CreateUser(ctx context.Context, in CreateUserRequest) (*CreateUserResponse, error) {
	token, e := c.tokenFor(ctx)
	if e != nil {
		return nil, e
	}
	return c.createWithToken(ctx, token, in)
}
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	token, e := c.tokenFor(ctx)
	if e != nil {
		return nil, e
	}

	const maxProjectsResponseSize = 16 << 20
	var projects []Project
	seen := make(map[string]struct{})
	endpoint := strings.TrimRight(c.cfg.OZMapBaseURL, "/") + "/api/v2/projects"
	requestURL := endpoint + "?limit=25"
	visitedPages := make(map[string]struct{})
	for pageNumber := 1; ; pageNumber++ {
		if pageNumber > 1000 {
			return nil, fmt.Errorf("ozmap project pagination exceeded 1000 pages")
		}
		parsedRequestURL, e := url.Parse(requestURL)
		if e != nil {
			return nil, fmt.Errorf("invalid OZmap projects page URL: %w", e)
		}
		pageKey := parsedRequestURL.RequestURI()
		if _, ok := visitedPages[pageKey]; ok {
			return nil, fmt.Errorf("ozmap project pagination repeated page %q", pageKey)
		}
		visitedPages[pageKey] = struct{}{}

		req, e := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if e != nil {
			return nil, e
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, e := c.http.Do(req)
		if e != nil {
			return nil, e
		}
		b, readErr := io.ReadAll(io.LimitReader(resp.Body, maxProjectsResponseSize+1))
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if len(b) > maxProjectsResponseSize {
			return nil, fmt.Errorf("ozmap projects response exceeds %d bytes", maxProjectsResponseSize)
		}
		if resp.StatusCode == http.StatusUnauthorized {
			c.invalidateToken()
			return nil, &APIError{resp.StatusCode, "unauthorized"}
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, &APIError{resp.StatusCode, safeAPIMessage(b)}
		}
		var page struct {
			Total       int       `json:"total"`
			Rows        []Project `json:"rows"`
			HasNextPage bool      `json:"hasNextPage"`
			NextURL     string    `json:"nextUrl"`
		}
		if e := json.Unmarshal(b, &page); e != nil {
			return nil, fmt.Errorf("ozmap returned invalid projects response: %w", e)
		}
		for _, project := range page.Rows {
			if project.ID == "" {
				return nil, fmt.Errorf("ozmap returned a project without an ID")
			}
		}
		newProjects := 0
		for _, project := range page.Rows {
			if _, ok := seen[project.ID]; ok {
				continue
			}
			seen[project.ID] = struct{}{}
			projects = append(projects, project)
			newProjects++
		}
		if !page.HasNextPage {
			return projects, nil
		}
		if newProjects == 0 {
			return nil, fmt.Errorf("ozmap project pagination returned no new projects on page %d", pageNumber)
		}
		if page.NextURL == "" {
			return nil, fmt.Errorf("ozmap indicated another projects page but omitted nextUrl")
		}
		nextURL, e := url.Parse(page.NextURL)
		if e != nil {
			return nil, fmt.Errorf("invalid OZmap projects nextUrl: %w", e)
		}
		if nextURL.IsAbs() || nextURL.Host != "" || nextURL.User != nil || nextURL.Path != "/api/v2/projects" || nextURL.Fragment != "" {
			return nil, fmt.Errorf("OZmap projects nextUrl points outside the projects endpoint")
		}
		requestURL = strings.TrimRight(c.cfg.OZMapBaseURL, "/") + nextURL.RequestURI()
	}
}
func (c *Client) createWithToken(ctx context.Context, token string, in CreateUserRequest) (*CreateUserResponse, error) {
	body, e := json.Marshal(in)
	if e != nil {
		return nil, e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.OZMapBaseURL, "/")+"/api/v2/users", bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, e := c.http.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == 401 {
		c.invalidateToken()
		return nil, &APIError{401, "unauthorized"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{resp.StatusCode, safeAPIMessage(b)}
	}
	var out CreateUserResponse
	if e := json.Unmarshal(b, &out); e != nil {
		return nil, fmt.Errorf("ozmap returned invalid user response")
	}
	return &out, nil
}
func (c *Client) invalidateToken() {
	c.mu.Lock()
	c.token = ""
	c.expires = time.Time{}
	c.mu.Unlock()
}
func safeAPIMessage(b []byte) string {
	var x struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if json.Unmarshal(b, &x) == nil {
		if x.Message != "" {
			return x.Message
		}
		if x.Error != "" {
			return x.Error
		}
	}
	return "request rejected by OZmap"
}
