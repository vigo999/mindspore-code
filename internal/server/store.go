package server

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/vigo999/ms-cli/internal/issues"
	"github.com/vigo999/ms-cli/internal/project"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) (*Store, error) {
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS bugs (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			title      TEXT    NOT NULL,
			status     TEXT    NOT NULL DEFAULT 'open',
			lead       TEXT    NOT NULL DEFAULT '',
			reporter   TEXT    NOT NULL,
			created_at TEXT    NOT NULL,
			updated_at TEXT    NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS notes (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			bug_id     INTEGER NOT NULL REFERENCES bugs(id),
			author     TEXT    NOT NULL,
			content    TEXT    NOT NULL,
			created_at TEXT    NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS activities (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			bug_id     INTEGER NOT NULL REFERENCES bugs(id),
			actor      TEXT    NOT NULL,
			type       TEXT    NOT NULL,
			text       TEXT    NOT NULL DEFAULT '',
			created_at TEXT    NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS project_overview (
			id         INTEGER PRIMARY KEY CHECK (id = 1),
			phase      TEXT NOT NULL DEFAULT '',
			owner      TEXT NOT NULL DEFAULT '',
			repo       TEXT NOT NULL DEFAULT '',
			branch     TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL
		)`,
		`INSERT OR IGNORE INTO project_overview (id, phase, owner, repo, branch, updated_at)
		 VALUES (1, '', '', '', '', datetime('now'))`,
		`CREATE TABLE IF NOT EXISTS project_tasks (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			section     TEXT    NOT NULL DEFAULT 'tasks',
			title       TEXT    NOT NULL,
			status      TEXT    NOT NULL DEFAULT 'todo',
			progress    INTEGER NOT NULL DEFAULT 0,
			owner       TEXT    NOT NULL DEFAULT '',
			due         TEXT    NOT NULL DEFAULT '',
			created_by  TEXT    NOT NULL,
			created_at  TEXT    NOT NULL,
			updated_at  TEXT    NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS user_sessions (
			user       TEXT PRIMARY KEY,
			last_seen  TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:40], err)
		}
	}
	return nil
}

func (s *Store) CreateBug(title, reporter string) (*issues.Bug, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`INSERT INTO bugs (title, status, reporter, created_at, updated_at) VALUES (?, 'open', ?, ?, ?)`,
		title, reporter, now, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	if _, err := s.db.Exec(
		`INSERT INTO activities (bug_id, actor, type, text, created_at) VALUES (?, ?, 'report', ?, ?)`,
		id, reporter, fmt.Sprintf("reported bug: %s", title), now,
	); err != nil {
		return nil, err
	}
	return s.GetBug(int(id))
}

func (s *Store) ListBugs(status string) ([]issues.Bug, error) {
	query := `SELECT id, title, status, lead, reporter, created_at, updated_at FROM bugs`
	var args []any
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var bugs []issues.Bug
	for rows.Next() {
		var b issues.Bug
		var createdAt, updatedAt string
		if err := rows.Scan(&b.ID, &b.Title, &b.Status, &b.Lead, &b.Reporter, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		b.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		b.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		bugs = append(bugs, b)
	}
	return bugs, rows.Err()
}

func (s *Store) GetBug(id int) (*issues.Bug, error) {
	var b issues.Bug
	var createdAt, updatedAt string
	err := s.db.QueryRow(
		`SELECT id, title, status, lead, reporter, created_at, updated_at FROM bugs WHERE id = ?`, id,
	).Scan(&b.ID, &b.Title, &b.Status, &b.Lead, &b.Reporter, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	b.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	b.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &b, nil
}

func (s *Store) ClaimBug(id int, lead string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`UPDATE bugs SET lead = ?, status = 'doing', updated_at = ? WHERE id = ?`,
		lead, now, id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("bug %d not found", id)
	}
	_, err = s.db.Exec(
		`INSERT INTO activities (bug_id, actor, type, text, created_at) VALUES (?, ?, 'claim', ?, ?)`,
		id, lead, fmt.Sprintf("%s claimed bug", lead), now,
	)
	return err
}

func (s *Store) AddNote(bugID int, author, content string) (*issues.Note, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`INSERT INTO notes (bug_id, author, content, created_at) VALUES (?, ?, ?, ?)`,
		bugID, author, content, now,
	)
	if err != nil {
		return nil, err
	}
	noteID, _ := res.LastInsertId()
	if _, err := s.db.Exec(
		`INSERT INTO activities (bug_id, actor, type, text, created_at) VALUES (?, ?, 'note', ?, ?)`,
		bugID, author, fmt.Sprintf("added note: %s", content), now,
	); err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(`UPDATE bugs SET updated_at = ? WHERE id = ?`, now, bugID); err != nil {
		return nil, err
	}
	createdAt, _ := time.Parse(time.RFC3339, now)
	return &issues.Note{
		ID:        int(noteID),
		BugID:     bugID,
		Author:    author,
		Content:   content,
		CreatedAt: createdAt,
	}, nil
}

func (s *Store) ListActivity(bugID int) ([]issues.Activity, error) {
	rows, err := s.db.Query(
		`SELECT id, bug_id, actor, type, text, created_at FROM activities WHERE bug_id = ? ORDER BY created_at ASC`,
		bugID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var acts []issues.Activity
	for rows.Next() {
		var a issues.Activity
		var createdAt string
		if err := rows.Scan(&a.ID, &a.BugID, &a.Actor, &a.Type, &a.Text, &createdAt); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		acts = append(acts, a)
	}
	return acts, rows.Err()
}

func (s *Store) DockSummary() (*issues.DockData, error) {
	var openCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM bugs WHERE status IN ('open','doing')`).Scan(&openCount); err != nil {
		return nil, err
	}
	readyBugs, err := s.ListBugs("open")
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT id, bug_id, actor, type, text, created_at FROM activities ORDER BY created_at DESC LIMIT 10`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var feed []issues.Activity
	for rows.Next() {
		var a issues.Activity
		var createdAt string
		if err := rows.Scan(&a.ID, &a.BugID, &a.Actor, &a.Type, &a.Text, &createdAt); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		feed = append(feed, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	onlineCount, _ := s.RecentUserCount(24 * time.Hour)

	return &issues.DockData{
		OpenCount:   openCount,
		OnlineCount: onlineCount,
		ReadyBugs:   readyBugs,
		RecentFeed:  feed,
	}, nil
}

// --- Session methods ---

func (s *Store) TouchSession(user string) {
	now := time.Now().UTC().Format(time.RFC3339)
	s.db.Exec(`INSERT INTO user_sessions (user, last_seen) VALUES (?, ?)
		ON CONFLICT(user) DO UPDATE SET last_seen = ?`, user, now, now)
}

func (s *Store) RecentUserCount(since time.Duration) (int, error) {
	cutoff := time.Now().UTC().Add(-since).Format(time.RFC3339)
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM user_sessions WHERE last_seen >= ?`, cutoff).Scan(&count)
	return count, err
}

// --- Project methods ---

func (s *Store) GetProjectSnapshot() (*project.Snapshot, error) {
	var ov project.Overview
	err := s.db.QueryRow(`SELECT phase, owner, repo, branch FROM project_overview WHERE id = 1`).
		Scan(&ov.Phase, &ov.Owner, &ov.Repo, &ov.Branch)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(
		`SELECT id, section, title, status, progress, owner, due, created_by, created_at, updated_at
		 FROM project_tasks ORDER BY id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []project.Task
	for rows.Next() {
		var t project.Task
		var createdAt, updatedAt string
		if err := rows.Scan(&t.ID, &t.Section, &t.Title, &t.Status, &t.Progress, &t.Owner, &t.Due, &t.CreatedBy, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if tasks == nil {
		tasks = []project.Task{}
	}
	return &project.Snapshot{Overview: ov, Tasks: tasks}, nil
}

func (s *Store) CreateProjectTask(section, title, owner, createdBy, due string, progress *int) (*project.Task, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	prog := 0
	if progress != nil {
		prog = *progress
	}
	res, err := s.db.Exec(
		`INSERT INTO project_tasks (section, title, status, progress, owner, due, created_by, created_at, updated_at)
		 VALUES (?, ?, 'todo', ?, ?, ?, ?, ?, ?)`,
		section, title, prog, owner, due, createdBy, now, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.getProjectTask(int(id))
}

func (s *Store) UpdateProjectTask(id int, title, owner, status, due *string, progress *int) (*project.Task, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if title != nil {
		if _, err := s.db.Exec(`UPDATE project_tasks SET title = ?, updated_at = ? WHERE id = ?`, *title, now, id); err != nil {
			return nil, err
		}
	}
	if owner != nil {
		if _, err := s.db.Exec(`UPDATE project_tasks SET owner = ?, updated_at = ? WHERE id = ?`, *owner, now, id); err != nil {
			return nil, err
		}
	}
	if status != nil {
		if _, err := s.db.Exec(`UPDATE project_tasks SET status = ?, updated_at = ? WHERE id = ?`, *status, now, id); err != nil {
			return nil, err
		}
	}
	if due != nil {
		if _, err := s.db.Exec(`UPDATE project_tasks SET due = ?, updated_at = ? WHERE id = ?`, *due, now, id); err != nil {
			return nil, err
		}
	}
	if progress != nil {
		if _, err := s.db.Exec(`UPDATE project_tasks SET progress = ?, updated_at = ? WHERE id = ?`, *progress, now, id); err != nil {
			return nil, err
		}
	}
	return s.getProjectTask(id)
}

func (s *Store) DeleteProjectTask(id int) error {
	res, err := s.db.Exec(`DELETE FROM project_tasks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("task %d not found", id)
	}
	return nil
}

func (s *Store) UpdateProjectOverview(phase, owner, repo, branch string) (*project.Overview, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if phase != "" {
		if _, err := s.db.Exec(`UPDATE project_overview SET phase = ?, updated_at = ? WHERE id = 1`, phase, now); err != nil {
			return nil, err
		}
	}
	if owner != "" {
		if _, err := s.db.Exec(`UPDATE project_overview SET owner = ?, updated_at = ? WHERE id = 1`, owner, now); err != nil {
			return nil, err
		}
	}
	if repo != "" {
		if _, err := s.db.Exec(`UPDATE project_overview SET repo = ?, updated_at = ? WHERE id = 1`, repo, now); err != nil {
			return nil, err
		}
	}
	if branch != "" {
		if _, err := s.db.Exec(`UPDATE project_overview SET branch = ?, updated_at = ? WHERE id = 1`, branch, now); err != nil {
			return nil, err
		}
	}
	var ov project.Overview
	err := s.db.QueryRow(`SELECT phase, owner, repo, branch FROM project_overview WHERE id = 1`).
		Scan(&ov.Phase, &ov.Owner, &ov.Repo, &ov.Branch)
	if err != nil {
		return nil, err
	}
	return &ov, nil
}

func (s *Store) getProjectTask(id int) (*project.Task, error) {
	var t project.Task
	var createdAt, updatedAt string
	err := s.db.QueryRow(
		`SELECT id, section, title, status, progress, owner, due, created_by, created_at, updated_at
		 FROM project_tasks WHERE id = ?`, id,
	).Scan(&t.ID, &t.Section, &t.Title, &t.Status, &t.Progress, &t.Owner, &t.Due, &t.CreatedBy, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &t, nil
}
