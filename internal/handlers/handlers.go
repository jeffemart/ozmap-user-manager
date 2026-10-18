package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/example/ozmap-user-manager/internal/auth"
	"github.com/example/ozmap-user-manager/internal/config"
	"github.com/example/ozmap-user-manager/internal/services"
	"github.com/xuri/excelize/v2"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Handler struct {
	tmpl     *template.Template
	roles    *services.RoleService
	groups   *services.ProjectGroupService
	imports  *services.ImportService
	auth     *auth.Service
	sessions *auth.SessionStore
	cfg      config.Config
}

func New(t *template.Template, _ interface{}, r *services.RoleService, g *services.ProjectGroupService, i *services.ImportService, a *auth.Service, s *auth.SessionStore, c config.Config) *Handler {
	return &Handler{t, r, g, i, a, s, c}
}
func (h *Handler) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if e := h.tmpl.ExecuteTemplate(w, name, data); e != nil {
		http.Error(w, "internal server error", 500)
	}
}
func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "login.html", map[string]any{"CSRF": h.sessions.CSRFToken(cookieID(r)), "Authenticated": false})
}
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if e := r.ParseForm(); e != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	if !h.auth.Verify(strings.TrimSpace(r.FormValue("username")), r.FormValue("password")) {
		http.Error(w, "Usuário ou senha inválidos", 401)
		return
	}
	id, s, e := h.sessions.Create()
	if e != nil {
		http.Error(w, "não foi possível iniciar sessão", 500)
		return
	}
	h.sessions.SetCookie(w, id, auth.IsSecureRequest(r))
	http.Redirect(w, r, "/", 303)
	_ = s
}
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	h.sessions.Delete(r)
	h.sessions.ClearCookie(w, auth.IsSecureRequest(r))
	http.Redirect(w, r, "/login", 303)
}
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	p, c, f, _ := h.imports.Stats(r.Context())
	roles, _ := h.roles.List(r.Context())
	h.render(w, r, "dashboard.html", map[string]any{"Processed": p, "Created": c, "Failed": f, "Roles": len(roles), "CSRF": h.sessions.CSRFToken(cookieID(r)), "Authenticated": true})
}
func (h *Handler) RolesPage(w http.ResponseWriter, r *http.Request) {
	roles, _ := h.roles.List(r.Context())
	h.render(w, r, "roles.html", map[string]any{"Roles": roles, "CSRF": h.sessions.CSRFToken(cookieID(r)), "Authenticated": true})
}
func (h *Handler) ProjectGroupsPage(w http.ResponseWriter, r *http.Request) {
	groups, e := h.groups.List(r.Context())
	if e != nil {
		http.Error(w, "internal error", 500)
		return
	}
	h.render(w, r, "project-groups.html", map[string]any{"Groups": groups, "CSRF": h.sessions.CSRFToken(cookieID(r)), "Authenticated": true})
}
func (h *Handler) ImportPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "import.html", map[string]any{"CSRF": h.sessions.CSRFToken(cookieID(r)), "Authenticated": true})
}
func (h *Handler) HistoryPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "history.html", map[string]any{"CSRF": h.sessions.CSRFToken(cookieID(r)), "Authenticated": true})
}
func cookieID(r *http.Request) string {
	c, e := r.Cookie("oz_session")
	if e != nil {
		return ""
	}
	return c.Value
}
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
func (h *Handler) RolesAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		x, e := h.roles.List(r.Context())
		if e != nil {
			http.Error(w, "internal error", 500)
			return
		}
		json.NewEncoder(w).Encode(x)
	case "POST":
		var in struct{ Name, ID string }
		if json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		x, e := h.roles.Save(r.Context(), strings.TrimSpace(in.Name), strings.TrimSpace(in.ID))
		if e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		json.NewEncoder(w).Encode(x)
	case "DELETE":
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id <= 0 || h.roles.Delete(r.Context(), id) != nil {
			http.Error(w, "could not delete role", 400)
			return
		}
		w.WriteHeader(204)
	default:
		http.Error(w, "method not allowed", 405)
	}
}
func (h *Handler) ProjectGroupsAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		x, e := h.groups.List(r.Context())
		if e != nil {
			http.Error(w, "internal error", 500)
			return
		}
		json.NewEncoder(w).Encode(x)
	case "POST":
		var in struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		}
		if json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		x, e := h.groups.Save(r.Context(), strings.TrimSpace(in.Name), strings.TrimSpace(in.ID))
		if e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		json.NewEncoder(w).Encode(x)
	case "DELETE":
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id <= 0 || h.groups.Delete(r.Context(), id) != nil {
			http.Error(w, "could not delete project group", 400)
			return
		}
		w.WriteHeader(204)
	default:
		http.Error(w, "method not allowed", 405)
	}
}
func (h *Handler) UploadAPI(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, int64(h.cfg.MaxUploadSizeMB)*1024*1024)
	if e := r.ParseMultipartForm(int64(h.cfg.MaxUploadSizeMB) * 1024 * 1024); e != nil {
		http.Error(w, "arquivo excede o tamanho máximo", 400)
		return
	}
	f, fh, e := r.FormFile("file")
	if e != nil {
		http.Error(w, "arquivo obrigatório", 400)
		return
	}
	defer f.Close()
	if !strings.HasSuffix(strings.ToLower(fh.Filename), ".xlsx") {
		http.Error(w, "somente XLSX é permitido", 400)
		return
	}
	id, users, e := h.imports.ValidateAndStore(r.Context(), fh.Filename, fh.Size, f)
	if e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	type row struct {
		Row           int      `json:"row"`
		Email         string   `json:"email"`
		Role          string   `json:"role"`
		Resources     []string `json:"resources"`
		ProjectGroups []string `json:"projectGroups"`
		Valid         bool     `json:"valid"`
		Errors        []string `json:"errors"`
	}
	out := map[string]any{"import_id": id, "total": len(users), "valid": 0, "invalid": 0, "users": make([]row, 0, len(users))}
	for _, u := range users {
		if u.Valid {
			out["valid"] = out["valid"].(int) + 1
		} else {
			out["invalid"] = out["invalid"].(int) + 1
		}
		out["users"] = append(out["users"].([]row), row{u.Row, u.Email, u.Role, u.Resources, u.ProjectGroups, u.Valid, u.Errors})
	}
	json.NewEncoder(w).Encode(out)
}
func (h *Handler) CreateAPI(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ImportID string `json:"import_id"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in) != nil || in.ImportID == "" {
		http.Error(w, "import_id obrigatório", 400)
		return
	}
	if e := h.imports.StartCreate(in.ImportID); e != nil {
		http.Error(w, e.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"import_id": in.ImportID, "status": "preparing"})
}
func (h *Handler) CreationProgressAPI(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/create/")
	if id == "" || strings.Contains(id, "/") {
		http.Error(w, "import_id inválido", http.StatusBadRequest)
		return
	}
	progress, ok := h.imports.CreationProgress(id)
	if !ok {
		http.Error(w, "criação inexistente", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(progress); err != nil {
		http.Error(w, "não foi possível gerar a resposta do progresso", http.StatusInternalServerError)
	}
}
func (h *Handler) HistoryDetailAPI(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/history/")
	x, e := h.imports.Details(r.Context(), id)
	if e != nil {
		http.Error(w, "internal error", 500)
		return
	}
	json.NewEncoder(w).Encode(x)
}
func (h *Handler) HistoryListAPI(w http.ResponseWriter, r *http.Request) {
	page := 1
	if value := r.URL.Query().Get("page"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 1000000 {
			http.Error(w, "parâmetro page inválido", http.StatusBadRequest)
			return
		}
		page = parsed
	}

	var start, end *time.Time
	startValue, endValue := r.URL.Query().Get("start"), r.URL.Query().Get("end")
	if startValue != "" || endValue != "" {
		if startValue == "" || endValue == "" {
			http.Error(w, "parâmetros start e end devem ser informados juntos", http.StatusBadRequest)
			return
		}
		parsedStart, err := time.Parse(time.RFC3339Nano, startValue)
		if err != nil {
			http.Error(w, "parâmetro start inválido", http.StatusBadRequest)
			return
		}
		parsedEnd, err := time.Parse(time.RFC3339Nano, endValue)
		if err != nil || !parsedEnd.After(parsedStart) || parsedEnd.Sub(parsedStart) > 26*time.Hour {
			http.Error(w, "parâmetro end inválido", http.StatusBadRequest)
			return
		}
		start, end = &parsedStart, &parsedEnd
	}

	const pageSize = 20
	imports, total, err := h.imports.HistoryPage(r.Context(), page, pageSize, start, end)
	if err != nil {
		http.Error(w, "não foi possível carregar o histórico", http.StatusInternalServerError)
		return
	}
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"imports":     imports,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	}); err != nil {
		http.Error(w, "não foi possível gerar a resposta do histórico", http.StatusInternalServerError)
	}
}
func (h *Handler) DownloadTemplate(w http.ResponseWriter, r *http.Request) {
	x := excelize.NewFile()
	defer x.Close()
	sheet := x.GetSheetName(0)
	_ = x.SetSheetName(sheet, "USUARIOS")
	headers := []string{"email", "password", "role", "resources", "projectGroups"}
	for i, v := range headers {
		_ = x.SetCellValue("USUARIOS", fmt.Sprintf("%s1", string(rune('A'+i))), v)
	}
	_ = x.SetCellValue("USUARIOS", "A2", "usuario@empresa.com")
	_ = x.SetCellValue("USUARIOS", "B2", "senha-inicial")
	_ = x.SetCellValue("USUARIOS", "C2", "COMERCIAL")
	_ = x.SetCellValue("USUARIOS", "D2", "Loki")
	idx, _ := x.NewSheet("INSTRUCOES")
	_ = idx
	instructions := [][]string{{"Campo", "Instrução"}, {"email", "E-mail do usuário."}, {"password", "Senha inicial; espaços e quebras de linha nas extremidades são removidos automaticamente. Espaços dentro da senha são preservados."}, {"role", "Nome amigável cadastrado em Roles."}, {"resources", "Um ou mais recursos separados por vírgula."}, {"projectGroups", "Nomes cadastrados em Grupos de projetos, separados por vírgula. Quando informado, substitui a associação automática a todos os projetos."}}
	for r, row := range instructions {
		for c, v := range row {
			_ = x.SetCellValue("INSTRUCOES", fmt.Sprintf("%c%d", 'A'+c, r+1), v)
		}
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=modelo_usuarios_ozmap.xlsx")
	_ = x.Write(w)
}
