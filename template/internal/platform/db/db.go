// Package db — инфраструктурный слой доступа к SQLite с сериализацией записей.
//
// ПОЧЕМУ ОЧЕРЕДЬ ЗАПИСИ. SQLite допускает много одновременных читателей, но
// только одного писателя в момент времени. При параллельных запросах второй
// писатель получает SQLITE_BUSY. Вместо того чтобы ловить эту ошибку и делать
// ретраи в каждом репозитории, мы заводим ОДНУ writer-горутину: все записи
// проходят через неё последовательно. Тогда конкуренции писателей нет в
// принципе, а вызывающий код остаётся простым (просто db.Write(...)).
//
// Это закрывает 99% потребностей небольшого веб-приложения на одном инстансе.
// Чтения идут напрямую (db.Read), параллельно, без очереди.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite" // чистый Go SQLite-драйвер, без CGO → статический бинарь
)

// DB оборачивает *sql.DB и канал команд записи. Создавать через Open, закрывать
// через Close (он останавливает writer-горутину и закрывает соединение).
type DB struct {
	sql    *sql.DB
	writes chan writeOp
	done   chan struct{}
}

// writeOp — единица работы для writer-горутины: функция записи + канал ответа.
type writeOp struct {
	fn    func(*sql.DB) error
	reply chan error
}

// Open открывает базу по пути dsn (например, "file:app.db") и запускает
// writer-горутину. Включает WAL и busy_timeout как дополнительную страховку.
func Open(dsn string) (*DB, error) {
	// _pragma в DSN драйвера modernc применяются к каждому соединению.
	// Склеиваем корректно: ? если параметров ещё нет, иначе & — чтобы dsn,
	// уже содержащий query-строку, не ломался.
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	full := dsn + sep + "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(on)"

	sqlDB, err := sql.Open("sqlite", full)
	if err != nil {
		return nil, fmt.Errorf("открыть sqlite: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	d := &DB{
		sql:    sqlDB,
		writes: make(chan writeOp),
		done:   make(chan struct{}),
	}
	go d.writer()
	return d, nil
}

// writer — единственная горутина, выполняющая записи последовательно.
func (d *DB) writer() {
	for {
		select {
		case op := <-d.writes:
			op.reply <- op.fn(d.sql)
		case <-d.done:
			return
		}
	}
}

// Write ставит операцию записи в очередь и ждёт её завершения. Все INSERT/
// UPDATE/DELETE репозиториев идут через этот метод — так гарантируется один
// писатель. Уважает отмену контекста на время ожидания слота в очереди.
func (d *DB) Write(ctx context.Context, fn func(*sql.DB) error) error {
	op := writeOp{fn: fn, reply: make(chan error, 1)}
	select {
	case d.writes <- op:
	case <-ctx.Done():
		return ctx.Err()
	case <-d.done:
		return fmt.Errorf("db закрыта")
	}
	select {
	case err := <-op.reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Read выполняет чтение напрямую (без очереди) — читатели в SQLite/WAL
// работают параллельно.
func (d *DB) Read(fn func(*sql.DB) error) error {
	return fn(d.sql)
}

// Close останавливает writer-горутину и закрывает соединение.
func (d *DB) Close() error {
	close(d.done)
	return d.sql.Close()
}
