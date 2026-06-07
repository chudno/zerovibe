# Плагин zerovibe для Claude Code

Даёт Claude Code всё, чтобы строить и деплоить приложения на стэке
**Go + HTMX + SQLite** по эталону zerovibe — от первой фичи до прод-деплоя на
`<поддомен>.zerovibe.ru` с авто-TLS.

## Быстрый старт (для вайбкодера)

1. **Получи доступ.** Оставь заявку на https://zerovibe.ru — мы заведём аккаунт и
   пришлём данные для входа.
2. **Создай ключ.** Войди в кабинет → раздел «Ключи» (`/app/keys`) → «Создать ключ»,
   скопируй секрет (показывается один раз).
3. **Поставь плагин** (в Claude Code):
   ```
   /plugin marketplace add chudin/zerovibe
   /plugin install zerovibe@zerovibe
   ```
   (или локально для разработки: `claude --plugin-dir ./plugin`)
4. **Подключи ключ** (один раз на машину):
   ```
   /zerovibe:install
   ```
   Claude попросит вставить ключ — он сохранится в `~/.zerovibe/config.json`.
5. **Заведи проект** в папке приложения:
   ```
   /zerovibe:new моё-приложение
   ```
   Создаёт проект на платформе и пишет его id в `.env` папки.
6. **Пиши и деплой.** Проси Claude «добавь раздел задач» (skills new-feature +
   conventions + testing-rules), затем:
   ```
   /zerovibe:deploy
   ```
   Платформа соберёт образ из твоего кода и поднимет приложение по HTTPS.

## Команды (skills под namespace `zerovibe:`)

| Команда | Что делает |
|---|---|
| `/zerovibe:install` | сохранить API-ключ глобально (`~/.zerovibe/config.json`), один раз |
| `/zerovibe:new <имя>` | создать проект на платформе, привязать папку (`.env` → ZEROVIBE_PROJECT) |
| `/zerovibe:deploy` | собрать архив, залить, задеплоить → `<sub>.zerovibe.ru` |
| `/zerovibe:new-feature` | добавить сущность срезом через все слои (по образцу `Note`) |
| `/zerovibe:conventions` | архитектура и паттерны стека (model-invoked) |
| `/zerovibe:testing-rules` | что и как покрывать тестами (model-invoked) |

## Модель доступа

- **Ключ — глобальный**, на все проекты вайбкодера. Хранится в
  `~/.zerovibe/config.json` (права 0600, как `~/.aws/credentials`). Не коммить.
- **Проект — на папку.** `/zerovibe:new` пишет `ZEROVIBE_PROJECT` и `ZEROVIBE_API`
  в `.env` папки (не секреты). Ключ в `.env` НЕ попадает.
- Все вызовы к платформе — заголовком `X-API-Key`. Тот же ключ работает и в кабинете.

## Структура

```
zerovibe/                       ← корень репо = плагин
  .claude-plugin/
    plugin.json                 манифест плагина
    marketplace.json            для установки через /plugin marketplace add
  skills/
    install/SKILL.md            подключение ключа (глобально)
    new/SKILL.md                развернуть шаблон + создать проект + привязка папки
    deploy/SKILL.md             сборка + деплой
    conventions/SKILL.md        архитектура стека
    new-feature/SKILL.md        рецепт фичи через слои
    testing-rules/SKILL.md      правила тестов
  template/                     код-шаблон Go+HTMX+SQLite (его копирует /zerovibe:new)
```

## Распространение

Через marketplace Claude Code (корневой `.claude-plugin/marketplace.json`,
source `./`) или `--plugin-dir .`. Версия — поле `version` в `plugin.json`.
