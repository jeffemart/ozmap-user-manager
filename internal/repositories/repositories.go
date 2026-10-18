package repositories

import (
	"context"
	"embed"
	"fmt"
	"github.com/example/ozmap-user-manager/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func Migrate(ctx context.Context, db *pgxpool.Pool) error {
	b, e := migrationFS.ReadFile("migrations/001_init.sql")
	if e != nil {
		return e
	}
	_, e = db.Exec(ctx, string(b))
	return e
}

type RoleRepository struct{ db *pgxpool.Pool }

func NewRoleRepository(db *pgxpool.Pool) *RoleRepository { return &RoleRepository{db} }
func (r *RoleRepository) List(ctx context.Context) ([]models.Role, error) {
	rows, e := r.db.Query(ctx, "SELECT id,name,ozmap_role_id,created_at,updated_at FROM roles ORDER BY name")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []models.Role
	for rows.Next() {
		var x models.Role
		if e := rows.Scan(&x.ID, &x.Name, &x.OZMapRoleID, &x.CreatedAt, &x.UpdatedAt); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *RoleRepository) GetByName(ctx context.Context, n string) (models.Role, error) {
	var x models.Role
	e := r.db.QueryRow(ctx, "SELECT id,name,ozmap_role_id,created_at,updated_at FROM roles WHERE lower(name)=lower($1)", n).Scan(&x.ID, &x.Name, &x.OZMapRoleID, &x.CreatedAt, &x.UpdatedAt)
	return x, e
}
func (r *RoleRepository) Upsert(ctx context.Context, n, id string) (models.Role, error) {
	var x models.Role
	e := r.db.QueryRow(ctx, `INSERT INTO roles(name,ozmap_role_id) VALUES($1,$2) ON CONFLICT(name) DO UPDATE SET ozmap_role_id=EXCLUDED.ozmap_role_id,updated_at=now() RETURNING id,name,ozmap_role_id,created_at,updated_at`, n, id).Scan(&x.ID, &x.Name, &x.OZMapRoleID, &x.CreatedAt, &x.UpdatedAt)
	return x, e
}
func (r *RoleRepository) Delete(ctx context.Context, id int64) error {
	_, e := r.db.Exec(ctx, "DELETE FROM roles WHERE id=$1", id)
	return e
}

type ProjectGroupRepository struct{ db *pgxpool.Pool }

func NewProjectGroupRepository(db *pgxpool.Pool) *ProjectGroupRepository {
	return &ProjectGroupRepository{db}
}
func (r *ProjectGroupRepository) List(ctx context.Context) ([]models.ProjectGroup, error) {
	rows, e := r.db.Query(ctx, "SELECT id,name,ozmap_project_group_id,created_at,updated_at FROM project_groups ORDER BY name")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []models.ProjectGroup
	for rows.Next() {
		var x models.ProjectGroup
		if e := rows.Scan(&x.ID, &x.Name, &x.OZMapProjectGroupID, &x.CreatedAt, &x.UpdatedAt); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *ProjectGroupRepository) GetByName(ctx context.Context, n string) (models.ProjectGroup, error) {
	var x models.ProjectGroup
	e := r.db.QueryRow(ctx, "SELECT id,name,ozmap_project_group_id,created_at,updated_at FROM project_groups WHERE lower(name)=lower($1)", n).Scan(&x.ID, &x.Name, &x.OZMapProjectGroupID, &x.CreatedAt, &x.UpdatedAt)
	return x, e
}
func (r *ProjectGroupRepository) Upsert(ctx context.Context, n, id string) (models.ProjectGroup, error) {
	var x models.ProjectGroup
	e := r.db.QueryRow(ctx, `INSERT INTO project_groups(name,ozmap_project_group_id) VALUES($1,$2) ON CONFLICT(name) DO UPDATE SET ozmap_project_group_id=EXCLUDED.ozmap_project_group_id,updated_at=now() RETURNING id,name,ozmap_project_group_id,created_at,updated_at`, n, id).Scan(&x.ID, &x.Name, &x.OZMapProjectGroupID, &x.CreatedAt, &x.UpdatedAt)
	return x, e
}
func (r *ProjectGroupRepository) Delete(ctx context.Context, id int64) error {
	_, e := r.db.Exec(ctx, "DELETE FROM project_groups WHERE id=$1", id)
	return e
}

type ImportRepository struct{ db *pgxpool.Pool }

func NewImportRepository(db *pgxpool.Pool) *ImportRepository { return &ImportRepository{db} }
func (r *ImportRepository) Create(ctx context.Context, id, fn, status string, total int) error {
	_, e := r.db.Exec(ctx, "INSERT INTO imports(id,filename,status,total) VALUES($1,$2,$3,$4)", id, fn, status, total)
	return e
}
func (r *ImportRepository) Finish(ctx context.Context, id string, ok, fail int, status string) error {
	_, e := r.db.Exec(ctx, "UPDATE imports SET success_count=$2,error_count=$3,status=$4,finished_at=now() WHERE id=$1", id, ok, fail, status)
	return e
}
func (r *ImportRepository) AddResult(ctx context.Context, id, email, role, status, msg string) error {
	_, e := r.db.Exec(ctx, "INSERT INTO import_results(import_id,email,role_name,status,error_message) VALUES($1,$2,$3,$4,$5)", id, email, role, status, msg)
	return e
}
func (r *ImportRepository) ListPage(ctx context.Context, page, pageSize int, start, end *time.Time) ([]models.Import, int, error) {
	var total int
	var rows pgx.Rows
	var err error
	offset := (page - 1) * pageSize

	if start != nil && end != nil {
		err = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM imports WHERE created_at >= $1 AND created_at < $2", *start, *end).Scan(&total)
		if err != nil {
			return nil, 0, err
		}
		pageCount := (total + pageSize - 1) / pageSize
		if pageCount > 0 && page > pageCount {
			page = pageCount
			offset = (page - 1) * pageSize
		}
		rows, err = r.db.Query(ctx, "SELECT id::text,filename,total,success_count,error_count,status,created_at,finished_at FROM imports WHERE created_at >= $1 AND created_at < $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4", *start, *end, pageSize, offset)
	} else {
		err = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM imports").Scan(&total)
		if err != nil {
			return nil, 0, err
		}
		pageCount := (total + pageSize - 1) / pageSize
		if pageCount > 0 && page > pageCount {
			page = pageCount
			offset = (page - 1) * pageSize
		}
		rows, err = r.db.Query(ctx, "SELECT id::text,filename,total,success_count,error_count,status,created_at,finished_at FROM imports ORDER BY created_at DESC LIMIT $1 OFFSET $2", pageSize, offset)
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]models.Import, 0, pageSize)
	for rows.Next() {
		var item models.Import
		if err := rows.Scan(&item.ID, &item.Filename, &item.Total, &item.SuccessCount, &item.ErrorCount, &item.Status, &item.CreatedAt, &item.FinishedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
func (r *ImportRepository) Results(ctx context.Context, id string) ([]models.ImportResult, error) {
	rows, e := r.db.Query(ctx, "SELECT id,import_id::text,email,role_name,status,error_message,created_at FROM import_results WHERE import_id=$1 ORDER BY id", id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := make([]models.ImportResult, 0)
	for rows.Next() {
		var x models.ImportResult
		if e := rows.Scan(&x.ID, &x.ImportID, &x.Email, &x.RoleName, &x.Status, &x.ErrorMessage, &x.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *ImportRepository) Stats(ctx context.Context) (int, int, int, error) {
	var processed, created, failed int
	e := r.db.QueryRow(ctx, "SELECT COALESCE(SUM(total),0),COALESCE(SUM(success_count),0),COALESCE(SUM(error_count),0) FROM imports").Scan(&processed, &created, &failed)
	return processed, created, failed, e
}

var _ = fmt.Sprintf
