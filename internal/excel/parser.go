package excel

import (
	"fmt"
	"github.com/example/ozmap-user-manager/internal/models"
	"github.com/xuri/excelize/v2"
	"io"
	"mime"
	"net/mail"
	"path/filepath"
	"strings"
)

var headers = []string{"email", "password", "role", "resources"}

func Parse(r io.Reader, size int64, maxRows int) ([]models.SpreadsheetUser, error) {
	if size <= 0 {
		return nil, fmt.Errorf("arquivo vazio")
	}
	if ext := strings.ToLower(filepath.Ext("upload.xlsx")); ext != ".xlsx" {
		return nil, fmt.Errorf("arquivo deve ser XLSX")
	}
	x, e := excelize.OpenReader(r)
	if e != nil {
		return nil, fmt.Errorf("arquivo XLSX inválido")
	}
	defer x.Close()
	sheet := x.GetSheetName(0)
	if sheet == "" {
		return nil, fmt.Errorf("planilha sem abas")
	}
	rows, e := x.Rows(sheet)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, fmt.Errorf("planilha vazia")
	}
	h, e := rows.Columns()
	if e != nil {
		return nil, e
	}
	for i, v := range headers {
		if i >= len(h) || strings.ToLower(strings.TrimSpace(h[i])) != v {
			return nil, fmt.Errorf("cabeçalho inválido: esperado %s na coluna %d", v, i+1)
		}
	}
	var out []models.SpreadsheetUser
	rowNum := 1
	for rows.Next() {
		rowNum++
		cols, e := rows.Columns()
		if e != nil {
			return nil, e
		}
		if len(cols) == 0 {
			continue
		}
		u := models.SpreadsheetUser{Row: rowNum}
		if len(cols) > 0 {
			u.Email = strings.TrimSpace(cols[0])
		}
		if len(cols) > 1 {
			u.Password = normalizePassword(cols[1])
		}
		if len(cols) > 2 {
			u.Role = strings.TrimSpace(cols[2])
		}
		if len(cols) > 3 {
			u.Resources = parseResources(cols[3])
		}
		if len(h) > 4 && strings.EqualFold(strings.TrimSpace(h[4]), "projectGroups") && len(cols) > 4 {
			u.ProjectGroups = parseResources(cols[4])
		}
		if len(out) >= maxRows {
			return nil, fmt.Errorf("quantidade máxima de %d linhas excedida", maxRows)
		}
		validate(&u)
		out = append(out, u)
	}
	return out, nil
}
func normalizePassword(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return strings.TrimSpace(s)
}
func parseResources(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
func validate(u *models.SpreadsheetUser) {
	if _, e := mail.ParseAddress(u.Email); e != nil || !strings.Contains(u.Email, "@") {
		u.Errors = append(u.Errors, "email inválido")
	}
	if u.Password == "" {
		u.Errors = append(u.Errors, "password obrigatória")
	}
	if u.Role == "" {
		u.Errors = append(u.Errors, "role obrigatória")
	}
	if len(u.Resources) == 0 {
		u.Errors = append(u.Errors, "resources obrigatório")
	}
	u.Username = u.Email
	u.Valid = len(u.Errors) == 0
}
func MimeOK(v string) bool {
	m, _, e := mime.ParseMediaType(v)
	if e != nil {
		return false
	}
	return m == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" || m == "application/octet-stream"
}
