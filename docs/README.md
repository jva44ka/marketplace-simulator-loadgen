# marketplace-simulator-loadgen

🇷🇺 Русский · [🇬🇧 English](README.en.md)

Генератор нагрузки — сервис в рамках петпроекта «Симулятор маркетплейса».

## Стек

- **Go** — язык реализации
- **kafka-go** — Kafka-консьюмер (отслеживание остатков товаров)
- **golang.org/x/time/rate** — контроль RPS (token bucket)
- **etcd** — хранилище динамической конфигурации (hot-reload RPS и порогов без рестарта)

## Архитектура

```
cmd/loadgen/         — точка входа
internal/
  config/
    config.go        — структура конфига и загрузка из YAML
    store.go         — ConfigStore (atomic.Pointer, thread-safe hot-reload)
  infra/
    etcd/            — etcd-клиент, чтение/seed конфига, watcher
  client/
    product_client.go — HTTP-клиент к marketplace-simulator-product
    cart_client.go    — HTTP-клиент к marketplace-simulator-cart
  workers/
    replenisher.go   — Kafka-консьюмер + worker pool → пополнение товаров
    order_flow.go    — флоу заказа: добавить в корзину → checkout
    cart_viewer.go   — просмотр содержимого корзины
```

## Воркеры

### Replenisher

Читает события из Kafka-топика `product.events`. Если в событии `new.count < low-stock-threshold` — отправляет задачу в пул воркеров, который вызывает `POST /v1/products/increase-count`.

Паттерн: один Kafka-консьюмер → buffered channel → `parallelism` горутин-воркеров.

`low-stock-threshold` и `replenish-count` читаются из `ConfigStore` на каждом Kafka-сообщении — подхватываются без рестарта.

### Order Flow

Симулирует флоу заказа: добавляет случайный товар в корзину случайного пользователя, затем делает checkout. При ошибке checkout — очищает корзину (`DELETE /user/{id}/cart`).

Паттерн: `parallelism` горутин с общим `rate.Limiter` на весь воркер. RPS обновляется через `limiter.SetLimit/SetBurst` при изменении конфига в etcd.

### Cart Viewer

Периодически запрашивает содержимое корзины случайного пользователя (`GET /user/{id}/cart`).

Паттерн: `parallelism` горутин с общим `rate.Limiter`. RPS обновляется аналогично Order Flow.

## Конфигурация

Путь до файла конфигурации задаётся переменной окружения `CONFIG_PATH`.  
По умолчанию: `configs/values.yaml`.

```yaml
product-client:
  host: product
  port: 5000
  auth-token: admin
  timeout: 30s

cart-client:
  host: cart
  port: 5000
  timeout: 30s

kafka:
  brokers:
    - kafka:9092
  product-events-topic: product.events
  consumer-group: loadgen

sku-range:
  min: 1
  max: 200

etcd:
  endpoints:
    - etcd:2379
  dial-timeout: 5s
  config-key: /config/loadgen   # ключ в etcd где хранится конфиг

workers:
  replenisher:
    enabled: true
    parallelism: 3          # размер worker pool
    low-stock-threshold: 10 # порог остатка для пополнения
    replenish-count: 50     # на сколько единиц пополнять

  order-flow:
    enabled: true
    parallelism: 200        # количество горутин
    rps: 100                # общий RPS воркера

  cart-viewer:
    enabled: true
    parallelism: 10
    rps: 200
```

## Динамическая конфигурация (etcd)

При старте сервис читает конфиг из YAML, затем подключается к etcd:
- если ключ существует — загружает конфиг из etcd поверх YAML-дефолтов;
- если ключа нет — записывает YAML-конфиг в etcd (первый старт).

Затем запускает `Watch` на ключ — любое изменение в etcd применяется в реальном времени.

Если etcd недоступен при старте — сервис продолжает работу с YAML-конфигом (graceful degradation).

| Параметр | Обновляется без рестарта | Механизм |
|---|---|---|
| `workers.order-flow.rps` | ✅ | `limiter.SetLimit/SetBurst` в callback |
| `workers.cart-viewer.rps` | ✅ | `limiter.SetLimit/SetBurst` в callback |
| `workers.replenisher.low-stock-threshold` | ✅ | читается из `cfgStore.Load()` на каждом Kafka-сообщении |
| `workers.replenisher.replenish-count` | ✅ | читается из `cfgStore.Load()` на каждом Kafka-сообщении |
| `product-client.*` / `cart-client.*` host/port | ⚠️ требует рестарта | лог warning при изменении |
| `kafka.*` | ⚠️ требует рестарта | лог warning при изменении |
| `sku-range.*` | ⚠️ требует рестарта | лог warning при изменении |

### Изменить конфиг через etcdctl

```bash
# Снизить нагрузку order-flow до 10 RPS
docker exec etcd etcdctl put /config/loadgen "$(
  docker exec etcd etcdctl get /config/loadgen --print-value-only \
  | sed 's/rps: 100/rps: 10/'
)"
```

Или через **etcd UI** → [http://localhost:8091](http://localhost:8091).

## Запуск локально

### Зависимости

- Go 1.25+
- Kafka
- Запущенные `marketplace-simulator-product` и `marketplace-simulator-cart`

### Сервер

```bash
go run ./cmd/loadgen
```

Или с явным путём к конфигу:

```bash
CONFIG_PATH=configs/values.yaml go run ./cmd/loadgen
```

## Docker

```bash
# Собрать образ
make docker-build-latest

# Опубликовать
make docker-push-latest
```
