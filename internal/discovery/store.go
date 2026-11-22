package discovery

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
)

var ErrStaleScan = errors.New("discovery settings or results changed; scan again")

type Settings struct {
	Root     string
	Revision int64
}

type Project struct {
	Path            string
	Root            string
	Active          bool
	Available       bool
	ResolutionError string
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Settings(ctx context.Context) (Settings, error) {
	var settings Settings
	err := s.db.QueryRowContext(ctx, `SELECT root, revision FROM discovery_settings WHERE id = 1`).Scan(&settings.Root, &settings.Revision)
	if errors.Is(err, sql.ErrNoRows) {
		return settings, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("read discovery root: %w", err)
	}
	return settings, nil
}

func (s *Store) SetRoot(ctx context.Context, root string) error {
	if root == "" || !filepath.IsAbs(root) {
		return errors.New("discovery root must be an absolute path")
	}
	root = filepath.Clean(root)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO discovery_settings (id, root, revision) VALUES (1, ?, 1)
		ON CONFLICT(id) DO UPDATE SET root = excluded.root, revision = revision + 1
	`, root)
	if err != nil {
		return fmt.Errorf("save discovery root: %w", err)
	}
	return nil
}

func (s *Store) Projects(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path, root, active, available, resolution_error FROM discovered_projects ORDER BY path`)
	if err != nil {
		return nil, fmt.Errorf("read discovered projects: %w", err)
	}
	defer rows.Close()
	var projects []Project
	for rows.Next() {
		var project Project
		if err := rows.Scan(&project.Path, &project.Root, &project.Active, &project.Available, &project.ResolutionError); err != nil {
			return nil, fmt.Errorf("read discovered project: %w", err)
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read discovered projects: %w", err)
	}
	return projects, nil
}

func (s *Store) ApplyScan(ctx context.Context, settings Settings, projects []Project) error {
	if settings.Root == "" || settings.Revision < 1 {
		return errors.New("a saved discovery root is required")
	}
	for _, project := range projects {
		if project.Root != settings.Root || filepath.Dir(project.Path) != settings.Root {
			return errors.New("scan contains a project outside the saved root")
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin discovery update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE discovery_settings SET revision = revision + 1 WHERE id = 1 AND root = ? AND revision = ?`, settings.Root, settings.Revision)
	if err != nil {
		return fmt.Errorf("validate discovery scan: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("validate discovery scan: %w", err)
	}
	if count != 1 {
		return ErrStaleScan
	}
	if _, err := tx.ExecContext(ctx, `UPDATE discovered_projects SET available = 0 WHERE root = ?`, settings.Root); err != nil {
		return fmt.Errorf("mark absent projects: %w", err)
	}
	for _, project := range projects {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO discovered_projects(path, root, active, available, resolution_error)
			VALUES (?, ?, 0, 1, ?)
			ON CONFLICT(path) DO UPDATE SET
				root = excluded.root, available = 1, resolution_error = excluded.resolution_error
		`, project.Path, project.Root, project.ResolutionError); err != nil {
			return fmt.Errorf("save discovered project: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit discovery update: %w", err)
	}
	return nil
}

func (s *Store) Activate(ctx context.Context, path string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE discovered_projects SET active = 1 WHERE path = ? AND available = 1 AND resolution_error = ''`, path)
	if err != nil {
		return fmt.Errorf("activate discovered project: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check activation: %w", err)
	}
	if count != 1 {
		return errors.New("project is unavailable or has unresolved inputs; scan again before activation")
	}
	return nil
}
