# Multi-stage сборка. Драйвер SQLite — modernc.org/sqlite (чистый Go, без CGO),
# поэтому бинарь статический и кладётся в минимальный distroless-образ без libc.
#
# Образ слушает :8080 и хранит БД в /data (смонтируй volume сюда, чтобы данные
# переживали рестарт контейнера — это и есть persistent-хранилище приложения).

# --- build ---
FROM golang:1.25-alpine AS build
WORKDIR /src

# Кэш зависимостей отдельным слоем.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0 → статический бинарь. -ldflags для уменьшения размера.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/zerovibe ./cmd/server
# Готовим каталог данных в build-стадии: distroless не имеет shell/mkdir/chown,
# поэтому создаём /data здесь и копируем его в runtime с нужным владельцем.
RUN mkdir -p /data

# --- runtime ---
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/zerovibe /app/zerovibe
# Каталог /data во владении nonroot (uid 65532) — иначе SQLite не создаст файл БД
# в смонтированном volume (ошибка "unable to open database file").
COPY --from=build --chown=nonroot:nonroot /data /data

ENV ADDR=:8080
ENV DB_PATH=file:/data/zerovibe.db
EXPOSE 8080
VOLUME ["/data"]

ENTRYPOINT ["/app/zerovibe"]
