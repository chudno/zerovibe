// Тест миграций: на временной БД MigrateUp создаёт схему и идемпотентен при повторе.
package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// tableExists проверяет наличие таблицы в sqlite_master.
func tableExists(t *testing.T, d *DB, name string) bool {
	t.Helper()
	var got string
	err := d.Read(func(s *sql.DB) error {
		return s.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name,
		).Scan(&got)
	})
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("проверка таблицы %q: %v", name, err)
	}
	return got == name
}

func TestMigrateUp_CreatesSchema(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "m.db")
	d, err := Open(dsn)
	if err != nil {
		t.Fatalf("открыть БД: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	if err := d.MigrateUp(context.Background()); err != nil {
		t.Fatalf("миграции: %v", err)
	}

	// notes — из первой миграции; goose_db_version — служебная таблица goose.
	for _, tbl := range []string{"notes", "goose_db_version"} {
		if !tableExists(t, d, tbl) {
			t.Errorf("ожидалась таблица %q после миграций", tbl)
		}
	}
}

func TestMigrateUp_Idempotent(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "m.db")
	d, err := Open(dsn)
	if err != nil {
		t.Fatalf("открыть БД: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	ctx := context.Background()
	if err := d.MigrateUp(ctx); err != nil {
		t.Fatalf("первый прогон: %v", err)
	}
	// Повторный прогон не должен падать (уже применённые миграции пропускаются).
	if err := d.MigrateUp(ctx); err != nil {
		t.Fatalf("повторный прогон должен быть no-op, получено: %v", err)
	}
}
