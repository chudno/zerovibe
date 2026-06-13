.PHONY: run build test vet check docker docker-run tidy clean

# Локальный запуск (БД в текущем каталоге)
run:
	go run ./cmd/server

# Проверка перед публикацией: сборка + статанализ + тесты одной командой.
check:
	go build ./... && go vet ./... && go test ./...

# Сборка бинаря
build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/zerovibe ./cmd/server

# Тесты (unit + e2e, без сети)
test:
	go test ./...

vet:
	go vet ./...

# Сборка Docker-образа
docker:
	docker build -t zerovibe:local .

# Запуск контейнера с volume под данные (порт 8080)
docker-run: docker
	docker run --rm -p 8080:8080 -v zerovibe-data:/data zerovibe:local

tidy:
	go mod tidy

clean:
	rm -rf bin zerovibe.db zerovibe.db-wal zerovibe.db-shm
