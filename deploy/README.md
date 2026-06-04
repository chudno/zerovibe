# Деплой zerovibe на VM (Фаза 2 — вручную, воспроизводимо)

Доказывает деплой-флоу end-to-end: Terraform поднимает VM на Timeweb Cloud с
Docker (через cloud-init), скрипт доставляет код и запускает стек
**Caddy (авто-TLS) + zerovibe** за поддоменом по https.

Логика осознанно простая — она же станет ядром оркестратора (Фаза 3).

## Что внутри

```
deploy/
  terraform/   VM на Timeweb (twc_server) + SSH-ключ + cloud-init
  vm/
    cloud-init.yaml      ставит Docker на чистую Ubuntu
    docker-compose.yml   стек: app (zerovibe) + caddy (реверс-прокси, TLS)
    Caddyfile            домен из $APP_DOMAIN → reverse_proxy app:8080
    deploy.sh            rsync кода на VM + docker compose up
```

## Что нужно один раз

1. **Токен Timeweb**: https://timeweb.cloud/my/api-keys → `export TWC_TOKEN=...`
2. **SSH-ключ**: `ssh-keygen -t ed25519` (если нет). Понадобится публичный
   (`~/.ssh/id_ed25519.pub`).
3. **Домен**: `zerovibe.ru` уже есть. Выбери поддомен, напр. `app.zerovibe.ru`.

## Запуск

```sh
cd deploy/terraform
export TWC_TOKEN=...

terraform init
terraform apply \
  -var "ssh_public_key=$(cat ~/.ssh/id_ed25519.pub)" \
  -var "app_domain=app.zerovibe.ru"
# → выведет server_ip
```

Дальше:

1. **DNS**: создай A-record `app.zerovibe.ru → <server_ip>` (у регистратора/DNS
   зоны). Дождись распространения (`dig app.zerovibe.ru` должен вернуть IP).
2. **Доставка и запуск**:
   ```sh
   bash ../vm/deploy.sh <server_ip> app.zerovibe.ru
   ```
   Скрипт дождётся готовности Docker, скопирует код, поднимет стек.
3. **Проверка**:
   ```sh
   curl -fsS https://app.zerovibe.ru/healthz   # ok
   ```
   Открой `https://app.zerovibe.ru` — заметки. TLS Caddy выпустит при первом
   запросе (нужен корректный A-record до этого).

## Удаление

```sh
cd deploy/terraform && terraform destroy -var "ssh_public_key=..." -var "app_domain=..."
```

## Заметки

- Локация `ru-1` (СПб) — данные в РФ (152-ФЗ).
- Сертификаты Caddy хранятся в volume `caddy-data` — переживают рестарт.
- На Фазе 2 образ собирается на VM из исходников. Фаза 3 заменит на pull готового
  образа из реестра + автоматическую доставку (без ручного deploy.sh).
- Стейт Terraform (`*.tfstate`) и `.terraform/` в .gitignore — не коммитим.
