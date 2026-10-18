package excel

import (
	"bytes"
	"testing"

	"github.com/example/ozmap-user-manager/internal/models"
	"github.com/xuri/excelize/v2"
)

func TestParseUsesFullEmailAsUsername(t *testing.T) {
	users := parseWorkbook(t, []string{"email", "password", "role", "resources"}, []string{
		"person@example.com", "initial-password", "operator", "resource-1",
	})
	if len(users) != 1 {
		t.Fatalf("Parse() returned %d users, want 1", len(users))
	}
	if users[0].Username != users[0].Email {
		t.Errorf("username = %q, want full email %q", users[0].Username, users[0].Email)
	}
}

func TestParseNormalizesPasswordLineBreaksAndBoundaryWhitespace(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "edge whitespace", input: "\r\n  P@ssw0rd! with spaces \t\r\n", want: "P@ssw0rd! with spaces"},
		{name: "embedded line breaks", input: "P@ss\r\nw0rd!\n with spaces", want: "P@ssw0rd! with spaces"},
	} {
		t.Run(test.name, func(t *testing.T) {
			users := parseWorkbook(t, []string{"email", "password", "role", "resources"}, []string{
				"person@example.com", test.input, "operator", "resource-1",
			})
			if len(users) != 1 {
				t.Fatalf("Parse() returned %d users, want 1", len(users))
			}
			if users[0].Password != test.want {
				t.Errorf("password normalization did not match expected output")
			}
		})
	}
}

func TestParseProjectGroupsAndLegacyColumns(t *testing.T) {
	for _, test := range []struct {
		name       string
		headers    []string
		values     []string
		wantGroups []string
	}{
		{
			name:       "project groups",
			headers:    []string{"email", "password", "role", "resources", "projectGroups"},
			values:     []string{"person@example.com", "initial-password", "operator", "resource-1", "group-1, group-2"},
			wantGroups: []string{"group-1", "group-2"},
		},
		{
			name:    "legacy columns",
			headers: []string{"email", "password", "role", "resources"},
			values:  []string{"person@example.com", "initial-password", "operator", "resource-1"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			users := parseWorkbook(t, test.headers, test.values)
			if len(users) != 1 {
				t.Fatalf("Parse() returned %d users, want 1", len(users))
			}
			if len(users[0].ProjectGroups) != len(test.wantGroups) {
				t.Fatalf("ProjectGroups = %#v, want %#v", users[0].ProjectGroups, test.wantGroups)
			}
			for i, want := range test.wantGroups {
				if users[0].ProjectGroups[i] != want {
					t.Errorf("ProjectGroups[%d] = %q, want %q", i, users[0].ProjectGroups[i], want)
				}
			}
		})
	}
}

func parseWorkbook(t *testing.T, headers, values []string) []models.SpreadsheetUser {
	t.Helper()
	file := excelize.NewFile()
	defer file.Close()

	for row, cells := range [][]string{headers, values} {
		for col, value := range cells {
			cell, err := excelize.CoordinatesToCellName(col+1, row+1)
			if err != nil {
				t.Fatalf("create cell name: %v", err)
			}
			if err := file.SetCellValue("Sheet1", cell, value); err != nil {
				t.Fatalf("set cell %s: %v", cell, err)
			}
		}
	}

	var workbook bytes.Buffer
	if err := file.Write(&workbook); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	users, err := Parse(bytes.NewReader(workbook.Bytes()), int64(workbook.Len()), 10)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	return users
}
