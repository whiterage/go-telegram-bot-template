# имя бинарника и путь вывода
BIN_DIR := bin
BIN_NAME := bot
BIN := $(BIN_DIR)/$(BIN_NAME)

# точка входа
MAIN := ./cmd/bot

# Docker
DOCKER_IMAGE := tgbot
DOCKER_TAG := latest

# Цели:
# clean     - очистка артефактов сборки (безопасно для продакшена)
# clean-db  - очистка базы данных (только для разработки!)

.PHONY: run build clean clean-db lint docker-build docker-run docker-run-webhook up down logs deploy deploy-server server-logs

run:
	@echo "→ running..."
	go mod tidy
	go run $(MAIN)

build:
	@echo "→ building $(BIN)..."
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN) $(MAIN)
	@echo "✓ built: $(BIN)"

clean:
	@echo "→ cleaning..."
	@rm -rf $(BIN_DIR)
	@rm -f *.log
	@rm -f bot
	@echo "✓ cleaned"

clean-db:
	@echo "→ cleaning database..."
	@rm -f data.db
	@echo "⚠️  Database cleaned (use with caution!)"

lint:
	@echo "→ running linters..."
	go vet ./...
	gofmt -s -w .
	@echo "✓ linted"

docker-build:
	@echo "→ building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "✓ Docker image built"

docker-run:
	mkdir -p $(PWD)/data
	docker run --rm --env-file .env -v $(PWD)/data:/app/data $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-run-webhook:
	docker run --rm --env-file .env -p 8080:8080 -v $(PWD)/data:/app/data $(DOCKER_IMAGE):$(DOCKER_TAG)

# --- docker compose: основной способ запуска на сервере ---
up:
	@mkdir -p $(PWD)/data
	docker compose up -d --build
	@echo "OK: запущено. Логи: make logs"

down:
	docker compose down

logs:
	docker compose logs -f bot

# Обновление на сервере: git pull + бэкап БД + пересборка + рестарт
deploy:
	@chmod +x scripts/deploy.sh
	@APP_DIR=$${APP_DIR:-$(PWD)} ./scripts/deploy.sh

# --- деплой на VPS без Docker (systemd + статический бинарник) ---
# Укажите свой сервер: make deploy-server SERVER=root@1.2.3.4
SERVER ?=

deploy-server:
	@SERVER=$(SERVER) ./scripts/deploy-systemd.sh

server-logs:
	ssh $(SERVER) 'journalctl -u tgbot -f -o cat'
