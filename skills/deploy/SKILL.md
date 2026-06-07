---
description: Задеплоить приложение на платформу Zerovibe (платформа собирает образ из твоего кода, контейнер на VM, поддомен, авто-TLS). Используй, когда пользователь просит развернуть, задеплоить, выложить приложение или "запустить в прод".
---

# Деплой на платформу Zerovibe

Разворачивает приложение на наших VM: платформа собирает Docker-образ из ТВОЕГО
кода (Dockerfile проекта), поднимает контейнер и выдаёт поддомен
`<subdomain>.zerovibe.ru` с авто-TLS.

## Предусловия (проверь и направь, если чего-то нет)

- **Ключ настроен**: `~/.zerovibe/config.json` существует. Нет → `/zerovibe:install`.
- **Папка привязана к проекту**: в `.env` есть `ZEROVIBE_PROJECT`. Нет → `/zerovibe:new`.
- **В корне есть `Dockerfile`** (multi-stage, distroless, без CGO — как в эталонном
  скелете; приложение слушает `:8080`).

Достань креды и привязку:
```sh
KEY=$(python3 -c 'import json,os;print(json.load(open(os.path.expanduser("~/.zerovibe/config.json")))["key"])')
API=$(python3 -c 'import json,os;print(json.load(open(os.path.expanduser("~/.zerovibe/config.json")))["api"])')
set -a; . ./.env; set +a   # ZEROVIBE_PROJECT, ZEROVIBE_API из .env проекта
API="${ZEROVIBE_API:-$API}"
```

## Шаг 0 — проверка готовности

```sh
go build ./... && go vet ./... && go test ./...
```
Красное — НЕ деплой, сначала почини (см. skill testing-rules).

## Шаг 1 — собрать архив кода

```sh
tar --exclude=.git --exclude=data --exclude='*.db' --exclude=tmp \
    --exclude=node_modules --exclude=.env -czf /tmp/zv-app.tar.gz -C . .
```
Архив содержит `Dockerfile` и исходники в КОРНЕ (без обёртки-папки).

## Шаг 2 — получить URL и залить архив

```sh
RESP=$(curl -fsS -X POST "$API/dev/projects/$ZEROVIBE_PROJECT/source" -H "X-API-Key: $KEY")
UPLOAD_URL=$(echo "$RESP" | python3 -c 'import sys,json;print(json.load(sys.stdin)["upload_url"])')
SOURCE_KEY=$(echo "$RESP" | python3 -c 'import sys,json;print(json.load(sys.stdin)["key"])')
curl -fsS -X PUT --upload-file /tmp/zv-app.tar.gz "$UPLOAD_URL"
```

## Шаг 3 — запросить деплой

```sh
curl -fsS -X POST "$API/dev/projects/$ZEROVIBE_PROJECT/deploy" \
  -H "X-API-Key: $KEY" -H "Content-Type: application/json" \
  -d "{\"subdomain\":\"<sub>\",\"source_key\":\"$SOURCE_KEY\"}"
```
Поддомен `<sub>`: только `a-z`, `0-9`, дефис; 1..63 символа; уникален на платформе.
Спроси у пользователя желаемый поддомен, если не задан.

## Результат

Ответ содержит `domain` и `status`:
- `status: running` — собрано и поднято, открывай `https://<sub>.zerovibe.ru`;
- `status: failed` — причина: занятый поддомен, красные тесты, ошибка сборки образа.
  Сборка идёт на платформе из присланного архива.

Проверка:
```sh
curl -fsS "https://<sub>.zerovibe.ru/healthz"
```

## Аутентификация

Все вызовы к платформе — заголовком `X-API-Key: <ключ>` (ключ из
`~/.zerovibe/config.json`, настраивается `/zerovibe:install`). Bearer-токен не нужен.

## Замечания

- TLS edge-Caddy выпускает автоматически при первом запросе (DNS уже настроен платформой).
- Данные приложения (SQLite) — в volume на VM в РФ (152-ФЗ), переживают рестарт контейнера.
- Сборка на стороне платформы — локально Docker не нужен.

Аргумент пользователя (поддомен): $ARGUMENTS
