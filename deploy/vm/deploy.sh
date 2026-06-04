#!/usr/bin/env bash
# Доставка кода приложения и запуск стека на VM (Фаза 2, вручную-но-воспроизводимо).
# Логика осознанно простая — она же станет ядром оркестратора (Фаза 3).
#
# Использование:
#   bash deploy/vm/deploy.sh <SERVER_IP> <APP_DOMAIN> [SSH_USER]
# Пример:
#   bash deploy/vm/deploy.sh 1.2.3.4 app.zerovibe.ru root
#
# Что делает:
#   1) копирует исходники приложения (репозиторий zerovibe) в /opt/zerovibe/app;
#   2) копирует compose-стек (docker-compose.yml, Caddyfile) в /opt/zerovibe;
#   3) пишет .env с APP_DOMAIN;
#   4) docker compose up -d --build на VM.
set -euo pipefail

SERVER_IP="${1:?нужен IP VM первым аргументом}"
APP_DOMAIN="${2:?нужен домен вторым аргументом}"
SSH_USER="${3:-root}"

# Корень репозитория (на два уровня выше этого скрипта: deploy/vm → repo root).
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VM_DIR="${REPO_ROOT}/deploy/vm"
REMOTE="${SSH_USER}@${SERVER_IP}"
SSH_OPTS="-o StrictHostKeyChecking=accept-new"

echo ">> ожидаем готовности SSH на ${REMOTE}..."
for i in $(seq 1 30); do
  if ssh ${SSH_OPTS} -o ConnectTimeout=5 "${REMOTE}" 'command -v docker >/dev/null 2>&1 && echo ready' 2>/dev/null | grep -q ready; then
    echo ">> docker готов"
    break
  fi
  echo "   ...cloud-init ещё ставит docker ($i/30)"; sleep 10
done

echo ">> копируем исходники приложения в /opt/zerovibe/app"
# rsync исходников приложения; .git и локальные БД исключаем.
rsync -az --delete \
  --exclude '.git' --exclude 'bin' --exclude '*.db' --exclude '*.db-wal' --exclude '*.db-shm' \
  --exclude 'deploy' \
  -e "ssh ${SSH_OPTS}" \
  "${REPO_ROOT}/" "${REMOTE}:/opt/zerovibe/app/"

echo ">> копируем compose-стек в /opt/zerovibe"
rsync -az -e "ssh ${SSH_OPTS}" \
  "${VM_DIR}/docker-compose.yml" "${VM_DIR}/Caddyfile" \
  "${REMOTE}:/opt/zerovibe/"

echo ">> пишем .env (APP_DOMAIN=${APP_DOMAIN})"
ssh ${SSH_OPTS} "${REMOTE}" "printf 'APP_DOMAIN=%s\n' '${APP_DOMAIN}' > /opt/zerovibe/.env"

echo ">> запускаем стек (docker compose up -d --build)"
ssh ${SSH_OPTS} "${REMOTE}" 'cd /opt/zerovibe && docker compose up -d --build'

echo ">> готово. Проверь: https://${APP_DOMAIN} (Caddy выпустит TLS при первом запросе)"
echo ">> health: curl -fsS https://${APP_DOMAIN}/healthz"
