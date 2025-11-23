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

.PHONY: run build clean clean-db lint docker-build docker-run

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