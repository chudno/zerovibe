# zerovibe orchestrator — управляющая плоскость

Сервис, который разворачивает приложения вайбкодеров в контейнерах на наших VM.
Автоматизирует то, что в `deploy/` (Фаза 2) делается вручную.

## Что делает

`POST /v1/apps {owner_id, name, subdomain}`:
1. валидирует и регистрирует приложение;
2. подбирает VM со свободным местом (или создаёт новую через Provider);
3. разворачивает контейнер на VM и поднимает маршрут (Deployer);
4. фиксирует развёртывание (домен `<subdomain>.<base>`), статус.

Чтение: `GET /v1/apps`, `/v1/nodes`, `/v1/deployments`, `/healthz`.

## Архитектура (чистая, порты/адаптеры)

```
internal/
  domain/              Node / App / Deployment + статусы + ошибки
  usecase/             Orchestrator (ядро) + порты: Provider, Deployer, Store, IDGen, Clock
  provider/timeweb/    Provider — создание VM (сейчас Fake; далее Terraform-exec/REST)
  deployer/ssh/        Deployer — доставка+запуск на VM (сейчас NoOp; далее SSH = логика deploy.sh)
  store/sqlite/        Store — метаданные (SQLite + очередь записи, как в скелете)
  transport/http/      JSON API
cmd/orchestrator/      composition root
```

Швы (порты) позволяют подключить реальное облако, не трогая ядро. Сменить
провайдера = новая реализация `Provider`. Реальный деплой = реализация `Deployer`
поверх логики `deploy/vm/deploy.sh`.

## Статус (Фаза 3)

- ✅ Ядро размещения (выбор/создание узла по ёмкости), метаданные, JSON API.
- ✅ Тесты ядра (in-memory store) + smoke через HTTP на реальном SQLite.
- 🔲 Реальный Provider (Timeweb): `terraform apply` модуля `deploy/terraform`
  через exec ИЛИ REST-клиент к Timeweb API.
- 🔲 Реальный Deployer (SSH): перенести логику `deploy.sh` в Go (golang.org/x/crypto/ssh).
- 🔲 Приём кода/образа приложения, реестр образов.
- 🔲 Аутентификация API (ключ платформы), привязка к вайбкодеру.

## Запуск (локально, заглушки)

```sh
go run ./cmd/orchestrator           # :8090, db=orchestrator.db
curl -XPOST localhost:8090/v1/apps -d '{"owner_id":"u1","name":"CRM","subdomain":"crm"}'
```

ENV: `ADDR` (`:8090`), `DB_PATH` (`file:orchestrator.db`), `BASE_DOMAIN`
(`zerovibe.ru`), `FAKE_NODE_IP` (`127.0.0.1`).
