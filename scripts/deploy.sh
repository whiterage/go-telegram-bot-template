#!/usr/bin/env bash
# Деплой/обновление бота на сервере.
# Использование:  cd /opt/tgbot/app && ./scripts/deploy.sh
# или:            APP_DIR=/opt/tgbot/app make deploy
set -euo pipefail

APP_DIR="${APP_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
cd "$APP_DIR"

echo "→ каталог: $APP_DIR"

if [[ ! -f .env ]]; then
  echo "✗ .env не найден в $APP_DIR — скопируйте env.example в .env и заполните" >&2
  exit 1
fi

mkdir -p data

if [[ -d .git ]]; then
  echo "→ git pull..."
  git fetch --all --prune
  git pull --rebase --autostash
else
  echo "⚠ не git-репозиторий, пропускаю pull"
fi

# Бэкап базы перед обновлением (SQLite — просто файл)
if [[ -f data/data.db ]]; then
  backup="data/data.db.$(date +%Y%m%d-%H%M%S).bak"
  cp data/data.db "$backup"
  echo "→ бэкап БД: $backup"
  # держим только 7 последних бэкапов
  ls -1t data/data.db.*.bak 2>/dev/null | tail -n +8 | xargs -r rm --
fi

echo "→ сборка образа..."
docker compose build

echo "→ перезапуск..."
docker compose up -d --force-recreate --remove-orphans

echo "→ чистка старых образов..."
docker image prune -f >/dev/null

echo "✓ готово. Логи: docker compose logs -f bot"
docker compose ps
