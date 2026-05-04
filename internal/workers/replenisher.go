package workers

import (
	"context"
	"encoding/json"
	"log/slog"

	kafka "github.com/segmentio/kafka-go"

	"github.com/jva44ka/marketplace-simulator-loadgen/internal/client"
	"github.com/jva44ka/marketplace-simulator-loadgen/internal/config"
)

// productState represents a single product record inside a Kafka event.
type productState struct {
	SKU   uint64 `json:"sku"`
	Count int    `json:"count"`
}

// productEventData is the "data" field of the Kafka message.
type productEventData struct {
	Old productState `json:"old"`
	New productState `json:"new"`
}

// productEvent is the top-level Kafka message body.
type productEvent struct {
	RecordID string           `json:"recordId"`
	Data     productEventData `json:"data"`
}

// replenishTask carries the work to be done by a pool worker.
type replenishTask struct {
	sku   uint64
	count uint32
}

// Replenisher consumes product.events from Kafka and calls IncreaseCount
// when stock drops below the configured threshold.
type Replenisher struct {
	cfgStore      *config.ConfigStore
	productClient *client.ProductClient
}

func NewReplenisher(
	cfgStore *config.ConfigStore,
	productClient *client.ProductClient,
) *Replenisher {
	return &Replenisher{
		cfgStore:      cfgStore,
		productClient: productClient,
	}
}

// Run starts the Kafka consumer and the worker pool. It blocks until ctx is
// cancelled.
func (r *Replenisher) Run(ctx context.Context) error {
	kafkaCfg := r.cfgStore.Load().Kafka
	workerCfg := r.cfgStore.Load().Workers.Replenisher

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: kafkaCfg.Brokers,
		Topic:   kafkaCfg.ProductEventsTopic,
		GroupID: kafkaCfg.ConsumerGroup,
	})
	defer func() {
		if err := reader.Close(); err != nil {
			slog.Error("replenisher: close kafka reader", "err", err)
		}
	}()

	tasks := make(chan replenishTask, workerCfg.Parallelism*2)

	// Start worker pool.
	for i := 0; i < workerCfg.Parallelism; i++ {
		go r.worker(ctx, tasks)
	}

	slog.Info("replenisher: started", "parallelism", workerCfg.Parallelism,
		"low_stock_threshold", workerCfg.LowStockThreshold)

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				// Context cancelled — clean shutdown.
				return nil
			}
			slog.Error("replenisher: read kafka message", "err", err)
			continue
		}

		var event productEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			slog.Error("replenisher: unmarshal kafka message", "err", err,
				"raw", string(msg.Value))
			continue
		}

		slog.Info("replenisher: received event",
			"record_id", event.RecordID,
			"sku", event.Data.New.SKU,
			"old_count", event.Data.Old.Count,
			"new_count", event.Data.New.Count,
		)

		// Read config fresh on every event — picks up hot-reloaded values.
		cfg := r.cfgStore.Load().Workers.Replenisher
		if event.Data.New.Count < cfg.LowStockThreshold {
			task := replenishTask{
				sku:   event.Data.New.SKU,
				count: cfg.ReplenishCount,
			}
			select {
			case tasks <- task:
			case <-ctx.Done():
				return nil
			}
		}
	}
}

func (r *Replenisher) worker(ctx context.Context, tasks <-chan replenishTask) {
	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-tasks:
			if !ok {
				return
			}
			slog.Info("replenisher: replenishing stock",
				"sku", task.sku, "count", task.count)

			if err := r.productClient.IncreaseCount(ctx, task.sku, task.count); err != nil {
				slog.Error("replenisher: increase count failed",
					"sku", task.sku, "err", err)
			}
		}
	}
}
