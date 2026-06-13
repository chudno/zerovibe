package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sync"

	"github.com/pressly/goose/v3"
)

// migrationsFS — встроенные SQL-миграции goose. Лежат рядом, в каталоге migrations/.
// Встраиваем в бинарь, чтобы не зависеть от файловой системы в рантайме (важно для
// distroless-образа без shell). Новая фича = новый файл migrations/NNNNN_*.sql.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// gooseSetup гарантирует одноразовую настройку goose (диалект + источник файлов).
// goose хранит это в глобальном состоянии, поэтому настраиваем под мьютексом и один раз.
var gooseSetup sync.Once

// MigrateUp применяет все непрогнанные миграции (idempotent: уже применённые goose
// пропускает по своей служебной таблице goose_db_version). Вызывается на старте
// приложения до приёма трафика. DDL идёт через очередь записи (db.Write) — единый
// писатель, как и для всех остальных записей.
func (d *DB) MigrateUp(ctx context.Context) error {
	var setupErr error
	gooseSetup.Do(func() {
		goose.SetBaseFS(migrationsFS)
		// modernc.org/sqlite регистрируется как драйвер "sqlite"; goose принимает и
		// "sqlite", и "sqlite3" как один диалект SQLite.
		setupErr = goose.SetDialect("sqlite")
	})
	if setupErr != nil {
		return fmt.Errorf("goose: установить диалект: %w", setupErr)
	}

	return d.Write(ctx, func(s *sql.DB) error {
		if err := goose.UpContext(ctx, s, "migrations"); err != nil {
			return fmt.Errorf("goose: применить миграции: %w", err)
		}
		return nil
	})
}
