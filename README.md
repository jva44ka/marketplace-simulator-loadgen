# marketplace-simulator-loadgen

Генератор нагрузки — сервис в рамках учебного проекта «Симулятор маркетплейса».

## Стек

- **Go** — язык реализации
- **kafka-go** — Kafka-консьюмер (отслеживание остатков товаров)
- **golang.org/x/time/rate** — контроль RPS (token bucket)

## Архитектура

```
cmd/loadgen/         — точка входа
internal/
  config/            — загрузка конфигурации из YAML
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

### Order Flow

Симулирует флоу заказа: добавляет случайный товар в корзину случайного пользователя, затем делает checkout. При ошибке checkout — очищает корзину (`DELETE /user/{id}/cart`).

Паттерн: `parallelism` горутин с общим `rate.Limiter` на весь воркер.

### Cart Viewer

Периодически запрашивает содержимое корзины случайного пользователя (`GET /user/{id}/cart`).

Паттерн: `parallelism` горутин с общим `rate.Limiter`.

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

skus: [1, 2, 3, 4, 5]   # список SKU для генерации нагрузки

workers:
  replenisher:
    enabled: true
    parallelism: 3          # размер worker pool
    low-stock-threshold: 10 # порог остатка для пополнения
    replenish-count: 50     # на сколько единиц пополнять

  order-flow:
    enabled: true
    parallelism: 5          # количество горутин
    rps: 10                 # общий RPS воркера

  cart-viewer:
    enabled: true
    parallelism: 2
    rps: 20
```

## Запуск локально

### Зависимости

- Go 1.24+
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
