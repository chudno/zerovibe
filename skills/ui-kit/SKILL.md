---
description: UI-кит Zerovibe — Franken UI (shadcn-вид на UIkit) для веб-приложений. Используй ВСЕГДА при создании или изменении HTML-шаблонов (templates/*.html): какие компоненты и классы применять, как держать единый фирменный стиль. Полная документация всех ~70 компонентов — в contexts/.
---

# UI-кит Zerovibe — Franken UI

Веб-приложения Zerovibe оформляются библиотекой **Franken UI** — это компоненты
в стиле **shadcn/ui** на базе UIkit 3. Цель: каждое приложение креатора выглядит
современно и единообразно без ручной вёрстки CSS с нуля.

## Что уже подключено (НЕ трогай без причины)

Вся статика **локальная** (вшита в бинарь через `embed`, раздаётся с `/static/`) —
никаких CDN в рантайме (важно для работы в РФ и доступности). Подключено в
`internal/transport/web/templates/layout.html`:

- `frankenui-core.min.css` + `frankenui-utilities.min.css` — стили и utility-классы.
- `frankenui-core.iife.js` — компоненты (модалки, дропдауны, табы и т.п.).
- `frankenui-icon.iife.js` — иконки `<uk-icon>`.
- `htmx.min.js` — интерактивность.

Тема выставляется инлайн-скриптом в `<head>`: по умолчанию **тёмная** (`dark` на
`<html>`), тема `uk-theme-zinc`, скругления `uk-radii-md`, тени `uk-shadows-sm`.
`<body>` несёт `class="bg-background text-foreground"`.

## Правила вёрстки

1. **Используй компоненты Franken UI, не пиши свой CSS.** Для нового UI бери готовые
   классы `uk-*` и semantic-цвета (`bg-background`, `text-foreground`, `text-muted`,
   `uk-text-destructive`). Inline-`<style>` и кастомный CSS — только в крайнем случае.
2. **Сохраняй HTMX-паттерн** (см. skill `conventions`): `GET` → полная страница,
   мутации → HTML-фрагмент. Franken UI это не меняет — просто оформляет разметку.
3. **Иконки:** `<uk-icon icon="имя"></uk-icon>` (lucide-набор, напр. `trash`, `plus`).
4. **Тёмная тема — по умолчанию.** Не навязывай светлую; semantic-цвета сами
   адаптируются под `dark`.

## Шпаргалка по частым компонентам

| Нужно | Классы / разметка |
|---|---|
| Карточка | `<div class="uk-card uk-card-body">…</div>` |
| Заголовок карточки | `<h3 class="uk-card-title">…</h3>` |
| Кнопка (основная) | `<button class="uk-btn uk-btn-primary">…</button>` |
| Кнопка (вторичная/призрак/опасная) | `uk-btn-secondary` / `uk-btn-ghost` / `uk-btn-destructive` |
| Текстовое поле | `<input class="uk-input">` |
| Многострочное | `<textarea class="uk-textarea">` |
| Селект | `<select class="uk-select">` |
| Чекбокс / радио | `uk-checkbox` / `uk-radio` |
| Группа полей | `<fieldset class="uk-fieldset space-y-4">` |
| Алерт | `<div class="uk-alert">` (+ `uk-alert-destructive`) |
| Бейдж | `<span class="uk-badge">` |
| Таблица | `<table class="uk-table">` |
| Контейнер страницы | `<div class="uk-container uk-container-small">` |

Эталон применения — в `internal/transport/web/templates/` (`layout.html`, `notes.html`,
`note.html`).

## Полная документация компонентов

Папка `contexts/` рядом с этим файлом — официальные context-файлы Franken UI
(~70 md, по одному на компонент: `card.md`, `form.md`, `table.md`, `modal.md`,
`dropdown.md`, `tab.md`, `select.md` и т.д., плюс `theming.md`, `installation.md`,
`icon-library.md`). Когда нужен компонент, которого нет в шпаргалке, или его модификаторы —
**прочитай соответствующий `contexts/<имя>.md`** перед вёрсткой, не угадывай классы.
