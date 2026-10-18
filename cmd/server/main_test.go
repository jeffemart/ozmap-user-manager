package main

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

func TestTemplatesParseAndRender(t *testing.T) {
	tmpl, err := template.ParseFS(templateFiles, "templates/*.html")
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}

	pages := map[string]any{
		"dashboard.html": map[string]any{
			"CSRF":          "csrf-token",
			"Processed":     0,
			"Created":       0,
			"Failed":        0,
			"Roles":         0,
			"Authenticated": true,
		},
		"history.html": map[string]any{"CSRF": "csrf-token", "Authenticated": true},
		"import.html":  map[string]any{"CSRF": "csrf-token", "Authenticated": true},
		"login.html":   map[string]any{"CSRF": "csrf-token", "Authenticated": false},
		"roles.html":   map[string]any{"CSRF": "csrf-token", "Authenticated": true},
		"project-groups.html": map[string]any{
			"CSRF":          "csrf-token",
			"Authenticated": true,
			"Groups":        nil,
		},
	}

	for name, data := range pages {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			if err := tmpl.ExecuteTemplate(&output, name, data); err != nil {
				t.Fatalf("render template: %v", err)
			}
			if name != "login.html" && !strings.Contains(output.String(), `<meta name="csrf-token" content="csrf-token">`) {
				t.Fatal("rendered page does not expose the CSRF token to external scripts")
			}
			hasNavigation := strings.Contains(output.String(), "<nav>")
			if wantNavigation := name != "login.html"; hasNavigation != wantNavigation {
				t.Fatalf("rendered navigation = %t, want %t", hasNavigation, wantNavigation)
			}
		})
	}
}
