// Package sqlite — хранилище метаданных оркестратора на SQLite.
// Тот же паттерн, что в эталонном скелете: единая writer-горутина сериализует
// записи (нет SQLITE_BUSY), чтения идут напрямую. Оркестратор — обычное
// Go-приложение и деплоится так же, как приложения вайбкодеров.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/chudno/zerovibe/orchestrator/internal/domain"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS nodes (
	id          TEXT PRIMARY KEY,
	provider_id TEXT NOT NULL,
	ipv4        TEXT NOT NULL,
	status      TEXT NOT NULL,
	capacity    INTEGER NOT NULL,
	created_at  TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS apps (
	id         TEXT PRIMARY KEY,
	owner_id   TEXT NOT NULL,
	name       TEXT NOT NULL,
	subdomain  TEXT NOT NULL UNIQUE,
	image      TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS deployments (
	id         TEXT PRIMARY KEY,
	app_id     TEXT NOT NULL,
	node_id    TEXT NOT NULL,
	domain     TEXT NOT NULL,
	status     TEXT NOT NULL,
	created_at TEXT NOT NULL
);`

// Store — SQLite-хранилище с очередью записи.
type Store struct {
	db     *sql.DB
	writes chan func(*sql.DB) error
	done   chan struct{}
}

// Open открывает БД, применяет схему и запускает writer-горутину.
func Open(ctx context.Context, dsn string) (*Store, error) {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	full := dsn + sep + "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", full)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	s := &Store{db: db, writes: make(chan func(*sql.DB) error), done: make(chan struct{})}
	go s.writer()
	if err := s.write(ctx, func(d *sql.DB) error {
		_, err := d.ExecContext(ctx, schema)
		return err
	}); err != nil {
		return nil, fmt.Errorf("миграция: %w", err)
	}
	return s, nil
}

func (s *Store) writer() {
	for {
		select {
		case fn := <-s.writes:
			// reply передаётся через замыкание ниже (write).
			fn(s.db)
		case <-s.done:
			return
		}
	}
}

// write сериализует операцию записи через writer-горутину.
func (s *Store) write(ctx context.Context, fn func(*sql.DB) error) error {
	reply := make(chan error, 1)
	op := func(d *sql.DB) error { err := fn(d); reply <- err; return err }
	select {
	case s.writes <- op:
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return fmt.Errorf("store закрыт")
	}
	select {
	case err := <-reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close останавливает writer и закрывает БД.
func (s *Store) Close() error {
	close(s.done)
	return s.db.Close()
}

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339) }
func pt(s string) time.Time { t, _ := time.Parse(time.RFC3339, s); return t }

// --- Apps ---

func (s *Store) CreateApp(ctx context.Context, a domain.App) (domain.App, error) {
	err := s.write(ctx, func(d *sql.DB) error {
		_, err := d.ExecContext(ctx,
			`INSERT INTO apps (id, owner_id, name, subdomain, image, created_at) VALUES (?,?,?,?,?,?)`,
			a.ID, a.OwnerID, a.Name, a.Subdomain, a.Image, ts(a.CreatedAt))
		return err
	})
	return a, err
}

func (s *Store) GetApp(ctx context.Context, id string) (domain.App, error) {
	var a domain.App
	var created string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, owner_id, name, subdomain, image, created_at FROM apps WHERE id=?`, id).
		Scan(&a.ID, &a.OwnerID, &a.Name, &a.Subdomain, &a.Image, &created)
	if err == sql.ErrNoRows {
		return domain.App{}, domain.ErrNotFound{Entity: "app", ID: id}
	}
	if err != nil {
		return domain.App{}, err
	}
	a.CreatedAt = pt(created)
	return a, nil
}

func (s *Store) ListApps(ctx context.Context) ([]domain.App, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, owner_id, name, subdomain, image, created_at FROM apps ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.App
	for rows.Next() {
		var a domain.App
		var created string
		if err := rows.Scan(&a.ID, &a.OwnerID, &a.Name, &a.Subdomain, &a.Image, &created); err != nil {
			return nil, err
		}
		a.CreatedAt = pt(created)
		out = append(out, a)
	}
	return out, rows.Err()
}

// --- Nodes ---

func (s *Store) CreateNode(ctx context.Context, n domain.Node) (domain.Node, error) {
	err := s.write(ctx, func(d *sql.DB) error {
		_, err := d.ExecContext(ctx,
			`INSERT INTO nodes (id, provider_id, ipv4, status, capacity, created_at) VALUES (?,?,?,?,?,?)`,
			n.ID, n.ProviderID, n.IPv4, string(n.Status), n.Capacity, ts(n.CreatedAt))
		return err
	})
	return n, err
}

func (s *Store) ListNodes(ctx context.Context) ([]domain.Node, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, provider_id, ipv4, status, capacity, created_at FROM nodes ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Node
	for rows.Next() {
		var n domain.Node
		var status, created string
		if err := rows.Scan(&n.ID, &n.ProviderID, &n.IPv4, &status, &n.Capacity, &created); err != nil {
			return nil, err
		}
		n.Status = domain.NodeStatus(status)
		n.CreatedAt = pt(created)
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) SetNodeStatus(ctx context.Context, id string, status domain.NodeStatus) error {
	return s.write(ctx, func(d *sql.DB) error {
		_, err := d.ExecContext(ctx, `UPDATE nodes SET status=? WHERE id=?`, string(status), id)
		return err
	})
}

func (s *Store) CountDeploymentsOnNode(ctx context.Context, nodeID string) (int, error) {
	var c int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM deployments WHERE node_id=? AND status != ?`,
		nodeID, string(domain.DeployFailed)).Scan(&c)
	return c, err
}

// --- Deployments ---

func (s *Store) CreateDeployment(ctx context.Context, dep domain.Deployment) (domain.Deployment, error) {
	err := s.write(ctx, func(d *sql.DB) error {
		_, err := d.ExecContext(ctx,
			`INSERT INTO deployments (id, app_id, node_id, domain, status, created_at) VALUES (?,?,?,?,?,?)`,
			dep.ID, dep.AppID, dep.NodeID, dep.Domain, string(dep.Status), ts(dep.CreatedAt))
		return err
	})
	return dep, err
}

func (s *Store) SetDeploymentStatus(ctx context.Context, id string, status domain.DeployStatus) error {
	return s.write(ctx, func(d *sql.DB) error {
		_, err := d.ExecContext(ctx, `UPDATE deployments SET status=? WHERE id=?`, string(status), id)
		return err
	})
}

func (s *Store) ListDeployments(ctx context.Context) ([]domain.Deployment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, app_id, node_id, domain, status, created_at FROM deployments ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Deployment
	for rows.Next() {
		var dep domain.Deployment
		var status, created string
		if err := rows.Scan(&dep.ID, &dep.AppID, &dep.NodeID, &dep.Domain, &status, &created); err != nil {
			return nil, err
		}
		dep.Status = domain.DeployStatus(status)
		dep.CreatedAt = pt(created)
		out = append(out, dep)
	}
	return out, rows.Err()
}
