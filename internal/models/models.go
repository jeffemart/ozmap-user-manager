package models

import "time"

type Role struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	OZMapRoleID string    `json:"ozmap_role_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type ProjectGroup struct {
	ID                  int64     `json:"id"`
	Name                string    `json:"name"`
	OZMapProjectGroupID string    `json:"ozmap_project_group_id"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
type Import struct {
	ID           string     `json:"id"`
	Filename     string     `json:"filename"`
	Total        int        `json:"total"`
	SuccessCount int        `json:"success_count"`
	ErrorCount   int        `json:"error_count"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	FinishedAt   *time.Time `json:"finished_at"`
}
type ImportResult struct {
	ID           int64     `json:"id"`
	ImportID     string    `json:"import_id"`
	Email        string    `json:"email"`
	RoleName     string    `json:"role_name"`
	Status       string    `json:"status"`
	ErrorMessage *string   `json:"error_message"`
	CreatedAt    time.Time `json:"created_at"`
}
type SpreadsheetUser struct {
	Row                   int
	Email, Password, Role string
	Resources             []string
	ProjectGroups         []string
	ProjectGroupIDs       []string
	Username              string
	RoleID                string
	Valid                 bool
	Errors                []string
}
type UserResult struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	RoleName string `json:"roleName"`
	Status   string `json:"status"`
	Message  string `json:"message"`
}
