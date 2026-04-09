package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

type User struct {
	ID        int64
	Email     string
	Name      string
	Role      Role
	IsActive  bool
	CreatedAt string
	UpdatedAt string
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) SeedDefaultAdmin() error {
	var cnt int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM users`).Scan(&cnt); err != nil {
		return err
	}
	// Dev convenience:
	// - If DB is empty: create default admin.
	// - If DB already has the default user: ensure it stays admin+active so you don't lock yourself out.
	if cnt == 0 {
		_, err := s.CreateUser(CreateUserInput{
			Email:    "admin@example.com",
			Name:     "Admin",
			Role:     RoleAdmin,
			Password: "admin",
		})
		return err
	}

	var id int64
	err := s.db.QueryRow(`SELECT id FROM users WHERE email = ? LIMIT 1`, "admin@example.com").Scan(&id)
	if err != nil {
		// If there are users but not the default admin, don't change anything.
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if _, err := s.db.Exec(
		`UPDATE users SET role = ?, is_active = 1, updated_at = (strftime('%Y-%m-%dT%H:%M:%fZ','now')) WHERE id = ?`,
		string(RoleAdmin),
		id,
	); err != nil {
		return err
	}
	return nil
}

type CreateUserInput struct {
	Email    string
	Name     string
	Role     Role
	Password string
}

func (s *Store) CreateUser(in CreateUserInput) (*User, error) {
	email := strings.TrimSpace(strings.ToLower(in.Email))
	name := strings.TrimSpace(in.Name)
	if email == "" || name == "" {
		return nil, errors.New("email and name are required")
	}
	if in.Role != RoleAdmin && in.Role != RoleEditor && in.Role != RoleViewer {
		return nil, errors.New("invalid role")
	}
	if len(in.Password) < 4 {
		return nil, errors.New("password too short")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	res, err := s.db.Exec(`
INSERT INTO users(email, name, role, password_hash, is_active)
VALUES(?,?,?,?,1)`, email, name, string(in.Role), hash)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetUserByID(id)
}

func (s *Store) GetUserByID(id int64) (*User, error) {
	u := &User{}
	var role string
	var isActiveInt int
	err := s.db.QueryRow(`
SELECT id, email, name, role, is_active, created_at, updated_at
FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Email, &u.Name, &role, &isActiveInt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	u.Role = Role(role)
	u.IsActive = isActiveInt == 1
	return u, nil
}

func (s *Store) Authenticate(email, password string) (*User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return nil, errors.New("email and password required")
	}
	var (
		id     int64
		name   string
		role   string
		hash   []byte
		active int
	)
	err := s.db.QueryRow(`
SELECT id, name, role, password_hash, is_active
FROM users WHERE email = ? LIMIT 1`, email).
		Scan(&id, &name, &role, &hash, &active)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}
	if active != 1 {
		return nil, errors.New("account disabled")
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		return nil, errors.New("invalid credentials")
	}
	return s.GetUserByID(id)
}

func (s *Store) ListUsers(q string, role Role, offset, limit int) ([]User, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	q = strings.TrimSpace(q)

	where := []string{"1=1"}
	args := []any{}
	if q != "" {
		where = append(where, "(email LIKE ? OR name LIKE ?)")
		args = append(args, "%"+q+"%", "%"+q+"%")
	}
	if role != "" {
		where = append(where, "role = ?")
		args = append(args, string(role))
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM users WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(`
SELECT id, email, name, role, is_active, created_at, updated_at
FROM users
WHERE `+whereSQL+`
ORDER BY id DESC
LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		var r string
		var active int
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &r, &active, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, 0, err
		}
		u.Role = Role(r)
		u.IsActive = active == 1
		out = append(out, u)
	}
	return out, total, rows.Err()
}

func (s *Store) UpdateUserRole(id int64, role Role) (*User, error) {
	if role != RoleAdmin && role != RoleEditor && role != RoleViewer {
		return nil, errors.New("invalid role")
	}
	if _, err := s.db.Exec(`UPDATE users SET role = ?, updated_at = (strftime('%Y-%m-%dT%H:%M:%fZ','now')) WHERE id = ?`, string(role), id); err != nil {
		return nil, err
	}
	return s.GetUserByID(id)
}

func (s *Store) SetUserActive(id int64, active bool) (*User, error) {
	v := 0
	if active {
		v = 1
	}
	if _, err := s.db.Exec(`UPDATE users SET is_active = ?, updated_at = (strftime('%Y-%m-%dT%H:%M:%fZ','now')) WHERE id = ?`, v, id); err != nil {
		return nil, err
	}
	return s.GetUserByID(id)
}

func (s *Store) ResetUserPassword(id int64, password string) error {
	if len(password) < 4 {
		return errors.New("password too short")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE users SET password_hash = ?, updated_at = (strftime('%Y-%m-%dT%H:%M:%fZ','now')) WHERE id = ?`, hash, id)
	return err
}

type CreateProjectInput struct {
	Key         string
	Name        string
	Status      string
	Description string
}

type Project struct {
	ID          int64
	Key         string
	Name        string
	Status      string
	Description string
	CreatedAt   string
	UpdatedAt   string
}

func (s *Store) CreateProject(in CreateProjectInput) (*Project, error) {
	key := strings.TrimSpace(in.Key)
	name := strings.TrimSpace(in.Name)
	status := strings.TrimSpace(in.Status)
	if key == "" || name == "" {
		return nil, errors.New("key and name are required")
	}
	if status == "" {
		status = "active"
	}
	switch status {
	case "active", "paused", "archived":
	default:
		return nil, errors.New("invalid status")
	}
	desc := strings.TrimSpace(in.Description)
	res, err := s.db.Exec(`
INSERT INTO projects(key, name, status, description)
VALUES(?,?,?,?)`, key, name, status, desc)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetProjectByID(id)
}

func (s *Store) GetProjectByID(id int64) (*Project, error) {
	p := &Project{}
	err := s.db.QueryRow(`
SELECT id, key, name, status, description, created_at, updated_at
FROM projects WHERE id = ?`, id).
		Scan(&p.ID, &p.Key, &p.Name, &p.Status, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) ListProjects(q string, status string, offset, limit int) ([]Project, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	q = strings.TrimSpace(q)
	status = strings.TrimSpace(status)

	where := []string{"1=1"}
	args := []any{}
	if q != "" {
		where = append(where, "(key LIKE ? OR name LIKE ?)")
		args = append(args, "%"+q+"%", "%"+q+"%")
	}
	if status != "" {
		where = append(where, "status = ?")
		args = append(args, status)
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM projects WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(`
SELECT id, key, name, status, description, created_at, updated_at
FROM projects
WHERE `+whereSQL+`
ORDER BY id DESC
LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Key, &p.Name, &p.Status, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

func (s *Store) WriteAudit(actorUserID *int64, action, entityType, entityID string, meta any) error {
	metaJSON := "{}"
	if meta != nil {
		b, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		metaJSON = string(b)
	}
	var actor any = nil
	if actorUserID != nil {
		actor = *actorUserID
	}
	_, err := s.db.Exec(`
INSERT INTO audit_log(actor_user_id, action, entity_type, entity_id, meta_json)
VALUES(?,?,?,?,?)`, actor, action, entityType, entityID, metaJSON)
	return err
}

func (r Role) String() string { return string(r) }

func (s *Store) MustRole(r Role) Role {
	switch r {
	case RoleAdmin, RoleEditor, RoleViewer:
		return r
	default:
		panic(fmt.Sprintf("invalid role: %q", r))
	}
}

