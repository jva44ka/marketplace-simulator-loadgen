# marketplace-simulator-loadgen

[🇷🇺 Русский](README.md) · 🇬🇧 English

Load generator service in the "Marketplace Simulator" pet project.

## Stack

- **Go** — implementation language
- **kafka-go** — Kafka consumer (tracking product stock levels)
- **golang.org/x/time/rate** — RPS control (token bucket)
- **etcd** — dynamic configuration store (hot-reload of RPS and thresholds without restarts)

## Architecture

```
cmd/loadgen/         — entry point
internal/
  config/
    config.go        — config structure and YAML loading
    store.go         — ConfigStore (atomic.Pointer, thread-safe hot-reload)
  infra/
    etcd/            — etcd client, config read/seed, watcher
  client/
    product_client.go — HTTP client for marketplace-simulator-product
    cart_client.go    — HTTP client for marketplace-simulator-cart
  workers/
    replenisher.go   — Kafka consumer + worker pool → restock products
    order_flow.go    — order flow: add to cart → checkout
    cart_viewer.go   — view cart contents
```

## Workers

### Replenisher

Reads events from the Kafka topic `product.events`. If `new.count < low-stock-threshold` in an event, sends a task to the worker pool which calls `POST /v1/products/increase-count`.

Pattern: one Kafka consumer → buffered channel → `parallelism` worker goroutines.

`low-stock-threshold` and `replenish-count` are read from `ConfigStore` on every Kafka message — picked up without a restart.

### Order Flow

Simulates the order flow: adds a random product to a random user's cart, then checks out. On checkout failure — clears the cart (`DELETE /user/{id}/cart`).

Pattern: `parallelism` goroutines sharing a single `rate.Limiter` for the whole worker. RPS is updated via `limiter.SetLimit/SetBurst` when config changes in etcd.

### Cart Viewer

Periodically fetches the contents of a random user's cart (`GET /user/{id}/cart`).

Pattern: `parallelism` goroutines sharing a single `rate.Limiter`. RPS is updated the same way as Order Flow.

## Configuration

The config file path is set via the `CONFIG_PATH` environment variable.  
Default: `configs/values.yaml`.

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
  config-key: /config/loadgen   # etcd key where the config is stored

workers:
  replenisher:
    enabled: true
    parallelism: 3          # worker pool size
    low-stock-threshold: 10 # stock level that triggers replenishment
    replenish-count: 50     # units to add per replenishment

  order-flow:
    enabled: true
    parallelism: 200        # number of goroutines
    rps: 100                # total RPS for the worker

  cart-viewer:
    enabled: true
    parallelism: 10
    rps: 200
```

## Dynamic configuration (etcd)

On startup the service reads config from YAML, then connects to etcd:
- if the key exists — loads config from etcd on top of YAML defaults;
- if the key is absent — writes the YAML config to etcd (first start).

It then starts a `Watch` on the key — any change in etcd is applied in real time.

If etcd is unavailable on startup — the service continues with YAML config (graceful degradation).

| Parameter | Updated without restart | Mechanism |
|---|---|---|
| `workers.order-flow.rps` | ✅ | `limiter.SetLimit/SetBurst` in callback |
| `workers.cart-viewer.rps` | ✅ | `limiter.SetLimit/SetBurst` in callback |
| `workers.replenisher.low-stock-threshold` | ✅ | read from `cfgStore.Load()` on every Kafka message |
| `workers.replenisher.replenish-count` | ✅ | read from `cfgStore.Load()` on every Kafka message |
| `product-client.*` / `cart-client.*` host/port | ⚠️ requires restart | warning log on change |
| `kafka.*` | ⚠️ requires restart | warning log on change |
| `sku-range.*` | ⚠️ requires restart | warning log on change |

### Change config via etcdctl

```bash
# Lower order-flow load to 10 RPS
docker exec etcd etcdctl put /config/loadgen "$(
  docker exec etcd etcdctl get /config/loadgen --print-value-only \
  | sed 's/rps: 100/rps: 10/'
)"
```

Or via **etcd UI** → [http://localhost:8091](http://localhost:8091).

## Running locally

### Dependencies

- Go 1.25+
- Kafka
- Running `marketplace-simulator-product` and `marketplace-simulator-cart`

### Server

```bash
go run ./cmd/loadgen
```

Or with an explicit config path:

```bash
CONFIG_PATH=configs/values.yaml go run ./cmd/loadgen
```

## Docker

```bash
# Build image
make docker-build-latest

# Publish
make docker-push-latest
```
