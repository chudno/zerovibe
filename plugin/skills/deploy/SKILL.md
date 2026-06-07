---
description: Задеплоить zerovibe-приложение на хостинг zerovibe (собираем образ из твоего кода, контейнер на VM, поддомен, авто-TLS). Используй, когда пользователь просит развернуть, задеплоить, выложить приложение или "запустить в прод".
---

# Деплой на хостинг zerovibe

Разворачивает приложение на наших VM: платформа собирает Docker-образ из ТВОЕГО
кода (Dockerfile проекта), поднимает контейнер и выдаёт поддомен
`<subdomain>.zerovibe.ru` с авто-TLS. Маршрутизацию и сборку делает оркестратор.

## Поток деплоя (3 шага)

1. Проверить готовность и собрать архив кода.
2. Получить presigned-URL и залить архив в хранилище.
3. Запросить деплой по ключу архива.

Переменные окружения (выдаёт платформа при регистрации):
- `ZEROVIBE_API` — базовый URL оркестратора;
- `ZEROVIBE_TOKEN` — токен вайбкодера (Bearer);
- `ZEROVIBE_PROJECT` — id проекта.

## Шаг 0 — проверка готовности

```sh
go build ./... && go vet ./... && go test ./...
```
Красное — НЕ деплой, сначала почини (см. skill testing-rules).
Убедись, что в корне есть `Dockerfile` (multi-stage, distroless, без CGO — как в
эталонном скелете; приложение слушает `:8080`).

## Шаг 1 — собрать архив кода

Пакуем проект в tar.gz, исключая лишнее (git, локальную БД, бинарь, артефакты):

```sh
tar --exclude=.git --exclude=data --exclude='*.db' --exclude=tmp \
    --exclude=node_modules -czf /tmp/zv-app.tar.gz -C . .
```

Архив должен содержать `Dockerfile` и исходники в КОРНЕ (без обёртки-папки) —
платформа собирает `docker build` из распакованного содержимого.

## Шаг 2 — получить URL и залить архив

```sh
# Получаем presigned PUT-URL и ключ.
RESP=$(curl -fsS -X POST "$ZEROVIBE_API/v1/dev/projects/$ZEROVIBE_PROJECT/source" \
  -H "Authorization: Bearer $ZEROVIBE_TOKEN")
UPLOAD_URL=$(echo "$RESP" | python3 -c 'import sys,json;print(json.load(sys.stdin)["upload_url"])')
SOURCE_KEY=$(echo "$RESP" | python3 -c 'import sys,json;print(json.load(sys.stdin)["key"])')

# Заливаем архив (PUT по presigned-URL, без доп. заголовков авторизации).
curl -fsS -X PUT --upload-file /tmp/zv-app.tar.gz "$UPLOAD_URL"
```

## Шаг 3 — запросить деплой

```sh
curl -fsS -X POST "$ZEROVIBE_API/v1/dev/projects/$ZEROVIBE_PROJECT/deploy" \
  -H "Authorization: Bearer $ZEROVIBE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"subdomain\":\"<sub>\",\"source_key\":\"$SOURCE_KEY\"}"
```

Поддомен `<sub>`: только `a-z`, `0-9`, дефис; 1..63 символа; уникален на платформе.

## Результат

Ответ содержит `domain` и `status`:
- `status: running` — приложение собрано и поднято, открывай `https://<sub>.zerovibe.ru`;
- `status: failed` — смотри причину: занятый поддомен, красные тесты, ошибка сборки образа.

Проверка после деплоя:
```sh
curl -fsS "https://<sub>.zerovibe.ru/healthz"
```

## Замечания

- TLS edge-Caddy выпускает автоматически при первом запросе (нужен корректный DNS — он уже настроен платформой).
- Данные приложения (SQLite) хранятся в volume на VM в РФ (152-ФЗ) и переживают рестарт.
- Сборка идёт на стороне платформы из присланного архива — локально Docker не нужен.

Аргумент пользователя: $ARGUMENTS
