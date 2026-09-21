#!/usr/bin/env bash
# Деплой на маленький VPS без Docker: кросс-сборка на локальной машине,
# заливка статического бинарника и рестарт systemd-сервиса.
#
# Использование:
#   SERVER=root@1.2.3.4 ./scripts/deploy-systemd.sh
#   SERVER=root@1.2.3.4 WITH_ENV=1 ./scripts/deploy-systemd.sh   # заодно обновить .env
set -euo pipefail

SERVER="${SERVER:?укажите SERVER=root@host}"
REMOTE_DIR="${REMOTE_DIR:-/opt/tgbot}"
SERVICE="${SERVICE:-tgbot}"
WITH_ENV="${WITH_ENV:-0}"

cd "$(dirname "${BASH_SOURCE[0]}")/.."

echo "→ кросс-сборка linux/amd64..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o bin/bot-linux-amd64 ./cmd/bot
ls -lh bin/bot-linux-amd64

echo "→ бэкап БД на сервере..."
ssh "$SERVER" "
  if [ -f $REMOTE_DIR/data/data.db ]; then
    cp $REMOTE_DIR/data/data.db $REMOTE_DIR/data/data.db.\$(date +%Y%m%d-%H%M%S).bak
    ls -1t $REMOTE_DIR/data/data.db.*.bak | tail -n +8 | xargs -r rm --
  fi
"

echo "→ заливка бинарника..."
# Во временный файл + mv: бинарник нельзя перезаписать, пока он запущен (ETXTBSY)
scp bin/bot-linux-amd64 "$SERVER:$REMOTE_DIR/bin/bot.new"

if [[ "$WITH_ENV" == "1" ]]; then
  echo "→ заливка .env..."
  scp .env "$SERVER:$REMOTE_DIR/.env"
fi

echo "→ рестарт сервиса..."
ssh "$SERVER" "
  set -e
  systemctl stop $SERVICE || true
  mv $REMOTE_DIR/bin/bot.new $REMOTE_DIR/bin/bot
  chown tgbot:tgbot $REMOTE_DIR/bin/bot $REMOTE_DIR/.env
  chmod 755 $REMOTE_DIR/bin/bot
  chmod 600 $REMOTE_DIR/.env
  systemctl start $SERVICE
  sleep 2
  systemctl is-active $SERVICE
"

echo "✓ готово. Логи: ssh $SERVER 'journalctl -u $SERVICE -f'"
