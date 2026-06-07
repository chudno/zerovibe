---
description: UI-кит Zerovibe — Franken UI (компоненты) + дизайн-принципы impeccable (композиция, против ИИ-слопа). Используй ВСЕГДА при создании/изменении HTML-шаблонов (templates/*.html). Обязательный минимум перед вёрсткой: прочитать impeccable/product.md + impeccable/layout.md; перед сдачей — пройти impeccable/critique.md. Полная дока компонентов — в contexts/.
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

Тема Franken UI выставляется инлайн-скриптом в `<head>` (тема `uk-theme-zinc`,
скругления `uk-radii-md`, тени `uk-shadows-sm`); светлая/тёмная — по системной
настройке пользователя и его выбору, не навязывается. `<body>` несёт
`class="bg-background text-foreground"`.

> Графики (`<uk-chart>`) НЕ подключены по умолчанию — скрипт большой (~0.5 МБ) и
> бьёт по скорости загрузки, а графики нужны редко. Понадобился график —
> подключи `frankenui-chart.iife.js` в `layout.html` для этого проекта (см.
> `contexts/chart.md`), иначе не тащи.

## Правила вёрстки

1. **Используй компоненты Franken UI, не пиши свой CSS.** Для нового UI бери готовые
   классы `uk-*` и semantic-цвета (`bg-background`, `text-foreground`, `text-muted`,
   `uk-text-destructive`). Inline-`<style>` и кастомный CSS — только в крайнем случае.
2. **Сохраняй HTMX-паттерн** (см. skill `conventions`): `GET` → полная страница,
   мутации → HTML-фрагмент. Franken UI это не меняет — просто оформляет разметку.
3. **Иконки:** `<uk-icon icon="имя"></uk-icon>` (lucide-набор, напр. `trash`, `plus`).
4. **Тему не навязывай.** Используй semantic-цвета (`bg-background`, `text-foreground`,
   `text-muted`) — они работают и в светлой, и в тёмной. Не форсируй конкретную тему.

## Композиция и «красиво, не как ИИ-слоп» — читай impeccable/

Franken UI даёт компоненты, но НЕ собирает из них хорошую страницу. «Дешёвый,
сгенерированный-ИИ» вид (растянуто на весь экран, плоский ритм отступов, цвета
мимо, слабая иерархия) — это композиция, твоя зона. В папке `impeccable/` рядом —
выверенные дизайн-принципы (адаптировано из github.com/pbakaus/impeccable, Apache-2.0).

**ОБЯЗАТЕЛЬНЫЙ МИНИМУМ (не пропускать).** Достаточно прочитать ОДИН раз за сессию —
дальше держи в контексте, повторно открывать не нужно. Но прочитать НАДО все три:

1. **`impeccable/product.md`** — режим продуктового UI (дашборды, формы, таблицы):
   консистентность, плотность, состояния, что выдаёт слоп. Наши приложения — это он.
2. **`impeccable/layout.md`** — отступы, РИТМ, иерархия, ширина, сетка. **Самый
   частый провал — пропустить этот файл и оставить плоские отступы. Читать ВСЕГДА,
   не только когда «расползается».**
3. **`impeccable/critique.md`** — чек-лист самопроверки. **Пройди ПЕРЕД тем как
   сказать пользователю «готово»** («если кто-то скажет *это сделал ИИ* — переделай»).

Дополнительно по нужде (если задача про это): `impeccable/typeset.md` (типографика),
`impeccable/colorize.md` (цвет/контраст, графики в тон темы), `impeccable/craft.md`
(общая планка качества).

На нашем стеке всё это реализуй ЧЕРЕЗ Franken UI (классы `uk-*`, контейнеры,
semantic-цвета), а не сочиняя свой CSS/палитру с нуля. impeccable даёт ПРИНЦИП —
Franken UI даёт ИНСТРУМЕНТ.

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
| Контейнер страницы | `<div class="uk-container uk-container-small">` (или `-xsmall` для узких форм) |
| График (если подключён) | `<uk-chart>` — см. `contexts/chart.md` (скрипт подключать вручную) |

Эталон применения — в `internal/transport/web/templates/` (`layout.html`, `notes.html`,
`note.html`).

## Полная документация компонентов

Папка `contexts/` рядом с этим файлом — официальные context-файлы Franken UI
(~70 md, по одному на компонент: `card.md`, `form.md`, `table.md`, `modal.md`,
`dropdown.md`, `tab.md`, `select.md` и т.д., плюс `theming.md`, `installation.md`,
`icon-library.md`). Когда нужен компонент, которого нет в шпаргалке, или его модификаторы —
**прочитай соответствующий `contexts/<имя>.md`** перед вёрсткой, не угадывай классы.
