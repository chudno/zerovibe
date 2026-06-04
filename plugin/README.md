# Плагин zerovibe для Claude Code

Даёт Claude Code всё, чтобы строить и деплоить приложения на стэке
**Go + HTMX + SQLite** по эталону zerovibe.

## Что внутри (skills)

- **conventions** — архитектура и паттерны (чистые слои, очередь записи SQLite,
  HTMX-рендеринг фрагментов, обработка ошибок). Claude следует им при любом коде.
- **new-feature** — рецепт добавления сущности срезом через все слои (по образцу `Note`).
- **testing-rules** — что и как покрывать (unit usecase + e2e транспорта).
- **deploy** — деплой на хостинг zerovibe одной командой (через API оркестратора).

## Установка (локально, для разработки)

```sh
claude --plugin-dir ./plugin
```

Скиллы появятся под namespace `zerovibe:` — например `/zerovibe:new-feature`,
`/zerovibe:deploy`. Большинство скиллов model-invoked: Claude подхватывает их
по контексту задачи (описание в `description`).

## Использование (как у вайбкодера)

1. Старт с эталонного скелета (этот репозиторий — образец).
2. «Добавь раздел задач» → Claude применяет skill **new-feature** + **conventions**,
   генерирует слой за слоем, пишет тесты (**testing-rules**).
3. «Задеплой» → skill **deploy** дёргает оркестратор, приложение едет на
   `<subdomain>.zerovibe.ru` с авто-TLS.

## Структура

```
plugin/
  .claude-plugin/plugin.json   манифест
  skills/
    conventions/SKILL.md
    new-feature/SKILL.md
    testing-rules/SKILL.md
    deploy/SKILL.md
```

## Распространение

Через marketplace Claude Code (`/plugin`) или `--plugin-dir`. Версия — поле
`version` в `plugin.json`.
