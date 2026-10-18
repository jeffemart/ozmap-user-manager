package services

import (
	"context"
	"fmt"
	"github.com/example/ozmap-user-manager/internal/config"
	"github.com/example/ozmap-user-manager/internal/excel"
	"github.com/example/ozmap-user-manager/internal/models"
	"github.com/example/ozmap-user-manager/internal/ozmap"
	"github.com/example/ozmap-user-manager/internal/repositories"
	"github.com/example/ozmap-user-manager/internal/security"
	"github.com/google/uuid"
	"sync"
	"time"
)

type RoleService struct{ repo *repositories.RoleRepository }

func NewRoleService(r *repositories.RoleRepository) *RoleService       { return &RoleService{r} }
func (s *RoleService) List(ctx context.Context) ([]models.Role, error) { return s.repo.List(ctx) }
func (s *RoleService) Save(ctx context.Context, n, id string) (models.Role, error) {
	if len(n) < 2 || len(n) > 100 {
		return models.Role{}, fmt.Errorf("nome de role inválido")
	}
	if id == "" || len(id) > 200 {
		return models.Role{}, fmt.Errorf("ID da role inválido")
	}
	return s.repo.Upsert(ctx, n, id)
}
func (s *RoleService) Delete(ctx context.Context, id int64) error { return s.repo.Delete(ctx, id) }

type ProjectGroupService struct {
	repo *repositories.ProjectGroupRepository
}

func NewProjectGroupService(r *repositories.ProjectGroupRepository) *ProjectGroupService {
	return &ProjectGroupService{r}
}
func (s *ProjectGroupService) List(ctx context.Context) ([]models.ProjectGroup, error) {
	return s.repo.List(ctx)
}
func (s *ProjectGroupService) GetByName(ctx context.Context, name string) (models.ProjectGroup, error) {
	return s.repo.GetByName(ctx, name)
}
func (s *ProjectGroupService) Save(ctx context.Context, name, id string) (models.ProjectGroup, error) {
	if len(name) < 2 || len(name) > 100 {
		return models.ProjectGroup{}, fmt.Errorf("nome do grupo inválido")
	}
	if id == "" || len(id) > 200 {
		return models.ProjectGroup{}, fmt.Errorf("ID do grupo inválido")
	}
	return s.repo.Upsert(ctx, name, id)
}
func (s *ProjectGroupService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

type ImportService struct {
	imports      *repositories.ImportRepository
	roles        *repositories.RoleRepository
	groups       *ProjectGroupService
	oz           *ozmap.Client
	cfg          config.Config
	pending      sync.Map
	creationJobs sync.Map
}

type CreationProgress struct {
	ImportID  string              `json:"import_id"`
	Total     int                 `json:"total"`
	Processed int                 `json:"processed"`
	Created   int                 `json:"created"`
	Failed    int                 `json:"failed"`
	Status    string              `json:"status"`
	Complete  bool                `json:"complete"`
	Error     string              `json:"error,omitempty"`
	Results   []models.UserResult `json:"results,omitempty"`
}

type creationJob struct {
	mu       sync.RWMutex
	progress CreationProgress
}
type Pending struct {
	ID        string
	Filename  string
	Users     []models.SpreadsheetUser
	CreatedAt time.Time
}

func NewImportService(i *repositories.ImportRepository, r *repositories.RoleRepository, g *ProjectGroupService, o *ozmap.Client, c config.Config) *ImportService {
	return &ImportService{
		imports: i,
		roles:   r,
		groups:  g,
		oz:      o,
		cfg:     c,
	}
}
func (s *ImportService) Preview(ctx context.Context, filename string, size int64, body interface{ Read([]byte) (int, error) }) (string, []models.SpreadsheetUser, error) {
	return "", nil, nil
}
func (s *ImportService) ValidateAndStore(ctx context.Context, filename string, size int64, reader interface{ Read([]byte) (int, error) }) (string, []models.SpreadsheetUser, error) {
	users, e := excel.Parse(reader, size, s.cfg.MaxImportRows)
	if e != nil {
		return "", nil, e
	}
	for i := range users {
		if users[i].Valid {
			r, e := s.roles.GetByName(ctx, users[i].Role)
			if e != nil {
				users[i].Valid = false
				users[i].Errors = append(users[i].Errors, fmt.Sprintf("role %q não cadastrada", users[i].Role))
			} else {
				users[i].RoleID = r.OZMapRoleID
			}
		}
		for _, groupName := range users[i].ProjectGroups {
			group, err := s.groups.GetByName(ctx, groupName)
			if err != nil {
				users[i].Valid = false
				users[i].Errors = append(users[i].Errors, fmt.Sprintf("grupo de projeto %q não cadastrado", groupName))
				continue
			}
			users[i].ProjectGroupIDs = append(users[i].ProjectGroupIDs, group.OZMapProjectGroupID)
		}
	}
	id := uuid.NewString()
	s.pending.Store(id, &Pending{ID: id, Filename: filename, Users: users, CreatedAt: time.Now()})
	return id, users, nil
}
func (s *ImportService) Create(ctx context.Context, id string) ([]models.UserResult, error) {
	v, ok := s.pending.LoadAndDelete(id)
	if !ok {
		return nil, fmt.Errorf("importação expirada ou inexistente")
	}
	p := v.(*Pending)
	s.updateCreationStatus(id, "preparing")
	valid := 0
	for _, u := range p.Users {
		if u.Valid {
			valid++
		}
	}
	if valid == 0 {
		return nil, fmt.Errorf("nenhum usuário válido para criação")
	}
	needsProjects := false
	for _, u := range p.Users {
		if u.Valid && len(u.ProjectGroupIDs) == 0 {
			needsProjects = true
			break
		}
	}
	var projects []ozmap.Project
	if needsProjects {
		var err error
		projects, err = s.oz.ListProjects(ctx)
		if err != nil {
			s.pending.Store(id, p)
			return nil, fmt.Errorf("não foi possível listar os projetos do OZmap: %w", err)
		}
		if len(projects) == 0 {
			s.pending.Store(id, p)
			return nil, fmt.Errorf("nenhum projeto retornado pelo OZmap; importação não executada")
		}
	}
	if e := s.imports.Create(ctx, id, p.Filename, "processing", len(p.Users)); e != nil {
		s.pending.Store(id, p)
		return nil, e
	}
	s.updateCreationStatus(id, "processing")
	results := make([]models.UserResult, 0, len(p.Users))
	success, fail := 0, 0
	for _, u := range p.Users {
		if !u.Valid {
			fail++
			msg := joinErrors(u.Errors)
			_ = s.imports.AddResult(ctx, id, u.Email, u.Role, "validation_error", msg)
			result := models.UserResult{Email: u.Email, Username: u.Username, RoleName: u.Role, Status: "error", Message: msg}
			results = append(results, result)
			s.recordCreationResult(id, result)
			continue
		}
		pwd := u.Password
		if s.cfg.PasswordMode == "sha256" {
			pwd = security.SHA256Hex(pwd)
		}
		projectRoles := make([]ozmap.ProjectRole, 0, len(projects))
		var projectGroups []ozmap.ProjectGroup
		if len(u.ProjectGroupIDs) > 0 {
			projectGroups = make([]ozmap.ProjectGroup, 0, len(u.ProjectGroupIDs))
			for _, group := range u.ProjectGroupIDs {
				projectGroups = append(projectGroups, ozmap.ProjectGroup{Group: group})
			}
		} else {
			for _, project := range projects {
				projectRoles = append(projectRoles, ozmap.ProjectRole{Project: project.ID, Role: u.RoleID})
			}
		}
		req := ozmap.CreateUserRequest{Resources: u.Resources, Username: u.Username, Password: pwd, Email: u.Email, Role: u.RoleID, Projects: projectRoles, ProjectGroups: projectGroups, Locale: "pt_BR"}
		res, e := s.oz.CreateUser(ctx, req)
		if e != nil {
			fail++
			msg := publicOZError(e)
			_ = s.imports.AddResult(ctx, id, u.Email, u.Role, "error", msg)
			result := models.UserResult{Email: u.Email, Username: u.Username, RoleName: u.Role, Status: "error", Message: msg}
			results = append(results, result)
			s.recordCreationResult(id, result)
			continue
		}
		success++
		_ = res
		_ = s.imports.AddResult(ctx, id, u.Email, u.Role, "created", "")
		result := models.UserResult{Email: u.Email, Username: u.Username, RoleName: u.Role, Status: "created", Message: "Usuário criado com sucesso"}
		results = append(results, result)
		s.recordCreationResult(id, result)
	}
	status := "completed"
	if fail > 0 {
		status = "completed_with_errors"
	}
	_ = s.imports.Finish(ctx, id, success, fail, status)
	return results, nil
}

func (s *ImportService) StartCreate(id string) error {
	value, ok := s.pending.Load(id)
	if !ok {
		return fmt.Errorf("importação expirada ou inexistente")
	}
	pending := value.(*Pending)
	valid := 0
	for _, user := range pending.Users {
		if user.Valid {
			valid++
		}
	}
	if valid == 0 {
		return fmt.Errorf("nenhum usuário válido para criação")
	}

	job := &creationJob{progress: CreationProgress{
		ImportID: id,
		Total:    len(pending.Users),
		Status:   "preparing",
		Results:  make([]models.UserResult, 0, len(pending.Users)),
	}}
	if _, loaded := s.creationJobs.LoadOrStore(id, job); loaded {
		return fmt.Errorf("a criação desta importação já foi iniciada")
	}

	go func() {
		_, err := s.Create(context.Background(), id)
		job.mu.Lock()
		defer job.mu.Unlock()
		if err != nil {
			job.progress.Status = "error"
			job.progress.Error = err.Error()
		} else {
			job.progress.Status = "completed"
		}
		job.progress.Complete = true
		time.AfterFunc(30*time.Minute, func() {
			s.creationJobs.CompareAndDelete(id, job)
		})
	}()
	return nil
}

func (s *ImportService) CreationProgress(id string) (CreationProgress, bool) {
	value, ok := s.creationJobs.Load(id)
	if !ok {
		return CreationProgress{}, false
	}
	job := value.(*creationJob)
	job.mu.RLock()
	defer job.mu.RUnlock()
	progress := job.progress
	if progress.Complete {
		progress.Results = append([]models.UserResult(nil), progress.Results...)
	} else {
		progress.Results = nil
	}
	return progress, true
}

func (s *ImportService) updateCreationStatus(id, status string) {
	value, ok := s.creationJobs.Load(id)
	if !ok {
		return
	}
	job := value.(*creationJob)
	job.mu.Lock()
	job.progress.Status = status
	job.mu.Unlock()
}

func (s *ImportService) recordCreationResult(id string, result models.UserResult) {
	value, ok := s.creationJobs.Load(id)
	if !ok {
		return
	}
	job := value.(*creationJob)
	job.mu.Lock()
	job.progress.Results = append(job.progress.Results, result)
	job.progress.Processed++
	if result.Status == "created" {
		job.progress.Created++
	} else {
		job.progress.Failed++
	}
	job.mu.Unlock()
}
func joinErrors(x []string) string {
	var s string
	for i, v := range x {
		if i > 0 {
			s += "; "
		}
		s += v
	}
	return s
}
func publicOZError(e error) string {
	if a, ok := e.(*ozmap.APIError); ok {
		switch a.Status {
		case 400:
			return "OZmap rejeitou os dados do usuário."
		case 401:
			return "Não foi possível autenticar no OZmap."
		case 409:
			return "Usuário já existe no OZmap."
		case 429:
			return "OZmap limitou as requisições. Tente novamente mais tarde."
		case 500:
			return "OZmap apresentou um erro interno."
		}
	}
	return "Não foi possível criar o usuário no OZmap."
}
func (s *ImportService) HistoryPage(ctx context.Context, page, pageSize int, start, end *time.Time) ([]models.Import, int, error) {
	return s.imports.ListPage(ctx, page, pageSize, start, end)
}
func (s *ImportService) Details(ctx context.Context, id string) ([]models.ImportResult, error) {
	return s.imports.Results(ctx, id)
}
func (s *ImportService) Stats(ctx context.Context) (int, int, int, error) {
	return s.imports.Stats(ctx)
}
