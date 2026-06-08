---
description: Добавить новую сущность/фичу в zerovibe-приложение (CRUD через все слои по эталону Note). Используй, когда пользователь просит добавить новую модель, страницу, раздел или CRUD (например "добавь задачи", "сделай раздел клиентов").
---

# Новая фича в Zerovibe

Пользователь — создатель продукта, не программист (см. «Общение с пользователем»
в skill `conventions`). Реши технику сам по этим правилам, **не задавай вопросов
про реализацию**; уточняй только продуктовый смысл и только если он правда неясен.
По итогу опиши результат словами продукта, без жаргона.

Добавляй сущность срезом через ВСЕ слои, по образцу существующей `Note`. Для
сущности `X` (пример: `Task`, `Client`) выполни по порядку:

## 1. domain — `internal/domain/<x>.go`
- Структура `X` (поля, без тегов), `time.Time` для дат.
- Конструктор-валидатор `NewX(...) (X, error)` — все инварианты здесь
  (обязательные поля, лимиты длины) через `ErrValidation`.

## 2. usecase — `internal/usecase/<x>s.go`
- Порт `XRepository` (интерфейс: `Create/List/Get/Update/Delete` — что нужно фиче).
- Сервис `XService` с методами-операциями: валидирует через `domain.NewX`,
  делегирует порту. Без SQL и HTTP.

## 3. repository/sqlite — `internal/repository/sqlite/<x>s.go`
- `const Schema` с `CREATE TABLE IF NOT EXISTS <x>s (...)`.
- `XRepo` реализует порт. Записи — через `db.Write`, чтения — через `db.Read`.
- `INSERT ... RETURNING id, created_at` чтобы вернуть заполненную сущность.
- Применить схему: добавь её в `db.Migrate` в `cmd/server/main.go`.

## 4. transport/web — хендлеры + шаблоны
- В `web.go`: маршруты (`GET`/`POST`/`DELETE`) и хендлеры.
  - `GET` раздела — полная страница.
  - мутации — возвращают HTML-ФРАГМЕНТ изменённого элемента.
- Шаблоны в `templates/`: `{{define "content"}}` для страницы, `{{define "<x>"}}`
  для фрагмента. HTMX-атрибуты: `hx-post`/`hx-delete`, `hx-target`, `hx-swap`.
- **Оформление — из макета Pencil (skill `design`).** Не сочиняй внешний вид сам:
  открой skill `design`, возьми дизайн экрана из Pencil через MCP и перенеси в
  шаблон + `static/app.css` по точным числам (отступы, цвета, размеры). Вёрстка —
  чистый CSS, без UI-библиотек.

## 5. composition root — `cmd/server/main.go`
- Собрать: `repo := sqlite.NewXRepo(db)` → `svc := usecase.NewXService(repo)` →
  передать в `web.NewServer`.

## 6. Тесты (ОБЯЗАТЕЛЬНО — см. skill testing-rules)
- Unit usecase на фейк-репозитории.
- E2E транспорта через `httptest` на временной SQLite.

## Проверка
```sh
go build ./... && go vet ./... && go test ./...
```
Всё зелёное — фича готова. Запусти `make run` и проверь в браузере.

Аргумент пользователя: $ARGUMENTS
