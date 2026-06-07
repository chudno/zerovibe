---
description: Создать новый проект на платформе Zerovibe и привязать к нему текущую папку. Используй, когда пользователь пишет "/zerovibe:init", "создай проект zerovibe", "новый проект на zerovibe", "заведи приложение на платформе", или хочет впервые задеплоить папку, ещё не привязанную к проекту (нет ZEROVIBE_PROJECT в .env).
---

# Новый проект на Zerovibe

Разворачивает в текущей папке рабочий шаблон Go + HTMX + SQLite, создаёт проект
через API платформы (по сохранённому ключу) и привязывает папку (`.env`). После
этого можно сразу писать фичи и деплоить — `/zerovibe:deploy`.

## Предусловие

Должен быть настроен ключ (`~/.zerovibe/config.json`). Если файла нет —
сначала выполни `/zerovibe:install` (попроси пользователя).

## Что делает skill

0. **Развернуть шаблон в текущую папку.** Шаблон лежит внутри плагина; путь —
   относительно этого skill (`${CLAUDE_SKILL_DIR}` = .../skills/init, шаблон на два
   уровня выше, в корне плагина). Копируем, ТОЛЬКО если папка пуста (нет go.mod /
   Dockerfile) — иначе не затираем существующий код, сообщаем и переходим к шагу 1.

   ```sh
   TEMPLATE="${CLAUDE_SKILL_DIR}/../../template"
   if [ -f go.mod ] || [ -f Dockerfile ]; then
     echo "Папка не пуста — пропускаю копирование шаблона, привязываю существующий код."
   else
     cp -R "$TEMPLATE"/. ./
     echo "Шаблон Go+HTMX развёрнут в $(pwd)"
   fi
   ```

   После копирования стоит инициализировать git (`git init`) — по желанию пользователя.

1. **Проверь конфиг** и достань ключ/API:

   ```sh
   CFG=~/.zerovibe/config.json
   test -f "$CFG" || { echo "Нет ключа — выполни /zerovibe:install"; exit 1; }
   KEY=$(python3 -c 'import json,os;print(json.load(open(os.path.expanduser("~/.zerovibe/config.json")))["key"])')
   API=$(python3 -c 'import json,os;print(json.load(open(os.path.expanduser("~/.zerovibe/config.json")))["api"])')
   ```

2. **Имя проекта**: из аргумента пользователя ($ARGUMENTS); если пусто — предложи
   имя текущей папки (`basename "$PWD"`) и подтверди у пользователя.

3. **Создай проект** через API (X-API-Key):

   ```sh
   RESP=$(curl -fsS -X POST "$API/dev/projects" \
     -H "X-API-Key: $KEY" -H "Content-Type: application/json" \
     -d "{\"name\":\"ИМЯ_ПРОЕКТА\"}")
   PROJECT_ID=$(echo "$RESP" | python3 -c 'import sys,json;print(json.load(sys.stdin)["project"]["id"])')
   echo "Создан проект: $PROJECT_ID"
   ```

4. **Запиши привязку в `.env`** текущей папки (идемпотентно — обнови, если уже есть):

   ```sh
   touch .env
   # убрать старые строки, если были, и дописать актуальные
   grep -v -E '^ZEROVIBE_(PROJECT|API)=' .env > .env.tmp 2>/dev/null || true
   mv .env.tmp .env 2>/dev/null || true
   printf 'ZEROVIBE_API=%s\nZEROVIBE_PROJECT=%s\n' "$API" "$PROJECT_ID" >> .env
   ```

5. **Убедись, что `.env` в `.gitignore`** (не коммитить привязку):

   ```sh
   grep -qxF '.env' .gitignore 2>/dev/null || echo '.env' >> .gitignore
   ```

6. Подтверди пользователю: «Проект создан и привязан. Дальше — `/zerovibe:deploy`».

## Замечания

- Ключ НЕ пишется в `.env` (он глобальный, в `~/.zerovibe/config.json`). В `.env`
  проекта — только `ZEROVIBE_PROJECT` и `ZEROVIBE_API` (не секреты).
- Один проект = одна папка. Для другого приложения — отдельная папка + свой `/zerovibe:init`.

Аргумент пользователя (имя проекта): $ARGUMENTS
