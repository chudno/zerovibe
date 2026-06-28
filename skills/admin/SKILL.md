---
description: Зарегистрировать сущность во встроенной админ-панели zerovibe-приложения (generic-CRUD «бэкофис из коробки»: список/форма/поиск/фильтры/сортировка/экспорт/файлы). Используй, когда пользователь просит «админку», «бэкофис», «панель управления», «CRUD-панель», «раздел в админке», «чтобы я мог править данные руками», или редактировать/добавлять записи через интерфейс администратора.
---

# Админка из коробки (регистрация сущности)

В шаблоне zerovibe есть встроенная **админ-панель** (`internal/admin`): ОДИН generic-слой
рисует список/форму/CRUD/поиск/фильтры/сортировку/экспорт CSV/загрузку файлов для ЛЮБОЙ
сущности. Создателю приложения это даёт «бэкофис» без отдельного кода на каждый экран.

Чтобы сущность появилась в админке — её надо **зарегистрировать** дескриптором. Сам
generic-слой (хендлеры, шаблоны, стиль) трогать НЕ нужно. Эталон — `Note` и `User`
(`internal/adminres/note.go`, `user.go`). Пользователь — не программист (см. skill
`conventions`): технику реши сам, не задавай вопросов про реализацию.

> Доступ в админку: `/admin`, отдельный вход `/admin/login` (свой дизайн), только роль
> `admin`. Это уже готово — вход/выход/защиту НЕ трогай.

## Когда сущности ещё нет

Если сущности (`domain.X`, репозиторий, таблица) ещё нет — сперва создай её вертикальным
срезом по skill `new-feature` (domain → usecase → repository → миграция), ПОТОМ
регистрируй в админке по этому скиллу. Если сущность уже есть — сразу сюда.

## Шаг 1. Admin-методы репозитория

Админка видит ВСЕ записи (не по владельцу). Добавь в `internal/repository/sqlite/<x>s_admin.go`
по образцу `notes_admin.go`:

```go
func (r *XRepo) ListAll(ctx context.Context) ([]domain.X, error)        // список (новые сверху)
func (r *XRepo) GetByID(ctx context.Context, id int64) (domain.X, error) // одна запись для формы
func (r *XRepo) UpdateAny(ctx context.Context, x domain.X) error         // обновить любую
func (r *XRepo) DeleteAny(ctx context.Context, id int64) error           // удалить любую
```
Записи через `db.Write`, чтения через `db.Read`. `Create` обычно уже есть из `new-feature`.

## Шаг 2. Дескриптор — `internal/adminres/<x>.go`

ЭТАЛОН целиком (скопируй и адаптируй под свою сущность — это `note.go`):

```go
// Если есть связи (FieldRelation) — прокинь источник опций параметром, как в note.go
// (там owners). Для связи на пользователей готов adminres.UserOptions(userRepo).
func RegisterX(reg *admin.Registry, repo XAdminRepo, owners func(context.Context) ([]admin.Option, error)) {
    admin.Register(reg, admin.Resource[domain.X]{
        Name:  "xs",            // URL: /admin/xs (латиницей, мн.ч.)
        Title: "Записи",        // в меню/шапке
        Icon:  "list",          // имя Lucide-иконки раздела (см. список ниже)
        Fields: []admin.Field{
            {Name: "title", Label: "Заголовок", Type: admin.FieldText, Required: true, InList: true, Sortable: true},
            {Name: "note",  Label: "Описание",  Type: admin.FieldTextarea, Help: "Подсказка под полем"},
            {Name: "count", Label: "Кол-во",    Type: admin.FieldNumber, InList: true, Sortable: true},
            {Name: "due",   Label: "Срок",      Type: admin.FieldDate, InList: true},          // нативный календарь
            {Name: "active",Label: "Активна",   Type: admin.FieldBool, InList: true},          // switch
            {Name: "status",Label: "Статус",    Type: admin.FieldSelect, Options: statusOptions, InList: true}, // фильтр+select
            // связь на другую сущность: combobox по имени
            {Name: "owner_id", Label: "Владелец", Type: admin.FieldRelation, Required: true, InList: true,
                RelationOptions: func(ctx context.Context) ([]admin.Option, error) { return owners(ctx) }},
            {Name: "photo", Label: "Фото", Type: admin.FieldFile},                              // загрузка файла
        },

        // Row — как запись выглядит СТРОКОЙ списка (ячейка на каждое InList-поле).
        Row: func(x domain.X) admin.Record {
            return admin.Record{
                ID: strconv.FormatInt(x.ID, 10),
                Cells: map[string]admin.Cell{
                    "title":    {Value: x.Title, Display: x.Title},
                    "count":    {Value: strconv.Itoa(x.Count), Display: strconv.Itoa(x.Count)},
                    "due":      {Value: x.Due, Display: x.Due},
                    "active":   {Value: strconv.FormatBool(x.Active)},                 // bool: только Value "true"/"false"
                    "status":   {Value: x.Status, Display: statusLabel(x.Status), Tone: "success"}, // badge: Tone красит пилюлю
                    "owner_id": {Value: strconv.FormatInt(x.OwnerID, 10), Display: ownerName(x.OwnerID)},
                    "photo":    {Value: x.PhotoKey, IsFile: true},                      // file: IsFile → миниатюра
                },
            }
        },

        List: func(ctx context.Context) ([]domain.X, error) { return repo.ListAll(ctx) },

        // Get — ВСЕ редактируемые поля для формы. Дата — формат YYYY-MM-DD. bool — "true"/"false".
        Get: func(ctx context.Context, id string) (admin.FormValues, error) {
            xid, _ := strconv.ParseInt(id, 10, 64)
            x, err := repo.GetByID(ctx, xid)
            if err != nil { return nil, err }
            return admin.FormValues{
                "title":  x.Title,
                "note":   x.Note,
                "count":  strconv.Itoa(x.Count),
                "due":    x.Due,
                "active": strconv.FormatBool(x.Active),
                "status": x.Status,
                "owner_id": strconv.FormatInt(x.OwnerID, 10),
                "photo":  x.PhotoKey,
            }, nil
        },

        Create: func(ctx context.Context, v admin.FormValues, _ map[string]admin.FileInput) error {
            x, err := domain.NewX(v["title"], v["note"], ...) // валидация — в доменном конструкторе
            if err != nil { return err }
            x.Active = v["active"] == "true"
            x.OwnerID, _ = strconv.ParseInt(v["owner_id"], 10, 64)
            x.PhotoKey = v["photo"]                            // файл уже сохранён, тут только ключ
            _, err = repo.Create(ctx, x)
            return err
        },

        Update: func(ctx context.Context, id string, v admin.FormValues, _ map[string]admin.FileInput) error {
            x, err := domain.NewX(v["title"], v["note"], ...)
            if err != nil { return err }
            x.ID, _ = strconv.ParseInt(id, 10, 64)
            x.Active = v["active"] == "true"                   // ← КАЖДОЕ поле формы перенеси в сущность
            x.OwnerID, _ = strconv.ParseInt(v["owner_id"], 10, 64)
            x.PhotoKey = v["photo"]
            return repo.UpdateAny(ctx, x)
        },

        Delete: func(ctx context.Context, id string) error {
            xid, _ := strconv.ParseInt(id, 10, 64)
            return repo.DeleteAny(ctx, xid)
        },
    })
}
```

## Шаг 3. Регистрация — одна строка в `cmd/server/main.go`

Рядом с `adminres.RegisterNote(...)` / `RegisterUser(...)`:
```go
adminres.RegisterX(reg, xRepo)
```
Всё. Сущность сразу в админке со списком, формой, CRUD, поиском, фильтрами, экспортом.

## Типы полей (`admin.Field.Type`)

| Тип | Контрол формы | В списке (`Cell`) | Нюанс |
|---|---|---|---|
| `FieldText` | input text | `Display` | короткая строка |
| `FieldTextarea` | textarea | — | длинный текст; обычно `InList:false` |
| `FieldNumber` | input number | число | в БД своя колонка; сортировка числовая |
| `FieldDate` | **нативный календарь** (input date) | дата | формат строго `YYYY-MM-DD` в Get/Create/Update |
| `FieldBool` | **switch** | галочка/прочерк | `Cell.Value` = `"true"`/`"false"`; читать `v["name"]=="true"` |
| `FieldSelect` | select | `Display` | задай `Options []admin.Option`; ещё и фильтр над таблицей |
| `FieldRelation` | combobox по имени | `Display` (имя) | задай `RelationOptions func(ctx)`; значение — id |
| `FieldFile` | загрузка файла | миниатюра | `Cell.IsFile:true`; в форме хранится ключ; см. ниже |
| `FieldBadge` | (нет в форме) | пилюля-статус | задай `Cell.Tone`: `success`/`warning`/`destructive`/`info` |

`Option`: `{Value: "active", Label: "Активна", Tone: "success"}` (Tone только для badge).

### Файлы (`FieldFile`)
Загрузка идёт через платформенную фичу файлов (`adapter/platformfiles` → presigned-URL S3,
как почта; локально — каталог `uploads/`). Generic-слой сам сохраняет файл и кладёт его
КЛЮЧ в `FormValues[name]` — в Create/Update просто перенеси `v["name"]` в поле сущности.
Превью и ссылку для показа админка строит сама. Ничего настраивать не надо: `files` уже
собран в `main.go`. В БД храни КЛЮЧ файла (строку), не байты.

## ✅ Чек-лист «полный цикл поля» (ОБЯЗАТЕЛЬНО)

Каждое поле, которое должно сохраняться, обязано пройти ВЕСЬ цикл — иначе оно
ДЕКОРАТИВНОЕ (показывается, но не пишется в БД). Это самый частый и «тихий» баг.
Для КАЖДОГО поля проверь по пунктам:

1. **`Fields`** — поле объявлено с нужным `Type`.
2. **`Get`** — возвращает текущее значение поля (иначе форма всегда покажет пустое/выкл).
3. **`Create`** — значение из `v["name"]` перенесено в сущность ПЕРЕД сохранением.
4. **`Update`** — то же, что в Create (легко забыть → правка «не сохраняется»).
5. **репозиторий** — `Create`/`UpdateAny` реально пишут эту колонку в SQL (`INSERT`/`UPDATE`).
6. **миграция** — колонка есть в схеме (новое поле = новый goose-файл, см. `conventions`).
7. **`Row`** — если поле `InList:true`, в `Cells` есть ячейка с этим именем.

Пропустишь п.2 → форма врёт. Пропустишь п.4 или п.5 → правка молча теряется.

## Грабли (проверены вживую)

- **bool не сохраняется** — самый частый: забыли в `Update`/`Get` (`v["x"]=="true"`) или
  `UPDATE` в репозитории не пишет колонку. Switch будет «кликаться», но не сохранять.
- **дата как текст** — используй `FieldDate` (не `FieldText`), формат `YYYY-MM-DD` везде
  в Get/Create/Update; для показа в списке можно форматировать (`formatDate` в `note.go`).
- **связь** — `FieldRelation` хранит id; `RelationOptions` отдаёт `{Value: id, Label: имя}`;
  в `Row` показывай ИМЯ (Display), не голый id.
- **User-подобные сущности** — осторожно с паролем/ролью/удалением последнего админа
  (см. `user.go`: пароль не возвращаем в Get, защита последнего админа в репозитории).
- **стиль НЕ трогать per-сущность** — вёрстка админки (`internal/admin/templates`,
  shadcn-на-CSS) общая на все сущности, в этом её смысл. Дизайн — из `design/admin.pen`.

## Проверка

`make check` (build+vet+test), затем подними локально, войди в `/admin/login` под
админом и ПРОЙДИ полный цикл руками: создать запись → она в списке → открыть на
редактирование (все поля подставились) → поменять КАЖДОЕ поле → сохранить → проверить,
что изменения видны и в списке, и при повторном открытии формы. Особенно switch/файл/связь.

Тест-регрессия (рекомендуется для полей с логикой, по образцу `users_admin_test.go`):
на временной SQLite проверить, что `UpdateAny` реально пишет поле в БД.

Опиши результат пользователю словами продукта (без жаргона — см. `conventions`).

Аргумент пользователя: $ARGUMENTS
